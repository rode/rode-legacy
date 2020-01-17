package collector

import "context"

// Collector converts events to occurrences
type Collector interface {
	Reconcile(ctx context.Context) error
}
