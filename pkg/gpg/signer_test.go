package gpg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSigner(t *testing.T) {
	assert := assert.New(t)

	signer, err := NewSigner("foo")
	assert.NoError(err)
	assert.NotNil(signer)

	message := "hello world!"
	signedMessage, err := signer.Sign(message)
	assert.NoError(err)

	verifiedMessage, err := signer.Verify(signedMessage)
	assert.NoError(err)
	assert.Equal(message, verifiedMessage)

	_, err = signer.Verify("foobar")
	assert.Error(err)

	keyID := signer.KeyID()
	assert.NotEmpty(keyID)
}
