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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/go-logr/logr"
	"github.com/liatrio/rode/api/util"
	"github.com/liatrio/rode/pkg/collector"
	"github.com/liatrio/rode/pkg/occurrence"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rodev1alpha1 "github.com/liatrio/rode/api/v1alpha1"
)

// CollectorReconciler reconciles a Collector object
type CollectorReconciler struct {
	client.Client
	Log               logr.Logger
	Scheme            *runtime.Scheme
	AWSConfig         *aws.Config
	OccurrenceCreator occurrence.Creator
	Workers           map[string]*CollectorWorker
}

type CollectorWorker struct {
	context   context.Context
	collector *collector.Collector
	stopChan  chan interface{}
	done      context.CancelFunc
}

var (
	collectorFinalizerName = "collectors.finalizers.rode.liatr.io"
)

// +kubebuilder:rbac:groups=rode.liatr.io,resources=collectors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rode.liatr.io,resources=collectors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

func (r *CollectorReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("collector", req.NamespacedName)

	log.Info("Reconciling collector")

	col := &rodev1alpha1.Collector{}
	err := r.Get(ctx, req.NamespacedName, col)
	if err != nil {
		log.Error(err, "Unable to load collector")
		return ctrl.Result{}, err
	}

	err = r.registerFinalizer(log, col)
	if err != nil {
		log.Error(err, "error registering finalizer")
		return ctrl.Result{}, err
	}

	var c collector.Collector
	collectorWorker, collectorExists := r.Workers[req.NamespacedName.String()]
	if collectorExists {
		c = *collectorWorker.collector
	} else {
		switch col.Spec.CollectorType {
		case "ecr_event":
			c = collector.NewEcrEventCollector(r.Log, r.AWSConfig, col.Spec.ECR.QueueName)
		case "test":
			c = collector.NewTestCollector(r.Log, "foo")
		default:
			err = errors.New("Unknown collector type")
			// Loud output when erroring, getting more reconciles than expected.
			log.Error(err, "Unknown collector type")
			return ctrl.Result{}, err
		}
	}

	if !col.ObjectMeta.DeletionTimestamp.IsZero() && containsFinalizer(col.ObjectMeta.Finalizers, collectorFinalizerName) {
		log.Info("stopping worker")

		collectorWorker, ok := r.Workers[req.NamespacedName.String()]
		if ok {
			collectorWorker.done()
			<-collectorWorker.stopChan

			delete(r.Workers, req.NamespacedName.String())
		} else {
			log.Info("worker not found for collector", "collector", req.NamespacedName.String())
		}

		err := c.Destroy(collectorWorker.context)
		if err != nil {
			return r.setCollectorActive(ctx, col, err)
		}

		log.Info("removing finalizer")

		col.ObjectMeta.Finalizers = removeFinalizer(col.ObjectMeta.Finalizers, collectorFinalizerName)
		err = r.Update(context.Background(), col)

		return ctrl.Result{}, err
	}

	err = c.Reconcile(ctx)
	if err != nil {
		log.Error(err, "error reconciling collector")
		return r.setCollectorActive(ctx, col, err)
	}

	if collectorExists {
		return r.setCollectorActive(ctx, col, nil)
	}

	workerContext, cancel := context.WithCancel(ctx)
	collectorWorker = &CollectorWorker{
		context:   workerContext,
		collector: &c,
		stopChan:  make(chan interface{}),
		done:      cancel,
	}

	err = c.Start(collectorWorker.context, collectorWorker.stopChan, r.OccurrenceCreator)
	if err != nil {
		log.Error(err, "error starting collector")
		return r.setCollectorActive(ctx, col, err)
	}

	r.Workers[req.NamespacedName.String()] = collectorWorker

	return r.setCollectorActive(ctx, col, nil)
}

func (r *CollectorReconciler) setCollectorActive(ctx context.Context, collector *rodev1alpha1.Collector, ctrlError error) (ctrl.Result, error) {
	var conditionStatus rodev1alpha1.ConditionStatus
	var conditionMessage string
	if ctrlError == nil {
		conditionStatus = rodev1alpha1.ConditionStatusTrue
	} else {
		conditionStatus = rodev1alpha1.ConditionStatusFalse
		conditionMessage = ctrlError.Error()
	}

	util.SetCollectorCondition(collector, rodev1alpha1.ConditionActive, conditionStatus, conditionMessage)
	err := r.Status().Update(ctx, collector)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "Unable to update collector status")
	}

	return ctrl.Result{}, errors.Wrap(ctrlError, "Setting controller to errored status")
}

func (r *CollectorReconciler) registerFinalizer(logger logr.Logger, collector *rodev1alpha1.Collector) error {
	if collector.ObjectMeta.DeletionTimestamp.IsZero() && !containsFinalizer(collector.ObjectMeta.Finalizers, collectorFinalizerName) {
		logger.Info("Creating collector finalizer...")
		collector.ObjectMeta.Finalizers = append(collector.ObjectMeta.Finalizers, collectorFinalizerName)

		if err := r.Update(context.Background(), collector); err != nil {
			return err
		}
	}

	return nil
}

func (r *CollectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rodev1alpha1.Collector{}).
		WithEventFilter(ignoreConditionStatusUpdateToActive(func(o runtime.Object) util.Conditioner {
			return o.(*rodev1alpha1.Collector)
		}, rodev1alpha1.ConditionActive)).
		WithEventFilter(ignoreFinalizerUpdate()).
		WithEventFilter(ignoreDelete()).
		Complete(r)
}
