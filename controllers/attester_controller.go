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
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liatrio/rode/api/util"
	rodev1alpha1 "github.com/liatrio/rode/api/v1alpha1"
	"github.com/liatrio/rode/pkg/attester"
)

// AttesterReconciler reconciles a Attester object
type AttesterReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Attesters map[string]attester.Attester
}

// ListAttesters returns a list of Attester objects
func (r *AttesterReconciler) ListAttesters() map[string]attester.Attester {
	return r.Attesters
}

var (
	attesterFinalizerName = "attester.finalizers.rode.liatr.io"
)

// +kubebuilder:rbac:groups=rode.liatr.io,resources=attesters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rode.liatr.io,resources=attesters/status,verbs=get;update;patch

// Reconcile runs whenever a change to an Attester is made. It attempts to match the current state of the attester to the desired state.
// nolint: gocyclo
func (r *AttesterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithName("Reconcile()").WithValues("attester", req.NamespacedName)
	opaTrace := false

	log.Info("Reconciling attester")

	att := &rodev1alpha1.Attester{}
	err := r.Get(ctx, req.NamespacedName, att)
	if err != nil {
		log.Error(err, "Unable to load attester")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Register finalizer
	err = r.registerFinalizer(att)
	if err != nil {
		log.Error(err, "Error registering finalizer")
	}

	// If the attester is being deleted then remove the finalizer, delete the secret,
	// and remove the Attester object from r.Attesters
	if !att.ObjectMeta.DeletionTimestamp.IsZero() && containsFinalizer(att.ObjectMeta.Finalizers, attesterFinalizerName) {
		log.Info("Removing finalizer")

		// Removing finalizer
		att.ObjectMeta.Finalizers = removeFinalizer(att.ObjectMeta.Finalizers, attesterFinalizerName)
		err := r.Update(ctx, att)
		if err != nil {
			log.Error(err, "Error Removing the finalizer")
			return ctrl.Result{}, err
		}

		// Deleting secret
		err = attester.DeleteSecret(ctx, r.Client, att)
		if err != nil {
			log.Info(fmt.Sprintf("Error deleteing secret: %s", err))
			err = nil
		}

		// Deleting attester object
		delete(r.Attesters, req.NamespacedName.String())

		return ctrl.Result{}, err
	}

	// If there are 0 conditions then initialize conditions by adding two with false statuses
	if len(att.Status.Conditions) == 0 {
		rodev1alpha1.SetCondition(att, rodev1alpha1.ConditionCompiled, rodev1alpha1.ConditionStatusFalse, "")
		rodev1alpha1.SetCondition(att, rodev1alpha1.ConditionSecret, rodev1alpha1.ConditionStatusFalse, "")

		if err := r.Status().Update(ctx, att); err != nil {
			log.Error(err, "Unable to initialize attester status")
			return ctrl.Result{}, err
		}
	}

	// Always recompile the policy
	policy, err := attester.NewPolicy(req.Name, att.Spec.Policy, opaTrace)
	if err != nil {
		log.Error(err, "Unable to create policy")
		_ = r.updateStatus(ctx, att, rodev1alpha1.ConditionCompiled, rodev1alpha1.ConditionStatusFalse)
		return ctrl.Result{}, err
	}
	err = r.updateStatus(ctx, att, rodev1alpha1.ConditionCompiled, rodev1alpha1.ConditionStatusTrue)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create signer
	signer, err := r.createSigner(ctx, att)
	if err != nil {
		_ = r.updateStatus(ctx, att, rodev1alpha1.ConditionSecret, rodev1alpha1.ConditionStatusFalse)
		return ctrl.Result{}, err
	}
	err = r.updateStatus(ctx, att, rodev1alpha1.ConditionSecret, rodev1alpha1.ConditionStatusTrue)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Add or replace attester in list
	r.Attesters[req.NamespacedName.String()] = attester.NewAttester(req.NamespacedName.String(), policy, signer)
	log.Info(fmt.Sprintf("Add / Update %s", r.Attesters[req.NamespacedName.String()]))

	return ctrl.Result{}, nil
}

func (r *AttesterReconciler) registerFinalizer(attester *rodev1alpha1.Attester) error {
	log := r.Log.WithName("registerFinalizer()").WithValues("attester", attester.Name)
	// If the attester isn't being deleted and it doesn't contain a finalizer, then add one
	if attester.ObjectMeta.DeletionTimestamp.IsZero() && !containsFinalizer(attester.ObjectMeta.Finalizers, attesterFinalizerName) {
		log.Info("Creating attester finalizer...")
		attester.ObjectMeta.Finalizers = append(attester.ObjectMeta.Finalizers, attesterFinalizerName)

		if err := r.Update(context.Background(), attester); err != nil {
			return err
		}
	}

	return nil
}

func (r *AttesterReconciler) updateStatus(ctx context.Context, attester *rodev1alpha1.Attester, conditionType rodev1alpha1.ConditionType, status rodev1alpha1.ConditionStatus) error {
	log := r.Log.WithName("updateStatus()").WithValues("type", conditionType, "status", status)

	log.Info("Updating Attester status")
	rodev1alpha1.SetCondition(attester, conditionType, status, "") // TODO: add message

	patch, err := json.Marshal(rodev1alpha1.Attester{Status: attester.Status})
	if err != nil {
		log.Error(err, "Error creating status patch")
		return err
	}
	if err := r.Status().Patch(ctx, attester, client.RawPatch(types.MergePatchType, patch)); err != nil {
		log.Error(err, "Unable to update Attester status")
		return err
	}

	return nil
}

// SetupWithManager sets up the watching of Attester objects and filters out the events we don't want to watch
func (r *AttesterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rodev1alpha1.Attester{}).
		WithEventFilter(ignoreConditionStatusUpdateToActive(attesterToConditioner, rodev1alpha1.ConditionCompiled)).
		WithEventFilter(ignoreConditionStatusUpdateToActive(attesterToConditioner, rodev1alpha1.ConditionSecret)).
		WithEventFilter(ignoreFinalizerUpdate()).
		WithEventFilter(ignoreDelete()).
		Complete(r)
}

// Create Signer instance by reading OpenPGP keys from Kubernetes secret or generating new keys and storing them in secret
func (r *AttesterReconciler) createSigner(ctx context.Context, attesterResource *rodev1alpha1.Attester) (attester.Signer, error) {
	log := r.Log.WithName("CreateSigner()")
	var signer attester.Signer
	secret := &corev1.Secret{}

	err := r.Get(ctx, types.NamespacedName{Name: attesterResource.Spec.PgpSecret, Namespace: attesterResource.Namespace}, secret)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}

		log.Info("Creating new attester secret")
		signer, err = attester.NewSigner(attesterResource.Name)
		if err != nil {
			log.Error(err, "Error creating new attester signer")
			return nil, err
		}

		_, err = attester.CreateSecret(ctx, r.Client, attesterResource, signer)
		if err != nil {
			log.Error(err, "Error creating attester secret")
			return nil, err
		}

		return signer, nil
	}

	log.Info("Create Attester Signer from existing secret", "secret", fmt.Sprintf("%s.%s", secret.Name, secret.Namespace))
	signer, err = attester.NewSignerFromKeys(secret.Data["privateKey"])

	return signer, err
}

// AttesterToConditioner takes an Attester and returns a util.Conditioner
func attesterToConditioner(o runtime.Object) util.Conditioner {
	return o.(*rodev1alpha1.Attester)
}
