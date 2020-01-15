package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/liatrio/rode/pkg/aws"
	"github.com/liatrio/rode/pkg/controller"
	"github.com/liatrio/rode/pkg/ctx"
	"github.com/liatrio/rode/pkg/grafeas"
	"github.com/liatrio/rode/pkg/logger"
	"github.com/liatrio/rode/pkg/opa"
	"github.com/liatrio/rode/pkg/signals"
)

var (
	logLevel    string
	logEncoding string
)

func init() {
	flag.StringVar(&logLevel, "log-level", "debug", "Log level can be: debug, info, warning, error.")
	flag.StringVar(&logEncoding, "log-encoding", "json", "Log encoding can be: json, console.")
}

func main() {
	logger, err := logger.NewLoggerWithEncoding(logLevel, logEncoding)
	if err != nil {
		log.Fatalf("Error creating logger: %v", err)
	}

	grafeasEndpoint := os.Getenv("GRAFEAS_ENDPOINT")
	opaTrace, _ := strconv.ParseBool(os.Getenv("OPA_TRACE"))
	opa := opa.NewClient(logger, opaTrace)
	context := ctx.NewContext().
		WithLogger(logger).
		WithRouter(startServer(logger.Desugar())).
		WithAWSConfig(aws.NewAWSConfig(logger)).
		WithOPA(opa).
		WithGrafeas(grafeas.NewClient(logger, opa, grafeasEndpoint))

	c := controller.NewController(context)

	stopCh := signals.SetupSignalHandler()
	if err := c.Run(stopCh); err != nil {
		logger.Fatalf("Error running controller: %v", err)
	}
}

func startServer(logger *zap.Logger) *gin.Engine {
	router := gin.New()

	router.Use(ginzap.Ginzap(logger, time.RFC3339, true))
	router.Use(ginzap.RecoveryWithZap(logger, true))

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
		})
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	return router
}
