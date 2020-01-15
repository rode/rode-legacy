package grafeas

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"

	"google.golang.org/grpc/codes"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	attestation "github.com/grafeas/grafeas/proto/v1beta1/attestation_go_proto"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	project "github.com/grafeas/grafeas/proto/v1beta1/project_go_proto"
	"github.com/liatrio/rode/pkg/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"go.uber.org/zap"
)

const projectID = "rode"

// Client handle into grafeas
type Client struct {
	logger          *zap.SugaredLogger
	grafeasClient   grafeas.GrafeasV1Beta1Client
	projectClient   project.ProjectsClient
	policyEvaluator common.PolicyEvaluator
}

// NewClient creates a new client
func NewClient(logger *zap.SugaredLogger, policyEvaluator common.PolicyEvaluator, endpoint string) *Client {
	logger.Infof("Using Grafeas endpoint: %s", endpoint)

	grpcDialOption, err := newGRPCDialOption(logger)
	if err != nil {
		logger.Fatalf("Unable to configure grafeas client %v", err)
	}
	conn, err := grpc.Dial(endpoint, grpcDialOption)
	grafeasClient := grafeas.NewGrafeasV1Beta1Client(conn)
	projectClient := project.NewProjectsClient(conn)

	c := &Client{
		logger,
		grafeasClient,
		projectClient,
		policyEvaluator,
	}
	c.getProject(fmt.Sprintf("projects/%s", projectID))
	return c
}

// GetOccurrences will get the occurence for a resource
func (c *Client) GetOccurrences(resourceURI string) (*grafeas.ListOccurrencesResponse, error) {
	ctx := context.TODO()
	c.logger.Debugf("Get occurrences for resource '%s'", resourceURI)

	resp, err := c.grafeasClient.ListOccurrences(ctx, &grafeas.ListOccurrencesRequest{
		Parent:   fmt.Sprintf("projects/%s", projectID),
		Filter:   fmt.Sprintf("resource.uri = '%s'", resourceURI),
		PageSize: 1000,
	})

	if err != nil {
		return nil, err
	}

	// TODO: remove this hack...grafeas doesn't support filter yet
	occurrences := make([]*grafeas.Occurrence, 0, 0)
	for _, o := range resp.GetOccurrences() {
		if o.Resource.Uri == resourceURI {
			occurrences = append(occurrences, o)
		}
	}

	return &grafeas.ListOccurrencesResponse{
		Occurrences: occurrences,
	}, nil
}

// PutOccurrences will save the occurence in grafeas
func (c *Client) PutOccurrences(occurrences ...*grafeas.Occurrence) error {
	if occurrences == nil || len(occurrences) == 0 {
		return nil
	}
	ctx := context.TODO()

	_, err := c.grafeasClient.BatchCreateOccurrences(ctx, &grafeas.BatchCreateOccurrencesRequest{
		Occurrences: occurrences,
		Parent:      fmt.Sprintf("projects/%s", projectID),
	})
	if err != nil {
		c.logger.Errorf("Unable to put occurrence %v", err)
		return err
	}

	// perform attestations for each distinct resource
	visited := make(map[string]bool)
	for _, o := range occurrences {
		uri := o.Resource.Uri
		if !visited[uri] {
			visited[uri] = true
			err = c.attemptAttestation(uri)
			if err != nil {
				c.logger.Errorf("Error attesting %s -> %v", uri, err)
			}
		}
	}

	return nil
}

