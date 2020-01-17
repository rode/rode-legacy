package controller

import (
	"context"
	"fmt"

	"github.com/liatrio/rode/pkg/attester"
	"go.uber.org/zap"
)

const defaultName = "default_attester"
const defaultPolicyBody = `
package default_attester

violation[{"msg":"analysis failed"}]{
	input.occurrences[_].discovered.discovered.analysisStatus != "FINISHED_SUCCESS"
}
violation[{"msg":"analysis not performed"}]{
	analysisStatus := [s | s := input.occurrences[_].discovered.discovered.analysisStatus]
	count(analysisStatus) = 0
}
violation[{"msg":"critical vulnerability found"}]{
	severityCount("CRITICAL") > 0
}
violation[{"msg":"high vulnerability found"}]{
	severityCount("HIGH") > 10
}
severityCount(severity) = cnt {
	cnt := count([v | v := input.occurrences[_].vulnerability.severity; v == severity])
}
`

// StartAttesters monitors the attesters
func StartAttesters(ctx context.Context, logger *zap.SugaredLogger, opaTrace bool, attesters *[]attester.Attester) error {
	// TODO: monitor kubernetes CR for attesters
	defaultPolicy, err := attester.NewPolicy(defaultName, defaultPolicyBody, opaTrace)
	if err != nil {
		return fmt.Errorf("Unable to create policy %v", err)
	}
	defaultSigner, err := attester.NewSigner(defaultName)
	if err != nil {
		return fmt.Errorf("Unable to create signer %v", err)
	}
	defaultAttester := attester.NewAttester(defaultName, defaultPolicy, defaultSigner)
	logger.Debugf("Adding attester '%s'", defaultName)
	*attesters = append(*attesters, defaultAttester)
	return nil
}
