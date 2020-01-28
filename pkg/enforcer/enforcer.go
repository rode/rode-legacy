package enforcer

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	"github.com/liatrio/rode/pkg/occurrence"

	"github.com/liatrio/rode/pkg/attester"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Enforcer enforces attestations on a resource
type Enforcer interface {
	Enforce(ctx context.Context, namespace string, resourceURI string) error
}

type enforcer struct {
	log              logr.Logger
	excludeNS        []string
	attesterLister   attester.Lister
	occurrenceLister occurrence.Lister
	client           client.Client
}

// NewEnforcer creates an enforcer
func NewEnforcer(log logr.Logger, excludeNS []string, attesterLister attester.Lister, occurrenceLister occurrence.Lister, c client.Client) Enforcer {
	return &enforcer{
		log,
		excludeNS,
		attesterLister,
		occurrenceLister,
		c,
	}
}

func (e *enforcer) Enforce(ctx context.Context, namespace string, resourceURI string) error {
	for _, ns := range e.excludeNS {
		if namespace == ns {
			// skip - this namespace is excluded
			return nil
		}
	}

	e.log.Info("About to enforce resource", resourceURI, namespace)

	// Begin: Determine enforced attesters
	// TODO: use different client to load namespace labels
	result, err := e.client.Get(ctx, "???", "???")
	if err != nil {
		return fmt.Errorf("Unable to get namespace: %v", err)
	}
	resultLabels := result.ObjectMeta.Labels
	if resultLabels == nil {
		return nil
	}
	enforcedAttesters := resultLabels["rode.liatr.io/enforce-attesters"]
	// End: Determine enforced attesters
	occurrenceList, err := e.occurrenceLister.ListOccurrences(ctx, resourceURI)
	if err != nil {
		return err
	}
	for _, att := range e.attesterLister.ListAttesters() {
		if enforcedAttesters != "*" && enforcedAttesters != att.String() {
			continue
		}
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
