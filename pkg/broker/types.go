package broker

import (
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
)

type Broker interface {
  Publish(occurence *grafeas.Occurrence)
}
