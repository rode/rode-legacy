package collector

import (
	"context"
	"github.com/liatrio/rode/pkg/occurrence"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
)

// Collector converts events to occurrences
type Collector interface {
	// Reconcile handles creating and updating any external resources that are required for your collector to function
	// properly. Reconcile should be idempotent.
	// The `name` parameter can be used to help provide names for the external resources managed by Reconcile.
	// Example: the ECR collector will use the Reconcile function to create and update SQS queues and CloudWatch events
	Reconcile(ctx context.Context, name types.NamespacedName) error

	// Destroy handles the deletion of resources that were created in the Reconcile function
	Destroy(ctx context.Context, name types.NamespacedName) error

	// Type returns the type of this collector
	Type() string
}

// WebhookCollector receives events as HTTP payloads and converts them to occurrences
type WebhookCollector interface {
	// HandleWebhook handles a given HTTP request for this collector and converts it into occurrences using the provided
	// `occurrenceCreator`
	HandleWebhook(writer http.ResponseWriter, request *http.Request, occurrenceCreator occurrence.Creator)
}

type StartableCollector interface {
	// Start handles the logic required for the collector to receive events and create occurrences using the provided
	// `occurrenceCreator`
	Start(ctx context.Context, stopChan chan interface{}, occurrenceCreator occurrence.Creator) error
}
