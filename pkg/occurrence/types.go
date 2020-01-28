package occurrence

import (
	"context"

	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
)

// Lister implements the listing of occurrences
type Lister interface {
	ListOccurrences(context.Context, string) (*grafeas.ListOccurrencesResponse, error)
}

// Creator implements the creation of new occurrences
type Creator interface {
	CreateOccurrences(context.Context, ...*grafeas.Occurrence) error
}
