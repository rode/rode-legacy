package aws

import (
	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/sqs"
)

type Request interface {
	Send() error
}

// SQS is a partial interface definition for the AWS SQS service
// Full definition: https://docs.aws.amazon.com/sdk-for-go/api/service/sqs/sqsiface/
type SQS interface {
	CreateQueueRequest(*sqs.CreateQueueInput) (Request, *sqs.CreateQueueOutput)
	DeleteMessageRequest(*sqs.DeleteMessageInput) (Request, *sqs.DeleteMessageOutput)
	DeleteQueueRequest(*sqs.DeleteQueueInput) (Request, *sqs.DeleteQueueOutput)
	GetQueueAttributesRequest(*sqs.GetQueueAttributesInput) (Request, *sqs.GetQueueAttributesOutput)
	GetQueueUrlRequest(*sqs.GetQueueUrlInput) (Request, *sqs.GetQueueUrlOutput)
	ReceiveMessageRequest(*sqs.ReceiveMessageInput) (Request, *sqs.ReceiveMessageOutput)
	SetQueueAttributesRequest(*sqs.SetQueueAttributesInput) (Request, *sqs.SetQueueAttributesOutput)
}

// CWE is a partial interface definition for the AWS CloudWatchEvents service
// Full definition: https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatchevents/cloudwatcheventsiface/
type CWE interface {
	DeleteRuleRequest(*cloudwatchevents.DeleteRuleInput) (Request, *cloudwatchevents.DeleteRuleOutput)
	ListRulesRequest(*cloudwatchevents.ListRulesInput) (Request, *cloudwatchevents.ListRulesOutput)
	PutRuleRequest(*cloudwatchevents.PutRuleInput) (Request, *cloudwatchevents.PutRuleOutput)
	PutTargetsRequest(*cloudwatchevents.PutTargetsInput) (Request, *cloudwatchevents.PutTargetsOutput)
}

// ECR is a partial interface definition for the AWS ECR service
// Full definition: https://docs.aws.amazon.com/sdk-for-go/api/service/ecr/ecriface/
type ECR interface {
	DescribeImageScanFindings(*ecr.DescribeImageScanFindingsInput) (*ecr.DescribeImageScanFindingsOutput, error)
}
