package attester

import (
	"context"
	"fmt"
	"io"
	"testing"

	discovery "github.com/grafeas/grafeas/proto/v1beta1/discovery_go_proto"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/rand"
)

var (
	attesterName string
	ctx          = context.Background()
)

func TestAttester_AttestBadSigner(t *testing.T) {
	assert := assert.New(t)

	attesterName = fmt.Sprintf("attester%s", rand.String(10))

	policyModule := `
	package default_attester
	violation[{"msg":"analysis failed"}]{
		input.occurrences[_].discovered.discovered.analysisStatus != "FINISHED_SUCCESS"
	}
	`
	att := createAttester(attesterName, policyModule, true)

	attestRequest := &AttestRequest{
		ResourceURI: attesterName,
		Occurrences: []*grafeas.Occurrence{
			{
				Resource: &grafeas.Resource{
					Uri: attesterName,
				},
				NoteName: fmt.Sprintf("projects/rode/notes/%s", attesterName),
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

	_, err := att.Attest(ctx, attestRequest)
	assert.Error(err)
}

func TestAttester_AttestValid(t *testing.T) {
	assert := assert.New(t)

	attesterName = fmt.Sprintf("attester%s", rand.String(10))

	policyModule := `
	package default_attester
	violation[{"msg":"analysis failed"}]{
		input.occurrences[_].discovered.discovered.analysisStatus != "FINISHED_SUCCESS"
	}
	`
	att := createAttester(attesterName, policyModule, false)

	attestRequest := &AttestRequest{
		ResourceURI: attesterName,
		Occurrences: []*grafeas.Occurrence{
			{
				Resource: &grafeas.Resource{
					Uri: attesterName,
				},
				NoteName: fmt.Sprintf("projects/rode/notes/%s", attesterName),
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
	assert.NotNil(res.Attestation)
	assert.Equal(res.Attestation.NoteName, fmt.Sprintf("projects/rode/notes/%s", attesterName))
	assert.Equal(res.Attestation.Resource.Uri, attesterName)
}

func TestAtester_VerifyBadOccurrence(t *testing.T) {
	assert := assert.New(t)

	attesterName = fmt.Sprintf("attester%s", rand.String(10))

	policyModule := `
	package default_attester
	violation[{"msg":"analysis failed"}]{
		input.occurrences[_].discovered.discovered.analysisStatus != "FINISHED_SUCCESS"
	}
	`
	att := createAttester(attesterName, policyModule, false)

	req := &VerifyRequest{Occurrence: nil}

	err := att.Verify(ctx, req)
	assert.Error(err)
}

func TestAttester_VerifyBadKey(t *testing.T) {
	assert := assert.New(t)

	attesterName = fmt.Sprintf("attester%s", rand.String(10))

	policyModule := `
	package default_attester
	violation[{"msg":"analysis failed"}]{
		input.occurrences[_].discovered.discovered.analysisStatus != "FINISHED_SUCCESS"
	}
	`
	att := createAttester(attesterName, policyModule, false)

	attestRequest := &AttestRequest{
		ResourceURI: attesterName,
		Occurrences: []*grafeas.Occurrence{
			{
				Resource: &grafeas.Resource{
					Uri: attesterName,
				},
				NoteName: fmt.Sprintf("projects/rode/notes/%s", attesterName),
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

	newAttester := createAttester(attesterName, policyModule, false)
	req := &VerifyRequest{res.Attestation}
	err = newAttester.Verify(ctx, req)
	assert.Error(err)
}

func TestAttester_VerifyValid(t *testing.T) {
	assert := assert.New(t)

	attesterName = fmt.Sprintf("attester%s", rand.String(10))

	policyModule := `
	package default_attester
	violation[{"msg":"analysis failed"}]{
		input.occurrences[_].discovered.discovered.analysisStatus != "FINISHED_SUCCESS"
	}
	`
	att := createAttester(attesterName, policyModule, false)

	attestRequest := &AttestRequest{
		ResourceURI: attesterName,
		Occurrences: []*grafeas.Occurrence{
			{
				Resource: &grafeas.Resource{
					Uri: attesterName,
				},
				NoteName: fmt.Sprintf("projects/rode/notes/%s", attesterName),
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

	req := &VerifyRequest{res.Attestation}

	err = att.Verify(ctx, req)
	assert.NoError(err)
}

func createAttester(attesterName string, policyModule string, badSigner bool) Attester {
	policy, err := NewPolicy(attesterName, policyModule, true)
	if err != nil {
		panic(err)
	}

	if badSigner {
		signer := &FakeSigner{name: attesterName}
		return NewAttester(attesterName, policy, signer)
	}

	signer, err := NewSigner(attesterName)
	if err != nil {
		panic(err)
	}

	return NewAttester(attesterName, policy, signer)
}

type FakeSigner struct {
	name string
}

func (s *FakeSigner) Sign(message string) (string, error) {
	return s.name, fmt.Errorf("invalid signer")
}

func (s *FakeSigner) Verify(string) (string, error) {
	return s.name, fmt.Errorf("invalid signer")
}

func (s *FakeSigner) KeyID() string {
	return s.name
}

func (s *FakeSigner) Serialize(out io.Writer) error {
	out.Write([]byte(s.name))
	return fmt.Errorf("invalid signer")
}
