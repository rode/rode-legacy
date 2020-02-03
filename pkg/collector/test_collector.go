package collector

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/liatrio/rode/pkg/occurrence"
)

type testCollector struct {
	logger            logr.Logger
	occurrenceCreator occurrence.Creator
	testMessage       string
}

func NewTestCollector(logger logr.Logger, occurrenceCreator occurrence.Creator, testMessage string) Collector {
	return &testCollector{
		logger:            logger,
		occurrenceCreator: occurrenceCreator,
		testMessage:       testMessage,
	}
}

func (t *testCollector) Reconcile(ctx context.Context) error {
	t.logger.Info(t.testMessage)

	return nil
}
