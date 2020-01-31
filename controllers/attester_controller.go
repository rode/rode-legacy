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
	Attesters []attester.Attester
}

func (r *AttesterReconciler) ListAttesters() []attester.Attester {
	return r.Attesters
}

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
		return ctrl.Result{}, err
	}

	policy, err := attester.NewPolicy(req.Name, att.Spec.Policy, opaTrace)
	if err != nil {
		log.Error(err, "Unable to create policy")
		//att.Status.CompiledPolicy = "Failed to compile"
		return ctrl.Result{}, err
	}

	// TODO: update status based on results of compiling the policy
	//att.Status.CompiledPolicy = "Compiled"

	if err := r.Status().Update(ctx, att); err != nil {
		log.Error(err, "Unable to update Attester status")
		return ctrl.Result{}, err
	}

	// TODO: check if secret already exists before doing this...maybe just load secret

	signerSecret := &corev1.Secret{}
	var signer attester.Signer

    if att.Spec.PgpSecret == req.Name {
        err := r.Get(ctx, client.ObjectKey{
            Namespace: req.Namespace,
            Name: att.Spec.PgpSecret,
            }, signerSecret)
        if err != nil {
		    log.Error(err, "Unable to load signerSecret")
		    return ctrl.Result{}, err
        }
	} else {
        signer, err = attester.NewSigner(req.Name)
		if err != nil {
			log.Error(err, "Unable to create signer")
			return ctrl.Result{}, err
		}
        log.Info("Created the signer successfully")

        //publicKey := signer.KeyID()

        //var out io.Writer

        //err := signer.Serialize(out)

        //var privateKey string

        //_, err = io.WriteString(out, privateKey)

        signerSecret = &corev1.Secret{
            ObjectMeta: metav1.ObjectMeta{
                Namespace: req.Namespace,
                Name:      req.Name,
            },
            Data: map[string][]byte{"publicKey": make([]byte, 5), "privateKey": make([]byte, 5)},
        }
        err = r.Create(ctx, signerSecret)
        if err != nil {
            log.Error(err, "Unable to create signer Secret")
            return ctrl.Result{}, err
        }
    }

	// TODO: update secret with signer material.

	r.Attesters = append(r.Attesters, attester.NewAttester(req.Name, policy, signer))

	return ctrl.Result{}, nil
}

func (r *AttesterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rodev1.Attester{}).
		Complete(r)
}
