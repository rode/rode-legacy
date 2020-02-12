package enforcer

import (
	"context"
	"fmt"
	rodev1alpha1 "github.com/liatrio/rode/api/v1alpha1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/go-logr/logr"

	"github.com/liatrio/rode/pkg/occurrence"

	"github.com/liatrio/rode/pkg/attester"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:webhook:path=/validate-v1-pod,mutating=false,failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,name=vpod.rode.liatr.io

// Enforcer enforces attestations on a resource
type Enforcer interface {
	admission.Handler
	admission.DecoderInjector
}

type enforcer struct {
	log              logr.Logger
	attesterLister   attester.Lister
	occurrenceLister occurrence.Lister
	client           client.Client
	decoder          *admission.Decoder
}

// NewEnforcer creates an enforcer
func NewEnforcer(log logr.Logger, attesterLister attester.Lister, occurrenceLister occurrence.Lister, c client.Client) Enforcer {
	return &enforcer{
		log,
		attesterLister,
		occurrenceLister,
		c,
		nil,
	}
}

func (e *enforcer) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := e.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// get enforcers
	enforcers := &rodev1alpha1.EnforcerList{}
	err = e.client.List(ctx, enforcers, client.InNamespace(pod.Namespace))
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	e.log.Info("handling enforcement request", "pod", fmt.Sprintf("%s/%s", pod.Namespace, pod.Name), "enforcers", enforcers)

	// get all attesters
	attesters := e.attesterLister.ListAttesters()

	enforcerAttesters := make(map[string]attester.Attester)
	for _, enforcer := range enforcers.Items {
		for _, enforcerAttester := range enforcer.Spec.Attesters {
			a, attesterExists := attesters[enforcerAttester.String()]
			if !attesterExists {
				return admission.Denied(fmt.Sprintf("enforcer %s/%s requires attester %s which does not exist", enforcer.Namespace, enforcer.Name, enforcerAttester.String()))
			}

			_, enforcerAttesterExists := enforcerAttesters[enforcerAttester.String()]
			if !enforcerAttesterExists {
				enforcerAttesters[enforcerAttester.String()] = a
			}
		}
	}

	for _, container := range pod.Spec.Containers {
		occurrenceList, err := e.occurrenceLister.ListOccurrences(ctx, container.Image) // probably have to convert to sha256 here
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		for _, enforcerAttester := range enforcerAttesters {
			attested := false
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
