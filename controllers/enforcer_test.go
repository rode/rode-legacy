package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"net/http"
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
})

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
