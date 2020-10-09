package attester_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/liatrio/rode/pkg/attester"
	"k8s.io/apimachinery/pkg/util/rand"

	discovery "github.com/grafeas/grafeas/proto/v1beta1/discovery_go_proto"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Context("attester", func() {
	var (
		attesterName  string
		noteName      string
		policyModule  string
		attestRequest *attester.AttestRequest
		ctx           context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		attesterName = fmt.Sprintf("attester%s", rand.String(10))
		noteName = fmt.Sprintf("projects/rode/notes/%s", attesterName)
		policyModule = `
	package default_attester
	violation[{"msg":"analysis failed"}]{
		input.occurrences[_].discovered.discovered.analysisStatus != "FINISHED_SUCCESS"
	}
	`

		attestRequest = &attester.AttestRequest{
			ResourceURI: attesterName,
			Occurrences: []*grafeas.Occurrence{
				{
					Resource: &grafeas.Resource{
						Uri: attesterName,
					},
					NoteName: noteName,
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

	})

	When("the attestation fails to sign", func() {
		It("should return the signer error", func() {
			att, err := createAttester(attesterName, policyModule, true)
			Expect(err).ToNot(HaveOccurred())

			attestResponse, err := att.Attest(ctx, attestRequest)
			Expect(err).To(HaveOccurred())
			Expect(attestResponse).To(BeNil())
		})
	})

	When("the attestation is successfully signed", func() {
		It("should return the attestation response", func() {
			att, err := createAttester(attesterName, policyModule, false)
			Expect(err).ToNot(HaveOccurred())

			attestResponse, err := att.Attest(ctx, attestRequest)
			Expect(err).To(BeNil())
			Expect(attestResponse).ToNot(BeNil())
			Expect(attestResponse.Attestation.NoteName).To(Equal(noteName))
			Expect(attestResponse.Attestation.Resource.Uri).To(Equal(attesterName))
		})
	})

	When("the occurrence is nil", func() {
		It("should fail the verification", func() {
			att, err := createAttester(attesterName, policyModule, false)
			Expect(err).ToNot(HaveOccurred())

			verifyRequest := &attester.VerifyRequest{Occurrence: nil}

			err = att.Verify(ctx, verifyRequest)
			Expect(err).To(HaveOccurred())
		})
	})

	When("the attestation key does not match", func() {
		It("should fail to verify", func() {
			att, err := createAttester(attesterName, policyModule, false)
			Expect(err).ToNot(HaveOccurred())

			response, err := att.Attest(ctx, attestRequest)
			Expect(err).ToNot(HaveOccurred())

			newAttester, err := createAttester(attesterName, policyModule, false)
			Expect(err).ToNot(HaveOccurred())

			req := &attester.VerifyRequest{response.Attestation}
			err = newAttester.Verify(ctx, req)

			Expect(err).To(HaveOccurred())
		})
	})

	When("the attestation is valid", func() {
		It("should pass verification", func() {
			att, err := createAttester(attesterName, policyModule, false)
			Expect(err).ToNot(HaveOccurred())

			response, err := att.Attest(ctx, attestRequest)
			Expect(err).ToNot(HaveOccurred())

			verifyRequest := &attester.VerifyRequest{response.Attestation}

			err = att.Verify(ctx, verifyRequest)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the attestation is printed", func() {
		It("should display the struct name and signer info", func() {
			policy, err := attester.NewPolicy(attesterName, policyModule, true)
			Expect(err).ToNot(HaveOccurred())

			signer, err := attester.NewSigner(attesterName)
			Expect(err).ToNot(HaveOccurred())

			att := attester.NewAttester(attesterName, policy, signer)

			expectedAttestation := strings.Join([]string{
				"Attester",
				attesterName,
				"(" + signer.String() + ")",
			}, " ")
			Expect(err).ToNot(HaveOccurred())

			Expect(att.String()).To(Equal(expectedAttestation))
		})
	})

	When("the name getter is called", func() {
		It("should return the attester name", func() {
			att, err := createAttester(attesterName, policyModule, false)
			Expect(err).ToNot(HaveOccurred())

			Expect(att.Name()).To(Equal(attesterName))
		})
	})

	When("the key id getter is called", func() {
		It("should return the signer's keyID", func() {
			policy, err := attester.NewPolicy(attesterName, policyModule, true)
			Expect(err).ToNot(HaveOccurred())

			signer, err := attester.NewSigner(attesterName)
			Expect(err).ToNot(HaveOccurred())

			att := attester.NewAttester(attesterName, policy, signer)

			Expect(att.KeyID()).To(Equal(signer.KeyID()))
		})
	})

	When("an attester list is created", func() {
		var attesterList *attester.List

		BeforeEach(func() {
			attesterList = attester.NewList()
		})

		It("should be returned", func() {
			Expect(attesterList).ToNot(BeNil())
		})

		It("should allow for attesters to be added", func() {
			att, err := createAttester(attesterName, policyModule, false)
			Expect(err).ToNot(HaveOccurred())

			attesterList.Add(att)
			attesterFromList, exists := attesterList.Get(attesterName)

			Expect(exists).To(BeTrue())
			Expect(attesterFromList.Name()).To(Equal(att.Name()))
			Expect(attesterFromList.KeyID()).To(Equal(att.KeyID()))
		})

		It("should allow for all attesters to be fetched at once", func() {
			att, err := createAttester(attesterName, policyModule, false)
			Expect(err).ToNot(HaveOccurred())

			otherAttesterName := fmt.Sprintf("attester%s", rand.String(10))
			otherAtt, err := createAttester(otherAttesterName, policyModule, false)
			Expect(err).ToNot(HaveOccurred())
			expectedNumberOfAttesters := 2

			attesterList.Add(att)
			attesterList.Add(otherAtt)

			list := attesterList.GetAll()

			Expect(len(list)).To(Equal(expectedNumberOfAttesters))
			Expect(list[attesterName]).NotTo(BeNil())
			Expect(list[otherAttesterName]).NotTo(BeNil())
		})

		It("should allow for attesters to be removed", func() {
			att, err := createAttester(attesterName, policyModule, false)
			Expect(err).ToNot(HaveOccurred())

			attesterList.Add(att)
			attesterList.Remove(attesterName)
			_, exists := attesterList.Get(attesterName)

			Expect(exists).To(BeFalse())
			Expect(len(attesterList.GetAll())).To(Equal(0))
		})

		It("should be able to find an attester by key ID", func() {
			policy, err := attester.NewPolicy(attesterName, policyModule, true)
			Expect(err).ToNot(HaveOccurred())

			signer, err := attester.NewSigner(attesterName)
			Expect(err).ToNot(HaveOccurred())

			att := attester.NewAttester(attesterName, policy, signer)

			otherAttester, err := createAttester(fmt.Sprintf("attester%s", rand.String(10)), policyModule, false)
			Expect(err).ToNot(HaveOccurred())

			attesterList.Add(att)
			attesterList.Add(otherAttester)

			attestersByKeyID := attesterList.FindByKeyID(signer.KeyID())

			Expect(len(attestersByKeyID.GetAll())).To(Equal(1))
		})

		It("should be able to find multiple attesters with the same key ID", func() {
			policy, err := attester.NewPolicy(attesterName, policyModule, true)
			Expect(err).ToNot(HaveOccurred())

			signer, err := attester.NewSigner(attesterName)
			Expect(err).ToNot(HaveOccurred())

			att := attester.NewAttester(attesterName, policy, signer)
			otherAttester := attester.NewAttester(fmt.Sprintf("attester%s", rand.String(10)), policy, signer)
			Expect(err).ToNot(HaveOccurred())

			thirdAttester, err := createAttester(fmt.Sprintf("attester%s", rand.String(10)), policyModule, false)

			attesterList.Add(att)
			attesterList.Add(otherAttester)
			attesterList.Add(thirdAttester)

			attestersByKeyID := attesterList.FindByKeyID(signer.KeyID())

			Expect(len(attestersByKeyID.GetAll())).To(Equal(2))
			_, exists := attestersByKeyID.Get(attesterName)
			Expect(exists).To(BeTrue())
			_, exists = attestersByKeyID.Get(otherAttester.Name())
			Expect(exists).To(BeTrue())
		})
	})
})

func createAttester(attesterName string, policyModule string, badSigner bool) (attester.Attester, error) {
	policy, err := attester.NewPolicy(attesterName, policyModule, true)
	if err != nil {
		return nil, err
	}

	if badSigner {
		signer := &FakeSigner{name: attesterName}
		return attester.NewAttester(attesterName, policy, signer), nil
	}

	signer, err := attester.NewSigner(attesterName)
	if err != nil {
		return nil, err
	}

	return attester.NewAttester(attesterName, policy, signer), nil
}

type FakeSigner struct {
	name string
}

func (s *FakeSigner) String() string {
	return s.name
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

func (s *FakeSigner) SerializeKeys() ([]byte, error) {
	return []byte{}, nil
}

func (s *FakeSigner) SerializePublicKey() ([]byte, error) {
	return []byte{}, nil
}
