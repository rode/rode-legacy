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
	attesterLister   attester.Lister
	occurrenceLister occurrence.Lister
	signerList       attester.SignerList
	client           client.Client
	decoder          *admission.Decoder
}

// NewEnforcer creates an enforcer
func NewEnforcer(log logr.Logger, attesterLister attester.Lister, occurrenceLister occurrence.Lister, signerList attester.SignerList, c client.Client) Enforcer {
	return &enforcer{
		log,
		attesterLister,
		occurrenceLister,
		signerList,
		c,
		nil,
	}
}

func (e *enforcer) AddEnforcerAttesters(ctx context.Context, enforcerAttesters map[string]attester.Attester, namespace string) error {
	// get all attesters
	attesters := e.attesterLister.ListAttesters()

	// get enforcers
	enforcers := &rodev1alpha1.EnforcerList{}
	err := e.client.List(ctx, enforcers, client.InNamespace(namespace))
	if err != nil {
		return err
	}

	for _, enforcer := range enforcers.Items {
		for _, enforcerAttester := range enforcer.Spec.Attesters {
			a, attesterExists := attesters[enforcerAttester.String()]
			if !attesterExists {
				return fmt.Errorf("enforcer %s/%s requires attester %s which does not exist", enforcer.Namespace, enforcer.Name, enforcerAttester.String())
			}

			_, enforcerAttesterExists := enforcerAttesters[enforcerAttester.String()]
			if !enforcerAttesterExists {
				enforcerAttesters[enforcerAttester.String()] = a
			}
		}
	}

	return nil
}

func (e *enforcer) AddClusterEnforcerAttesters(ctx context.Context, enforcerAttesters map[string]attester.Attester, namespace string) error {
	// get all attesters
	attesters := e.attesterLister.ListAttesters()

	clusterEnforcers := &rodev1alpha1.ClusterEnforcerList{}
	err := e.client.List(ctx, clusterEnforcers)
	if err != nil {
		return err
	}

	for _, clusterEnforcer := range clusterEnforcers.Items {
		if clusterEnforcer.EnforcesNamespace(namespace) {
			for _, clusterEnforcerAttester := range clusterEnforcer.Spec.Attesters {
				a, attesterExists := attesters[clusterEnforcerAttester.String()]
				if !attesterExists {
					return fmt.Errorf("cluster enforcer %s/%s requires attester %s which does not exist", clusterEnforcer.Namespace, clusterEnforcer.Name, clusterEnforcerAttester.String())
				}

				_, enforcerAttesterExists := enforcerAttesters[clusterEnforcerAttester.String()]
				if !enforcerAttesterExists {
					enforcerAttesters[clusterEnforcerAttester.String()] = a
				}
			}
		}
	}
	return nil
}

func (e *enforcer) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := e.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	e.log.Info("handling enforcement request", "pod", fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))

	enforcerAttesters := make(map[string]attester.Attester)

	err = e.AddEnforcerAttesters(ctx, enforcerAttesters, pod.Namespace)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	err = e.AddClusterEnforcerAttesters(ctx, enforcerAttesters, pod.Namespace)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	for _, container := range pod.Spec.Containers {
		occurrenceList, err := e.occurrenceLister.ListOccurrences(ctx, container.Image) // probably have to convert to sha256 here
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		e.log.Info("ListOccurrances", "occurrences", occurrenceList.Occurrences)

		for _, enforcerAttester := range enforcerAttesters {
			attested := false
			signer, err := e.signerList.Get(enforcerAttester.String())
			if err != nil {
				return admission.Errored(http.StatusInternalServerError, err)
			}
			for _, occ := range occurrenceList.GetOccurrences() {
				if err = enforcerAttester.Verify(ctx, &attester.VerifyRequest{Occurrence: occ}); err == nil {
					attested = true
					break
				}
			}

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
