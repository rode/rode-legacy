package eventmanager

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	grafeasAttestation "github.com/grafeas/grafeas/proto/v1beta1/attestation_go_proto"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"

	"github.com/go-logr/logr"
	"github.com/liatrio/rode/pkg/occurrence"
)

type JetstreamClient struct {
	streamsInitialized map[string]bool
	log                logr.Logger
	url                string
	CTX                context.Context
	OccurrenceCreator  occurrence.Creator
	Consumers          map[string]*JetstreamConsumer
}

func NewJetstreamClient(log logr.Logger, url string, occurrenceCreator occurrence.Creator) EventManager {
	return &JetstreamClient{
		log: log.WithName("JetstreamClient"),
		streamsInitialized: map[string]bool{
			"ATTESTATION":     false,
			"ATTESTATION_KEY": false,
		},
		url:               url,
		CTX:               context.Background(),
		OccurrenceCreator: occurrenceCreator,
		Consumers:         map[string]*JetstreamConsumer{},
	}
}

func (c *JetstreamClient) new() (*nats.Conn, error) {
	return nats.Connect(c.url)
}

func (c *JetstreamClient) Initialize(attesterName string) error {
	log := c.log.WithName("Initialize()").WithValues("attester", attesterName)

	var (
		nc  *nats.Conn
		err error
	)

	for streamName, initialized := range c.streamsInitialized {
		if initialized {
			continue
		}

		if nc == nil {
			nc, err = c.new()
			if err != nil {
				return err
			}
		}

		_, err = jsm.NewStream(
			streamName,
			jsm.Subjects(fmt.Sprintf("%s.*", streamName)),
			jsm.StreamConnection(jsm.WithConnection(nc)),
			jsm.MaxAge(24*365*time.Hour),
			jsm.FileStorage())

		if err != nil {
			return err
		}

		c.streamsInitialized[streamName] = true
		log.Info("Created stream", "streamName", streamName)
	}

	return nil
}

func (c *JetstreamClient) PublishAttestation(attesterName string, occurrence *grafeas.Occurrence) error {
	am := newAttestationMessage(*occurrence)
	return c.publish("ATTESTATION", attesterName, &am)
}

func (c *JetstreamClient) PublishPublicKey(attesterName string, publicKey []byte) error {
	message := struct {
		Base64PublicKey string `json:"base64PublicKey"`
	}{
		Base64PublicKey: base64.StdEncoding.EncodeToString(publicKey),
	}

	return c.publish("ATTESTATION_KEY", attesterName, &message)
}

func (c *JetstreamClient) publish(streamName, attesterName string, message interface{}) error {
	log := c.log.
		WithName("publish()").
		WithValues("attester", attesterName, "stream", streamName)

	nc, err := c.new()
	if err != nil {
		return err
	}
	_, err = jsm.LoadStream(streamName, jsm.WithConnection(nc))
	jsonMessage, err := json.Marshal(message)
	if err != nil {
		return err
	}
	subject := fmt.Sprintf("%s.%s", streamName, attesterName)
	log.Info("Publishing message", "subject", subject)

	return nc.Publish(subject, jsonMessage)
}

func (c *JetstreamClient) Subscribe(attester string) error {
	log := c.log.WithName("Subscribe()").WithValues("attester", attester)

	nc, err := c.new()
	if err != nil {
		return err
	}

	jetstreamConsumer, exists := c.Consumers[attester]
	if exists {
		log.Info("Found existing Jetstream Consumer")
	} else {
		log.Info("Creating new Jetstream Consumer")

		stream, err := jsm.LoadStream("ATTESTATION", jsm.WithConnection(nc))
		if err != nil {
			log.Error(err, "Error loading stream")
			return err
		}
		log.Info("Stream opened")

		streamInfo, err := stream.Information()
		if err != nil {
			log.Error(err, "Error getting stream information")
		} else {
			log.V(1).Info("Steam info", "STREAM INFO", streamInfo)
		}

		consumer, err := stream.LoadOrNewConsumerFromDefault(
			attester,
			jsm.DefaultConsumer,
			jsm.DurableName(attester),
			jsm.FilterStreamBySubject(fmt.Sprintf("ATTESTATION.%s", attester)))
		if err != nil {
			log.Error(err, "Error loading consumer")
			return err
		}

		jetstreamConsumer = newJetstreamConsumer(c.CTX, c.log, c.OccurrenceCreator, consumer)
		c.Consumers[attester] = jetstreamConsumer
	}
	go jetstreamConsumer.Run()
	return nil
}

