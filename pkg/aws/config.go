package aws

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
)

// NewAWSConfig creates the AWS config
func NewAWSConfig(log logr.Logger) *aws.Config {
	cfg := &aws.Config{}
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = endpoints.UsEast1RegionID
	}
	cfg.Region = aws.String(region)

	customResolver := func(service, region string, optFns ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
		endpoint := os.Getenv(fmt.Sprintf("AWS_%s_ENDPOINT", strings.ToUpper(service)))
		log.Info("mapping service '%s' to endpoint '%s'", service, endpoint)
		if endpoint != "" {
			return endpoints.ResolvedEndpoint{
				URL: fmt.Sprintf("http://%s", endpoint),
				//SigningRegion: "custom-signing-region",
			}, nil
		}

		return endpoints.DefaultResolver().EndpointFor(service, region, optFns...)
	}
	cfg.EndpointResolver = endpoints.ResolverFunc(customResolver)

	session := session.Must(session.NewSession(cfg))
	svc := sts.New(session)
	result, err := svc.GetCallerIdentity(nil)
	if err != nil {
		log.Error(err, "Error getting caller identity")
	} else {
		log.Info("Finished AWS Identity", result)
	}

	return cfg
}