func (c *Client) attemptAttestation(resourceURI string) error {
	ctx := context.TODO()
	uriOccurrences, err := c.GetOccurrences(resourceURI)
	if err != nil {
		c.logger.Errorf("Unable to attempt attestation for occurrence %v", err)
		return err
	}

	// TODO: load from attester configs
	attesterName := "default_attester"

	// TODO: load from attester configs
	attesterRego := `
package default_attester

violation[{"msg":"analysis failed"}]{
	input.occurrences[_].discovered.discovered.analysisStatus != "FINISHED_SUCCESS"
}
violation[{"msg":"analysis not performed"}]{
	analysisStatus := [s | s := input.occurrences[_].discovered.discovered.analysisStatus]
	count(analysisStatus) = 0
}
violation[{"msg":"critical vulnerability found"}]{
	severityCount("CRITICAL") > 0
}
violation[{"msg":"high vulnerability found"}]{
	severityCount("HIGH") > 10
}
severityCount(severity) = cnt {
	cnt := count([v | v := input.occurrences[_].vulnerability.severity; v == severity])
}
`

	input := make(map[string]interface{})
	err = messageToMap(&input, uriOccurrences)
	if err != nil {
		c.logger.Errorf("Unable to prepare input for attestation of occurrence %v", err)
		return err
	}
	violations := c.policyEvaluator.Evaluate(attesterName, attesterRego, input)

	if len(violations) > 0 {
		c.logger.Infof("Unable to attest resource %s violations=%v", resourceURI, violations)
	} else {
		c.logger.Infof("Attesting resource %s", resourceURI)

		// TODO: sign the resourceURI
		sig := resourceURI
		keyID := attesterName

		attestOccurrence := &grafeas.Occurrence{}
		attestOccurrence.NoteName = fmt.Sprintf("projects/%s/notes/%s", projectID, attesterName)
		attestOccurrence.Resource = &grafeas.Resource{Uri: resourceURI}
		attestOccurrence.Details = &grafeas.Occurrence_Attestation{
			Attestation: &attestation.Details{
				Attestation: &attestation.Attestation{
					Signature: &attestation.Attestation_PgpSignedAttestation{
						PgpSignedAttestation: &attestation.PgpSignedAttestation{
							ContentType: attestation.PgpSignedAttestation_CONTENT_TYPE_UNSPECIFIED,
							Signature:   sig,
							KeyId: &attestation.PgpSignedAttestation_PgpKeyId{
								PgpKeyId: keyID,
							},
						},
					},
				},
			},
		}
		_, err := c.grafeasClient.CreateOccurrence(ctx, &grafeas.CreateOccurrenceRequest{
			Occurrence: attestOccurrence,
			Parent:     fmt.Sprintf("projects/%s", projectID),
		})
		if err != nil {
			c.logger.Errorf("Unable to store attestation for occurrence %v", err)
			return err
		}
	}
	return nil
}

func (c *Client) getProject(name string) *project.Project {
	ctx := context.TODO()
	re := regexp.MustCompile(`^(projects/[^\/]+)`)
	matches := re.FindSubmatch([]byte(name))
	var p *project.Project
	var err error
	if len(matches) > 1 {
		projectName := string(matches[1])
		p, err = c.projectClient.GetProject(ctx, &project.GetProjectRequest{
			Name: projectName,
		})
		if err != nil {
			if grpc.Code(err) == codes.NotFound {
				c.logger.Infof("Creating project %s", projectName)
				p, err = c.projectClient.CreateProject(ctx, &project.CreateProjectRequest{
					Project: &project.Project{
						Name: projectName,
					},
				})
				if err != nil {
					c.logger.Errorf("Unable to create project %v", err)
				}
			} else {
				c.logger.Errorf("Unable to get project %v", err)
			}
		}
	}
	if p == nil {
		p = &project.Project{}
	}
	c.logger.Debugf("Got project %#v", p)
	return p
}

func newGRPCDialOption(logger *zap.SugaredLogger) (grpc.DialOption, error) {
	clientCert, err := tls.LoadX509KeyPair(os.Getenv("TLS_CLIENT_CERT"), os.Getenv("TLS_CLIENT_KEY"))
	if err != nil {
		logger.Errorf("Unable to load client cert %v", err)
		return nil, err
	}

	cf, err := ioutil.ReadFile(os.Getenv("TLS_CA_CERT"))
	if err != nil {
		logger.Errorf("Unable to load CA cert %v", err)
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(cf)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
		//InsecureSkipVerify: true,
	}
	tlsConfig.BuildNameToCertificate()
	creds := credentials.NewTLS(tlsConfig)
	return grpc.WithTransportCredentials(creds), nil
}

func messageToJSON(jsonWriter io.Writer, pb proto.Message) error {
	marshaler := &jsonpb.Marshaler{}
	return marshaler.Marshal(jsonWriter, pb)
}

func messageToMap(destMap *map[string]interface{}, pb proto.Message) error {
	buf := new(bytes.Buffer)
	err := messageToJSON(buf, pb)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf.Bytes(), destMap)
}
