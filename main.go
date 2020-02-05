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

package main

import (
	"flag"
	"net/http"
	"os"
	"strings"

	"github.com/liatrio/rode/pkg/enforcer"

	rodev1 "github.com/liatrio/rode/api/v1"
	"github.com/liatrio/rode/controllers"
	"github.com/liatrio/rode/pkg/attester"
	"github.com/liatrio/rode/pkg/aws"
	"github.com/liatrio/rode/pkg/occurrence"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = rodev1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var healthAddr string
	var certDir string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&healthAddr, "health-addr", ":4000", "The address the health endpoint binds to.")
	flag.StringVar(&certDir, "cert-dir", "/certificates", "The path to tls certificates.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = true
	}))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		HealthProbeBindAddress: healthAddr,
		CertDir:                certDir,
		LeaderElection:         enableLeaderElection,
		Port:                   9443,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	attesters := &controllers.AttesterReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Attester"),
		Scheme: mgr.GetScheme(),
	}
	if err = attesters.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Attester")
		os.Exit(1)
	}

	awsConfig := aws.NewAWSConfig(ctrl.Log.WithName("aws").WithName("AWSConfig"))
	grafeasClient := occurrence.NewGrafeasClient(ctrl.Log.WithName("occurrence").WithName("GrafeasClient"), os.Getenv("GRAFEAS_ENDPOINT"))
	occurrenceCreator := attester.NewAttestWrapper(ctrl.Log.WithName("attester").WithName("AttestWrapper"), grafeasClient, grafeasClient, attesters)

	if err = (&controllers.CollectorReconciler{
		Client:            mgr.GetClient(),
		Log:               ctrl.Log.WithName("controllers").WithName("Collector"),
		AWSConfig:         awsConfig,
		OccurrenceCreator: occurrenceCreator,
		Workers:           make(map[string]*controllers.CollectorWorker),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Collector")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	excludeNS := strings.Split(os.Getenv("EXCLUDED_NAMESPACES"), ",")
	enforcer := enforcer.NewEnforcer(ctrl.Log.WithName("enforcer"), excludeNS, attesters, grafeasClient, mgr.GetClient())

	// TODO: add webhook route

	checker := func(req *http.Request) error {
		return nil
	}

	mgr.AddHealthzCheck("test", checker)
	mgr.AddReadyzCheck("test", checker)
	mgr.GetWebhookServer().Register("/validate", enforcer)

	// TODO: add occurrences route

	// TODO: setup TLS for endpoints

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
