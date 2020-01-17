package attester

import (
	"context"
	"fmt"

	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	"github.com/liatrio/rode/pkg/occurrence"
	"go.uber.org/zap"
)

type attestWrapper struct {
	logger *zap.SugaredLogger

	// delegate for creating occurrences.  used to create initial occcurences as well as the attestations.
	occurrenceCreator occurrence.Creator

	// list of attesters
	attesters []Attester

	// used to retieve all occurrences for a resource
	occurrenceLister occurrence.Lister
}

// NewAttestWrapper creates an Creator that also performs attestation
func NewAttestWrapper(logger *zap.SugaredLogger, delegate occurrence.Creator, lister occurrence.Lister, attesters []Attester) occurrence.Creator {
	return &attestWrapper{
		logger,
		delegate,
		attesters,
		lister,
	}
}

// CreateOccurrences will attempt attestation
func (a *attestWrapper) CreateOccurrences(ctx context.Context, occurrences ...*grafeas.Occurrence) error {
	if occurrences == nil || len(occurrences) == 0 {
		return nil
	}
	// call the delegate
	err := a.occurrenceCreator.CreateOccurrences(ctx, occurrences...)
	if err != nil {
		return err
	}

	// perform attestations for each distinct resource
	visited := make(map[string]bool)
	for _, o := range occurrences {
		uri := o.Resource.Uri
		if !visited[uri] {
			visited[uri] = true

			// fetch existing occurrences for this resource
			allOccurrences, err := a.occurrenceLister.ListOccurrences(ctx, uri)
			if err != nil {
				return fmt.Errorf("Unable to attempt attestation for occurrence %v", err)
			}

			for _, att := range a.attesters {
				resp, err := att.Attest(ctx, &AttestRequest{
					ResourceURI: uri,
					Occurrences: allOccurrences.GetOccurrences(),
				})
				if err != nil {
					a.logger.Warnf("Unable to perform attestation for occurrence %v", err)
				} else {
					a.logger.Infof("Storing attestation for resource '%s'", uri)
					err = a.occurrenceCreator.CreateOccurrences(ctx, resp.Attestation)
				}

				if err != nil {
					return fmt.Errorf("Unable to store attestation for occurrence %v", err)
				}
			}
		}
	}

	return nil
}
