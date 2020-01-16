package gpg

import (
	"bytes"
	"crypto"
	"encoding/base64"
	"io"
	"io/ioutil"

	"golang.org/x/crypto/openpgp/packet"

	"golang.org/x/crypto/openpgp"
)

type signer struct {
	entity *openpgp.Entity
}

// Signer is the interface for managing gpg signing
type Signer interface {
	Sign(string) (string, error)
	Verify(string) (string, error)
	KeyID() string
	Serialize(out io.Writer) error
}

// NewSigner creates a new signer
func NewSigner(name string) (Signer, error) {
	config := &packet.Config{
		DefaultHash: crypto.SHA256,
	}
	entity, err := openpgp.NewEntity(name, "", "", config)
	if err != nil {
		return nil, err
	}
	return &signer{
		entity,
	}, nil
}

// ReadSigner creates a signer from reader
func ReadSigner(in io.Reader) (Signer, error) {
	entity, err := openpgp.ReadEntity(packet.NewReader(in))
	if err != nil {
		return nil, err
	}
	return &signer{
		entity,
	}, nil
}

func (s *signer) Sign(message string) (string, error) {
	buf := new(bytes.Buffer)
	writer, err := openpgp.Sign(buf, s.entity, nil, nil)
	if err != nil {
		return "", err
	}
	_, err = writer.Write([]byte(message))
	if err != nil {
		return "", err
	}
	err = writer.Close()
	if err != nil {
		return "", err
	}

	signedBytes, err := ioutil.ReadAll(buf)
	if err != nil {
		return "", err
	}
	encStr := base64.StdEncoding.EncodeToString(signedBytes)
	return encStr, nil
}

func (s *signer) Verify(signedMessage string) (string, error) {
	var entities openpgp.EntityList
	entities = []*openpgp.Entity{
		s.entity,
	}

	signedBytes, err := base64.StdEncoding.DecodeString(signedMessage)
	if err != nil {
		return "", err
	}

	message, err := openpgp.ReadMessage(bytes.NewBuffer(signedBytes), entities, func(keys []openpgp.Key, symmetric bool) ([]byte, error) {
		return []byte(""), nil
	}, nil)
	if err != nil {
		return "", err
	}

	b, err := ioutil.ReadAll(message.UnverifiedBody)
	if err != nil {
		return "", err
	} else if message.SignatureError != nil {
		return "", message.SignatureError
	}
	return string(b), nil
}

func (s *signer) KeyID() string {
	return s.entity.PrimaryKey.KeyIdString()
}

func (s *signer) Serialize(out io.Writer) error {
	return s.entity.SerializePrivate(out, nil)
}
