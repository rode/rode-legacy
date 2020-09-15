package attesteventmanager

import (
	"bytes"
	"encoding/json"
	"time"

	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	jsm "github.com/nats-io/jsm.go"
	nats "github.com/nats-io/nats.go"
)

type JetstreamClient struct {
	URL string
}

func (c *JetstreamClient) new() (*nats.Conn, error) {
	nc, err := nats.Connect(c.URL)
	if err != nil {
		return nil, err
	}
	return nc, nil
}

func (c *JetstreamClient) Publish(attesterName string, occurrence *grafeas.Occurrence) error {
	nc, err := c.new()
	if err != nil {
		return err
	}

	_, err = jsm.LoadOrNewStream("ATTESTATION", jsm.Subjects("ATTESTATION.*"), jsm.StreamConnection(jsm.WithConnection(nc)), jsm.MaxAge(24*365*time.Hour), jsm.FileStorage())
	if err != nil {
		return err
	}

	subSubject := "ATTESTATION." + attesterName

	occurrenceBytes := new(bytes.Buffer)
	err = json.NewEncoder(occurrenceBytes).Encode(occurrence)
	if err != nil {
		return err
	}

	return nc.Publish(subSubject, occurrenceBytes.Bytes())
}
