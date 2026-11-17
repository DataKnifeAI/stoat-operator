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

		lat, lon := liveKitCoords(instance.Spec)
		cm.Data = map[string]string{
			"Revolt.toml": renderRevoltTOML(domain, mongoHost, redisHost, rabbitHost, minioEndpoint, secrets, liveKitEnabled(instance.Spec), lat, lon),
			"stoat.json":  fmt.Sprintf(`{"api":"https://%s/api"}`, domain),
		}
		return nil
	})
	return err
}

func renderRevoltTOML(domain, mongoURL, redisURL, rabbitHost, s3Endpoint string, secrets *stoatSecrets, withLiveKit bool, lat, lon float64) string {
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
	if withLiveKit {
		fmt.Fprintf(&b, "[hosts.livekit]\n")
		fmt.Fprintf(&b, "worldwide = \"wss://%s/livekit\"\n\n", domain)
	}
	fmt.Fprintf(&b, "[rabbit]\n")
	fmt.Fprintf(&b, "host = %q\n", rabbitHost)
	fmt.Fprintf(&b, "port = 5672\n")
	fmt.Fprintf(&b, "username = %q\n", rabbitUser)
	fmt.Fprintf(&b, "password = %q\n", rabbitPass)
	fmt.Fprintf(&b, "default_exchange = \"revolt.default\"\n\n")
	fmt.Fprintf(&b, "[rabbit.queues]\n")
	fmt.Fprintf(&b, "acks = \"internal.ack\"\n")
	fmt.Fprintf(&b, "events = \"internal.event\"\n\n")
	if withLiveKit {
		fmt.Fprintf(&b, "[api.livekit.nodes.worldwide]\n")
		fmt.Fprintf(&b, "url = \"http://%s:%d\"\n", svcLiveKit, portLiveKitHTTP)
		fmt.Fprintf(&b, "lat = %g\n", lat)
		fmt.Fprintf(&b, "lon = %g\n", lon)
		fmt.Fprintf(&b, "key = %q\n", secrets.LiveKitKey)
		fmt.Fprintf(&b, "secret = %q\n\n", secrets.LiveKitSec)
	}
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
	if !liveKitEnabled(instance.Spec) {
		return nil
	}
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
			"livekit.yml": renderLiveKitYAML(instance.Spec, secrets, rtc, udpStart, udpEnd),
		}
		return nil
	})
	return err
}

func renderLiveKitYAML(spec stoatv1alpha1.StoatSpec, secrets *stoatSecrets, rtc, udpStart, udpEnd int32) string {
	var b strings.Builder
	fmt.Fprintf(&b, "rtc:\n")
	fmt.Fprintf(&b, "  use_external_ip: %t\n", liveKitUseExternalIP(spec))
	if addr := strings.TrimSpace(spec.LiveKit.Address); addr != "" && !liveKitUseExternalIP(spec) {
		fmt.Fprintf(&b, "  node_ip: %q\n", addr)
	}
	fmt.Fprintf(&b, "  port_range_start: %d\n", udpStart)
	fmt.Fprintf(&b, "  port_range_end: %d\n", udpEnd)
	fmt.Fprintf(&b, "  tcp_port: %d\n\n", rtc)
	fmt.Fprintf(&b, "redis:\n")
	fmt.Fprintf(&b, "  address: %s:%d\n\n", svcRedis, portRedis)
	fmt.Fprintf(&b, "turn:\n")
	fmt.Fprintf(&b, "  enabled: false\n\n")
	fmt.Fprintf(&b, "keys:\n")
	fmt.Fprintf(&b, "  %s: %s\n\n", secrets.LiveKitKey, secrets.LiveKitSec)
	fmt.Fprintf(&b, "webhook:\n")
	fmt.Fprintf(&b, "  api_key: %s\n", secrets.LiveKitKey)
	fmt.Fprintf(&b, "  urls:\n")
	fmt.Fprintf(&b, "    - \"http://%s:%d/worldwide\"\n", svcVoiceIngress, portVoiceIngress)
	return b.String()
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
			"Caddyfile": renderCaddyfile(liveKitEnabled(instance.Spec)),
		}
		return nil
	})
	return err
}

func renderCaddyfile(withLiveKit bool) string {
	var b strings.Builder
	b.WriteString(`:80 {
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

	route /stoat.json {
		header {
			Access-Control-Allow-Origin *
		}
		rewrite * /stoat.json
		file_server {
			root /config
		}
	}

`)
	fmt.Fprintf(&b, "\troute /api* {\n\t\turi strip_prefix /api\n\t\treverse_proxy http://%s:%d\n\t}\n\n", svcAPI, portAPI)
	fmt.Fprintf(&b, "\troute /ws {\n\t\turi strip_prefix /ws\n\t\treverse_proxy http://%s:%d\n\t}\n\n", svcEvents, portEvents)
	fmt.Fprintf(&b, "\troute /autumn* {\n\t\turi strip_prefix /autumn\n\t\treverse_proxy http://%s:%d\n\t}\n\n", svcAutumn, portAutumn)
	fmt.Fprintf(&b, "\troute /january* {\n\t\turi strip_prefix /january\n\t\treverse_proxy http://%s:%d\n\t}\n\n", svcJanuary, portJanuary)
	fmt.Fprintf(&b, "\troute /gifbox* {\n\t\turi strip_prefix /gifbox\n\t\treverse_proxy http://%s:%d\n\t}\n\n", svcGifbox, portGifbox)
	if withLiveKit {
		fmt.Fprintf(&b, "\troute /livekit* {\n\t\turi strip_prefix /livekit\n\t\treverse_proxy http://%s:%d\n\t}\n\n", svcLiveKit, portLiveKitHTTP)
		fmt.Fprintf(&b, "\troute /ingress* {\n\t\turi strip_prefix /ingress\n\t\treverse_proxy http://%s:%d\n\t}\n\n", svcVoiceIngress, portVoiceIngress)
	}
	// Fingerprinted assets can be cached long; HTML must revalidate so clients
	// don't keep stale chunk refs (crashes like features.invite_only undefined).
	fmt.Fprintf(&b, "\thandle /assets* {\n\t\treverse_proxy http://%s:%d {\n\t\t\theader_down Cache-Control \"public, max-age=31536000, immutable\"\n\t\t}\n\t}\n\n", svcWeb, portWeb)
	fmt.Fprintf(&b, "\thandle {\n\t\treverse_proxy http://%s:%d {\n\t\t\theader_down Cache-Control \"no-cache\"\n\t\t}\n\t}\n}\n", svcWeb, portWeb)
	return b.String()
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
			"HOSTNAME":            ":80",
			"VITE_HOST":           domain,
			"VITE_API_URL":        fmt.Sprintf("https://%s/api", domain),
			"VITE_DEV_WS_URL":     fmt.Sprintf("wss://%s/ws", domain),
			"VITE_DEV_MEDIA_URL":  fmt.Sprintf("https://%s/autumn", domain),
			"VITE_DEV_PROXY_URL":  fmt.Sprintf("https://%s/january", domain),
			"VITE_DEV_GIFBOX_URL": fmt.Sprintf("https://%s/gifbox", domain),
		}
		return nil
	})
	return err
}
