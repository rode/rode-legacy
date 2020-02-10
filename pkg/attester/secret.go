package attester

import (
	"bytes"
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewSecret uses the kubernetes client library to create a new secret resource.
// The name parameter is used to name the secret and the namespace parameter
// is used to designate which namespace the secret is created in
// The function returns a signer object to be used by the reconcile loop.
func NewSecret(client client.Client, name string, namespace string) (Signer, error) {
	ctx := context.Background()

	// Create a new signer
	signer, err := NewSigner(name)
	if err != nil {
		return nil, err
	}

	var buffer []byte
	buf := bytes.NewBuffer(buffer)

	// signer.Serialize writes the public and private keys to a buffer object buf
	err = signer.Serialize(buf)
	if err != nil {
		return nil, err
	}

	// buf writes the private and public key to the signerData string
	signerData := buf.Bytes()

	// we create the secret with an annotation to let rode know that it owns the secret
	signerSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Annotations: map[string]string{"ownedByRode": "true"},
		},
		Data: map[string][]byte{"keys": signerData},
	}

	err = client.Create(ctx, signerSecret)
	if err != nil {
		return nil, err
	}

	return signer, nil
}

// DeleteSecret uses the kubernetes client library to delete a named secret resource.
// The name and namespace parameters are used to find the secret
// The function returns an err if the deletion fails
func DeleteSecret(c client.Client, name string, namespace string) error {

	ctx := context.Background()

	secret := &corev1.Secret{}
	err := c.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, secret)
	if err != nil {
		return err
	}

	if metav1.HasAnnotation(secret.ObjectMeta, "ownedByRode") {

		err = c.Delete(ctx, secret)
		if err != nil {
			return err
		}
	}

	return nil
}
