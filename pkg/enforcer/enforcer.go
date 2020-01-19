package enforcer

import (
	"context"
	"fmt"
	"strings"

	"github.com/liatrio/rode/pkg/occurrence"

	"github.com/liatrio/rode/pkg/attester"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
	clientset        *kubernetes.Clientset
}

// NewEnforcer creates an enforcer
func NewEnforcer(logger *zap.SugaredLogger, excludeNS []string, attesters []attester.Attester, occurrenceLister occurrence.Lister, clientset *kubernetes.Clientset) Enforcer {
	return &enforcer{
		logger,
		excludeNS,
		attesters,
		occurrenceLister,
		clientset,
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

	// Begin: Determine enforced attesters
	result, err := e.clientset.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Unable to get namespace: %v", err)
	}
	resultAnnotations := result.ObjectMeta.Annotations
	if resultAnnotations == nil {
		return nil
	}
	enforcedAttesters := strings.SplitN(result.ObjectMeta.Annotations["rode.liatr.io/enforce-attesters"],",", -1)
	enforcedAttestersMap := make(map[string]bool)
	for _, att := range enforcedAttesters {
		enforcedAttestersMap[att] = true
	}
	// End: Determine enforced attesters
	occurrenceList, err := e.occurrenceLister.ListOccurrences(ctx, resourceURI)
	if err != nil {
		return err
	}
	for _, att := range e.attesters {
		if !enforcedAttestersMap[att.String()] && !enforcedAttestersMap["*"] {
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
