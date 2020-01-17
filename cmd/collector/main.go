package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/liatrio/rode/pkg/controller"
	"github.com/liatrio/rode/pkg/logger"
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

	ctrl := controller.New(
		controller.WithLogger(logger),
		controller.WithOPATrace(os.Getenv("OPA_TRACE")),
		controller.WithGrafeasEndpoint(os.Getenv("GRAFEAS_ENDPOINT")),
		controller.WithExcludeNS(strings.Split(os.Getenv("EXCLUDED_NAMESPACES"), ",")),
	)

	ctx := signals.WithSignalCancel(context.Background())
	err = ctrl.Start(ctx)
	if err != nil {
		logger.Fatalf("Error starting controller %v", err)
	}

	<-ctx.Done()
}
