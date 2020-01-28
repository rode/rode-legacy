package occurrence

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/go-logr/logr"

	"google.golang.org/grpc/codes"

	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	project "github.com/grafeas/grafeas/proto/v1beta1/project_go_proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type grafeasClient struct {
	log       logr.Logger
	client    grafeas.GrafeasV1Beta1Client
	projectID string
}

// GrafeasClient handle into grafeas
type GrafeasClient interface {
	Creator
	Lister
}

// NewGrafeasClient creates a new client
func NewGrafeasClient(log logr.Logger, endpoint string) GrafeasClient {
	log.Info("Using Grafeas endpoint", endpoint)

	grpcDialOption, err := newGRPCDialOption(log)
	if err != nil {
		log.Error(err, "Unable to configure grafeas client")
		return nil
	}
	conn, err := grpc.Dial(endpoint, grpcDialOption)

	client := grafeas.NewGrafeasV1Beta1Client(conn)
	c := &grafeasClient{
		log,
		client,
		"rode",
	}

	projectClient := project.NewProjectsClient(conn)
	c.initProject(context.Background(), projectClient)

	return c
}

// ListOccurrences will get the occurence for a resource
func (c *grafeasClient) ListOccurrences(ctx context.Context, resourceURI string) (*grafeas.ListOccurrencesResponse, error) {
	c.log.Info("Get occurrences for resource '%s'", resourceURI)

	resp, err := c.client.ListOccurrences(ctx, &grafeas.ListOccurrencesRequest{
		Parent:   fmt.Sprintf("projects/%s", c.projectID),
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

// CreateOccurrences will save the occurence in grafeas
func (c *grafeasClient) CreateOccurrences(ctx context.Context, occurrences ...*grafeas.Occurrence) error {
	if occurrences == nil || len(occurrences) == 0 {
		return nil
	}
	_, err := c.client.BatchCreateOccurrences(ctx, &grafeas.BatchCreateOccurrencesRequest{
		Occurrences: occurrences,
		Parent:      fmt.Sprintf("projects/%s", c.projectID),
	})
	return err
}

func (c *grafeasClient) initProject(ctx context.Context, projectClient project.ProjectsClient) error {
	c.log.Info("Fetching project", c.projectID)
	_, err := projectClient.GetProject(ctx, &project.GetProjectRequest{
		Name: c.projectID,
	})
	if err != nil && grpc.Code(err) == codes.NotFound {
		c.log.Info("Creating project", c.projectID)
		_, err = projectClient.CreateProject(ctx, &project.CreateProjectRequest{
			Project: &project.Project{
				Name: c.projectID,
			},
		})
	}
	return err
}

func newGRPCDialOption(log logr.Logger) (grpc.DialOption, error) {
	clientCert, err := tls.LoadX509KeyPair(os.Getenv("TLS_CLIENT_CERT"), os.Getenv("TLS_CLIENT_KEY"))
	if err != nil {
		log.Error(err, "Unable to load client cert")
		return nil, err
	}

	cf, err := ioutil.ReadFile(os.Getenv("TLS_CA_CERT"))
	if err != nil {
		log.Error(err, "Unable to load CA cert")
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
