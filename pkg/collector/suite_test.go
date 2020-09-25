// +build !unit

/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package collector

import (
	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
	"github.com/aws/aws-sdk-go/service/sqs"
	awsTypes "github.com/liatrio/rode/pkg/aws"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Collector Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	//ctx := context.Background()

	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
})

type mockSqsService struct {
}

type mockCweService struct {
}

type mockAwsRequest struct {
}

func (m mockAwsRequest) Send() error {
	return nil
}

func (m mockSqsService) CreateQueueRequest(*sqs.CreateQueueInput) (awsTypes.Request, *sqs.CreateQueueOutput) {
	queueUrl := "test"

	return mockAwsRequest{}, &sqs.CreateQueueOutput{
		QueueUrl: &queueUrl,
	}
}

func (m mockSqsService) DeleteMessageRequest(*sqs.DeleteMessageInput) (awsTypes.Request, *sqs.DeleteMessageOutput) {
	return nil, nil
}

func (m mockSqsService) DeleteQueueRequest(*sqs.DeleteQueueInput) (awsTypes.Request, *sqs.DeleteQueueOutput) {
	return nil, nil
}

func (m mockSqsService) GetQueueAttributesRequest(*sqs.GetQueueAttributesInput) (awsTypes.Request, *sqs.GetQueueAttributesOutput) {
	queueArn := "test"

	return mockAwsRequest{}, &sqs.GetQueueAttributesOutput{
		Attributes: map[string]*string{
			"QueueArn": &queueArn,
		},
	}
}

func (m mockSqsService) GetQueueUrlRequest(*sqs.GetQueueUrlInput) (awsTypes.Request, *sqs.GetQueueUrlOutput) {
	return mockAwsRequest{}, &sqs.GetQueueUrlOutput{
		QueueUrl: nil,
	}
}

func (m mockSqsService) ReceiveMessageRequest(*sqs.ReceiveMessageInput) (awsTypes.Request, *sqs.ReceiveMessageOutput) {
	return nil, nil
}

func (m mockSqsService) SetQueueAttributesRequest(*sqs.SetQueueAttributesInput) (awsTypes.Request, *sqs.SetQueueAttributesOutput) {
	return mockAwsRequest{}, nil
}

func (m mockCweService) DeleteRuleRequest(*cloudwatchevents.DeleteRuleInput) (awsTypes.Request, *cloudwatchevents.DeleteRuleOutput) {
	return nil, nil
}

func (m mockCweService) ListRulesRequest(*cloudwatchevents.ListRulesInput) (awsTypes.Request, *cloudwatchevents.ListRulesOutput) {
	return nil, nil
}

func (m mockCweService) PutRuleRequest(*cloudwatchevents.PutRuleInput) (awsTypes.Request, *cloudwatchevents.PutRuleOutput) {
	ruleArn := "test"

	return mockAwsRequest{}, &cloudwatchevents.PutRuleOutput{
		RuleArn: &ruleArn,
	}
}

func (m mockCweService) PutTargetsRequest(*cloudwatchevents.PutTargetsInput) (awsTypes.Request, *cloudwatchevents.PutTargetsOutput) {
	failedEntryCount := int64(1)

	return mockAwsRequest{}, &cloudwatchevents.PutTargetsOutput{
		FailedEntryCount: &failedEntryCount,
	}
}
