package collector

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/liatrio/rode/pkg/occurrence"
)

type testCollector struct {
	logger      logr.Logger
	testMessage string
}

func NewTestCollector(logger logr.Logger, testMessage string) Collector {
	return &testCollector{
		logger:      logger,
		testMessage: testMessage,
	}
}

func (t *testCollector) Reconcile(ctx context.Context) error {
	t.logger.Info("reconciling test collector")

	return nil
}

func (t *testCollector) Start(ctx context.Context, stopChan chan interface{}, occurrenceCreator occurrence.Creator) error {
	go func() {
		for range time.NewTicker(5 * time.Second).C {
			select {
			case <-ctx.Done():
				stopChan <- true
				return
			default:
				t.logger.Info(t.testMessage)
			}
		}

		t.logger.Info("test collector goroutine finished")
	}()

	return nil
}

func (t *testCollector) Destroy(ctx context.Context) error {
	t.logger.Info("destroying test collector")

	return nil
}
