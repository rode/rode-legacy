package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	discovery "github.com/grafeas/grafeas/proto/v1beta1/discovery_go_proto"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	image "github.com/grafeas/grafeas/proto/v1beta1/image_go_proto"
	packag "github.com/grafeas/grafeas/proto/v1beta1/package_go_proto"
	vulnerability "github.com/grafeas/grafeas/proto/v1beta1/vulnerability_go_proto"
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
	harborCreds, err := t.getHarborCredentials(ctx, t.secret, t.namespace)
	if err != nil {
		return err
	}

	projectID, err := t.getProjectID(t.project, t.url)
	if err != nil {
		return err
	}
	webhookCheck, err := t.checkForWebhook(projectID, t.url, harborCreds)
	if err != nil {
		return err
	}
	if !webhookCheck {
		//TODO: Retrieve hostname for rode from Ingress
		err = t.createWebhook(projectID, t.url, harborCreds, "/webhook/harbor_event/"+name.String())
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *HarborEventCollector) Destroy(ctx context.Context) error {
	t.logger.Info("destroying test collector")
	harborCreds, err := t.getHarborCredentials(ctx, t.secret, t.namespace)
	if err != nil {
		return err
	}
	projectID, err := t.getProjectID(t.project, t.url)
	if err != nil {
		return err
	}
	policyID, err := t.getWebhookPolicyID(projectID, t.url, harborCreds)
	if err != nil {
		return err
	}
	t.deleteWebhookPolicy(projectID, t.url, policyID, harborCreds)
	if err != nil {
		return err
	}
	return nil
}

func (t *HarborEventCollector) Type() string {
	return "harbor_event"
}

