package attestEventManager

import (
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
)

type AttestEventManager interface {
  Publish(attesterName string, occurrence *grafeas.Occurrence) error
}
