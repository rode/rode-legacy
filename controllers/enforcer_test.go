package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	discovery "github.com/grafeas/grafeas/proto/v1beta1/discovery_go_proto"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	rodev1alpha1 "github.com/liatrio/rode/api/v1alpha1"
	"github.com/liatrio/rode/pkg/attester"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"net/http"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

var _ = Context("enforcers", func() {
	ctx := context.TODO()
	namespace := SetupTestNamespace(ctx)

	When("an enforcer does not exist for a namespace", func() {
		It("should allow a pod to be scheduled", func() {
			pod := corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Namespace: namespace.Name,
					Name:      "nginx",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "liatrio/nginx",
						},
					},
				},
			}

			admissionResponse := sendAdmissionRequestForPod(&pod)

			Expect(admissionResponse.Allowed).To(BeTrue())
		})
	})

	When("an enforcer exists for a namespace", func() {
		var (
			enforcerName string
			attesterName string
		)

		BeforeEach(func() {
			enforcerName = fmt.Sprintf("test-enforcer-%s", rand.String(10))
			attesterName = fmt.Sprintf("testattester%s", rand.String(10))

			enforcer := &rodev1alpha1.Enforcer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      enforcerName,
					Namespace: namespace.Name,
				},
				Spec: rodev1alpha1.EnforcerSpec{
					Attesters: []*rodev1alpha1.EnforcerAttester{
						{
							Name:      attesterName,
							Namespace: namespace.Name,
						},
					},
				},
			}

			err := k8sClient.Create(ctx, enforcer)
			Expect(err).ToNot(HaveOccurred(), "failed to create enforcer", err)
		})

		AfterEach(func() {
			enforcer := rodev1alpha1.Enforcer{}

			err := k8sClient.Get(ctx, types.NamespacedName{
				Namespace: namespace.Name,
				Name:      enforcerName,
			}, &enforcer)
			Expect(err).ToNot(HaveOccurred(), "error getting test enforcer during destroy", err)

			err = k8sClient.Delete(ctx, &enforcer)
			Expect(err).ToNot(HaveOccurred(), "error destroying test enforcer", err)
		})

		It("should not allow a pod to be scheduled if the attester does not exist", func() {
			pod := corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Namespace: namespace.Name,
					Name:      "nginx",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "liatrio/nginx",
						},
					},
				},
			}

			admissionResponse := sendAdmissionRequestForPod(&pod)

			Expect(admissionResponse.Allowed).To(BeFalse())
		})

		When("an attester exists in a namespace", func() {
			var internalAttester *attester.Attester

			BeforeEach(func() {
				att := &rodev1alpha1.Attester{
					ObjectMeta: v1.ObjectMeta{
						Name:      attesterName,
						Namespace: namespace.Name,
					},
					Spec: rodev1alpha1.AttesterSpec{
						Policy: basicAttesterPolicy(attesterName),
					},
				}

				createAttester(ctx, att)

				internalAttester = createInternalAttester(ctx, att)
			})

			AfterEach(func() {
				destroyAttester(ctx, attesterName, namespace.Name)
			})

			It("should not allow a pod to be scheduled if there are no attestations", func() {
				pod := corev1.Pod{
					ObjectMeta: v1.ObjectMeta{
						Namespace: namespace.Name,
						Name:      "nginx",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Image: "liatrio/nginx",
							},
						},
					},
				}

				admissionResponse := sendAdmissionRequestForPod(&pod)

				Expect(admissionResponse.Allowed).To(BeFalse())
			})

			It("should allow a pod to be scheduled if there is an attestation", func() {
				Skip("not working yet")
				attestRequest := &attester.AttestRequest{
					ResourceURI: "liatrio/nginx",
					Occurrences: []*grafeas.Occurrence{
						{
							Resource: &grafeas.Resource{
								Uri: "liatrio/nginx",
							},
							NoteName: fmt.Sprintf("projects/rode/notes/%s", attesterName),
							Details: &grafeas.Occurrence_Discovered{
								Discovered: &discovery.Details{
									Discovered: &discovery.Discovered{
										AnalysisStatus: discovery.Discovered_FINISHED_SUCCESS,
									},
								},
							},
						},
					},
				}

				_, err := (*internalAttester).Attest(ctx, attestRequest)
				Expect(err).ToNot(HaveOccurred(), "failed to attest", err)

				pod := corev1.Pod{
					ObjectMeta: v1.ObjectMeta{
						Namespace: namespace.Name,
						Name:      "nginx",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Image: "liatrio/nginx",
							},
						},
					},
				}

				admissionResponse := sendAdmissionRequestForPod(&pod)

				Expect(admissionResponse.Allowed).To(BeTrue())
			})
		})
	})
})

