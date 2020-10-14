package attester

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	"github.com/liatrio/rode/pkg/eventmanager"
	"github.com/liatrio/rode/pkg/occurrence"
)

// Lister is an interface for listing Attesters
type Lister interface {
	ListAttesters() map[string]Attester
}

type attestWrapper struct {
	log logr.Logger

	// delegate for creating occurrences.  used to create initial occurrences as well as the attestations.
	occurrenceCreator occurrence.Creator

	// list of attesters
	attesterLister Lister

	// used to retrieve all occurrences for a resource
	occurrenceLister occurrence.Lister

	// Used to publish attestation events to event streamer
	aem eventmanager.EventManager
}

// NewAttestWrapper creates an Creator that also performs attestation
func NewAttestWrapper(log logr.Logger, delegate occurrence.Creator, lister occurrence.Lister, attesterLister Lister, attEventManager eventmanager.EventManager) occurrence.Creator {
	return &attestWrapper{
		log,
		delegate,
		attesterLister,
		lister,
		attEventManager,
	}
}

// CreateOccurrences will attempt attestation
func (a *attestWrapper) CreateOccurrences(ctx context.Context, occurrences ...*grafeas.Occurrence) error {
	if len(occurrences) == 0 {
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
				return fmt.Errorf("unable to attempt attestation for occurrence %v", err)
			}

			for _, att := range a.attesterLister.ListAttesters() {
				resp, err := att.Attest(ctx, &AttestRequest{
					ResourceURI: uri,
					Occurrences: allOccurrences,
				})
				if err != nil {
					if vErr, ok := err.(ViolationError); ok {
						a.log.Info("Attestion resulted in violations", "violations", vErr.Violations)
					} else {
						return fmt.Errorf("unable to perform attestation for occurrence %v", err)
					}
				} else {
					a.log.Info("Storing attestation for resource", "uri", uri)
					err = a.occurrenceCreator.CreateOccurrences(ctx, resp.Attestation)
					if err != nil {
						return fmt.Errorf("unable to store attestation for occurrence %v", err)
					}
					err = a.aem.PublishAttestation(att.Name(), resp.Attestation)
					if err != nil {
						return fmt.Errorf("unable to publish attestation event: %s", err)
					}
				}
			}
		}
	}

	return nil
}
