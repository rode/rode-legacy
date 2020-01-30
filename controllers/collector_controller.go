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
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/liatrio/rode/pkg/occurrence"
	"github.com/liatrio/rode/pkg/collector"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rodev1 "github.com/liatrio/rode/api/v1"
)

// CollectorReconciler reconciles a Collector object
type CollectorReconciler struct {
	client.Client
	Log               logr.Logger
	Scheme            *runtime.Scheme
	AWSConfig         *aws.Config
	OccurrenceCreator occurrence.Creator
}

// +kubebuilder:rbac:groups=rode.liatr.io,resources=collectors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rode.liatr.io,resources=collectors/status,verbs=get;update;patch

func (r *CollectorReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("collector", req.NamespacedName)

	log.Info("Reconciling collector")

	col := &rodev1.Collector{}
	err := r.Get(ctx, req.NamespacedName, col)
	if err != nil {
		log.Error(err, "Unable to load collector")
		return ctrl.Result{}, err
	}

	var c collector.Collector
	switch col.Spec.CollectorType {
	case "ecr_event":
		c = collector.NewEcrEventCollector(r.Log, r.AWSConfig, r.OccurrenceCreator, col.Spec.ECR.QueueName)
	default:
		err = errors.New("Unknown collector type")
		// Loud output when erroring, getting more reconciles than expected.
		log.Error(err, "Unknown collector type")
		return ctrl.Result{}, err
	}
	err = c.Reconcile(ctx)
	if err != nil {
		log.Error(err, "Unable to load collector")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *CollectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rodev1.Collector{}).
		Complete(r)
}
