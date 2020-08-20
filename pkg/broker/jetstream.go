package broker

import (
  nats "github.com/nats-io/nats.go"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
)

type jetstreamClient struct {
  url string
}

func (c *jetstreamClient) new() (*nats.Conn, error) {
  nc, err := nats.Connect(c.url)
  if err != nil {
    return nil, err
  }
  return nc, nil
}

func (c *jetstreamClient) Publish(occurrence *grafeas.Occurrence) {
  nc, err := c.new()
  if err != nil {
    // do something with the error
  }

  nc.Publish("test-subject", nil)

}
