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
	"bytes"
	"context"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rodev1 "github.com/liatrio/rode/api/v1"
	"github.com/liatrio/rode/pkg/attester"
)

// AttesterReconciler reconciles a Attester object
type AttesterReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Attesters map[string]attester.Attester
}

func (r *AttesterReconciler) ListAttesters() map[string]attester.Attester {
	return r.Attesters
}

var (
	attesterFinalizerName = "attester.finalizers.rode.liatr.io"
)

// +kubebuilder:rbac:groups=rode.liatr.io,resources=attesters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rode.liatr.io,resources=attesters/status,verbs=get;update;patch

func (r *AttesterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("attester", req.NamespacedName)
	opaTrace := false

	log.Info("Reconciling attester")

	att := &rodev1.Attester{}
	err := r.Get(ctx, req.NamespacedName, att)
	if err != nil {
		log.Error(err, "Unable to load attester")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Register finalizer
	err = r.registerFinalizer(log, att)
	if err != nil {
		log.Error(err, "Error registering finalizer")
	}

	// If the attester is trying to be deleted remove the finalizer, delete the secret,
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
		secret := &corev1.Secret{}
		err = r.Get(ctx, req.NamespacedName, secret)
		if err != nil {
			log.Error(err, "Failed to get the secret")
		}

        if metav1.HasAnnotation(secret.ObjectMeta, "ownedByRode") {

            err = r.Delete(ctx, secret)
            if err != nil {
                log.Error(err, "Failed to delete the secret")
            }
        }

		// Deleting attester object
		delete(r.Attesters, req.Name)

		return ctrl.Result{}, err
	}

	// If there are 0 conditions then initialize conditions by adding two with false statuses
	if len(att.Status.Conditions) == 0 {

		policyCondition := rodev1.AttesterCondition{
			Type:   rodev1.AttesterConditionCompiled,
			Status: rodev1.ConditionStatusFalse,
		}

		att.Status.Conditions = append(att.Status.Conditions, policyCondition)

		secretCondition := rodev1.AttesterCondition{
			Type:   rodev1.AttesterConditionSecret,
			Status: rodev1.ConditionStatusFalse,
		}

		att.Status.Conditions = append(att.Status.Conditions, secretCondition)

		if err := r.Status().Update(ctx, att); err != nil {
			log.Error(err, "Unable to initialize attester status")
			return ctrl.Result{}, err
		}
	}

	// Always recompile the policy
	policy, err := attester.NewPolicy(req.Name, att.Spec.Policy, opaTrace)
	if err != nil {
		log.Error(err, "Unable to create policy")
		att.Status.Conditions[0].Status = rodev1.ConditionStatusFalse

		if err := r.Status().Update(ctx, att); err != nil {
			log.Error(err, "Unable to update Attester status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	att.Status.Conditions[0].Status = rodev1.ConditionStatusTrue

	if err := r.Status().Update(ctx, att); err != nil {
		log.Error(err, "Unable to update Attester status")
		return ctrl.Result{}, err
	}

	signerSecret := &corev1.Secret{}
	var signer attester.Signer

	// Check that the secret exists, if it does, recreate a signer from the secret
	if att.Status.Conditions[1].Status != rodev1.ConditionStatusTrue {

		err = r.Get(ctx, req.NamespacedName, signerSecret)
		if err != nil {

			// If the secret wasn't found then create the secret
			if err.Error() != "Secret \""+req.Name+"\" not found" {
				log.Error(err, "Unable to get the secret")
				return ctrl.Result{}, err
			}

			log.Info("Couldn't find secret, creating a new one")

			// Create a new signer
			signer, err = attester.NewSigner(req.Name)
			if err != nil {
				log.Error(err, "Unable to create signer")
				return ctrl.Result{}, err
			}
			log.Info("Created the signer successfully")

			var buffer []byte
			buf := bytes.NewBuffer(buffer)

			// signer.Serialize writes the public and private keys to a buffer object buf
			err = signer.Serialize(buf)
			if err != nil {
				log.Error(err, "Unable to read the private key data from the signer")
				return ctrl.Result{}, err
			}

			// buf writes the private and public key to the signerData string
			signerData := buf.Bytes()

			signerSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: req.Namespace,
					Name:      req.Name,
                    Annotations: map[string]string{"ownedByRode":"true"},
				},
				Data: map[string][]byte{"keys": signerData},
			}

			err = r.Create(ctx, signerSecret)
			if err != nil {
				log.Error(err, "Unable to create signer Secret")
				att.Status.Conditions[1].Status = rodev1.ConditionStatusFalse

				if err := r.Status().Update(ctx, att); err != nil {
					log.Error(err, "Unable to update Attester status")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}

			// Set PgpSecret to newly created secret name
			att.Spec.PgpSecret = req.Name
			err = r.Update(ctx, att)
			if err != nil {
				log.Error(err, "Could not update attester's secret field")
				return ctrl.Result{}, err
			}

			// Update the status to true
			att.Status.Conditions[1].Status = rodev1.ConditionStatusTrue
			if err := r.Status().Update(ctx, att); err != nil {
				log.Error(err, "Unable to update Attester status")
				return ctrl.Result{}, err
			}

			log.Info("Created the signer secret")
		}
	} else {
		// The secret does exist
		// Get the secret, then recreate the signer via the secret data
		err := r.Get(ctx, req.NamespacedName, signerSecret)
		if err != nil {
			log.Error(err, "Could not get secret")
			return ctrl.Result{}, err
		}

		buf := bytes.NewBuffer(signerSecret.Data["keys"])

		signer, err = attester.ReadSigner(buf)
		if err != nil {
			log.Error(err, "Unable to create signer from secret")
			return ctrl.Result{}, err
		}

	}

	// Create the attester in the map if it doesn't already exist, otherwise recreate it
	r.Attesters[req.Name] = attester.NewAttester(req.Name, policy, signer)

	return ctrl.Result{}, nil
}

func (r *AttesterReconciler) registerFinalizer(logger logr.Logger, attester *rodev1.Attester) error {
	// If the attester isn't being deleted and it doesn't contain a finalizer, then add one
	if attester.ObjectMeta.DeletionTimestamp.IsZero() && !containsFinalizer(attester.ObjectMeta.Finalizers, attesterFinalizerName) {
		logger.Info("Creating attester finalizer...")
		attester.ObjectMeta.Finalizers = append(attester.ObjectMeta.Finalizers, attesterFinalizerName)

		if err := r.Update(context.Background(), attester); err != nil {
			return err
		}
	}

	return nil
}

func (r *AttesterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rodev1.Attester{}).
		Complete(r)
}
