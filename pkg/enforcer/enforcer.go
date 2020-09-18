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
	signerList       attester.SignerList
	client           client.Client
	decoder          *admission.Decoder
}

// NewEnforcer creates an enforcer
func NewEnforcer(log logr.Logger, occurrenceLister occurrence.Lister, signerList attester.SignerList, c client.Client) Enforcer {
	return &enforcer{
		log,
		occurrenceLister,
		signerList,
		c,
		nil,
	}
}

func (e *enforcer) GetSignersForNamespace(ctx context.Context, namespace string) (map[string]attester.Signer, error) {
	signerList := map[string]attester.Signer{}

	// get enforcers
	enforcers := &rodev1alpha1.EnforcerList{}
	err := e.client.List(ctx, enforcers, client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}

	for _, enforcer := range enforcers.Items {
		for _, enforcerAttester := range enforcer.Spec.Attesters {
			signer, err := e.signerList.Get(enforcerAttester.String())
			if err != nil {
				return nil, fmt.Errorf("enforcer %s/%s requires attester %s which does not exist", enforcer.Namespace, enforcer.Name, enforcerAttester.String())
			}

			_, enforcerAttesterExists := signerList[enforcerAttester.String()]
			if !enforcerAttesterExists {
				signerList[enforcerAttester.String()] = signer
			}
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
				signer, err := e.signerList.Get(clusterEnforcerAttester.String())
				if err != nil {
					return nil, fmt.Errorf("cluster enforcer %s/%s requires attester %s which does not exist", clusterEnforcer.Namespace, clusterEnforcer.Name, clusterEnforcerAttester.String())
				}

				_, enforcerAttesterExists := signerList[clusterEnforcerAttester.String()]
				if !enforcerAttesterExists {
					signerList[clusterEnforcerAttester.String()] = signer
				}
			}
		}
	}

	return signerList, nil
}

func (e *enforcer) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := e.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	e.log.Info("handling enforcement request", "pod", fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))

	enforcerAttesters, err := e.GetSignersForNamespace(ctx, pod.Namespace)

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
				if err = signer.VerifyAttestation(occ); err == nil {
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
