package attester

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/rand"
)

var _ = Context("signer", func() {
	var (
		signer Signer
		signerName string
		message string
	)

	BeforeEach(func() {
		signerName = fmt.Sprintf("signer-%s", rand.String(10))
		message = fmt.Sprintf("message-%s", rand.String(10))
		var err error
		signer, err = NewSigner(signerName)
		Expect(err).To(BeNil())
	})

	It("should sign messages without error", func() {
		_, err := signer.Sign(message)

		Expect(err).To(BeNil())
	})

	It("should verify signed messages", func() {
		signedMessage, err := signer.Sign(message)
		Expect(err).To(BeNil())

		verifiedMessage, err := signer.Verify(signedMessage)

		Expect(err).To(BeNil())
		Expect(verifiedMessage).To(Equal(message))
	})

	It("should fail to verify unsigned messages", func() {
		_, err := signer.Verify(rand.String(10))

		Expect(err).NotTo(BeNil())
	})

	It("should return the keyID", func() {
		Expect(signer.KeyID()).NotTo(BeEmpty())
	})
})
