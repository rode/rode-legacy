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
	"github.com/pkg/errors"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/liatrio/rode/pkg/collector"
	"github.com/liatrio/rode/pkg/occurrence"

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
	Workers           map[string]chan interface{}
}

var (
	collectorFinalizerName = "collectors.finalizers.rode.liatr.io"
)

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
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	err = r.registerFinalizer(log, col)
	if err != nil {
		log.Error(err, "error registering finalizer")
		return ctrl.Result{}, err
	}

	if !col.ObjectMeta.DeletionTimestamp.IsZero() && containsFinalizer(col.ObjectMeta.Finalizers, collectorFinalizerName) {
		log.Info("stopping worker")

		close(r.Workers[req.NamespacedName.String()])
		delete(r.Workers, req.NamespacedName.String())

		log.Info("removing finalizer")

		col.ObjectMeta.Finalizers = removeFinalizer(col.ObjectMeta.Finalizers, collectorFinalizerName)
		err := r.Update(context.Background(), col)

		return ctrl.Result{}, err
	}

	var c collector.Collector
	switch col.Spec.CollectorType {
	case "ecr_event":
		c = collector.NewEcrEventCollector(r.Log, r.AWSConfig, r.OccurrenceCreator, col.Spec.ECR.QueueName)
	case "test":
		c = collector.NewTestCollector(r.Log, r.OccurrenceCreator, "foo")
	default:
		err = errors.New("Unknown collector type")
		// Loud output when erroring, getting more reconciles than expected.
		log.Error(err, "Unknown collector type")
		return r.setCollectorActive(ctx, col, err)
	}

	if _, exists := r.Workers[req.NamespacedName.String()]; exists {
		return r.setCollectorActive(ctx, col, nil)
	}

	w := worker(ctx, r.Log, c)
	r.Workers[req.NamespacedName.String()] = w

	return r.setCollectorActive(ctx, col, nil)
}

func (r *CollectorReconciler) setCollectorActive(ctx context.Context, collector *rodev1.Collector, ctrlError error) (ctrl.Result, error) {
	collector.Status.Active = ctrlError == nil
	err := r.Status().Update(ctx, collector)
	if err != nil {
		if ctrlError != nil {
			return ctrl.Result{}, errors.Wrap(ctrlError, err.Error())
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, ctrlError
}

func (r *CollectorReconciler) registerFinalizer(logger logr.Logger, collector *rodev1.Collector) error {
	if collector.ObjectMeta.DeletionTimestamp.IsZero() && !containsFinalizer(collector.ObjectMeta.Finalizers, collectorFinalizerName) {
		logger.Info("Creating collector finalizer...")
		collector.ObjectMeta.Finalizers = append(collector.ObjectMeta.Finalizers, collectorFinalizerName)

		if err := r.Update(context.Background(), collector); err != nil {
			return err
		}
	}

	return nil
}

func worker(ctx context.Context, logger logr.Logger, c collector.Collector) chan interface{} {
	ch := make(chan interface{})

	go func(ctx context.Context, logger logr.Logger, collector collector.Collector) {
		for range time.Tick(5 * time.Second) {
			select {
			case <-ch:
				return
			default:
				err := collector.Reconcile(ctx)
				if err != nil {
					logger.Error(err, "error reconciling collector")
				}
			}
		}
	}(ctx, logger, c)

	return ch
}

func (r *CollectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rodev1.Collector{}).
		Complete(r)
}
