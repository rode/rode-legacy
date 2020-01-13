package ingester

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/gin-gonic/gin"
	grafeas "github.com/grafeas/client-go/0.1.0"
	"github.com/liatrio/rode/pkg/ctx"
)

const path = "/ecr-healthz"

type ecrIngester struct {
	ctx.Context
	queueURL string
}

// NewEcrEventIngester will create an ingester of ECR events from Cloud watch
func NewEcrEventIngester(context *ctx.Context) Ingester {
	return &ecrIngester{
		*context,
		"",
	}
}

func (i *ecrIngester) Reconcile() error {
	err := i.reconcileRoutes()
	if err != nil {
		return err
	}
	err = i.reconcileSQS()
	if err != nil {
		return err
	}

	// TODO: reconcile CW event rule

	return nil
}

func (i *ecrIngester) reconcileRoutes() error {
	routes := i.Router.Routes()
	registered := false
	for _, route := range routes {
		if route.Path == path {
			registered = true
			break
		}
	}

	if !registered {
		i.Logger.Infof("Registering path '%s'", path)
		i.Router.GET(path, func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status": "healthy",
			})
		})
	}
	return nil
}

func (i *ecrIngester) reconcileSQS() error {
	if i.queueURL != "" {
		return nil
	}

	queueName := "rode-ecr-event-ingester"
	svc := sqs.New(*i.AWSConfig)
	req := svc.GetQueueUrlRequest(&sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	})
	resp, err := req.Send(context.TODO())
	var queueURL string
	if err != nil || resp.QueueUrl == nil {
		i.Logger.Infof("Creating new SQS queue %s", queueName)
		req := svc.CreateQueueRequest(&sqs.CreateQueueInput{
			QueueName: aws.String(queueName),
		})
		createResp, err := req.Send(context.TODO())
		if err != nil {
			return err
		}
		queueURL = aws.StringValue(createResp.QueueUrl)
	} else {
		queueURL = aws.StringValue(resp.QueueUrl)
	}
	i.queueURL = queueURL

	go i.watchQueue()
	return nil
}

func (i *ecrIngester) watchQueue() {
	i.Logger.Infof("Watching Queue: %s", i.queueURL)
	svc := sqs.New(*i.AWSConfig)
	for i.queueURL != "" {
		req := svc.ReceiveMessageRequest(&sqs.ReceiveMessageInput{
			QueueUrl:          aws.String(i.queueURL),
			VisibilityTimeout: aws.Int64(10),
			WaitTimeSeconds:   aws.Int64(20),
		})

		resp, err := req.Send(context.TODO())
		if err != nil {
			i.Logger.Error(err)
			return
		}

		for _, msg := range resp.Messages {
			occurrence := i.newOccurrence(aws.StringValue(msg.Body))
			err = i.Grafeas.PutOccurrence(occurrence)
			if err != nil {
				i.Logger.Error(err)
			}

			delReq := svc.DeleteMessageRequest(&sqs.DeleteMessageInput{
				QueueUrl:      aws.String(i.queueURL),
				ReceiptHandle: msg.ReceiptHandle,
			})
			_, err = delReq.Send(context.TODO())
			if err != nil {
				i.Logger.Error(err)
			}
		}
	}
}

func (i *ecrIngester) newOccurrence(body string) *grafeas.V1beta1Occurrence {
	// TODO: convert to grafeas occurrence
	i.Logger.Debugf("Got message: %s", body)
	return &grafeas.V1beta1Occurrence{}
}
