package grafeas

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

	"google.golang.org/grpc/codes"

	attestation "github.com/grafeas/grafeas/proto/v1beta1/attestation_go_proto"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	project "github.com/grafeas/grafeas/proto/v1beta1/project_go_proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"go.uber.org/zap"
)

const projectId = "rode"

// Client handle into grafeas
type Client struct {
	logger        *zap.SugaredLogger
	grafeasClient grafeas.GrafeasV1Beta1Client
	projectClient project.ProjectsClient
}

// NewClient creates a new client
func NewClient(logger *zap.SugaredLogger, endpoint string) *Client {
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
	}
	c.getProject(fmt.Sprintf("projects/%s", projectId))
	return c
}

// GetOccurrences will get the occurence for a resource
func (c *Client) GetOccurrences(resourceURI string) ([]*grafeas.Occurrence, error) {
	ctx := context.TODO()
	c.logger.Debugf("Get occurrences for resource '%s'", resourceURI)

	resp, err := c.grafeasClient.ListOccurrences(ctx, &grafeas.ListOccurrencesRequest{
		Parent:   fmt.Sprintf("projects/%s", projectId),
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

	return occurrences, nil
}

// PutOccurrences will save the occurence in grafeas
func (c *Client) PutOccurrences(occurrences ...*grafeas.Occurrence) error {
	if occurrences == nil || len(occurrences) == 0 {
		return nil
	}
	ctx := context.TODO()

	_, err := c.grafeasClient.BatchCreateOccurrences(ctx, &grafeas.BatchCreateOccurrencesRequest{
		Occurrences: occurrences,
		Parent:      fmt.Sprintf("projects/%s", projectId),
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
	c.logger.Infof("Attesting resource %s", resourceURI)
	ctx := context.TODO()
	uriOccurrences, err := c.GetOccurrences(resourceURI)
	if err != nil {
		c.logger.Errorf("Unable to attempt attestation for occurrence %v", err)
		return err
	}

	var pass bool

	// TODO: call OPA
	pass = len(uriOccurrences) > 0

	// TODO: load from attester configs
	attesterName := "opa"

	if pass {
		// TODO: sign the resourceURI
		sig := resourceURI
		keyID := attesterName

		attestOccurrence := &grafeas.Occurrence{}
		attestOccurrence.NoteName = fmt.Sprintf("projects/%s/notes/%s", projectId, attesterName)
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
			Parent:     fmt.Sprintf("projects/%s", projectId),
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
