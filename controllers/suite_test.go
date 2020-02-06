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
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/liatrio/rode/pkg/attester"
	"github.com/liatrio/rode/pkg/occurrence"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	rodev1 "github.com/liatrio/rode/api/v1"
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

var cfg *rest.Config
var awsConfig *aws.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{envtest.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = rodev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred(), "failed to create rode test manager")

	attesterReconciler := &AttesterReconciler{
		Client: mgr.GetClient(),
		Log:    logf.Log,
		Scheme: mgr.GetScheme(),
	}
	err = attesterReconciler.SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred(), "failed to setup rode attester reconciler")

	grafeasClient := occurrence.NewGrafeasClient(ctrl.Log.WithName("occurrence").WithName("GrafeasClient"), "http://localhost:30443")
	occurrenceCreator := attester.NewAttestWrapper(ctrl.Log.WithName("attester").WithName("AttestWrapper"), grafeasClient, grafeasClient, attesterReconciler)

	awsConfig = localstackAWSConfig()

	err = (&CollectorReconciler{
		Client:            mgr.GetClient(),
		Log:               logf.Log,
		Scheme:            mgr.GetScheme(),
		AWSConfig:         awsConfig,
		OccurrenceCreator: occurrenceCreator,
		Workers:           make(map[string]*CollectorWorker),
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred(), "failed to setup rode collector reconciler")

	go func() {
		err := mgr.Start(ctrl.SetupSignalHandler())
		Expect(err).NotTo(HaveOccurred(), "failed to start rode test manager")
	}()

	k8sClient = mgr.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	close(done)
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
