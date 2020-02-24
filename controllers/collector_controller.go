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
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/go-logr/logr"
	"github.com/liatrio/rode/api/util"
	"github.com/liatrio/rode/pkg/collector"
	"github.com/liatrio/rode/pkg/occurrence"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	WebhookHandlers   map[string]func(writer http.ResponseWriter, request *http.Request, occurrenceCreator occurrence.Creator)
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

// nolint: gocyclo
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
		case "ecr":
			c = collector.NewEcrEventCollector(r.Log, r.AWSConfig, col.Spec.ECR.QueueName)
		case "harbor":
			secret, err := r.getHarborSecret(col.Spec.Harbor.Secret, ctx)
			if err != nil {
				return ctrl.Result{}, err
			}
			ingress, err := r.getHarborIngress("rode", "rode", ctx)
			if err != nil {
				log.Info("Ingress doesn't exist or isn't properly configured; proceeding without ingress data")
				ingress = &v1beta1.Ingress{}
			}
			c = collector.NewHarborEventCollector(r.Log, col.Spec.Harbor.HarborURL, secret, col.Spec.Harbor.Project, col.ObjectMeta.Namespace, ingress)
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

		if collectorWorker, ok := r.Workers[req.NamespacedName.String()]; ok {
			if _, ok := c.(collector.StartableCollector); ok {
				collectorWorker.done()
				<-collectorWorker.stopChan
			}

			delete(r.Workers, req.NamespacedName.String())
		}

		delete(r.WebhookHandlers, webhookHandlerPath(c, req))

		var err error
		if collectorWorker.context != nil {
			err = c.Destroy(collectorWorker.context)
		} else {
			err = c.Destroy(ctx)
		}

		if err != nil {
			return r.setCollectorActive(ctx, col, err)
		}

		log.Info("removing finalizer")

		col.ObjectMeta.Finalizers = removeFinalizer(col.ObjectMeta.Finalizers, collectorFinalizerName)
		err = r.Update(context.Background(), col)

		return ctrl.Result{}, err
	}

	err = c.Reconcile(ctx, req.NamespacedName)
	if err != nil {
		log.Error(err, "error reconciling collector")
		return r.setCollectorActive(ctx, col, err)
	}

	if collectorExists {
		return r.setCollectorActive(ctx, col, nil)
	}

	if webhookCollector, ok := c.(collector.WebhookCollector); ok {
		r.WebhookHandlers[webhookHandlerPath(c, req)] = webhookCollector.HandleWebhook
		collectorWorker = &CollectorWorker{
			context:   nil,
			collector: &c,
			stopChan:  nil,
			done:      nil,
		}
	}

	if startableCollector, ok := c.(collector.StartableCollector); ok {
		workerContext, cancel := context.WithCancel(ctx)
		collectorWorker = &CollectorWorker{
			context:   workerContext,
			collector: &c,
			stopChan:  make(chan interface{}),
			done:      cancel,
		}

		err = startableCollector.Start(collectorWorker.context, collectorWorker.stopChan, r.OccurrenceCreator)
		if err != nil {
			log.Error(err, "error starting collector")
			return r.setCollectorActive(ctx, col, err)
		}
	}

	r.Workers[req.NamespacedName.String()] = collectorWorker

	return r.setCollectorActive(ctx, col, nil)
}

func (r *CollectorReconciler) getHarborSecret(ctx context.Context, harborSecret string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	secretDetails := strings.Split(harborSecret, "/")
	secretNamespace := secretDetails[0]
	secretName := secretDetails[1]
	secretInfo := types.NamespacedName{
		Name:      secretName,
		Namespace: secretNamespace,
	}
	err := r.Get(ctx, secretInfo, secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func (r *CollectorReconciler) getHarborIngress(ctx context.Context, ingressName string, ingressNamespace string) (*v1beta1.Ingress, error) {
	ingress := &v1beta1.Ingress{}
	ingressInfo := types.NamespacedName{
		Name:      ingressName,
		Namespace: ingressNamespace,
	}
	err := r.Get(ctx, ingressInfo, ingress)
	if err != nil {
		return nil, err
	}

	return ingress, nil
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

func webhookHandlerPath(c collector.Collector, req ctrl.Request) string {
	return fmt.Sprintf("webhook/%s/%s/%s", c.Type(), req.Namespace, req.Name)
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