func (c *JetstreamClient) Unsubscribe(attesterName string) error {
	log := c.log.WithName("Unsubscribe()").WithValues("attester", attesterName)

	consumer, ok := c.Consumers[attesterName]
	if !ok {
		log.Info("Listener not found for attester", "attester", attesterName)
		return nil
	}

	log.Info("Stoping consumer listener")
	consumer.Stop()

	if consumer.EnforcerCount == 0 {
		delete(c.Consumers, attesterName)
	}
	return nil
}

type JetstreamConsumer struct {
	ctx               context.Context
	log               logr.Logger
	occurrenceCreator occurrence.Creator
	consumer          *jsm.Consumer
	EnforcerCount     uint
	Cancel            context.CancelFunc
	Closed            chan bool
}

func newJetstreamConsumer(ctx context.Context, log logr.Logger, occurrenceCreator occurrence.Creator, consumer *jsm.Consumer) *JetstreamConsumer {
	consumerContext, cancel := context.WithCancel(ctx)
	return &JetstreamConsumer{
		ctx:               consumerContext,
		log:               log.WithName("Consumer").WithValues("CONSUMER", consumer.Name()),
		occurrenceCreator: occurrenceCreator,
		consumer:          consumer,
		EnforcerCount:     0,
		Cancel:            cancel,
		Closed:            make(chan bool),
	}
}

func (consumer *JetstreamConsumer) Name() string {
	return consumer.consumer.Name()
}

func (consumer *JetstreamConsumer) Run() {
	log := consumer.log.WithName("Run()")

	consumer.EnforcerCount++
	if consumer.EnforcerCount > 1 {
		log.Info("Listener already started", "count", consumer.EnforcerCount)
		return
	}

	log.Info("Listening for new messages")
	for {
		select {
		case <-consumer.ctx.Done():
			consumer.Closed <- true
			log.Info("Consumer context finished")
			return
		default:
			info, err := consumer.consumer.State()
			if err != nil {
				log.Error(err, "Error getting consumer state")
			}
			log.V(1).Info("Consumer State", "state", info)

			consumer.proccessMessage(consumer.ctx)
		}
	}
}

func (consumer *JetstreamConsumer) Stop() {
	log := consumer.log.WithName("Stop()")

	consumer.EnforcerCount--
	if consumer.EnforcerCount > 0 {
		log.Info("Listener not stopped because of remaining enforcers", "count", consumer.EnforcerCount)
		return
	}

	log.Info("Stopping consumer context")
	consumer.Cancel()
	<-consumer.Closed
}

func (consumer *JetstreamConsumer) proccessMessage(ctx context.Context) {
	log := consumer.log.WithName("ProccessMessage()")
	message, err := consumer.consumer.NextMsg(jsm.WithContext(ctx))
	if err != nil {
		if err.Error() == "context canceled" {
			log.Info("Listener context canceled")
		} else {
			log.Error(err, "Error fetching message")
		}
		return
	}
	log.Info("Message received")

	am := attestationMessage{}
	err = json.Unmarshal(message.Data, &am)
	if err != nil {
		log.Error(err, "Error parsing message data", "message", message)
		return
	}

	occurrence := am.CreateOccurrence()
	err = consumer.occurrenceCreator.CreateOccurrences(ctx, &occurrence)
	if err != nil {
		log.Error(err, "Error creating attestation occurrence", "occurrence", occurrence)
		return
	}

	err = message.Respond(nil)
	if err != nil {
		log.Error(err, "Error acknowledging message", "message", message)
	}
}

// attestationMessage structures attestation occurrence data so that it can be JSON encoded
type attestationMessage struct {
	Occurrence grafeas.Occurrence                                  `json:"occurrence"`
	Details    grafeas.Occurrence_Attestation                      `json:"details"`
	Signature  grafeasAttestation.Attestation_PgpSignedAttestation `json:"signature"`
	KeyID      grafeasAttestation.PgpSignedAttestation_PgpKeyId    `json:"key_id"`
}

func newAttestationMessage(occurrence grafeas.Occurrence) *attestationMessage {
	details := *occurrence.Details.(*grafeas.Occurrence_Attestation)
	occurrence.Details = nil
	signature := *details.Attestation.Attestation.Signature.(*grafeasAttestation.Attestation_PgpSignedAttestation)
	details.Attestation.Attestation.Signature = nil
	keyID := *signature.PgpSignedAttestation.KeyId.(*grafeasAttestation.PgpSignedAttestation_PgpKeyId)
	signature.PgpSignedAttestation.KeyId = nil
	return &attestationMessage{
		Occurrence: occurrence,
		Details:    details,
		Signature:  signature,
		KeyID:      keyID,
	}
}

func (am attestationMessage) CreateOccurrence() grafeas.Occurrence {
	am.Signature.PgpSignedAttestation.KeyId = &am.KeyID
	am.Details.Attestation.Attestation.Signature = &am.Signature
	am.Occurrence.Details = &am.Details
	return am.Occurrence
}
