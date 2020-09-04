package attester

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	attestation "github.com/grafeas/grafeas/proto/v1beta1/attestation_go_proto"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
)

type attester struct {
	projectID string
	name      string
	policy    Policy
	signer    Signer
}

// NewAttester creates a new attester
func NewAttester(name string, policy Policy, signer Signer) Attester {
	return &attester{
		"rode",
		name,
		policy,
		signer,
	}
}

// Attester for performing attestation.  returns `ok` if attestation created
type Attester interface {
	Attest(ctx context.Context, req *AttestRequest) (*AttestResponse, error)
	Verify(ctx context.Context, req *VerifyRequest) error
	String() string
}

// AttestRequest contains request for attester
type AttestRequest struct {
	ResourceURI string
	Occurrences []*grafeas.Occurrence
}

// AttestResponse contains response from attester
type AttestResponse struct {
	Attestation *grafeas.Occurrence
}

// ViolationError is a slice of Violations
type ViolationError struct {
	Violations []*Violation
}

func (ve ViolationError) Error() string {
	return fmt.Sprintf("%v", ve.Violations)
}

func (a *attester) String() string {
	return a.name
}

// Attest takes a list of Occurrences and uses the Attester's policy to determine how many violations have occurred,
// if there are no violations then the function will then create an Attestation Occurrence, sign it, and then return it.
func (a *attester) Attest(ctx context.Context, req *AttestRequest) (*AttestResponse, error) {
	// prepare the input
	input := new(occurrenceInput)
	for _, o := range req.Occurrences {
		err := input.addOccurrence(o)
		if err != nil {
			return nil, err
		}
	}

	violations := a.policy.Evaluate(ctx, input)

	if len(violations) > 0 {
		return nil, ViolationError{violations}
	}

	sig, err := a.signer.Sign(req.ResourceURI)
	if err != nil {
		return nil, fmt.Errorf("Error signing resourceURI %v", err)
	}

	attestOccurrence := &grafeas.Occurrence{}
	attestOccurrence.NoteName = fmt.Sprintf("projects/%s/notes/%s", a.projectID, strings.ReplaceAll(a.name, "/", "."))
	attestOccurrence.Resource = &grafeas.Resource{Uri: req.ResourceURI}
	attestOccurrence.Details = &grafeas.Occurrence_Attestation{
		Attestation: &attestation.Details{
			Attestation: &attestation.Attestation{
				Signature: &attestation.Attestation_PgpSignedAttestation{
					PgpSignedAttestation: &attestation.PgpSignedAttestation{
						ContentType: attestation.PgpSignedAttestation_CONTENT_TYPE_UNSPECIFIED,
						Signature:   sig,
						KeyId: &attestation.PgpSignedAttestation_PgpKeyId{
							PgpKeyId: a.signer.KeyID(),
						},
					},
				},
			},
		},
	}

	return &AttestResponse{
		Attestation: attestOccurrence,
	}, nil
}

// VerifyRequest contains request for attester
type VerifyRequest struct {
	Occurrence *grafeas.Occurrence
}

func (a *attester) Verify(ctx context.Context, req *VerifyRequest) error {
	if req.Occurrence == nil || req.Occurrence.GetAttestation() == nil {
		return fmt.Errorf("Occurrence is not an attestation")
	}
	if a.signer.KeyID() != req.Occurrence.GetAttestation().GetAttestation().GetPgpSignedAttestation().GetPgpKeyId() {
		return fmt.Errorf("Invalid keyID")
	}
	body, err := a.signer.Verify(req.Occurrence.GetAttestation().GetAttestation().GetPgpSignedAttestation().GetSignature())
	if err != nil {
		return err
	}
	if body != req.Occurrence.GetResource().GetUri() {
		return fmt.Errorf("Signature body doesn't match")
	}
	return nil
}

type occurrenceInput struct {
	Occurrences []map[string]interface{} `json:"occurrences"`
}

func (oi *occurrenceInput) addOccurrence(occurrence *grafeas.Occurrence) error {
	if oi.Occurrences == nil {
		oi.Occurrences = make([]map[string]interface{}, 0)
	}

	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(occurrence)
	if err != nil {
		return err
	}

	occurrenceAsMap := make(map[string]interface{})
	err = json.Unmarshal(buf.Bytes(), &occurrenceAsMap)
	if err != nil {
		return err
	}

	oi.Occurrences = append(oi.Occurrences, occurrenceAsMap)
	return nil
}
