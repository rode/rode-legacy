package controller

import (
	"context"
	"fmt"
	"strconv"

	"github.com/liatrio/rode/pkg/enforcer"

	"github.com/liatrio/rode/pkg/attester"
	"github.com/liatrio/rode/pkg/aws"
	"github.com/liatrio/rode/pkg/occurrence"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Controller is main entry
type Controller struct {
	attesters       []attester.Attester
	logger          *zap.SugaredLogger
	opaTrace        bool
	grafeasEndpoint string
	excludeNS       []string
}

// Option is interface to configure controller
type Option func(c *Controller)

// New creates a new controller
func New(options ...Option) *Controller {
	c := new(Controller)
	c.attesters = make([]attester.Attester, 0, 0)

	for _, option := range options {
		option(c)
	}
	return c
}

// WithLogger configures logger for
func WithLogger(logger *zap.SugaredLogger) Option {
	return func(c *Controller) {
		c.logger = logger
	}
}

// WithOPATrace configures tracing for OPA
func WithOPATrace(enabled string) Option {
	opaTrace, _ := strconv.ParseBool(enabled)
	return func(c *Controller) {
		c.opaTrace = opaTrace
	}
}

// WithGrafeasEndpoint controls the endpoint for grafeas
func WithGrafeasEndpoint(endpoint string) Option {
	return func(c *Controller) {
		c.grafeasEndpoint = endpoint
	}
}

// WithExcludeNS controls the namespace to exclude from enforcer
func WithExcludeNS(excludeNS []string) Option {
	return func(c *Controller) {
		c.excludeNS = excludeNS
	}
}

// Start the controller
func (c *Controller) Start(ctx context.Context) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("Error creating kubernetes config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Error creating kubernetes clientset: %v", err)
	}

	err = StartAttesters(ctx, c.logger, c.opaTrace, &c.attesters)
	if err != nil {
		return fmt.Errorf("Error starting attester: %v", err)
	}

	awsConfig := aws.NewAWSConfig(c.logger)
	grafeasClient := occurrence.NewGrafeasClient(c.logger, c.grafeasEndpoint)
	occurrenceCreator := attester.NewAttestWrapper(c.logger, grafeasClient, grafeasClient, c.attesters)

	err = StartCollectors(ctx, c.logger, awsConfig, occurrenceCreator)
	if err != nil {
		return fmt.Errorf("Error starting collectors: %v", err)
	}

	enforcer := enforcer.NewEnforcer(c.logger, c.excludeNS, c.attesters, grafeasClient, clientset)

	err = StartAPI(ctx, c.logger, grafeasClient, enforcer)
	if err != nil {
		return fmt.Errorf("Error starting APIs: %v", err)
	}
	return nil
}
