package attester

import (
	"context"
	rodev1alpha1 "github.com/liatrio/rode/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateSecret creates a Kubernetes secret for the attester using the OpenPGP keys from signer 
func CreateSecret(ctx context.Context, k8sClient client.Client, attester *rodev1alpha1.Attester, signer Signer) (secret *corev1.Secret, err error) {
	data := map[string][]byte{}
	data["primaryKey"], err = signer.SerializePublicKey()
	if err != nil {
		return
	}
	data["privateKey"], err = signer.SerializeKeys()
	if err != nil {
		return
	}
	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       attester.Namespace,
			Name:            attester.Spec.PgpSecret,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(attester, rodev1alpha1.GroupVersion.WithKind("Attester"))},
		},
		Data: data,
	}
	err = k8sClient.Create(ctx, secret)
	if err != nil {
		return
	}

	return
}

// DeleteSecret deletes the Kubernetes secret for an attester resource
func DeleteSecret(ctx context.Context, k8sClient client.Client, attester *rodev1alpha1.Attester) error {
	secret := &corev1.Secret{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: attester.Spec.PgpSecret, Namespace: attester.Namespace}, secret)
	if err != nil {
		return err
	}

	if metav1.IsControlledBy(secret, attester) {
		err = k8sClient.Delete(ctx, secret)
		if err != nil {
			return err
		}
	}

	return nil
}
