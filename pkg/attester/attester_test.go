package attester

import (
	"context"
	"fmt"
	"testing"

	discovery "github.com/grafeas/grafeas/proto/v1beta1/discovery_go_proto"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	"github.com/stretchr/testify/assert"
)

func TestClient_Attest(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	name := "testAttester"
	att := createAttester(name)

	attestRequest := &AttestRequest{
		ResourceURI: name,
		Occurrences: []*grafeas.Occurrence{
			{
				Resource: &grafeas.Resource{
					Uri: name,
				},
				NoteName: fmt.Sprintf("projects/rode/notes/%s", name),
				Details: &grafeas.Occurrence_Discovered{
					Discovered: &discovery.Details{
						Discovered: &discovery.Discovered{
							AnalysisStatus: discovery.Discovered_FINISHED_SUCCESS,
						},
					},
				},
			},
		},
	}

	res, err := att.Attest(ctx, attestRequest)
	assert.NoError(err)

	assert.NotNil(res)
}

func TestClient_Verify(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	name := "testAttester"
	att := createAttester(name)

	attestRequest := &AttestRequest{
		ResourceURI: name,
		Occurrences: []*grafeas.Occurrence{
			{
				Resource: &grafeas.Resource{
					Uri: name,
				},
				NoteName: fmt.Sprintf("projects/rode/notes/%s", name),
				Details: &grafeas.Occurrence_Discovered{
					Discovered: &discovery.Details{
						Discovered: &discovery.Discovered{
							AnalysisStatus: discovery.Discovered_FINISHED_SUCCESS,
						},
					},
				},
			},
		},
	}

	res, err := att.Attest(ctx, attestRequest)
	assert.NoError(err)
	assert.NotNil(res)

	req := &VerifyRequest{res.Attestation}

	err = att.Verify(ctx, req)
	assert.NoError(err)

}

/*
func TestClient_addOccurrence(t *testing.T) {

}

*/
func createAttester(name string) Attester {
	policyModule := `
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
		severityCount("HIGH") > 0
	}
	severityCount(severity) = cnt {
		cnt := count([v | v := input.occurrences[_].vulnerability.severity; v == severity])
	}
	`
	policy, err := NewPolicy(name, policyModule, true)
	if err != nil {
		panic(err)
	}

	signer, err := NewSigner(name)
	if err != nil {
		panic(err)
	}

	return NewAttester(name, policy, signer)
}
