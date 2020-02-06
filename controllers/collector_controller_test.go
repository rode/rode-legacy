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
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

var (
	checkDuration = 20 * time.Second
	checkInterval = 1 * time.Second
)

var _ = Context("collector controller", func() {
	ctx := context.TODO()
	namespace := Setup(ctx)

	When("an ECR collector is created", func() {
		var (
			ecrCollectorName      string
			ecrCollectorQueueName string
			awsSession            *session.Session
			sqsSvc                *sqs.SQS
			cweventsSvc           *cloudwatchevents.CloudWatchEvents
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

			err := k8sClient.Create(ctx, &collector)
			Expect(err).ToNot(HaveOccurred(), "failed to create test collector")

			Eventually(func() bool {
				col := rodev1.Collector{}

				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      ecrCollectorName,
					Namespace: namespace.Name,
				}, &col)

				if err != nil {
					logf.Log.Info("error getting test collector", "error", err)
					return false
				}

				return col.Status.Active
			}, checkDuration, checkInterval).Should(BeTrue())

			awsSession = session.Must(session.NewSession(awsConfig))
			sqsSvc = sqs.New(awsSession)
			cweventsSvc = cloudwatchevents.New(awsSession)
		})

		AfterEach(func() {
			collector := rodev1.Collector{}

			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      ecrCollectorName,
				Namespace: namespace.Name,
			}, &collector)
			Expect(err).ToNot(HaveOccurred(), "error getting test collector during destroy")

			err = k8sClient.Delete(ctx, &collector)
			Expect(err).ToNot(HaveOccurred(), "error destroying test collector")

			Eventually(func() bool {
				col := rodev1.Collector{}

				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      ecrCollectorName,
					Namespace: namespace.Name,
				}, &col)

				if err == nil {
					return false
				}

				return true
			}, checkDuration, checkInterval).Should(BeTrue())
		})

		It("should have created an SQS queue", func() {
			req, resp := sqsSvc.GetQueueUrlRequest(&sqs.GetQueueUrlInput{
				QueueName: aws.String(ecrCollectorQueueName),
			})

			err := req.Send()
			Expect(err).ToNot(HaveOccurred(), "failed to get SQS queue")

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
			Expect(err).ToNot(HaveOccurred(), "failed to get SQS queue")

			queueAttributesReq, queueAttributesResp := sqsSvc.GetQueueAttributesRequest(&sqs.GetQueueAttributesInput{
				QueueUrl: queueUrlResp.QueueUrl,
				AttributeNames: []*string{
					aws.String("QueueArn"),
				},
			})

			err = queueAttributesReq.Send()
			Expect(err).ToNot(HaveOccurred(), "failed to get SQS queue attributes")

			Expect(target.Arn).To(BeEquivalentTo(queueAttributesResp.Attributes["QueueArn"]))
		})
	})
})
