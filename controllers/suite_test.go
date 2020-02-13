/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	"google.golang.org/grpc"
	grpcCredentials "google.golang.org/grpc/credentials"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	rodev1alpha1 "github.com/liatrio/rode/api/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg           *rest.Config
	awsConfig     *aws.Config
	k8sClient     client.Client
	testEnv       *envtest.Environment
	grafeasClient grafeas.GrafeasV1Beta1Client
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{envtest.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")

	useExistingCluster := true
	testEnv = &envtest.Environment{
		UseExistingCluster: &useExistingCluster,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	awsConfig = localstackAWSConfig()

	err = rodev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())

	tlsConfig, err := testGrafeasTlsConfig(context.Background(), k8sClient)
	Expect(err).ToNot(HaveOccurred())

	grafeasClient, err = testGrafeasClient(tlsConfig)
	Expect(err).ToNot(HaveOccurred())
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func SetupTestNamespace(ctx context.Context) *corev1.Namespace {
	ns := &corev1.Namespace{}

	BeforeEach(func() {
		*ns = corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Name: fmt.Sprintf("rode-test-%s", rand.String(10)),
			},
		}

		err := k8sClient.Create(ctx, ns)
		Expect(err).ToNot(HaveOccurred(), "failed to create rode test namespace")
	})

	AfterEach(func() {
		err := k8sClient.Delete(ctx, ns)
		Expect(err).NotTo(HaveOccurred(), "failed to delete rode test namespace")
	})

	return ns
}

func localstackAWSConfig() *aws.Config {
	cfg := &aws.Config{}
	cfg.Region = aws.String(endpoints.UsEast1RegionID)
	cfg.Credentials = credentials.AnonymousCredentials

	localstackEndpoints := map[string]string{
		"sqs":    "http://localhost:30576",
		"events": "http://localhost:30587",
	}

	cfg.EndpointResolver = endpoints.ResolverFunc(func(service, region string, optFns ...func(options *endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
		if endpoint, ok := localstackEndpoints[strings.ToLower(service)]; ok {
			return endpoints.ResolvedEndpoint{
				URL: endpoint,
			}, nil
		}

		return endpoints.DefaultResolver().EndpointFor(service, region, optFns...)
	})

	return cfg
}

func testGrafeasTlsConfig(ctx context.Context, k8sClient client.Client) (*tls.Config, error) {
	grafeasTlsSecret := corev1.Secret{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Namespace: "default",
		Name:      "grafeas-ssl-certs",
	}, &grafeasTlsSecret)
	if err != nil {
		return nil, err
	}

	clientCert, err := tls.X509KeyPair(grafeasTlsSecret.Data["tls.crt"], grafeasTlsSecret.Data["tls.key"])
	if err != nil {
		return nil, err
	}

	caCert := grafeasTlsSecret.Data["ca.crt"]

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}

func testGrafeasClient(tlsConfig *tls.Config) (grafeas.GrafeasV1Beta1Client, error) {
	conn, err := grpc.Dial("localhost:30443", grpc.WithTransportCredentials(grpcCredentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, err
	}

	return grafeas.NewGrafeasV1Beta1Client(conn), nil
}
