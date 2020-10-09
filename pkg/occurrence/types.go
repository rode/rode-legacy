package occurrence

import (
	"context"

	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
)

//go:generate mockgen -destination=../../mocks/pkg/occurrence_mock/types.go -package=occurrence_mock . Lister,Creator

// Lister implements the listing of occurrences
type Lister interface {
	ListOccurrences(ctx context.Context, resourceURI string) ([]*grafeas.Occurrence, error)
	ListAttestations(ctx context.Context, resourceURI string) ([]*grafeas.Occurrence, error)
}

// Creator implements the creation of new occurrences
type Creator interface {
	CreateOccurrences(context.Context, ...*grafeas.Occurrence) error
}
