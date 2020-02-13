package collector

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/liatrio/rode/pkg/occurrence"
	"time"
  metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  "k8s.io/client-go/rest"
  "k8s.io/client-go/kubernetes"
  "log"
)

type HarborEventCollector struct {
	logger            logr.Logger
	occurrenceCreator occurrence.Creator
	testMessage       string
}

func NewHarborEventCollector(logger logr.Logger, testMessage string) Collector {
	return &HarborEventCollector{
		logger:      logger,
		testMessage: testMessage,
	}
}

func (t *HarborEventCollector) Reconcile(ctx context.Context) error {
	t.logger.Info("reconciling HARBOR collector")
  t.listSecret(ctx)

	return nil
}

func (t *HarborEventCollector) Start(ctx context.Context, stopChan chan interface{}, occurrenceCreator occurrence.Creator) error {
	go func() {
		for range time.Tick(8 * time.Second) {
			select {
			case <-ctx.Done():
				stopChan <- true
				return
			default:
				t.logger.Info(t.testMessage)
			}
		}

		t.logger.Info("harbor collector goroutine finished")
	}()

	return nil
}

func (t *HarborEventCollector) Destroy(ctx context.Context) error {
	t.logger.Info("destroying test collector")

	return nil
}

func (t *HarborEventCollector) listSecret(ctx context.Context) {
  t.logger.Info("Inside listSecret\n")
  config, configError := rest.InClusterConfig()
  if configError != nil {
    log.Fatal(configError)
  }

  clientset, clientErr := kubernetes.NewForConfig(config)
  if clientErr != nil {
    log.Fatal(clientErr)
  }

  pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
			panic(err.Error())
  }

  t.logger.Info("There are %d pods in the cluster\n", len(pods.Items))

}
