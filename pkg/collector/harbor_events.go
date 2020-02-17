package collector

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/liatrio/rode/pkg/occurrence"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"time"
	"strconv"
	"net/http"
	"bytes"
	"encoding/json"
	"io/ioutil"
)

type HarborEventCollector struct {
	logger            logr.Logger
	occurrenceCreator occurrence.Creator
	url               string
	secret            string
	project           string
}

type Project struct {
	ProjectID          int       `json:"project_id"`
	Name               string    `json:"name"`
}

type JsonInput struct {
	Targets    []Targets `json:"targets"`
	EventTypes []string `json:"event_types"`
  Enabled    bool `json:"enabled"`
}

type Targets struct {
	Type           string `json:"type"`
  Address        string `json:"address"`
  AuthHeader     string `json:"auth_header"`
  SkipCertVerify bool   `json:"skip_cert_verify"`
}

func NewHarborEventCollector(logger logr.Logger, harborUrl string, secret string, project string) Collector {
	return &HarborEventCollector{
		logger:  logger,
		url:     harborUrl,
		secret:  secret,
		project: project,
	}
}

func (t *HarborEventCollector) Reconcile(ctx context.Context) error {
	t.logger.Info("reconciling HARBOR collector")
	t.getHarborCredentials(ctx, t.secret)
	//checkForWebhook
	//If webhook doesn't exist, createWebhook

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
				t.logger.Info(t.project)
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

func (t *HarborEventCollector) getHarborCredentials(ctx context.Context, secretname string) {
	t.logger.Info("Inside getHarborCredentials\n")
	config, configError := rest.InClusterConfig()
	if configError != nil {
		log.Fatal(configError)
	}

	clientset, clientErr := kubernetes.NewForConfig(config)
	if clientErr != nil {
		log.Fatal(clientErr)
	}

	secrets, err := clientset.CoreV1().Secrets("rode").Get(secretname, metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	// t.logger.Info("Number of secrets", "numSec", len(secrets.Items))
	t.logger.Info("DATA", "data", secrets.Data["HARBOR_ADMIN_PASSWORD"])
}
