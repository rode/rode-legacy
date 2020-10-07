package enforcer

import (
	"context"
	"fmt"
	"net/http"

	rodev1alpha1 "github.com/liatrio/rode/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/go-logr/logr"

	"github.com/liatrio/rode/pkg/occurrence"

	"github.com/liatrio/rode/pkg/attester"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:webhook:path=/validate-v1-pod,mutating=false,failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,name=vpod.rode.liatr.io
// +kubebuilder:rbac:groups=rode.liatr.io,resources=enforcers,verbs=get;list;watch
// +kubebuilder:rbac:groups=rode.liatr.io,resources=clusterenforcers,verbs=get;list;watch

// Enforcer enforces attestations on a resource
type Enforcer interface {
	admission.Handler
	admission.DecoderInjector
}

type enforcer struct {
	log              logr.Logger
	occurrenceLister occurrence.Lister
	attesterList     *attester.List
	client           client.Client
	decoder          *admission.Decoder
}

// NewEnforcer creates an enforcer
func NewEnforcer(log logr.Logger, occurrenceLister occurrence.Lister, attesterList *attester.List, c client.Client) Enforcer {
	return &enforcer{
		log,
		occurrenceLister,
		attesterList,
		c,
		nil,
	}
}

// getEnforcerAttesters returns a list of attesters for all enforcers in the namespace and all cluster enforcers
func (e *enforcer) getEnforcerAttesters(ctx context.Context, namespace string) (*attester.List, error) {
	enforcerAttesters := attester.NewList()

	// get enforcers
	enforcers := &rodev1alpha1.EnforcerList{}
	err := e.client.List(ctx, enforcers, client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}

	for _, enforcer := range enforcers.Items {
		for _, enforcerAttester := range enforcer.Spec.Attesters {
			attester, err := e.attesterList.Get(enforcerAttester.String())
			if err != nil {
				return nil, fmt.Errorf("enforcer %s/%s requires attester %s which does not exist", enforcer.Namespace, enforcer.Name, enforcerAttester.String())
			}

			enforcerAttesters.Add(attester)
		}
	}

	// get cluster enforcers
	clusterEnforcers := &rodev1alpha1.ClusterEnforcerList{}
	err = e.client.List(ctx, clusterEnforcers)
	if err != nil {
		return nil, err
	}

	for _, clusterEnforcer := range clusterEnforcers.Items {
		if clusterEnforcer.EnforcesNamespace(namespace) {
			for _, clusterEnforcerAttester := range clusterEnforcer.Spec.Attesters {
				attester, err := e.attesterList.Get(clusterEnforcerAttester.String())
				if err != nil {
					return nil, fmt.Errorf("cluster enforcer %s/%s requires attester %s which does not exist", clusterEnforcer.Namespace, clusterEnforcer.Name, clusterEnforcerAttester.String())
				}

				enforcerAttesters.Add(attester)
			}
		}
	}

	return enforcerAttesters, nil
}

// Handle verifies each container image for the pod in the admission request has at lease one attestesation 
// signed by an attester for every attester in every enforcer responsible for the requests namespace
func (e *enforcer) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := e.log.WithName("Handle()").WithValues("NAMESPACE", req.Namespace, "NAME", req.Name)

	pod := &corev1.Pod{}
	err := e.decoder.Decode(req, pod)
	if err != nil {
		log.Error(err, "failed to decode admission request", "REQUEST", req)
		return admission.Errored(http.StatusBadRequest, err)
	}

	log.Info("handling enforcement request", "pod", fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))

	// compile list of attesters that should be enforced for this namespace
	enforcerAttesters, err := e.getEnforcerAttesters(ctx, pod.Namespace)
	if err != nil {
		log.Error(err, "failed to get list of enforcer attesters")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	for _, container := range pod.Spec.Containers {
		// Get attestation occurrences for image URI
		attestationList, err := e.occurrenceLister.ListAttestations(ctx, container.Image)
		if err != nil {
			log.Error(err, "failed to fetch attestations for image", "IMAGE", container.Image)
			return admission.Errored(http.StatusInternalServerError, err)
		}

		// verify each attester has a signed attestation
		for _, enforcerAttester := range enforcerAttesters.GetAll() {
			attested := false
			// check each attestation until we find one that was signed by the current attester
			for _, attestation := range attestationList {
				if err = enforcerAttester.Verify(ctx, &attester.VerifyRequest{Occurrence: attestation}); err == nil {
					attested = true
					break
				}
			}

			// if no valid attestations were found for this attester refuse the admission request
			if !attested {
				return admission.Denied(fmt.Sprintf("unable to find attestation for %s", enforcerAttester.String()))
			}
		}
	}

	return admission.Allowed("")
}

func (e *enforcer) InjectDecoder(d *admission.Decoder) error {
	e.decoder = d
	return nil
}
