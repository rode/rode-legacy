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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rodev1alpha1 "github.com/liatrio/rode/api/v1alpha1"
	"github.com/liatrio/rode/pkg/eventmanager"
)

const EnforcerFinalizer = "enforcer.finalizers.rode.liatr.io"

// EnforcerReconciler reconciles a Enforcer object
type EnforcerReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	EventManager eventmanager.EventManager
}

func (r *EnforcerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithName("Reconcile()").WithValues("enforcer", req.NamespacedName)

	log.Info("Reconcile start")

	enforcer, err := r.GetEnforcer(ctx, req)
	if err != nil {
		log.Info("Could not fetch enforcer resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if enforcer.GetObjectMeta().GetDeletionTimestamp().IsZero() {
		if !containsFinalizer(enforcer.GetObjectMeta().GetFinalizers(), EnforcerFinalizer) {
			r.Log.Info("Adding finalizer")
			enforcer.GetObjectMeta().SetFinalizers(append(enforcer.GetObjectMeta().GetFinalizers(), EnforcerFinalizer))
			err = r.Update(ctx, enforcer.(runtime.Object))
			if err != nil {
				r.Log.Error(err, "Error updating enforcer", "enforcer", enforcer)
				return ctrl.Result{}, err
			}
		}
	} else {
		if containsFinalizer(enforcer.GetObjectMeta().GetFinalizers(), EnforcerFinalizer) {
			r.Log.Info("Unsubscribing from attesters")
			for _, attester := range enforcer.Attesters() {
				if err = r.EventManager.Unsubscribe(types.NamespacedName{Name: attester.Name, Namespace: attester.Namespace}.String()); err != nil {
					return ctrl.Result{}, err
				}
			}

			r.Log.Info("Removing finalizer")
			enforcer.GetObjectMeta().SetFinalizers(removeFinalizer(enforcer.GetObjectMeta().GetFinalizers(), EnforcerFinalizer))
			if err = r.Update(ctx, enforcer.(runtime.Object)); err != nil {
				r.Log.Error(err, "Error updating enforcer", "enforcer", enforcer)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if listenerStatus := enforcer.GetConditionStatus(rodev1alpha1.ConditionListener); listenerStatus != rodev1alpha1.ConditionStatusTrue {
		for _, attester := range enforcer.Attesters() {
			if err = r.EventManager.Subscribe(types.NamespacedName{Name: attester.Name, Namespace: attester.Namespace}.String()); err != nil {
				log.Error(err, "Error subscribing to stream")
				enforcer.SetCondition(rodev1alpha1.ConditionListener, rodev1alpha1.ConditionStatusFalse, "Subscribe to attester messages failed")
				if updateErr := r.Update(ctx, enforcer.(runtime.Object)); err != nil {
					log.Error(updateErr, "Error updating enforcer status")
					return ctrl.Result{}, updateErr
				}
				return ctrl.Result{}, err
			}

			enforcer.SetCondition(rodev1alpha1.ConditionListener, rodev1alpha1.ConditionStatusTrue, "Subscribe to attester message succeeded")
			if err = r.Update(ctx, enforcer.(runtime.Object)); err != nil {
				log.Error(err, "Error updating enforcer status")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *EnforcerReconciler) GetEnforcer(ctx context.Context, req ctrl.Request) (rodev1alpha1.EnforcerInterface, error) {
	var enforcer rodev1alpha1.EnforcerInterface
	if req.Namespace == "" {
		enforcer = &rodev1alpha1.ClusterEnforcer{}
	} else {
		enforcer = &rodev1alpha1.Enforcer{}
	}

	err := r.Get(ctx, req.NamespacedName, enforcer.(runtime.Object))
	if err != nil {
		return nil, err
	}
	return enforcer, nil
}

func (r *EnforcerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
	  For(&rodev1alpha1.Enforcer{}).
		Complete(r)
	if err != nil {
		return err
	}

	err = ctrl.NewControllerManagedBy(mgr).
		For(&rodev1alpha1.ClusterEnforcer{}).
		Complete(r)

	return err
}
