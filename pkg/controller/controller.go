package controller

import (
	"io"
	"strings"
	"time"

	"github.com/golang/protobuf/jsonpb"

	"github.com/gin-gonic/gin"
	"github.com/liatrio/rode/pkg/ctx"
	"github.com/liatrio/rode/pkg/ingester"
)

// Controller is the main control loop for rode
type Controller struct {
	ctx.Context
}

// NewController creates a new controller instance
func NewController(
	context *ctx.Context,
) *Controller {
	context.Logger.Debug("Creating new controller")
	ctrl := &Controller{
		*context,
	}
	marshaler := &jsonpb.Marshaler{}
	context.Router.GET("/occurrences/*resource", func(c *gin.Context) {
		resourceURI := strings.TrimPrefix(c.Param("resource"), "/")
		o, err := context.Grafeas.GetOccurrences(resourceURI)
		if err != nil {
			c.AbortWithError(400, err)
		} else {
			c.Stream(func(w io.Writer) bool {
				err := marshaler.Marshal(w, o)
				if err != nil {
					context.Logger.Errorf("Unable to stream pb", err)
				}
				return false
			})
		}
	})
	return ctrl
}

// Run will begin the control loop for rode
func (c *Controller) Run(stopCh <-chan struct{}) error {
	running := true
	go func() {
		ingesters := make([]ingester.Ingester, 0, 0)
		for running {
			// TODO: pull the list of ingesters from CRs...
			if len(ingesters) == 0 {
				ingesters = append(ingesters, ingester.NewEcrEventIngester(&c.Context))
			}

			for _, ingester := range ingesters {
				err := ingester.Reconcile()
				if err != nil {
					c.Logger.Error(err)
				}
			}

			time.Sleep(15 * time.Second)
		}
	}()

	<-stopCh
	running = false
	return nil
}
