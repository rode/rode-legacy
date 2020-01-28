package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
  "errors"

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

const path = "/ecr-healthz"

type ecrCollector struct {
	logger            logr.Logger
	awsConfig         *aws.Config
	queueName         string
	queueURL          string
	queueARN          string
	ruleComplete      bool
	occurrenceCreator occurrence.Creator
}

// NewEcrEventCollector will create an collector of ECR events from Cloud watch
func NewEcrEventCollector(logger logr.Logger, awsConfig *aws.Config, occurrenceCreator occurrence.Creator, queueName string) Collector {
	return &ecrCollector{
		logger,
		awsConfig,
		queueName,
		"",
		"",
		false,
		occurrenceCreator,
	}
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

func (i *ecrCollector) reconcileCWEvent(ctx context.Context) error {
	if i.ruleComplete {
		return nil
	}
	session := session.Must(session.NewSession(i.awsConfig))
	svc := cloudwatchevents.New(session)
	sqsSvc := sqs.New(session)
	req, ruleResp := svc.PutRuleRequest(&cloudwatchevents.PutRuleInput{
		Name:         aws.String(i.queueName),
		EventPattern: aws.String(`{"source":["aws.ecr"],"detail-type":["ECR Image Action","ECR Image Scan"]}`),
	})
	i.logger.Info("Putting CW Event rule ","queueName", i.queueName)
	err := req.Send()
	if err != nil {
		return err
	}
	i.logger.Info("Setting queue policy", "queueArn", i.queueARN, "rule",aws.StringValue(ruleResp.RuleArn))
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
	req.Send()
	if err != nil {
		return err
	}

	i.logger.Info("Putting CW Event rule target","queueName", i.queueName,"queueArn", i.queueARN)
	req, resp := svc.PutTargetsRequest(&cloudwatchevents.PutTargetsInput{
		Rule: aws.String(i.queueName),
		Targets: []*cloudwatchevents.Target{
			&cloudwatchevents.Target{
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
		i.logger.Error(errors.New("Failure putting event targets"), "Failure with putting event targets","response", resp)
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
	i.queueARN = aws.StringValue(attrResp.Attributes["QueueArn"])

	go i.watchQueue(ctx)
	return nil
}

func (i *ecrCollector) watchQueue(ctx context.Context) {
	i.logger.Info("Watching Queue","queueUrl", i.queueURL)
	session := session.Must(session.NewSession())
	svc := sqs.New(session, i.awsConfig)
	for i.queueURL != "" {
		req, resp := svc.ReceiveMessageRequest(&sqs.ReceiveMessageInput{
			QueueUrl:          aws.String(i.queueURL),
			VisibilityTimeout: aws.Int64(10),
			WaitTimeSeconds:   aws.Int64(20),
		})

		err := req.Send()
		if err != nil {
			i.logger.Error(err, "Error watching queue")
			return
		}

		for _, msg := range resp.Messages {
			body := aws.StringValue(msg.Body)

			/*
			if i.logger.Desugar().Core().Enabled(zap.DebugLevel) {
				rawJSON := json.RawMessage(body)
				prettyJSON, err := json.MarshalIndent(rawJSON, "", "  ")
				if err != nil {
					i.logger.Errorf("Unable to generate JSON", err)
				}
				fmt.Println(string(prettyJSON))
			}
			*/

			event := &CloudWatchEvent{}
			err = json.Unmarshal([]byte(body), event)
			if err != nil {
				i.logger.Error(err, "Error watching queue")
			}

			var occurrences []*grafeas.Occurrence
			switch event.DetailType {
			case "ECR Image Action":
				details := &ECRImageActionDetail{}
				err = json.Unmarshal(event.Detail, details)
				if err != nil {
					i.logger.Error(err, "Error watching queue")

				}
				occurrences = i.newImageActionOccurrences(event, details)
			case "ECR Image Scan":
				details := &ECRImageScanDetail{}
				err = json.Unmarshal(event.Detail, details)
				if err != nil {
					i.logger.Error(err, "Error watching queue")
				}
				occurrences = i.newImageScanOccurrences(event, details)
			}
			err = i.occurrenceCreator.CreateOccurrences(ctx, occurrences...)
			if err != nil {
				i.logger.Error(err, "Error watching queue")
			}

			delReq, _ := svc.DeleteMessageRequest(&sqs.DeleteMessageInput{
				QueueUrl:      aws.String(i.queueURL),
				ReceiptHandle: msg.ReceiptHandle,
			})
			err = delReq.Send()
			if err != nil {
				i.logger.Error(err, "Error watching queue")
			}
		}
	}
}

func newImageScanOccurrence(event *CloudWatchEvent, detail *ECRImageScanDetail, tag string, noteName string) *grafeas.Occurrence {
	o := &grafeas.Occurrence{}
	o.NoteName = fmt.Sprintf("projects/%s/notes/%s", "rode", noteName)
	o.Resource = &grafeas.Resource{
		Uri: fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:%s@%s", event.AccountID, event.Region, detail.RepositoryName, tag, detail.ImageDigest),
	}
	return o
}

func (i *ecrCollector) getVulnerabilityDetails(detail *ECRImageScanDetail) []*grafeas.Occurrence_Vulnerability {
	// TODO: load from ecr scan results
	vulnerabilityDetails := make([]*grafeas.Occurrence_Vulnerability, 0, 0)
	for k, v := range detail.FindingsSeverityCounts {
		severity := vulnerability.Severity_SEVERITY_UNSPECIFIED
		switch k {
		case "CRITICAL":
			severity = vulnerability.Severity_CRITICAL
		case "HIGH":
			severity = vulnerability.Severity_HIGH
		case "MEDIUM":
			severity = vulnerability.Severity_MEDIUM
		case "LOW":
			severity = vulnerability.Severity_LOW
		case "INFORMATIONAL":
			severity = vulnerability.Severity_MINIMAL
		}
		for i := int64(0); i < v; i++ {
			v := &grafeas.Occurrence_Vulnerability{
				Vulnerability: &vulnerability.Details{
					Severity: severity,
					PackageIssue: []*vulnerability.PackageIssue{
						&vulnerability.PackageIssue{
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
func (i *ecrCollector) newImageScanOccurrences(event *CloudWatchEvent, detail *ECRImageScanDetail) []*grafeas.Occurrence {
	tags := detail.ImageTags
	if len(tags) == 0 {
		tags = []string{"latest"}
	}

	status := discovery.Discovered_ANALYSIS_STATUS_UNSPECIFIED
	vulnerabilityDetails := make([]*grafeas.Occurrence_Vulnerability, 0, 0)
	if detail.ScanStatus == "COMPLETE" {
		status = discovery.Discovered_FINISHED_SUCCESS
		vulnerabilityDetails = i.getVulnerabilityDetails(detail)
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

	occurrences := make([]*grafeas.Occurrence, 0, 0)
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
func newImageActionOccurrence(event *CloudWatchEvent, detail *ECRImageActionDetail) *grafeas.Occurrence {
	return nil
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
