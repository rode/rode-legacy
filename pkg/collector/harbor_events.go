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
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
)

type HarborEventCollector struct {
	logger    logr.Logger
	url       string
	secret    *corev1.Secret
	project   string
	namespace string
	hostname  *v1beta1.Ingress
}

func NewHarborEventCollector(logger logr.Logger, harborURL string, secret *corev1.Secret, project string, namespace string, hostname *v1beta1.Ingress) Collector {
	return &HarborEventCollector{
		logger:    logger,
		url:       harborURL,
		secret:    secret,
		project:   project,
		namespace: namespace,
		hostname:  hostname,
	}
}

func (t *HarborEventCollector) Reconcile(ctx context.Context, name types.NamespacedName) error {
	harborCreds, err := t.getHarborCredentials(t.secret)
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
		//TODO: Assuming ingress is called rode deployed into rode namespace
		hostName, err := t.getHarborIngress(t.hostname)
		if err != nil {
			return err
		}
		webhookURL := fmt.Sprintf("%s/webhook/harbor_event/%s", hostName, name.String())
		err = t.createWebhook(projectID, t.url, harborCreds, webhookURL)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *HarborEventCollector) Destroy(ctx context.Context) error {
	t.logger.Info("destroying test collector")
	harborCreds, err := t.getHarborCredentials(t.secret)
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
	err = t.deleteWebhookPolicy(projectID, t.url, policyID, harborCreds)
	if err != nil {
		return err
	}
	return nil
}

func (t *HarborEventCollector) Type() string {
	return "harbor"
}

func (t *HarborEventCollector) HandleWebhook(writer http.ResponseWriter, request *http.Request, occurrenceCreator occurrence.Creator) {
	var payload *payload
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Fatal(err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

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

func (t *HarborEventCollector) newImagePushOccurrences(resources []*imageResource) []*grafeas.Occurrence {
	occurrences := make([]*grafeas.Occurrence, 0)
	for i, resource := range resources {
		baseResourceURL := resource.ResourceURL
		derivedImageDetails := &grafeas.Occurrence_DerivedImage{
			DerivedImage: &image.Details{
				DerivedImage: &image.Derived{
					BaseResourceUrl: baseResourceURL,
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

func (t *HarborEventCollector) newImageScanOccurrences(resources []*imageResource) []*grafeas.Occurrence {
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

func newHarborImageScanOccurrence(resource *imageResource, projectName string) *grafeas.Occurrence {
	o := &grafeas.Occurrence{
		Resource: &grafeas.Resource{
			Uri: harborOccurrenceResourceURI(resource.ResourceURL, resource.Digest),
		},
		NoteName: harborOccurrenceNote(projectName),
	}
	return o
}

func harborOccurrenceResourceURI(url, digest string) string {
	return fmt.Sprintf("%s@%s", url, digest)
}

func harborOccurrenceNote(projectName string) string {
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
func (t *HarborEventCollector) getHarborCredentials(secrets *corev1.Secret) (string, error) {
	return string(secrets.Data["HARBOR_ADMIN_PASSWORD"]), nil
}

func (t *HarborEventCollector) getHarborIngress(ingresses *v1beta1.Ingress) (string, error) {
	return fmt.Sprintf("https://%s", ingresses.Spec.Rules[0].Host), nil
}

func (t *HarborEventCollector) getProjectID(name string, url string) (string, error) {
	client := &http.Client{}
	var projects []project
	var projectID string
	projectEndpointAPI := fmt.Sprintf("%s/api/projects/", url)

	req, err := http.NewRequest("GET", projectEndpointAPI, nil)
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

	err = json.Unmarshal(projectList, &projects)
	if err != nil {
		return "", err
	}

	for _, p := range projects {
		if p.Name == name {
			projectID = strconv.Itoa(p.ProjectID)
		}
	}
	return projectID, nil
}

func webhookPoliciesEndpoint(url string, projectID string) string {
	return fmt.Sprintf("%s/api/projects/%s/webhook/policies", url, projectID)
}

func (t *HarborEventCollector) getWebhookPolicyID(projectID string, url string, harborCreds string) (string, error) {
	client := &http.Client{}
	webhookURL := webhookPoliciesEndpoint(url, projectID)
	var policies []webhookPolicies

	req, err := http.NewRequest("GET", webhookURL, nil)
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

	err = json.Unmarshal(policyList, &policies)
	if err != nil {
		return "", err
	}

	policyID := strconv.Itoa(policies[0].ID)
	return policyID, nil
}

func (t *HarborEventCollector) checkForWebhook(projectID string, url string, harborCreds string) (bool, error) {
	client := &http.Client{}
	webhookURL := webhookPoliciesEndpoint(url, projectID)
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

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return true, err
	}

	_ = json.Unmarshal(body, &webhooks)
	/*
		  TODO : body can't be unmarshalled into a string array
			if err != nil {
				return true, err
			}
	*/

	if len(webhooks) == 0 {
		return false, nil
	}
	return true, nil
}

func (t *HarborEventCollector) createWebhook(projectID string, url string, harborCreds string, webhookEndpoint string) error {
	client := &http.Client{}
	webhookURL := webhookPoliciesEndpoint(url, projectID)
	webhooks := []targets{
		targets{
			Type:           "http",
			Address:        webhookEndpoint,
			AuthHeader:     "auth_header",
			SkipCertVerify: true,
		},
	}
	body := &webhookPolicies{
		Targets: webhooks,
		EventTypes: []string{
			"pushImage",
			"scanningFailed",
			"scanningCompleted",
		},
		Enabled: true,
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

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
	webhookDeleteURL := fmt.Sprintf("%s/api/projects/%s/webhook/policies/%s", url, projectID, policyID)

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
type project struct {
	ProjectID int    `json:"project_id"`
	Name      string `json:"name"`
}

// Harbor structured project
type webhookPolicies struct {
	Targets    []targets `json:"targets,omitempty"`
	EventTypes []string  `json:"event_types,omitempty"`
	Enabled    bool      `json:"enabled,omitempty"`
	ID         int       `json:"id,omitempty"`
}

type targets struct {
	Type           string `json:"type"`
	Address        string `json:"address"`
	AuthHeader     string `json:"auth_header"`
	SkipCertVerify bool   `json:"skip_cert_verify"`
}

type payload struct {
	Type      string     `json:"type"`
	OccurAt   int64      `json:"occur_at"`
	Operator  string     `json:"operator"`
	EventData *eventData `json:"event_data,omitempty"`
}

type eventData struct {
	Resources []*imageResource  `json:"resources"`
	Custom    map[string]string `json:"custom_attributes,omitempty"`
}

// Resource describe infos of resource triggered notification
type imageResource struct {
	Digest       string                 `json:"digest,omitempty"`
	Tag          string                 `json:"tag"`
	ResourceURL  string                 `json:"resource_url,omitempty"`
	ScanOverview map[string]interface{} `json:"scan_overview,omitempty"`
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