func (t *HarborEventCollector) HandleWebhook(writer http.ResponseWriter, request *http.Request, occurrenceCreator occurrence.Creator) {
	var payload *Payload
	body, err := ioutil.ReadAll(request.Body)
	err = json.Unmarshal(body, &payload)
	if err != nil {
		log.Fatal(err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	var occurrences []*grafeas.Occurrence
	switch payload.Type {
	case "pushImage":
		t.logger.Info("Creating Image Push Occurrence")
		occurrences = t.newImagePushOccurrences(payload.EventData.Resources)
	case "scanningCompleted":
		t.logger.Info("Creating Image Scan Occurrence")
		occurrences = t.newImageScanOccurrences(payload.EventData.Resources)
	default:
		t.logger.Info(payload.Type)
	}

	ctx := context.Background()
	err = occurrenceCreator.CreateOccurrences(ctx, occurrences...)
	if err != nil {
		t.logger.Info("Error creating Occurrence")
		log.Fatal(err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusOK)
}

func (t *HarborEventCollector) newImagePushOccurrences(resources []*Resource) []*grafeas.Occurrence {
	occurrences := make([]*grafeas.Occurrence, 0)
	for i, resource := range resources {
		baseResourceUrl := resource.ResourceURL
		derivedImageDetails := &grafeas.Occurrence_DerivedImage{
			DerivedImage: &image.Details{
				DerivedImage: &image.Derived{
					BaseResourceUrl: baseResourceUrl,
					Fingerprint: &image.Fingerprint{
						V1Name: "TODO",
						V2Blob: []string{"TODO"},
						V2Name: "TODO",
					},
				},
			},
		}

		o := newHarborImageScanOccurrence(resources[i], t.project)
		o.Details = derivedImageDetails
		occurrences = append(occurrences, o)
	}
	return occurrences
}

func (t *HarborEventCollector) newImageScanOccurrences(resources []*Resource) []*grafeas.Occurrence {
	var vulnerabilityDetails *grafeas.Occurrence_Vulnerability
	status := discovery.Discovered_ANALYSIS_STATUS_UNSPECIFIED

	occurrences := make([]*grafeas.Occurrence, 0)
	for i := range resources {
		scanOverview := resources[i].ScanOverview["application/vnd.scanner.adapter.vuln.report.harbor+json; version=1.0"].(map[string]interface{})
		if scanOverview["scan_status"].(string) == "Success" {
			status = discovery.Discovered_FINISHED_SUCCESS
			vulnerabilityDetails = t.getVulnerabilityDetails(scanOverview["severity"].(string))
		} else if scanOverview["scan_status"].(string) == "Error" {
			status = discovery.Discovered_FINISHED_FAILED
		}

		discoveryDetails := &grafeas.Occurrence_Discovered{
			Discovered: &discovery.Details{
				Discovered: &discovery.Discovered{
					AnalysisStatus: status,
				},
			},
		}

		o := newHarborImageScanOccurrence(resources[i], t.project)
		o.Details = discoveryDetails
		occurrences = append(occurrences, o)
		if scanOverview["scan_status"].(string) == "Success" {
			o = newHarborImageScanOccurrence(resources[i], t.project)
			o.Details = vulnerabilityDetails
			occurrences = append(occurrences, o)
		}

	}
	return occurrences
}

func newHarborImageScanOccurrence(resource *Resource, projectName string) *grafeas.Occurrence {
	o := &grafeas.Occurrence{
		Resource: &grafeas.Resource{
			Uri: HarborOccurrenceResourceURI(resource.ResourceURL, resource.Digest),
		},
		NoteName: HarborOccurrenceNote(projectName),
	}
	return o
}

func HarborOccurrenceResourceURI(url, digest string) string {
	return fmt.Sprintf("%s@%s", url, digest)
}

func HarborOccurrenceNote(projectName string) string {
	return fmt.Sprintf("projects/%s/notes/%s", "rode", projectName)
}

func (t *HarborEventCollector) getVulnerabilityDetails(severity string) *grafeas.Occurrence_Vulnerability {
	vulnerabilitySeverity := t.getVulnerabilitySeverity(severity)
	vulnerabilityDetails := &grafeas.Occurrence_Vulnerability{
		Vulnerability: &vulnerability.Details{
			Severity: vulnerabilitySeverity,
			PackageIssue: []*vulnerability.PackageIssue{
				{
					AffectedLocation: &vulnerability.VulnerabilityLocation{
						CpeUri:  "TODO",
						Package: "TODO",
						Version: &packag.Version{
							Kind: packag.Version_NORMAL,
							Name: "TODO",
						},
					},
				},
			},
		},
	}
	return vulnerabilityDetails
}

func (t *HarborEventCollector) getVulnerabilitySeverity(v string) vulnerability.Severity {
	switch v {
	case HarborSeverityCritical:
		return vulnerability.Severity_CRITICAL
	case HarborSeverityHigh:
		return vulnerability.Severity_HIGH
	case HarborSeverityMedium:
		return vulnerability.Severity_MEDIUM
	case HarborSeverityLow:
		return vulnerability.Severity_LOW
	case HarborSeverityNegligible:
		return vulnerability.Severity_MINIMAL
	case HarborSeverityNone:
		return vulnerability.Severity_MINIMAL
	default:
		return vulnerability.Severity_SEVERITY_UNSPECIFIED
	}
}

// Assumes Harbor admin creds are deployed in the same namespace as the Collector CR
func (t *HarborEventCollector) getHarborCredentials(ctx context.Context, secretname string, namespace string) (string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return "", err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}
	secrets, err := clientset.CoreV1().Secrets(namespace).Get(secretname, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(secrets.Data["HARBOR_ADMIN_PASSWORD"]), nil
}

func (t *HarborEventCollector) getProjectID(name string, url string) (string, error) {
	client := &http.Client{}
	var projects []Project
	var projectID string

	req, err := http.NewRequest("GET", url+"/api/projects/", nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	projectList, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	json.Unmarshal([]byte(projectList), &projects)
	for _, p := range projects {
		if p.Name == name {
			projectID = strconv.Itoa(p.ProjectID)
		}
	}
	return projectID, nil
}

func (t *HarborEventCollector) getWebhookPolicyID(projectID string, url string, harborCreds string) (string, error) {
	client := &http.Client{}
	webhookPolicyIDURL := url + "/api/projects/" + projectID + "/webhook/policies"
	var policies []WebhookPolicies

	req, err := http.NewRequest("GET", webhookPolicyIDURL, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth("admin", harborCreds)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	policyList, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	json.Unmarshal([]byte(policyList), &policies)
	policyID := strconv.Itoa(policies[0].ID)
	return policyID, nil
}

func (t *HarborEventCollector) checkForWebhook(projectID string, url string, harborCreds string) (bool, error) {
	client := &http.Client{}
	webhookURL := url + "/api/projects/" + projectID + "/webhook/policies"
	var webhooks []string

	req, err := http.NewRequest("GET", webhookURL, nil)
	if err != nil {
		return true, err
	}
	req.SetBasicAuth("admin", harborCreds)
	resp, err := client.Do(req)
	if err != nil {
		return true, err
	}
	defer resp.Body.Close()

	webhookJson, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return true, err
	}
	json.Unmarshal([]byte(webhookJson), &webhooks)

	if len(webhooks) == 0 {
		return false, nil
	}
	return true, nil
}

func (t *HarborEventCollector) createWebhook(projectID string, url string, harborCreds string, webhookEndpoint string) error {
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

	bodyJSON, err := json.Marshal(body)
	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(bodyJSON))
	if err != nil {
		return err
	}
	req.SetBasicAuth("admin", harborCreds)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	log.Print("Successfully created webhook.")
	return nil
}

func (t *HarborEventCollector) deleteWebhookPolicy(projectID string, url string, policyID string, harborCreds string) error {
	client := &http.Client{}
	webhookDeleteURL := url + "/api/projects/" + projectID + "/webhook/policies/" + policyID

	req, err := http.NewRequest("DELETE", webhookDeleteURL, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth("admin", harborCreds)
	_, err = client.Do(req)
	if err != nil {
		return err
	}
	log.Print("Successfully deleted webhook.")
	return nil
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

type Payload struct {
	Type      string     `json:"type"`
	OccurAt   int64      `json:"occur_at"`
	Operator  string     `json:"operator"`
	EventData *EventData `json:"event_data,omitempty"`
}

type EventData struct {
	Resources  []*Resource       `json:"resources"`
	Repository *Repository       `json:"repository"`
	Custom     map[string]string `json:"custom_attributes,omitempty"`
}

// Resource describe infos of resource triggered notification
type Resource struct {
	Digest       string                 `json:"digest,omitempty"`
	Tag          string                 `json:"tag"`
	ResourceURL  string                 `json:"resource_url,omitempty"`
	ScanOverview map[string]interface{} `json:"scan_overview,omitempty"`
}

// Repository info of notification event
type Repository struct {
	DateCreated  int64  `json:"date_created,omitempty"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	RepoFullName string `json:"repo_full_name"`
	RepoType     string `json:"repo_type"`
}

const (
	HarborSeverityCritical   = "Critical"
	HarborSeverityHigh       = "High"
	HarborSeverityMedium     = "Medium"
	HarborSeverityLow        = "Low"
	HarborSeverityNone       = "None"
	HarborSeverityUnknown    = "Unknown"
	HarborSeverityNegligible = "Negligible"
)
