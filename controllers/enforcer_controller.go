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
	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rodev1alpha1 "github.com/liatrio/rode/api/v1alpha1"
	"github.com/liatrio/rode/pkg/eventmanager"
	"github.com/liatrio/rode/pkg/attester"
)

const EnforcerFinalizer = "enforcer.finalizers.rode.liatr.io"

// EnforcerReconciler reconciles a Enforcer object
type EnforcerReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	EventManager eventmanager.EventManager
	SignerList   attester.SignerList
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

	// Determin if deleting
	if enforcer.GetObjectMeta().GetDeletionTimestamp().IsZero() {

		// not deletion logic
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

		// resource deletion logic
		if containsFinalizer(enforcer.GetObjectMeta().GetFinalizers(), EnforcerFinalizer) {
			r.Log.Info("Unsubscribing from attesters")
			for _, enforcerAttester := range enforcer.Attesters() {
				if err = r.EventManager.Unsubscribe(types.NamespacedName{Name: enforcerAttester.Name, Namespace: enforcerAttester.Namespace}.String()); err != nil {
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
		for _, enforcerAttester := range enforcer.Attesters() {
			if err = r.EventManager.Subscribe(types.NamespacedName{Name: enforcerAttester.Name, Namespace: enforcerAttester.Namespace}.String()); err != nil {
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

	// test for new condition for secret key exists
	if secretStatus := enforcer.GetConditionStatus(rodev1alpha1.ConditionSecret); secretStatus != rodev1alpha1.ConditionStatusTrue {
		for _, enforcerAttester := range enforcer.Attesters() {
			// secret name = "enforcer-{attester_namespace}-{attester_name}
			// check for secret to exist
			secretName := fmt.Sprintf("enforcer-%s-%s", enforcerAttester.Namespace, enforcerAttester.Name)
			secret := &corev1.Secret{}
			if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: req.Namespace}, secret ) ; err != nil {
				// secret doesn't exist
				log.Error(err, fmt.Sprintf("Secret %s not found", secretName))
				enforcer.SetCondition(rodev1alpha1.ConditionSecret, rodev1alpha1.ConditionStatusFalse, fmt.Sprintf("No secret %s found", secretName))
				if updateErr := r.Update(ctx, enforcer.(runtime.Object)) ; err != nil {
					log.Error(updateErr, "Error updating enforcer status")
					return ctrl.Result{}, updateErr
				}
				return ctrl.Result{}, err
			}

			// create signer
			signer, err := attester.NewSignerFromKeys([]byte(secret.Data["primaryKey"]))
			if err != nil {
				log.Error(err, "Failed to read primary key from secret")
				enforcer.SetCondition(rodev1alpha1.ConditionSecret, rodev1alpha1.ConditionStatusFalse, fmt.Sprintf("Failed to read primaryKey from secret from %s", secretName))
				if updateErr := r.Update(ctx, enforcer.(runtime.Object)) ; err != nil {
					log.Error(updateErr, "Error updating enforcer status")
					return ctrl.Result{}, updateErr
				}
				return ctrl.Result{}, err
			}
			log.Info(fmt.Sprintf("Found signer sig: %s", signer))

			r.SignerList.Add(types.NamespacedName{Name: enforcerAttester.Name, Namespace: enforcerAttester.Namespace}.String(), signer)

			enforcer.SetCondition(rodev1alpha1.ConditionSecret, rodev1alpha1.ConditionStatusTrue, "Found signer secret")
			if updateErr := r.Update(ctx, enforcer.(runtime.Object)) ; err != nil {
				log.Error(updateErr, "Error updating enforcer signer status")
				return ctrl.Result{}, updateErr
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
