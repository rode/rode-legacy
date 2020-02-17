package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/go-logr/logr"
	"github.com/liatrio/rode/pkg/occurrence"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"strconv"
	"time"
)

type HarborEventCollector struct {
	logger            logr.Logger
	occurrenceCreator occurrence.Creator
	url               string
	secret            string
	project           string
	namespace         string
}

type Project struct {
	ProjectID int    `json:"project_id"`
	Name      string `json:"name"`
}

type JsonInput struct {
	Targets    []Targets `json:"targets"`
	EventTypes []string  `json:"event_types"`
	Enabled    bool      `json:"enabled"`
}

type Targets struct {
	Type           string `json:"type"`
	Address        string `json:"address"`
	AuthHeader     string `json:"auth_header"`
	SkipCertVerify bool   `json:"skip_cert_verify"`
}

func NewHarborEventCollector(logger logr.Logger, harborUrl string, secret string, project string, namespace string) Collector {
	return &HarborEventCollector{
		logger:    logger,
		url:       harborUrl,
		secret:    secret,
		project:   project,
		namespace: namespace,
	}
}

func (t *HarborEventCollector) Reconcile(ctx context.Context) error {
	t.logger.Info("reconciling HARBOR collector")
	harborCreds := t.getHarborCredentials(ctx, t.secret, t.namespace)
	projectID := t.getProjectID(t.project, t.url)
	if projectID != "" && !t.checkForWebhook(projectID, t.url, harborCreds) {
		t.createWebhook(projectID, t.url, harborCreds, "www.example.com")
	}

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

func (t *HarborEventCollector) getHarborCredentials(ctx context.Context, secretname string, namespace string) string {
	t.logger.Info("Inside getHarborCredentials\n")
	config, configError := rest.InClusterConfig()
	if configError != nil {
		log.Fatal(configError)
	}

	clientset, clientErr := kubernetes.NewForConfig(config)
	if clientErr != nil {
		log.Fatal(clientErr)
	}

	secrets, err := clientset.CoreV1().Secrets(namespace).Get(secretname, metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	return string(secrets.Data["HARBOR_ADMIN_PASSWORD"])
}

func (t *HarborEventCollector) getProjectID(name string, url string) string {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url+"/api/projects/", nil)
	resp, err := client.Do(req)
	if err != nil {
		log.Print(err)
	}
	defer resp.Body.Close()

	projectList, err := ioutil.ReadAll(resp.Body)

	var projects []Project

	json.Unmarshal([]byte(projectList), &projects)

	projectId := ""
	for _, p := range projects {
		if p.Name == name {
			projectId = strconv.Itoa(p.ProjectID)
		}
	}
	return projectId
}

func (t *HarborEventCollector) checkForWebhook(projectId string, url string, harborCreds string) bool {
	client := &http.Client{}

	webhookURL := url + "/api/projects/" + projectId + "/webhook/policies"

	req, err := http.NewRequest("GET", webhookURL, nil)
	req.SetBasicAuth("admin", harborCreds)
	resp, err := client.Do(req)
	if err != nil {
		log.Print(err)
	}

	defer resp.Body.Close()

	var webhooks []string
	webhookJson, err := ioutil.ReadAll(resp.Body)
	json.Unmarshal([]byte(webhookJson), &webhooks)

	if len(webhooks) == 0 {
		return false
	}
	return true
}

func (t *HarborEventCollector) createWebhook(projectId string, url string, harborCreds string, webhookEndpoint string) {
	client := &http.Client{}

	webhookURL := url + "/api/projects/" + projectId + "/webhook/policies"

	bodyTargets := []Targets{
		Targets{
			Type:           "http",
			Address:        webhookEndpoint,
			AuthHeader:     "auth_header",
			SkipCertVerify: true,
		},
	}

	body := &JsonInput{
		Targets: bodyTargets,
		EventTypes: []string{
			"pushImage",
			"scanningFailed",
			"scanningCompleted",
		},
		Enabled: true,
	}

	bodyJson, err := json.Marshal(body)
	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(bodyJson))

	req.SetBasicAuth("admin", harborCreds)

	resp, err := client.Do(req)
	if err != nil {
		log.Print(err)
	}
	defer resp.Body.Close()

}
