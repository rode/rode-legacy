package grafeas

import (
	grafeas "github.com/grafeas/client-go/0.1.0"
	"go.uber.org/zap"
)

// Client handle into grafeas
type Client struct {
	logger *zap.SugaredLogger
}

// NewClient creates a new client
func NewClient(logger *zap.SugaredLogger) *Client {
	return &Client{
		logger,
	}
}

// PutOccurrence will save the occurence in grafeas
func (c *Client) PutOccurrence(occurrence *grafeas.V1beta1Occurrence) error {
	if occurrence == nil {
		return nil
	}
	c.logger.Infof("Recording occurrence %s", occurrence.Name)
	// TODO: call grafeeas
	return nil
}
