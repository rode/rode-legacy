package eventmanager

import (
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
)

type EventManager interface {
	Publish(attesterName string, occurrence *grafeas.Occurrence) error
	Subscribe(attesterName string) error
	Unsubscribe(attesterName string) error
}
