package controller

import (
	"context"
	"io"
	"strings"
	"time"

	"go.uber.org/zap"

	"net/http"

	ginzap "github.com/gin-contrib/zap"
	"github.com/golang/protobuf/jsonpb"
	"github.com/liatrio/rode/pkg/occurrence"

	"github.com/gin-gonic/gin"
)

// StartAPI creates a new API engine
func StartAPI(ctx context.Context, logger *zap.SugaredLogger, occurrenceLister occurrence.Lister) error {
	router := gin.New()

	router.Use(ginzap.Ginzap(logger.Desugar(), time.RFC3339, true))
	router.Use(ginzap.RecoveryWithZap(logger.Desugar(), true))

	logger.Debug("Registering health API")
	routeHealth(router)
	logger.Debug("Registering occurrence API")
	routeOccurrences(router, occurrenceLister)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("listen: %s\n", err)
		}
	}()
	return nil
}

func routeHealth(router gin.IRouter) {
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
		})
	})
}

func routeOccurrences(router gin.IRouter, occurrenceLister occurrence.Lister) {
	marshaler := &jsonpb.Marshaler{}
	router.GET("/occurrences/*resource", func(c *gin.Context) {
		resourceURI := strings.TrimPrefix(c.Param("resource"), "/")
		o, err := occurrenceLister.ListOccurrences(c, resourceURI)
		if err != nil {
			c.AbortWithError(400, err)
		} else {
			c.Stream(func(w io.Writer) bool {
				err := marshaler.Marshal(w, o)
				if err != nil {
					c.Error(err)
				}
				return false
			})
		}
	})
}
