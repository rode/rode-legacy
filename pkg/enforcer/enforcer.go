package enforcer

import (
	"context"
	"net/http"
	// "fmt"
	"io/ioutil"
	"encoding/json"

	"github.com/go-logr/logr"

	"github.com/liatrio/rode/pkg/occurrence"

	"github.com/liatrio/rode/pkg/attester"

	"sigs.k8s.io/controller-runtime/pkg/client"
	admissionv1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Enforcer enforces attestations on a resource
type Enforcer interface {
	Enforce(ctx context.Context, namespace string, resourceURI string) error
	http.Handler
}

type enforcer struct {
	log              logr.Logger
	excludeNS        []string
	attesterLister   attester.Lister
	occurrenceLister occurrence.Lister
	client           client.Client
}

// NewEnforcer creates an enforcer
func NewEnforcer(log logr.Logger, excludeNS []string, attesterLister attester.Lister, occurrenceLister occurrence.Lister, c client.Client) Enforcer {
	return &enforcer{
		log,
		excludeNS,
		attesterLister,
		occurrenceLister,
		c,
	}
}

func (e *enforcer) Enforce(ctx context.Context, namespace string, resourceURI string) error {
	for _, ns := range e.excludeNS {
		if namespace == ns {
			// skip - this namespace is excluded
			return nil
		}
	}

	e.log.Info("About to enforce resource", resourceURI, namespace)

	// Begin: Determine enforced attesters
	// TODO: use different client to load namespace labels
	// result, err := e.client.Get(ctx, "???", "???")
	/*
	if err != nil {
		return fmt.Errorf("Unable to get namespace: %v", err)
	}
	resultLabels := result.ObjectMeta.Labels
	if resultLabels == nil {
		return nil
	}
	enforcedAttesters := resultLabels["rode.liatr.io/enforce-attesters"]
	// End: Determine enforced attesters
	occurrenceList, err := e.occurrenceLister.ListOccurrences(ctx, resourceURI)
	if err != nil {
		return err
	}
	for _, att := range e.attesterLister.ListAttesters() {
		if enforcedAttesters != "*" && enforcedAttesters != att.String() {
			continue
		}
		attested := false
		for _, occ := range occurrenceList.GetOccurrences() {
			req := &attester.VerifyRequest{
				Occurrence: occ,
			}
			if err = att.Verify(ctx, req); err == nil {
				attested = true
				break
			}
		}

		if !attested {
			return fmt.Errorf("Unable to find an attestation for %s", att)
		}
	}
	*/

	return nil
}

func (e *enforcer) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	c := context.Background()
	data, err := ioutil.ReadAll(req.Body)
	if err != nil || len(data) == 0 {
		e.log.Error(err, "Error responding to webhook")
		return
	}

	var arRequest admissionv1.AdmissionReview
	err = json.Unmarshal(data, &arRequest)
	if err != nil {
		e.log.Error(err, "Error parsing webhook")
	}

	if arRequest.Request.Kind.Group == "" && arRequest.Request.Kind.Kind == "Pod" {
		namespace := arRequest.Request.Namespace
		pod := corev1.Pod{}
		if err = json.Unmarshal(arRequest.Request.Object.Raw, &pod); err == nil {
			for _, container := range pod.Spec.Containers {
				err = e.Enforce(c, namespace, container.Image)
				if err != nil {
					break
				}
			}
		}
	}

	var arResponse admissionv1.AdmissionReview
	if err != nil {
		arResponse = admissionv1.AdmissionReview{
			Response: &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: err.Error(),
				},
			},
		}
	} else {
		arResponse = admissionv1.AdmissionReview{
			Response: &admissionv1.AdmissionResponse{
				Allowed: true,
				Result: &metav1.Status{
					Message: "Approved by rode",
				},
			},
		}
	}

	responseData, err := json.Marshal(arResponse)
	if err != nil {
		e.log.Error(err, "Error sneding response")
	}
	resp.Write(responseData)
}
