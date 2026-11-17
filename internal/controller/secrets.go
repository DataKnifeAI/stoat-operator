package controller

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"

	stoatv1alpha1 "github.com/DataKnifeAI/stoat-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type stoatSecrets struct {
	VapidPrivate string
	VapidPublic  string
	FilesEncKey  string
	LiveKitKey   string
	LiveKitSec   string
}

func (r *StoatReconciler) reconcileSecrets(ctx context.Context, instance *stoatv1alpha1.Stoat) (*stoatSecrets, error) {
	name := secretNameFor(instance)
	existing := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: instance.Namespace}, existing)
	if err == nil {
		return secretsFromMap(existing.Data), nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}

	if instance.Spec.SecretsRef != "" {
		return nil, fmt.Errorf("secretsRef %q not found", instance.Spec.SecretsRef)
	}
	if !boolDefault(instance.Spec.GenerateSecrets, true) {
		return nil, fmt.Errorf("secret %q missing and generateSecrets is false", name)
	}

	generated, err := generateStoatSecrets()
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
			Labels:    stoatLabels(instance.Name),
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, secret, func() error {
		if err := controllerutil.SetControllerReference(instance, secret, r.Scheme); err != nil {
			return err
		}
		secret.Labels = stoatLabels(instance.Name)
		secret.Type = corev1.SecretTypeOpaque
		// Never rotate existing keys if somehow present.
		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		setSecretIfMissing(secret, keyVapidPrivate, generated.VapidPrivate)
		setSecretIfMissing(secret, keyVapidPublic, generated.VapidPublic)
		setSecretIfMissing(secret, keyFilesEnc, generated.FilesEncKey)
		setSecretIfMissing(secret, keyLiveKitKey, generated.LiveKitKey)
		setSecretIfMissing(secret, keyLiveKitSec, generated.LiveKitSec)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return secretsFromMap(secret.Data), nil
}

func setSecretIfMissing(secret *corev1.Secret, key, value string) {
	if len(secret.Data[key]) == 0 {
		secret.Data[key] = []byte(value)
	}
}

func secretsFromMap(data map[string][]byte) *stoatSecrets {
	return &stoatSecrets{
		VapidPrivate: string(data[keyVapidPrivate]),
		VapidPublic:  string(data[keyVapidPublic]),
		FilesEncKey:  string(data[keyFilesEnc]),
		LiveKitKey:   string(data[keyLiveKitKey]),
		LiveKitSec:   string(data[keyLiveKitSec]),
	}
}

func generateStoatSecrets() (*stoatSecrets, error) {
	priv, pub, err := generateVAPIDKeys()
	if err != nil {
		return nil, fmt.Errorf("generate vapid keys: %w", err)
	}
	filesKey, err := randomBase64(32)
	if err != nil {
		return nil, err
	}
	lkSec, err := randomHex(24)
	if err != nil {
		return nil, err
	}
	lkKey, err := randomHex(6)
	if err != nil {
		return nil, err
	}
	return &stoatSecrets{
		VapidPrivate: priv,
		VapidPublic:  pub,
		FilesEncKey:  filesKey,
		LiveKitKey:   lkKey,
		LiveKitSec:   lkSec,
	}, nil
}

func generateVAPIDKeys() (privateB64, publicB64 string, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", "", err
	}
	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	privateB64 = strings.TrimRight(base64.StdEncoding.EncodeToString(pemBlock), "=")
	// Uncompressed point: 0x04 || X || Y (65 bytes), URL-safe base64 without padding.
	pubBytes := append([]byte{0x04}, append(key.PublicKey.X.FillBytes(make([]byte, 32)), key.PublicKey.Y.FillBytes(make([]byte, 32))...)...)
	publicB64 = base64.RawURLEncoding.EncodeToString(pubBytes)
	return privateB64, publicB64, nil
}

func randomBase64(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
