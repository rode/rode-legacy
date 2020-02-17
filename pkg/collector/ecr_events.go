package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
	"github.com/aws/aws-sdk-go/service/sqs"
	discovery "github.com/grafeas/grafeas/proto/v1beta1/discovery_go_proto"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	packag "github.com/grafeas/grafeas/proto/v1beta1/package_go_proto"
	vulnerability "github.com/grafeas/grafeas/proto/v1beta1/vulnerability_go_proto"
	"github.com/liatrio/rode/pkg/occurrence"
)

type ecrCollector struct {
	logger       logr.Logger
	awsConfig    *aws.Config
	queueName    string
	queueURL     string
	queueARN     string
	ruleComplete bool
}

// NewEcrEventCollector will create an collector of ECR events from Cloud watch
func NewEcrEventCollector(logger logr.Logger, awsConfig *aws.Config, queueName string) Collector {
	return &ecrCollector{
		logger,
		awsConfig,
		queueName,
		"",
		"",
		false,
	}
}

func (i *ecrCollector) Type() string {
	return "ecr_event"
}

func (i *ecrCollector) Reconcile(ctx context.Context) error {
	err := i.reconcileSQS(ctx)
	if err != nil {
		return err
	}

	err = i.reconcileCWEvent(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (i *ecrCollector) Destroy(ctx context.Context) error {
	ses := session.Must(session.NewSession(i.awsConfig))

	if i.ruleComplete {
		cwSvc := cloudwatchevents.New(ses)
		ruleFound := false

		req, listRulesResponse := cwSvc.ListRulesRequest(&cloudwatchevents.ListRulesInput{})
		err := req.Send()
		if err != nil {
			return err
		}

		for _, rule := range listRulesResponse.Rules {
			if *rule.Name == i.queueName {
				ruleFound = true
			}
		}

		if ruleFound {
			req, _ := cwSvc.DeleteRuleRequest(&cloudwatchevents.DeleteRuleInput{
				Name: &i.queueName,
			})

			err = req.Send()
			if err != nil {
				return err
			}
		}

		i.logger.Info("successfully destroyed CW Event rule target", "queueName", i.queueName)
		i.ruleComplete = false
	}

	if i.queueURL != "" {
		sqsSvc := sqs.New(ses)
		req, _ := sqsSvc.DeleteQueueRequest(&sqs.DeleteQueueInput{
			QueueUrl: &i.queueURL,
		})

		err := req.Send()
		if err != nil {
			return err
		}

		i.logger.Info("successfully destroyed queue", "queue", i.queueURL)
		i.queueURL = ""
	}

	return nil
}

func (i *ecrCollector) Start(ctx context.Context, stopChan chan interface{}, occurrenceCreator occurrence.Creator) error {
	go func() {
		i.logger.Info("Watching Queue", "queueUrl", i.queueURL)
		ses := session.Must(session.NewSession())
		svc := sqs.New(ses, i.awsConfig)

		for {
			select {
			case <-ctx.Done():
				stopChan <- true
				return
			default:
				err := i.watchQueue(ctx, svc, occurrenceCreator)
				if err != nil {
					i.logger.Error(err, "error watching queue")
				}
			}
		}
	}()

	return nil
}

func (i *ecrCollector) reconcileCWEvent(ctx context.Context) error {
	if i.ruleComplete {
		return nil
	}

	session := session.Must(session.NewSession(i.awsConfig))
	svc := cloudwatchevents.New(session)
	sqsSvc := sqs.New(session)

	i.logger.Info("Putting CW Event rule ", "queueName", i.queueName)
	req, ruleResp := svc.PutRuleRequest(&cloudwatchevents.PutRuleInput{
		Name:         aws.String(i.queueName),
		EventPattern: aws.String(`{"source":["aws.ecr"],"detail-type":["ECR Image Action","ECR Image Scan"]}`),
	})

	err := req.Send()
	if err != nil {
		return err
	}

	i.logger.Info("Setting queue policy", "queueArn", i.queueARN, "rule", aws.StringValue(ruleResp.RuleArn))
	req, _ = sqsSvc.SetQueueAttributesRequest(&sqs.SetQueueAttributesInput{
		QueueUrl: aws.String(i.queueURL),
		Attributes: map[string]*string{
			"Policy": aws.String(fmt.Sprintf(`
{
	"Version": "2012-10-17",
	"Id": "queue-policy",
	"Statement": [
		{
			"Sid": "cloudwatch-event-rule",
			"Effect": "Allow",
			"Principal": {
				"Service": "events.amazonaws.com"
			},
			"Action": "sqs:SendMessage",
			"Resource": "%s",
			"Condition": {
				"ArnEquals": {
					"aws:SourceArn": "%s"
				}
			}
		}
	]
}`, i.queueARN, aws.StringValue(ruleResp.RuleArn)))}})

	err = req.Send()
	if err != nil {
		return err
	}

	i.logger.Info("Putting CW Event rule target", "queueName", i.queueName, "queueArn", i.queueARN)
	req, resp := svc.PutTargetsRequest(&cloudwatchevents.PutTargetsInput{
		Rule: aws.String(i.queueName),
		Targets: []*cloudwatchevents.Target{
			{
				Id:  aws.String("RodeCollector"),
				Arn: aws.String(i.queueARN),
			},
		},
	})

	err = req.Send()
	if err != nil {
		return err
	}

	if aws.Int64Value(resp.FailedEntryCount) > 0 {
		i.logger.Error(errors.New("Failure putting event targets"), "Failure with putting event targets", "response", resp)
	} else {
		i.ruleComplete = true
	}

	return nil
}

func (i *ecrCollector) reconcileSQS(ctx context.Context) error {
	if i.queueURL != "" {
		return nil
	}

	session := session.Must(session.NewSession(i.awsConfig))
	svc := sqs.New(session)

	req, resp := svc.GetQueueUrlRequest(&sqs.GetQueueUrlInput{
		QueueName: aws.String(i.queueName),
	})
	err := req.Send()

	var queueURL string
	if err != nil || resp.QueueUrl == nil {
		i.logger.Info("Creating new SQS queue", "queueName", i.queueName)
		req, createResp := svc.CreateQueueRequest(&sqs.CreateQueueInput{
			QueueName: aws.String(i.queueName),
		})

		err = req.Send()
		if err != nil {
			return err
		}

		queueURL = aws.StringValue(createResp.QueueUrl)
	} else {
		queueURL = aws.StringValue(resp.QueueUrl)
	}

	i.queueURL = queueURL

	req, attrResp := svc.GetQueueAttributesRequest(&sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(queueURL),
		AttributeNames: []*string{
			aws.String("QueueArn"),
		},
	})

	err = req.Send()
	if err != nil {
		return err
	}

	i.queueARN = aws.StringValue(attrResp.Attributes["QueueArn"])

	return nil
}

func (i *ecrCollector) watchQueue(ctx context.Context, svc *sqs.SQS, occurrenceCreator occurrence.Creator) error {
	req, resp := svc.ReceiveMessageRequest(&sqs.ReceiveMessageInput{
		QueueUrl:          aws.String(i.queueURL),
		VisibilityTimeout: aws.Int64(10),
		WaitTimeSeconds:   aws.Int64(5),
	})

	err := req.Send()
	if err != nil {
		return err
	}

	for _, msg := range resp.Messages {
		body := aws.StringValue(msg.Body)
		event := &CloudWatchEvent{}
		err = json.Unmarshal([]byte(body), event)
		if err != nil {
			return err
		}

		var occurrences []*grafeas.Occurrence
		switch event.DetailType {
		case "ECR Image Action":
			details := &ECRImageActionDetail{}
			err = json.Unmarshal(event.Detail, details)
			if err != nil {
				return err
			}

			occurrences = i.newImageActionOccurrences(event, details)
		case "ECR Image Scan":
			details := &ECRImageScanDetail{}
			err = json.Unmarshal(event.Detail, details)
			if err != nil {
				return err
			}

			occurrences = i.newImageScanOccurrences(event, details)
		}

		err = occurrenceCreator.CreateOccurrences(ctx, occurrences...)
		if err != nil {
			return err
		}

		delReq, _ := svc.DeleteMessageRequest(&sqs.DeleteMessageInput{
			QueueUrl:      aws.String(i.queueURL),
			ReceiptHandle: msg.ReceiptHandle,
		})

		err = delReq.Send()
		if err != nil {
			return err
		}
	}

	return nil
}

func newImageScanOccurrence(event *CloudWatchEvent, detail *ECRImageScanDetail, tag string, queueName string) *grafeas.Occurrence {
	o := &grafeas.Occurrence{
		Resource: &grafeas.Resource{
			Uri: EcrOccurrenceResourceURI(event.AccountID, event.Region, detail.RepositoryName, tag, detail.ImageDigest),
		},
		NoteName: EcrOccurrenceNote(queueName),
	}

	return o
}

func EcrOccurrenceResourceURI(account, region, repository, tag, digest string) string {
	return fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:%s@%s", account, region, repository, tag, digest)
}

func EcrOccurrenceNote(queueName string) string {
	return fmt.Sprintf("projects/%s/notes/%s", "rode", queueName)
}

func getVulnerabilityDetails(detail *ECRImageScanDetail) []*grafeas.Occurrence_Vulnerability {
	// TODO: load from ecr scan results
	vulnerabilityDetails := make([]*grafeas.Occurrence_Vulnerability, 0)

	for k, v := range detail.FindingsSeverityCounts {
		severity := getVulnerabilitySeverity(k)

		for i := int64(0); i < v; i++ {
			v := &grafeas.Occurrence_Vulnerability{
				Vulnerability: &vulnerability.Details{
					Severity: severity,
					PackageIssue: []*vulnerability.PackageIssue{
						{
							AffectedLocation: &vulnerability.VulnerabilityLocation{
								CpeUri:  "TODO",
								Package: "TODO",
								Version: &packag.Version{
									Kind: packag.Version_NORMAL,
									Name: "TODO",
								},
							},
						},
					},
				},
			}
			vulnerabilityDetails = append(vulnerabilityDetails, v)
		}
	}

	return vulnerabilityDetails
}

func getVulnerabilitySeverity(v string) vulnerability.Severity {
	switch v {
	case ECRSeverityCritical:
		return vulnerability.Severity_CRITICAL
	case ECRSeverityHigh:
		return vulnerability.Severity_HIGH
	case ECRSeverityMedium:
		return vulnerability.Severity_MEDIUM
	case ECRSeverityLow:
		return vulnerability.Severity_LOW
	case ECRSeverityInformational:
		return vulnerability.Severity_MINIMAL
	default:
		return vulnerability.Severity_SEVERITY_UNSPECIFIED
	}
}

func (i *ecrCollector) newImageScanOccurrences(event *CloudWatchEvent, detail *ECRImageScanDetail) []*grafeas.Occurrence {
	tags := detail.ImageTags
	if len(tags) == 0 {
		tags = []string{"latest"}
	}

	status := discovery.Discovered_ANALYSIS_STATUS_UNSPECIFIED
	vulnerabilityDetails := make([]*grafeas.Occurrence_Vulnerability, 0)

	if detail.ScanStatus == "COMPLETE" {
		status = discovery.Discovered_FINISHED_SUCCESS
		vulnerabilityDetails = getVulnerabilityDetails(detail)
	} else if detail.ScanStatus == "FAILED" {
		status = discovery.Discovered_FINISHED_FAILED
	}

	discoveryDetails := &grafeas.Occurrence_Discovered{
		Discovered: &discovery.Details{
			Discovered: &discovery.Discovered{
				AnalysisStatus: status,
			},
		},
	}

	occurrences := make([]*grafeas.Occurrence, 0)
	for _, tag := range tags {
		o := newImageScanOccurrence(event, detail, tag, i.queueName)
		o.Details = discoveryDetails
		occurrences = append(occurrences, o)

		for _, v := range vulnerabilityDetails {
			o = newImageScanOccurrence(event, detail, tag, i.queueName)
			o.Details = v
			occurrences = append(occurrences, o)
		}
	}

	return occurrences
}
func (i *ecrCollector) newImageActionOccurrences(event *CloudWatchEvent, detail *ECRImageActionDetail) []*grafeas.Occurrence {
	return nil
}

// CloudWatchEvent structured event
type CloudWatchEvent struct {
	Version    string          `json:"version"`
	ID         string          `json:"id"`
	DetailType string          `json:"detail-type"`
	Source     string          `json:"source"`
	AccountID  string          `json:"account"`
	Time       time.Time       `json:"time"`
	Region     string          `json:"region"`
	Resources  []string        `json:"resources"`
	Detail     json.RawMessage `json:"detail"`
}

// CloudTrailEventDetail structured event details
type CloudTrailEventDetail struct {
	EventVersion string    `json:"eventVersion"`
	EventID      string    `json:"eventID"`
	EventTime    time.Time `json:"eventTime"`
	EventType    string    `json:"eventType"`
	AwsRegion    string    `json:"awsRegion"`
	EventName    string    `json:"eventName"`
	UserIdentity struct {
		UserName    string `json:"userName"`
		PrincipalID string `json:"principalId"`
		AccessKeyID string `json:"accessKeyId"`
		InvokedBy   string `json:"invokedBy"`
		Type        string `json:"type"`
		Arn         string `json:"arn"`
		AccountID   string `json:"accountId"`
	} `json:"userIdentity"`
	EventSource       string                 `json:"eventSource"`
	RequestID         string                 `json:"requestID"`
	RequestParameters map[string]interface{} `json:"requestParameters"`
	ResponseElements  map[string]interface{} `json:"responseElements"`
}

// ECRImageActionDetail structured event details
type ECRImageActionDetail struct {
	ActionType     string `json:"action-type"`
	RepositoryName string `json:"repository-name"`
	ImageDigest    string `json:"image-digest"`
	ImageTag       string `json:"image-tag"`
	Result         string `json:"result"`
}

// ECRImageScanDetail structured event details
type ECRImageScanDetail struct {
	ScanStatus             string           `json:"scan-status"`
	RepositoryName         string           `json:"repository-name"`
	ImageDigest            string           `json:"image-digest"`
	ImageTags              []string         `json:"image-tags"`
	FindingsSeverityCounts map[string]int64 `json:"finding-severity-counts"`
}

type ECRImageScanSeverity string

const (
	ECRSeverityCritical      = "CRITICAL"
	ECRSeverityHigh          = "HIGH"
	ECRSeverityMedium        = "MEDIUM"
	ECRSeverityLow           = "LOW"
	ECRSeverityInformational = "INFORMATIONAL"
)
