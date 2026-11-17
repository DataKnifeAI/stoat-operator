package controller

import (
	"context"
	"fmt"
	"strings"

	stoatv1alpha1 "github.com/DataKnifeAI/stoat-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *StoatReconciler) reconcileConfig(ctx context.Context, instance *stoatv1alpha1.Stoat, secrets *stoatSecrets) error {
	if err := r.reconcileRevoltConfig(ctx, instance, secrets); err != nil {
		return err
	}
	if err := r.reconcileLiveKitConfig(ctx, instance, secrets); err != nil {
		return err
	}
	if err := r.reconcileCaddyConfig(ctx, instance); err != nil {
		return err
	}
	return r.reconcileWebEnv(ctx, instance)
}

func (r *StoatReconciler) reconcileRevoltConfig(ctx context.Context, instance *stoatv1alpha1.Stoat, secrets *stoatSecrets) error {
	domain := instance.Spec.Domain
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configName,
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if err := controllerutil.SetControllerReference(instance, cm, r.Scheme); err != nil {
			return err
		}
		cm.Labels = stoatLabels(instance.Name)
		mongoHost := svcDatabase
		redisHost := svcRedis
		rabbitHost := svcRabbit
		minioEndpoint := fmt.Sprintf("http://%s:9000", svcMinio)
		if instance.Spec.Dependencies.MongoDB != nil && instance.Spec.Dependencies.MongoDB.URI != "" {
			mongoHost = instance.Spec.Dependencies.MongoDB.URI
		} else {
			mongoHost = "mongodb://" + svcDatabase
		}
		if instance.Spec.Dependencies.Redis != nil && instance.Spec.Dependencies.Redis.URI != "" {
			redisHost = instance.Spec.Dependencies.Redis.URI
		} else {
			redisHost = "redis://" + svcRedis + "/"
		}
		if instance.Spec.Dependencies.ObjectStorage != nil && instance.Spec.Dependencies.ObjectStorage.URI != "" {
			minioEndpoint = instance.Spec.Dependencies.ObjectStorage.URI
		}
		if instance.Spec.Dependencies.RabbitMQ != nil && instance.Spec.Dependencies.RabbitMQ.URI != "" {
			// Expect host-only override for simplicity when URI is set without scheme.
			rabbitHost = strings.TrimPrefix(instance.Spec.Dependencies.RabbitMQ.URI, "amqp://")
			if i := strings.Index(rabbitHost, "@"); i >= 0 {
				rabbitHost = rabbitHost[i+1:]
			}
			if i := strings.Index(rabbitHost, ":"); i >= 0 {
				rabbitHost = rabbitHost[:i]
			}
			if i := strings.Index(rabbitHost, "/"); i >= 0 {
				rabbitHost = rabbitHost[:i]
			}
		}

		cm.Data = map[string]string{
			"Revolt.toml": renderRevoltTOML(domain, mongoHost, redisHost, rabbitHost, minioEndpoint, secrets),
			"stoat.json":  fmt.Sprintf(`{"api":"https://%s/api"}`, domain),
		}
		return nil
	})
	return err
}

func renderRevoltTOML(domain, mongoURL, redisURL, rabbitHost, s3Endpoint string, secrets *stoatSecrets) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Managed by stoat-operator — do not edit by hand\n")
	fmt.Fprintf(&b, "production = true\n\n")
	fmt.Fprintf(&b, "[database]\n")
	fmt.Fprintf(&b, "mongodb = %q\n", mongoURL)
	fmt.Fprintf(&b, "redis = %q\n\n", redisURL)
	fmt.Fprintf(&b, "[hosts]\n")
	fmt.Fprintf(&b, "app = \"https://%s\"\n", domain)
	fmt.Fprintf(&b, "api = \"https://%s/api\"\n", domain)
	fmt.Fprintf(&b, "events = \"wss://%s/ws\"\n", domain)
	fmt.Fprintf(&b, "autumn = \"https://%s/autumn\"\n", domain)
	fmt.Fprintf(&b, "january = \"https://%s/january\"\n", domain)
	fmt.Fprintf(&b, "gifbox = \"https://%s/gifbox\"\n\n", domain)
	fmt.Fprintf(&b, "[hosts.livekit]\n")
	fmt.Fprintf(&b, "worldwide = \"wss://%s/livekit\"\n\n", domain)
	fmt.Fprintf(&b, "[rabbit]\n")
	fmt.Fprintf(&b, "host = %q\n", rabbitHost)
	fmt.Fprintf(&b, "port = 5672\n")
	fmt.Fprintf(&b, "username = %q\n", rabbitUser)
	fmt.Fprintf(&b, "password = %q\n", rabbitPass)
	fmt.Fprintf(&b, "default_exchange = \"revolt.default\"\n\n")
	fmt.Fprintf(&b, "[rabbit.queues]\n")
	fmt.Fprintf(&b, "acks = \"internal.ack\"\n")
	fmt.Fprintf(&b, "events = \"internal.event\"\n\n")
	fmt.Fprintf(&b, "[api.livekit.nodes.worldwide]\n")
	fmt.Fprintf(&b, "url = \"http://%s:%d\"\n", svcLiveKit, portLiveKitHTTP)
	fmt.Fprintf(&b, "key = %q\n", secrets.LiveKitKey)
	fmt.Fprintf(&b, "secret = %q\n\n", secrets.LiveKitSec)
	fmt.Fprintf(&b, "[pushd.vapid]\n")
	fmt.Fprintf(&b, "private_key = %q\n", secrets.VapidPrivate)
	fmt.Fprintf(&b, "public_key = %q\n\n", secrets.VapidPublic)
	fmt.Fprintf(&b, "[files]\n")
	fmt.Fprintf(&b, "encryption_key = %q\n\n", secrets.FilesEncKey)
	fmt.Fprintf(&b, "[files.s3]\n")
	fmt.Fprintf(&b, "endpoint = %q\n", s3Endpoint)
	fmt.Fprintf(&b, "path_style_buckets = false\n")
	fmt.Fprintf(&b, "region = \"minio\"\n")
	fmt.Fprintf(&b, "access_key_id = %q\n", minioUser)
	fmt.Fprintf(&b, "secret_access_key = %q\n", minioPass)
	fmt.Fprintf(&b, "default_bucket = \"revolt-uploads\"\n")
	return b.String()
}

