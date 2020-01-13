package aws

import (
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/endpoints"
	"github.com/aws/aws-sdk-go-v2/aws/external"
)

// NewAWSConfig creates the AWS config
func NewAWSConfig() *aws.Config {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		panic("unable to load SDK config, " + err.Error())
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = endpoints.UsEast1RegionID
	}
	cfg.Region = region

	defaultResolver := endpoints.NewDefaultResolver()
	customResolver := func(service, region string) (aws.Endpoint, error) {
		endpoint := os.Getenv(fmt.Sprintf("AWS_%s_ENDPOINT", strings.ToUpper(service)))
		if endpoint != "" {
			return aws.Endpoint{
				URL:           fmt.Sprintf("http://%s", endpoint),
				SigningRegion: "custom-signing-region",
			}, nil
		}

		return defaultResolver.ResolveEndpoint(service, region)
	}
	cfg.EndpointResolver = aws.EndpointResolverFunc(customResolver)

	return &cfg
}
