package controllers

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
	"github.com/aws/aws-sdk-go/service/sqs"
	rodev1 "github.com/liatrio/rode/api/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"time"
)

var (
	checkDuration = 20 * time.Second
	checkInterval = 1 * time.Second
)

var _ = Context("collector controller", func() {
	ctx := context.TODO()
	namespace := SetupTestNamespace(ctx)

	When("an ECR collector is created", func() {
		var (
			ecrCollectorName      string
			ecrCollectorQueueName string
			awsSession            *session.Session
			sqsSvc                *sqs.SQS
			cweventsSvc           *cloudwatchevents.CloudWatchEvents
			shouldDestroy         = true
		)

		BeforeEach(func() {
			ecrCollectorName = fmt.Sprintf("test-ecr-collector-%s", rand.String(10))
			ecrCollectorQueueName = fmt.Sprintf("test-queue-%s", rand.String(10))

			collector := rodev1.Collector{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ecrCollectorName,
					Namespace: namespace.Name,
				},
				Spec: rodev1.CollectorSpec{
					CollectorType: "ecr_event",
					ECR: rodev1.CollectorECRConfig{
						QueueName: ecrCollectorQueueName,
					},
				},
			}

			createCollector(ctx, &collector)

			awsSession = session.Must(session.NewSession(awsConfig))
			sqsSvc = sqs.New(awsSession)
			cweventsSvc = cloudwatchevents.New(awsSession)
		})

		AfterEach(func() {
			if !shouldDestroy {
				shouldDestroy = true
				return
			}

			destroyCollector(ctx, ecrCollectorName, namespace.Name)
		})

		It("should have created an SQS queue", func() {
			req, resp := sqsSvc.GetQueueUrlRequest(&sqs.GetQueueUrlInput{
				QueueName: aws.String(ecrCollectorQueueName),
			})

			err := req.Send()
			Expect(err).ToNot(HaveOccurred(), "failed to get SQS queue", err)

			Expect(aws.StringValue(resp.QueueUrl)).To(ContainSubstring(ecrCollectorQueueName))
		})

		It("should have created a cloudwatch event rule for the SQS queue", func() {
			cwRequest, cwResp := cweventsSvc.ListTargetsByRuleRequest(&cloudwatchevents.ListTargetsByRuleInput{
				Rule: aws.String(ecrCollectorQueueName),
			})

			err := cwRequest.Send()
			Expect(err).ToNot(HaveOccurred(), "failed to list CW events rules", err)

			Expect(cwResp.Targets).To(HaveLen(1))
			target := cwResp.Targets[0]
			Expect(*target.Id).To(BeIdenticalTo("RodeCollector"))

			// fetch queue ARN to assert against target

			queueUrlReq, queueUrlResp := sqsSvc.GetQueueUrlRequest(&sqs.GetQueueUrlInput{
				QueueName: aws.String(ecrCollectorQueueName),
			})

			err = queueUrlReq.Send()
			Expect(err).ToNot(HaveOccurred(), "failed to get SQS queue", err)

			queueAttributesReq, queueAttributesResp := sqsSvc.GetQueueAttributesRequest(&sqs.GetQueueAttributesInput{
				QueueUrl: queueUrlResp.QueueUrl,
				AttributeNames: []*string{
					aws.String("QueueArn"),
				},
			})

			err = queueAttributesReq.Send()
			Expect(err).ToNot(HaveOccurred(), "failed to get SQS queue attributes", err)

			Expect(target.Arn).To(BeEquivalentTo(queueAttributesResp.Attributes["QueueArn"]))
		})

		It("should destroy AWS resources when deleted", func() {
			destroyCollector(ctx, ecrCollectorName, namespace.Name)
			shouldDestroy = false

			cwRequest, _ := cweventsSvc.ListTargetsByRuleRequest(&cloudwatchevents.ListTargetsByRuleInput{
				Rule: aws.String(ecrCollectorQueueName),
			})

			err := cwRequest.Send()
			Expect(err).To(HaveOccurred(), "expected get operation for CW events rules to fail")

			queueUrlReq, _ := sqsSvc.GetQueueUrlRequest(&sqs.GetQueueUrlInput{
				QueueName: aws.String(ecrCollectorQueueName),
			})

			err = queueUrlReq.Send()
			Expect(err).To(HaveOccurred(), "expected get operation for SQS queue to fail")
		})
	})
})

func createCollector(ctx context.Context, collector *rodev1.Collector) {
	err := k8sClient.Create(ctx, collector)
	Expect(err).ToNot(HaveOccurred(), "failed to create test collector", err)

	Eventually(func() bool {
		col := rodev1.Collector{}

		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      collector.Name,
			Namespace: collector.Namespace,
		}, &col)

		if err != nil {
			return false
		}

		return col.Status.Active
	}, checkDuration, checkInterval).Should(BeTrue())
}

func destroyCollector(ctx context.Context, name, namespace string) {
	collector := rodev1.Collector{}

	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &collector)
	Expect(err).ToNot(HaveOccurred(), "error getting test collector during destroy", err)

	err = k8sClient.Delete(ctx, &collector)
	Expect(err).ToNot(HaveOccurred(), "error destroying test collector", err)

	Eventually(func() bool {
		col := rodev1.Collector{}

		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, &col)

		if err == nil {
			return false
		}

		return true
	}, checkDuration, checkInterval).Should(BeTrue())
}
