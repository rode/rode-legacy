package attester

import (
	"bytes"
	"crypto"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"

	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
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
	VerifyAttestation(*grafeas.Occurrence) error
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

type SignerList interface {
	Add(string, Signer)
	Get(string) (Signer, error)
	GetAll() map[string]Signer
}

type signerList struct {
	signers map[string]Signer
}

func NewSignerList() SignerList {
	return &signerList{}
}

func (sl *signerList) Add(attesterName string, signer Signer) {
	sl.signers[attesterName] = signer
}

func (sl *signerList) Get(attesterName string) (Signer, error) {
	if signer, ok := sl.signers[attesterName]; ok == true {
		return signer, nil
	}
	return nil, fmt.Errorf("no signer for attester '%s'", attesterName)
}

func (sl *signerList) GetAll() map[string]Signer {
	return sl.signers
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
	var entities openpgp.EntityList = []*openpgp.Entity{
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

func (s *signer) VerifyAttestation(occurrence *grafeas.Occurrence) error {
	if occurrence == nil || occurrence.GetAttestation() == nil {
		return fmt.Errorf("occurrence is not an attestation")
	}

	if s.KeyID() != occurrence.GetAttestation().GetAttestation().GetPgpSignedAttestation().GetPgpKeyId() {
		return fmt.Errorf("invalid keyID")
	}
	body, err := s.Verify(occurrence.GetAttestation().GetAttestation().GetPgpSignedAttestation().GetSignature())
	if err != nil {
		return err
	}
	if body != occurrence.GetResource().GetUri() {
		return fmt.Errorf("signature body doesn't match")
	}
	return nil
}

func (s *signer) KeyID() string {
	return s.entity.PrimaryKey.KeyIdString()
}

func (s *signer) Serialize(out io.Writer) error {
	return s.entity.SerializePrivate(out, nil)
}
