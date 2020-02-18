package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	"github.com/liatrio/rode/pkg/occurrence"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type HarborEventCollector struct {
	logger            logr.Logger
	occurrenceCreator occurrence.Creator
	url               string
	secret            string
	project           string
	namespace         string
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

func (t *HarborEventCollector) Reconcile(ctx context.Context, name types.NamespacedName) error {

	t.logger.Info("reconciling HARBOR collector")
	harborCreds := t.getHarborCredentials(ctx, t.secret, t.namespace)
	projectID := t.getProjectID(t.project, t.url)
	if projectID != "" && !t.checkForWebhook(projectID, t.url, harborCreds) {
		t.createWebhook(projectID, t.url, harborCreds, "webhook/harbor/"+name.String())
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

func (t *HarborEventCollector) Destroy(ctx context.Context, name types.NamespacedName) error {
	t.logger.Info("destroying test collector")
	harborCreds := t.getHarborCredentials(ctx, t.secret, t.namespace)
	projectID := t.getProjectID(t.project, t.url)
	policyID := t.getWebhookPolicyID(projectID, t.url, harborCreds)
	t.deleteWebhookPolicy(projectID, t.url, policyID, harborCreds)

	return nil
}

func (t *HarborEventCollector) Type() string {
	return "harbor_event"
}

func (t *HarborEventCollector) HandleWebhook(writer http.ResponseWriter, request *http.Request, occurrenceCreator occurrence.Creator) {
	t.logger.Info("HARBOR WEBHOOK HIT")

	writer.WriteHeader(http.StatusOK)
}

func (t *HarborEventCollector) getHarborCredentials(ctx context.Context, secretname string, namespace string) string {
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

	projectID := ""
	for _, p := range projects {
		if p.Name == name {
			projectID = strconv.Itoa(p.ProjectID)
		}
	}
	return projectID
}

func (t *HarborEventCollector) getWebhookPolicyID(projectID string, url string, harborCreds string) string {
	client := &http.Client{}
	webhookPolicyIDURL := url + "/api/projects/" + projectID + "/webhook/policies"
	req, err := http.NewRequest("GET", webhookPolicyIDURL, nil)
	req.SetBasicAuth("admin", harborCreds)
	resp, err := client.Do(req)
	if err != nil {
		log.Print(err)
	}
	defer resp.Body.Close()
	policyList, err := ioutil.ReadAll(resp.Body)

	var policies []WebhookPolicies
	json.Unmarshal([]byte(policyList), &policies)
	policyID := strconv.Itoa(policies[0].ID)

	return policyID
}

func (t *HarborEventCollector) checkForWebhook(projectID string, url string, harborCreds string) bool {
	client := &http.Client{}
	webhookURL := url + "/api/projects/" + projectID + "/webhook/policies"

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

func (t *HarborEventCollector) createWebhook(projectID string, url string, harborCreds string, webhookEndpoint string) {
	client := &http.Client{}

	webhookURL := url + "/api/projects/" + projectID + "/webhook/policies"
	targets := []Targets{
		Targets{
			Type:           "http",
			Address:        webhookEndpoint,
			AuthHeader:     "auth_header",
			SkipCertVerify: true,
		},
	}
	body := &WebhookPolicies{
		Targets: targets,
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
		return
	}
	defer resp.Body.Close()

	log.Print("Successfully created webhook.")
}

func (t *HarborEventCollector) deleteWebhookPolicy(projectID string, url string, policyID string, harborCreds string) {
	client := &http.Client{}

	webhookDeleteURL := url + "/api/projects/" + projectID + "/webhook/policies" + policyID
	req, err := http.NewRequest("DELETE", webhookDeleteURL, nil)
	_, err = client.Do(req)
	if err != nil {
		log.Print(err)
		return
	}
	log.Print("Successfully deleted webhook.")
}

// Harbor structured project
type Project struct {
	ProjectID int    `json:"project_id"`
	Name      string `json:"name"`
}

// Harbor structured project
type WebhookPolicies struct {
	Targets    []Targets `json:"targets,omitempty"`
	EventTypes []string  `json:"event_types,omitempty"`
	Enabled    bool      `json:"enabled,omitempty"`
	ID         int       `json:"id,omitempty"`
}

type Targets struct {
	Type           string `json:"type"`
	Address        string `json:"address"`
	AuthHeader     string `json:"auth_header"`
	SkipCertVerify bool   `json:"skip_cert_verify"`
}
