package enforcer

import (
	"context"
	"fmt"

	"github.com/liatrio/rode/pkg/occurrence"

	"github.com/liatrio/rode/pkg/attester"

	"go.uber.org/zap"
)

// Enforcer enforces attestations on a resource
type Enforcer interface {
	Enforce(ctx context.Context, namespace string, resourceURI string) error
}

type enforcer struct {
	logger           *zap.SugaredLogger
	excludeNS        []string
	attesters        []attester.Attester
	occurrenceLister occurrence.Lister
}

// NewEnforcer creates an enforcer
func NewEnforcer(logger *zap.SugaredLogger, excludeNS []string, attesters []attester.Attester, occurrenceLister occurrence.Lister) Enforcer {
	return &enforcer{
		logger,
		excludeNS,
		attesters,
		occurrenceLister,
	}
}

func (e *enforcer) Enforce(ctx context.Context, namespace string, resourceURI string) error {
	for _, ns := range e.excludeNS {
		if namespace == ns {
			// skip - this namespace is excluded
			return nil
		}
	}

	e.logger.Debugf("About to enforce resource '%s' in namespace '%s'", resourceURI, namespace)

	// TODO: load namespace to look for annotations describing which attesters to enforce

	occurrenceList, err := e.occurrenceLister.ListOccurrences(ctx, resourceURI)
	if err != nil {
		return err
	}
	for _, att := range e.attesters {
		attested := false
		for _, occ := range occurrenceList.GetOccurrences() {
			req := &attester.VerifyRequest{
				Occurrence: occ,
			}
			if err = att.Verify(ctx, req); err == nil {
				attested = true
				break
			}
		}

		if !attested {
			return fmt.Errorf("Unable to find an attestation for %s", att)
		}
	}

	return nil
}