func basicAttesterPolicy(attesterName string) string {
	return fmt.Sprintf(`

package %s

violation[{"msg":"analysis failed"}]{
	input.occurrences[_].discovered.discovered.analysisStatus != "FINISHED_SUCCESS"
}

`, attesterName)
}

func createInternalAttester(ctx context.Context, att *rodev1alpha1.Attester) *attester.Attester {
	attesterNamespacedName := types.NamespacedName{
		Namespace: att.Namespace,
		Name:      att.Name,
	}

	policy, err := attester.NewPolicy(att.Name, att.Spec.Policy, false)
	Expect(err).ToNot(HaveOccurred(), "failed to create test attester policy", err)

	signerSecret := &corev1.Secret{}
	err = k8sClient.Get(ctx, attesterNamespacedName, signerSecret)
	Expect(err).ToNot(HaveOccurred(), "failed to fetch signer secret", err)

	signer, err := attester.ReadSigner(bytes.NewBuffer(signerSecret.Data["keys"]))
	Expect(err).ToNot(HaveOccurred(), "failed to read signer", err)

	internalAttester := attester.NewAttester(attesterNamespacedName.String(), policy, signer)

	return &internalAttester
}

func createAttester(ctx context.Context, attester *rodev1alpha1.Attester) {
	err := k8sClient.Create(ctx, attester)
	Expect(err).ToNot(HaveOccurred(), "failed to create test attester", err)

	Eventually(func() bool {
		att := rodev1alpha1.Attester{}

		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      attester.Name,
			Namespace: attester.Namespace,
		}, &att)

		if err != nil {
			return false
		}

		for _, cond := range att.Status.Conditions {
			logf.Log.Info("waiting", "condition", cond)

			if cond.Status != rodev1alpha1.ConditionStatusTrue {
				return false
			}
		}

		return true
	}, checkDuration, checkInterval).Should(BeTrue())

	time.Sleep(2 * time.Second)
}

func destroyAttester(ctx context.Context, name, namespace string) {
	att := rodev1alpha1.Attester{}

	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &att)
	Expect(err).ToNot(HaveOccurred(), "error getting test attester during destroy", err)

	err = k8sClient.Delete(ctx, &att)
	Expect(err).ToNot(HaveOccurred(), "error destroying test attester", err)

	Eventually(func() bool {
		att := rodev1alpha1.Attester{}

		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, &att)

		if err == nil {
			return false
		}

		return true
	}, checkDuration, checkInterval).Should(BeTrue())
}

func sendAdmissionRequestForPod(pod *corev1.Pod) *v1beta1.AdmissionResponse {
	podJson, err := json.Marshal(pod)
	Expect(err).ToNot(HaveOccurred(), "failed to marshal pod to JSON", err)

	admissionReview := v1beta1.AdmissionReview{
		Request: &v1beta1.AdmissionRequest{
			Kind: v1.GroupVersionKind{
				Group: "",
				Kind:  "Pod",
			},
			Namespace: pod.Namespace,
			Object: runtime.RawExtension{
				Raw: podJson,
			},
		},
	}

	admissionReviewJson, err := json.Marshal(admissionReview)
	Expect(err).ToNot(HaveOccurred(), "failed to marshal admissionReview to JSON", err)

	request, err := http.NewRequest("POST", "https://localhost:31443/validate-v1-pod", bytes.NewBuffer(admissionReviewJson))
	Expect(err).ToNot(HaveOccurred(), "failed to create admission review", err)

	request.Header.Set("Content-type", "application/json")

	response, err := httpClient.Do(request)
	Expect(err).ToNot(HaveOccurred(), "failed to send admission review", err)

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	Expect(err).ToNot(HaveOccurred(), "failed to read response body", err)

	var admissionReviewResponse v1beta1.AdmissionReview
	err = json.Unmarshal(body, &admissionReviewResponse)
	Expect(err).ToNot(HaveOccurred(), "failed to unmarshal JSON", err)

	return admissionReviewResponse.Response
}
