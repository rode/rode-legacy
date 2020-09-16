package aws

import (
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/sqs"
)

// SQS is a partial interface definition for the AWS SQS service
// Full definition: https://docs.aws.amazon.com/sdk-for-go/api/service/sqs/sqsiface/
type SQS interface {
	CreateQueueRequest(*sqs.CreateQueueInput) (*request.Request, *sqs.CreateQueueOutput)
	DeleteMessageRequest(*sqs.DeleteMessageInput) (*request.Request, *sqs.DeleteMessageOutput)
	DeleteQueueRequest(*sqs.DeleteQueueInput) (*request.Request, *sqs.DeleteQueueOutput)
	GetQueueAttributesRequest(*sqs.GetQueueAttributesInput) (*request.Request, *sqs.GetQueueAttributesOutput)
	GetQueueUrlRequest(*sqs.GetQueueUrlInput) (*request.Request, *sqs.GetQueueUrlOutput)
	ReceiveMessageRequest(*sqs.ReceiveMessageInput) (*request.Request, *sqs.ReceiveMessageOutput)
	SetQueueAttributesRequest(*sqs.SetQueueAttributesInput) (*request.Request, *sqs.SetQueueAttributesOutput)
}

// CWE is a partial interface definition for the AWS CloudWatchEvents service
// Full definition: https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatchevents/cloudwatcheventsiface/
type CWE interface {
	DeleteRuleRequest(*cloudwatchevents.DeleteRuleInput) (*request.Request, *cloudwatchevents.DeleteRuleOutput)
	ListRulesRequest(*cloudwatchevents.ListRulesInput) (*request.Request, *cloudwatchevents.ListRulesOutput)
	PutRuleRequest(*cloudwatchevents.PutRuleInput) (*request.Request, *cloudwatchevents.PutRuleOutput)
	PutTargetsRequest(*cloudwatchevents.PutTargetsInput) (*request.Request, *cloudwatchevents.PutTargetsOutput)
}

// ECR is a partial interface definition for the AWS ECR service
// Full definition: https://docs.aws.amazon.com/sdk-for-go/api/service/ecr/ecriface/
type ECR interface {
	DescribeImageScanFindings(*ecr.DescribeImageScanFindingsInput) (*ecr.DescribeImageScanFindingsOutput, error)
}
