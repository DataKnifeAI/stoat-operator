package controller

import (
	"context"
	"fmt"
	"strings"

	stoatv1alpha1 "github.com/DataKnifeAI/stoat-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *StoatReconciler) reconcileAdminPanel(ctx context.Context, instance *stoatv1alpha1.Stoat) error {
	if !adminEnabled(instance.Spec) {
		return nil
	}
	admin := instance.Spec.Admin
	if strings.TrimSpace(admin.Domain) == "" {
		return fmt.Errorf("admin.domain is required when admin.enabled is true")
	}
	if strings.TrimSpace(admin.CredentialsSecret) == "" {
		return fmt.Errorf("admin.credentialsSecret is required when admin.enabled is true")
	}
	if strings.TrimSpace(admin.PlatformAccountId) == "" {
		return fmt.Errorf("admin.platformAccountId is required when admin.enabled is true")
	}

	if err := r.reconcileAdminEnvSecret(ctx, instance); err != nil {
		return err
	}

	images := resolveImages(instance.Spec)
	if err := r.reconcileDeployment(ctx, instance, deploySpec{
		Name:      svcAdminPanel,
		Component: "admin-panel",
		Image:     images["admin-panel"],
		Ports:     []corev1.ContainerPort{containerPort("http", portAdminPanel)},
		EnvFrom: []corev1.EnvFromSource{{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: adminEnvName},
			},
		}},
		Readiness: tcpProbe(portAdminPanel),
		Liveness:  tcpProbe(portAdminPanel),
		Resources: defaultCPUMem("100m", "512Mi"),
	}); err != nil {
		return ensureErr("admin-panel deployment", err)
	}
	if err := r.reconcileService(ctx, instance, svcAdminPanel, "admin-panel",
		[]corev1.ServicePort{servicePort("http", portAdminPanel)}, corev1.ServiceTypeClusterIP); err != nil {
		return ensureErr("admin-panel service", err)
	}
	return r.reconcileAdminIngress(ctx, instance)
}

func (r *StoatReconciler) reconcileAdminEnvSecret(ctx context.Context, instance *stoatv1alpha1.Stoat) error {
	admin := instance.Spec.Admin
	src := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      admin.CredentialsSecret,
		Namespace: instance.Namespace,
	}, src); err != nil {
		return fmt.Errorf("admin credentials secret %q: %w", admin.CredentialsSecret, err)
	}

	authentikID := string(src.Data["authentikId"])
	authentikSecret := string(src.Data["authentikSecret"])
	// openid-client joins issuer + "/.well-known/..."; a trailing slash yields a 404.
	authentikIssuer := strings.TrimRight(string(src.Data["authentikIssuer"]), "/")
	if authentikID == "" || authentikSecret == "" || authentikIssuer == "" {
		return fmt.Errorf("secret %q must contain authentikId, authentikSecret, and authentikIssuer", admin.CredentialsSecret)
	}

	nextAuthSecret := string(src.Data["nextAuthSecret"])
	if nextAuthSecret == "" {
		// Prefer a previously generated value so restarts do not invalidate sessions.
		existing := &corev1.Secret{}
		err := r.Get(ctx, types.NamespacedName{Name: adminEnvName, Namespace: instance.Namespace}, existing)
		if err == nil {
			nextAuthSecret = string(existing.Data["NEXTAUTH_SECRET"])
		} else if !apierrors.IsNotFound(err) {
			return err
		}
		if nextAuthSecret == "" {
			generated, genErr := randomBase64(32)
			if genErr != nil {
				return fmt.Errorf("generate NEXTAUTH_SECRET: %w", genErr)
			}
			nextAuthSecret = generated
		}
	}

	mongoURL, err := r.resolveDependencyURI(ctx, instance.Namespace, instance.Spec.Dependencies.MongoDB, "mongodb://"+svcDatabase)
	if err != nil {
		return err
	}
	redisURL, err := r.resolveDependencyURI(ctx, instance.Namespace, instance.Spec.Dependencies.Redis, "redis://"+svcRedis+"/")
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      adminEnvName,
			Namespace: instance.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, secret, func() error {
		if err := controllerutil.SetControllerReference(instance, secret, r.Scheme); err != nil {
			return err
		}
		secret.Labels = componentLabels(instance.Name, "admin-panel")
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = map[string][]byte{
			"HOST":               []byte("0.0.0.0"),
			"PORT":               []byte(fmt.Sprintf("%d", portAdminPanel)),
			"NODE_ENV":           []byte("production"),
			"MONGODB":            []byte(mongoURL),
			"REDIS":              []byte(redisURL),
			"NEXTAUTH_URL":       []byte("https://" + strings.TrimSpace(admin.Domain)),
			"NEXTAUTH_SECRET":    []byte(nextAuthSecret),
			"AUTHENTIK_ID":       []byte(authentikID),
			"AUTHENTIK_SECRET":   []byte(authentikSecret),
			"AUTHENTIK_ISSUER":   []byte(authentikIssuer),
			"PLATFORM_ACCOUNT_ID": []byte(strings.TrimSpace(admin.PlatformAccountId)),
		}
		return nil
	})
	return err
}

func (r *StoatReconciler) reconcileAdminIngress(ctx context.Context, instance *stoatv1alpha1.Stoat) error {
	admin := instance.Spec.Admin
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      adminIngressName,
			Namespace: instance.Namespace,
		},
	}
	pathType := networkingv1.PathTypePrefix
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ing, func() error {
		if err := controllerutil.SetControllerReference(instance, ing, r.Scheme); err != nil {
			return err
		}
		ing.Labels = componentLabels(instance.Name, "admin-panel")
		annotations := map[string]string{}
		for k, v := range instance.Spec.Ingress.Annotations {
			annotations[k] = v
		}
		ing.Annotations = annotations

		className := ingressClass(instance.Spec)
		ing.Spec.IngressClassName = &className
		host := strings.TrimSpace(admin.Domain)
		ing.Spec.Rules = []networkingv1.IngressRule{{
			Host: host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{{
						Path:     "/",
						PathType: &pathType,
						Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: svcAdminPanel,
								Port: networkingv1.ServiceBackendPort{Number: portAdminPanel},
							},
						},
					}},
				},
			},
		}}

		tlsSecret := strings.TrimSpace(admin.TLSSecretName)
		if tlsSecret == "" {
			tlsSecret = strings.TrimSpace(instance.Spec.Ingress.TLSSecretName)
		}
		if tlsSecret != "" {
			ing.Spec.TLS = []networkingv1.IngressTLS{{
				Hosts:      []string{host},
				SecretName: tlsSecret,
			}}
		} else {
			ing.Spec.TLS = nil
		}
		return nil
	})
	return ensureErr("admin ingress", err)
}
