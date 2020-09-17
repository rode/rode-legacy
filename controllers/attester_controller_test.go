// +build !unit

package controllers

import (
	"context"
	"fmt"

	rodev1alpha1 "github.com/liatrio/rode/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
)

var _ = Context("attester controller", func() {
	var (
		attesterName    string
		secretName      string
		shouldBeDeleted bool
	)

	ctx := context.TODO()
	namespace := SetupTestNamespace(ctx)

	When("an Attester is created", func() {

		BeforeEach(func() {
			shouldBeDeleted = true

			attesterName = fmt.Sprintf("attester%s", rand.String(10))
			secretName = fmt.Sprintf("secret%s", rand.String(10))

			attester := &rodev1alpha1.Attester{
				ObjectMeta: metav1.ObjectMeta{
					Name:      attesterName,
					Namespace: namespace.Name,
				},
				Spec: rodev1alpha1.AttesterSpec{
					PgpSecret: secretName,
					Policy:    basicAttesterPolicy(attesterName),
				},
			}

			createAttester(ctx, attester)
		})

		AfterEach(func() {
			if shouldBeDeleted == true {
				destroyAttester(ctx, attesterName, namespace.Name)
			}
		})

		It("should have compiled policy", func() {
			att := rodev1alpha1.Attester{}

			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      attesterName,
				Namespace: namespace.Name,
			}, &att)

			Expect(err).ToNot(HaveOccurred(), "error getting test attester", err)

			status := att.Status.Conditions[0].Status
			Expect(status).To(BeEquivalentTo(rodev1alpha1.ConditionStatusTrue))
		})

		It("should have created a secret", func() {
			att := rodev1alpha1.Attester{}

			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      attesterName,
				Namespace: namespace.Name,
			}, &att)

			Expect(err).ToNot(HaveOccurred(), "error getting test attest", err)

			status := rodev1alpha1.GetConditionStatus(&att, rodev1alpha1.ConditionSecret)
			Expect(status).To(BeEquivalentTo(rodev1alpha1.ConditionStatusTrue))

			secret := corev1.Secret{}

			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      secretName,
				Namespace: namespace.Name,
			}, &secret)

			Expect(err).ToNot(HaveOccurred(), "error getting test secret", err)

		})

		It("should delete the secret if it created it", func() {
			destroyAttester(ctx, attesterName, namespace.Name)
			shouldBeDeleted = false

			secret := corev1.Secret{}

			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      secretName,
				Namespace: namespace.Name,
			}, &secret)

			Expect(err).To(HaveOccurred(), "secret still exists", err)
		})

	})
	//TODO: Actually create an attestation occurence in grafeas when occurences don't violate policy

	//TODO: Don't create an attestation occurence when occurences violate policy

	//TODO: Create invalid rego test

	//TODO: Create reuse existing secret test

	//TODO: Create a test that assures that the controller does not delete the secret if it didn't create it

	//TODO: Invalid Pgp key in existing secret that user applied

})
