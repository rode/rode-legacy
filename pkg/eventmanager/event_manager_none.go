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

func (emn *eventManagerNone) Initialize(_ string) error {
	emn.log.V(1).Info("Using dummy event manager. Skipping initialize")
	return nil
}

func (emn *eventManagerNone) PublishAttestation(_ string, _ *grafeas.Occurrence) error {
	emn.log.V(1).Info("Using dummy event manager. No message will be published")
	return nil
}

func (emn *eventManagerNone) PublishPublicKey(_ string, _ []byte) error {
	emn.log.V(1).Info("Using dummy event manager. No message will be published")
	return nil
}

func (emn *eventManagerNone) Subscribe(_ string) error {
	emn.log.V(1).Info("Using dummy event manager. No message will be received")
	return nil
}

func (emn *eventManagerNone) Unsubscribe(_ string) error {
	return nil
}
