package nats

import (
	"github.com/nats-io/jsm.go"
	natsGo "github.com/nats-io/nats.go"
	"time"
)

// The nats and jsm packages expose a number of functions directly, which makes testing difficult.
// ConnectionFactory and StreamManager interfaces include only the functions from those packages that are currently in use.
// Since the functions are exposed at the package level, we have to implement the interfaces.
// The implementations are a single line to call the actual functions, with some casting to work around the interfaces.

//go:generate mockgen -destination=../../mocks/pkg/nats_mock/types.go -package=nats_mock . Connection,ConnectionFactory,StreamManager

// nats.Conn
type Connection interface {
	Publish(subject string, data []byte) error
}

// nats.Connect
type ConnectionFactory interface {
	Connect(url string, options ...natsGo.Option) (Connection, error)
}

// jsm.*
type StreamManager interface {
	LoadStream(name string, opts ...jsm.RequestOption) (*jsm.Stream, error)
	NewStream(name string, opts ...jsm.StreamOption) (*jsm.Stream, error)

	FileStorage() jsm.StreamOption
	MaxAge(maxAge time.Duration) jsm.StreamOption
	StreamConnection(opts ...jsm.RequestOption) jsm.StreamOption
	Subjects(subjects ...string) jsm.StreamOption
	WithConnection(nc Connection) jsm.RequestOption
}

type NatsWrapper struct{}

func NewConnectionFactory() ConnectionFactory {
	return &NatsWrapper{}
}

type JetstreamWrapper struct{}

func NewStreamManager() StreamManager {
	return &JetstreamWrapper{}
}

func (nw *NatsWrapper) Connect(url string, options ...natsGo.Option) (Connection, error) {
	return natsGo.Connect(url, options...)
}

func (jw *JetstreamWrapper) LoadStream(name string, opts ...jsm.RequestOption) (*jsm.Stream, error) {
	return jsm.LoadStream(name, opts...)
}

func (jw *JetstreamWrapper) NewStream(name string, opts ...jsm.StreamOption) (*jsm.Stream, error) {
	return jsm.NewStream(name, opts...)
}

func (jw *JetstreamWrapper) FileStorage() jsm.StreamOption {
	return jsm.FileStorage()
}

func (jw *JetstreamWrapper) MaxAge(maxAge time.Duration) jsm.StreamOption {
	return jsm.MaxAge(maxAge)
}

func (jw *JetstreamWrapper) StreamConnection(opts ...jsm.RequestOption) jsm.StreamOption {
	return jsm.StreamConnection(opts...)
}

func (jw *JetstreamWrapper) Subjects(subjects ...string) jsm.StreamOption {
	return jsm.Subjects(subjects...)
}

func (jw *JetstreamWrapper) WithConnection(nc Connection) jsm.RequestOption {
	return jsm.WithConnection(nc.(*natsGo.Conn))
}
