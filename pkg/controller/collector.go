package controller

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/liatrio/rode/pkg/collector"
	"github.com/liatrio/rode/pkg/occurrence"
	"go.uber.org/zap"
)

// StartCollectors monitors the collectors
func StartCollectors(ctx context.Context, logger *zap.SugaredLogger, awsConfig *aws.Config, occurrenceCreator occurrence.Creator) error {
	// TODO: monitor kubernetes CR for collectors
	queueName := "rode-ecr-event-collector"
	c := collector.NewEcrEventCollector(logger, awsConfig, occurrenceCreator, queueName)
	logger.Debugf("Adding collector '%s'", "ecr_events")

	go func() {
		for ctx.Err() == nil {
			err := c.Reconcile(ctx)
			if err != nil {
				logger.Errorf("Error reconciling collector %v", err)
			}

			time.Sleep(15 * time.Second)
		}
	}()

	return nil
}