func (r *StoatReconciler) reconcileLiveKitConfig(ctx context.Context, instance *stoatv1alpha1.Stoat, secrets *stoatSecrets) error {
	rtc, udpStart, udpEnd := liveKitPorts(instance.Spec)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      livekitCfgName,
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if err := controllerutil.SetControllerReference(instance, cm, r.Scheme); err != nil {
			return err
		}
		cm.Labels = stoatLabels(instance.Name)
		cm.Data = map[string]string{
			"livekit.yml": fmt.Sprintf(`rtc:
  use_external_ip: true
  port_range_start: %d
  port_range_end: %d
  tcp_port: %d

redis:
  address: %s:%d

turn:
  enabled: false

keys:
  %s: %s

webhook:
  api_key: %s
`, udpStart, udpEnd, rtc, svcRedis, portRedis, secrets.LiveKitKey, secrets.LiveKitSec, secrets.LiveKitKey),
		}
		return nil
	})
	return err
}

func (r *StoatReconciler) reconcileCaddyConfig(ctx context.Context, instance *stoatv1alpha1.Stoat) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      caddyCfgName,
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if err := controllerutil.SetControllerReference(instance, cm, r.Scheme); err != nil {
			return err
		}
		cm.Labels = stoatLabels(instance.Name)
		// Behind Ingress: Caddy listens on :80 and serves all routes.
		cm.Data = map[string]string{
			"Caddyfile": fmt.Sprintf(`:80 {
	route /.well-known/stoat {
		uri strip_prefix /.well-known/stoat
		header {
			Access-Control-Allow-Origin *
		}
		rewrite * /stoat.json
		file_server {
			root /config
		}
	}

	route /api* {
		uri strip_prefix /api
		reverse_proxy http://%s:%d
	}

	route /ws {
		uri strip_prefix /ws
		reverse_proxy http://%s:%d
	}

	route /autumn* {
		uri strip_prefix /autumn
		reverse_proxy http://%s:%d
	}

	route /january* {
		uri strip_prefix /january
		reverse_proxy http://%s:%d
	}

	route /gifbox* {
		uri strip_prefix /gifbox
		reverse_proxy http://%s:%d
	}

	route /livekit* {
		uri strip_prefix /livekit
		reverse_proxy http://%s:%d
	}

	route /ingress* {
		uri strip_prefix /ingress
		reverse_proxy http://%s:%d
	}

	reverse_proxy http://%s:%d
}
`, svcAPI, portAPI, svcEvents, portEvents, svcAutumn, portAutumn, svcJanuary, portJanuary,
				svcGifbox, portGifbox, svcLiveKit, portLiveKitHTTP, svcVoiceIngress, portVoiceIngress, svcWeb, portWeb),
		}
		return nil
	})
	return err
}

func (r *StoatReconciler) reconcileWebEnv(ctx context.Context, instance *stoatv1alpha1.Stoat) error {
	domain := instance.Spec.Domain
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      webEnvName,
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if err := controllerutil.SetControllerReference(instance, cm, r.Scheme); err != nil {
			return err
		}
		cm.Labels = stoatLabels(instance.Name)
		cm.Data = map[string]string{
			"HOSTNAME":           ":80",
			"VITE_HOST":          domain,
			"VITE_API_URL":       fmt.Sprintf("https://%s/api", domain),
			"VITE_DEV_GIFBOX_URL": fmt.Sprintf("https://%s/gifbox", domain),
		}
		return nil
	})
	return err
}
