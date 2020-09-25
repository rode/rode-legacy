package collector

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Context("ECR collector", func() {
	When("an ECR collector is created for the first time", func() {
		var ecrCollector Collector

		BeforeEach(func() {
			sqs := mockSqsService{}
			cwe := mockCweService{}

			ecrCollector = NewEcrEventCollector(logf.Log, "", sqs, cwe, nil)
		})

		It("should create an SQS queue", func() {
			err := ecrCollector.Reconcile(context.Background(), types.NamespacedName{
				Namespace: "",
				Name:      "",
			})

			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("an ECR collector has an SQS queue but no CW event rule", func() {
		BeforeEach(func() {

		})

		It("should create a CW event rule", func() {

		})
	})

	When("an ECR collector already has a SQS queue and CW event rule", func() {
		BeforeEach(func() {

		})

		It("should not create an SQS queue", func() {

		})

		It("should not create a CW event rule", func() {

		})
	})

	It("should run", func() {
		Expect(true).To(BeTrue())
	})
})
