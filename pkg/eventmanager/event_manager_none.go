package eventmanager

import (
	"github.com/go-logr/logr"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
)

type eventManagerNone struct {
	log logr.Logger
}

func NewEventManagerNone(log logr.Logger) EventManager {
	return &eventManagerNone{log: log.WithName("eventManagerNone")}
}

func (emn *eventManagerNone) Initialize(attesterName string) error {
	emn.log.V(1).Info("Using dummy event manager. Skipping initialize")
	return nil
}

func (emn *eventManagerNone) Publish(attesterName string, occurrence *grafeas.Occurrence) error {
	emn.log.V(1).Info("Using dummy event manager. No message will by published")
	return nil
}

func (emn *eventManagerNone) Subscribe(attesterName string) error {
	emn.log.V(1).Info("Using dummy event manager. No message will by received")
	return nil
}

func (emn *eventManagerNone) Unsubscribe(attesterName string) error {
	return nil
}
