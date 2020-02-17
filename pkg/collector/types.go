package collector

import (
	"context"
	"github.com/liatrio/rode/pkg/occurrence"
	"net/http"
)

// Collector converts events to occurrences
type Collector interface {
	// Reconcile handles creating and updating any external resources that are required for your collector to function
	// properly. Reconcile should be idempotent.
	// Example: the ECR collector will use the Reconcile function to create and update SQS queues and CloudWatch events
	Reconcile(ctx context.Context) error

	// Destroy handles the deletion of resources that were created in the Reconcile function
	Destroy(ctx context.Context) error

	// Start handles the logic required for the collector to receive events and create occurrences using the provided
	// `occurrenceCreator`
	Start(ctx context.Context, stopChan chan interface{}, occurrenceCreator occurrence.Creator) error

	// Type returns the type of this collector
	Type() string
}

// WebhookCollector receives events as HTTP payloads and converts them to occurrences
type WebhookCollector interface {
	// HandleWebhook handles a given HTTP request for this collector and converts it into occurrences
	HandleWebhook(writer http.ResponseWriter, request *http.Request)
}
