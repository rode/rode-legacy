package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
	"github.com/aws/aws-sdk-go/service/sqs"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	rodev1alpha1 "github.com/liatrio/rode/api/v1alpha1"
	"github.com/liatrio/rode/pkg/collector"
	"github.com/liatrio/rode/pkg/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"time"
)

var (
	checkDuration = 15 * time.Second
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
			shouldDestroy         bool
			awsAccountNumber      = test.RandomAWSAccountNumber()
		)

		BeforeEach(func() {
			shouldDestroy = true

			ecrCollectorName = fmt.Sprintf("test-ecr-collector-%s", rand.String(10))
			ecrCollectorQueueName = fmt.Sprintf("test-queue-%s", rand.String(10))

			collector := rodev1alpha1.Collector{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ecrCollectorName,
					Namespace: namespace.Name,
				},
				Spec: rodev1alpha1.CollectorSpec{
					CollectorType: "ecr_event",
					ECR: rodev1alpha1.CollectorECRConfig{
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
			if shouldDestroy {
				destroyCollector(ctx, ecrCollectorName, namespace.Name)
			}
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
			Expect(*target.Id).To(BeEquivalentTo("RodeCollector"))

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

		It("should create an occurrence for ECR image scans", func() {
			queueUrlReq, queueUrlResp := sqsSvc.GetQueueUrlRequest(&sqs.GetQueueUrlInput{
				QueueName: aws.String(ecrCollectorQueueName),
			})

			err := queueUrlReq.Send()
			Expect(err).ToNot(HaveOccurred(), "failed to get SQS queue", err)

			queueUrl := queueUrlResp.QueueUrl

			ecrImageScanDetail := createTestECRImageScanDetail(awsAccountNumber, *awsConfig.Region, "springtrader", "COMPLETE")
			ecrImageScanEvent := createTestECRImageScanEvent(awsAccountNumber, *awsConfig.Region, "springtrader", ecrImageScanDetail)
			ecrImageScanEventBody, err := json.Marshal(ecrImageScanEvent)
			if err != nil {
				Expect(err).ToNot(HaveOccurred(), "failed to marshal ecr image event to json", err)
			}

			sqsMessageBody := string(ecrImageScanEventBody)
			sendMessageReq, _ := sqsSvc.SendMessageRequest(&sqs.SendMessageInput{
				MessageBody: &sqsMessageBody,
				QueueUrl:    queueUrl,
			})

			err = sendMessageReq.Send()
			Expect(err).ToNot(HaveOccurred(), "failed to send ECR image action message", err)

			expectedNumberOfOccurrences := getExpectedNumberOfOccurrences(ecrImageScanDetail)
			Eventually(func() []*grafeas.Occurrence {
				return getOccurrencesForQueue(ctx, ecrCollectorQueueName)
			}, checkDuration, checkInterval).Should(HaveLen(expectedNumberOfOccurrences))
		})
	})
})

func getExpectedNumberOfOccurrences(detail *collector.ECRImageScanDetail) int {
	result := 0
	for _, numberOfVulns := range detail.FindingsSeverityCounts {
		result += int(numberOfVulns) * len(detail.ImageTags)
	}

	return result + len(detail.ImageTags)
}

func createCollector(ctx context.Context, collector *rodev1alpha1.Collector) {
	err := k8sClient.Create(ctx, collector)
	Expect(err).ToNot(HaveOccurred(), "failed to create test collector", err)

	Eventually(func() rodev1alpha1.ConditionStatus {
		col := rodev1alpha1.Collector{}

		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      collector.Name,
			Namespace: collector.Namespace,
		}, &col)

		if err != nil {
			return rodev1alpha1.ConditionStatusFalse
		}

		for _, cond := range col.Status.Conditions {
			if cond.Type == rodev1alpha1.CollectorConditionActive {
				return cond.Status
			}
		}

		return rodev1alpha1.ConditionStatusFalse
	}, checkDuration, checkInterval).Should(BeEquivalentTo(rodev1alpha1.ConditionStatusTrue))
}

func destroyCollector(ctx context.Context, name, namespace string) {
	collector := rodev1alpha1.Collector{}

	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &collector)
	Expect(err).ToNot(HaveOccurred(), "error getting test collector during destroy", err)

	err = k8sClient.Delete(ctx, &collector)
	Expect(err).ToNot(HaveOccurred(), "error destroying test collector", err)

	Eventually(func() bool {
		col := rodev1alpha1.Collector{}

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

func getOccurrencesForQueue(ctx context.Context, queueName string) []*grafeas.Occurrence {
	var result []*grafeas.Occurrence

	occurrences, err := grafeasClient.ListOccurrences(ctx, &grafeas.ListOccurrencesRequest{
		Parent:   "projects/rode",
		PageSize: 1000,
	})
	Expect(err).ToNot(HaveOccurred(), "failed listing grafeas occurrences", err)

	for _, occurrence := range occurrences.Occurrences {
		if occurrence.NoteName == collector.EcrOccurrenceNote(queueName) {
			result = append(result, occurrence)
		}
	}

	return result
}

func createTestECRImageScanDetail(account, region, repository, status string) *collector.ECRImageScanDetail {
	severities := []string{
		collector.ECR_Severity_CRITICAL,
		collector.ECR_Severity_HIGH,
		collector.ECR_Severity_MEDIUM,
		collector.ECR_Severity_LOW,
		collector.ECR_Severity_INFORMATIONAL,
	}
	randomSeverities := test.RandomStringSliceSubset(severities)

	findingSeverityCounts := make(map[string]int64)
	for _, v := range randomSeverities {
		findingSeverityCounts[v] = int64(rand.Intn(10))
	}

	return &collector.ECRImageScanDetail{
		ScanStatus:             status,
		RepositoryName:         repository,
		ImageDigest:            fmt.Sprintf("sha256:%s", test.CreateTestSha256(account, region, repository, status)),
		ImageTags:              test.RandomStringSlice(),
		FindingsSeverityCounts: findingSeverityCounts,
	}
}

func createTestECRImageScanEvent(account, region, repository string, detail *collector.ECRImageScanDetail) *collector.CloudWatchEvent {
	ecrImageScanDetailJson, _ := json.Marshal(detail)

	return &collector.CloudWatchEvent{
		Version:    "0",
		ID:         "9e5f4498-b166-586f-1779-3589839bb916",
		DetailType: "ECR Image Scan",
		Source:     "aws.ecr",
		AccountID:  account,
		Time:       time.Now().UTC(),
		Region:     region,
		Resources: []string{
			fmt.Sprintf("arn:aws:ecr:%s:%s:repository/%s", region, account, repository),
		},
		Detail: ecrImageScanDetailJson,
	}
}
