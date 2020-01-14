package ctx

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gin-gonic/gin"
	"github.com/liatrio/rode/pkg/grafeas"
	"go.uber.org/zap"
)

// Context contains the state for the app
type Context struct {
	Router    *gin.Engine
	Logger    *zap.SugaredLogger
	AWSConfig *aws.Config
	Grafeas   *grafeas.Client
}

// NewContext creates a new context instance
func NewContext() *Context {
	return &Context{}
}

// WithLogger sets the logger for the context
func (c *Context) WithLogger(logger *zap.SugaredLogger) *Context {
	c.Logger = logger
	return c
}

// WithRouter sets the router for the context
func (c *Context) WithRouter(router *gin.Engine) *Context {
	c.Router = router
	return c
}

// WithAWSConfig sets the AWS Config for the context
func (c *Context) WithAWSConfig(config *aws.Config) *Context {
	c.AWSConfig = config
	return c
}

// WithGrafeas sets the Grafeas client for the context
func (c *Context) WithGrafeas(grafeas *grafeas.Client) *Context {
	c.Grafeas = grafeas
	return c
}
