package controller

import (
	"context"
	"fmt"
	"strings"

	stoatv1alpha1 "github.com/DataKnifeAI/stoat-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type s3Settings struct {
	Endpoint         string
	AccessKeyID      string
	SecretAccessKey  string
	Bucket           string
	Region           string
	PathStyleBuckets bool
}

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
	s3, err := r.resolveS3Settings(ctx, instance)
	if err != nil {
		return err
	}
	if err := r.reconcileS3EnvSecret(ctx, instance, s3); err != nil {
		return err
	}
	if err := r.reconcileGifboxEnvSecret(ctx, instance); err != nil {
		return err
	}
	domain := instance.Spec.Domain
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configName,
			Namespace: instance.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if err := controllerutil.SetControllerReference(instance, cm, r.Scheme); err != nil {
			return err
		}
		cm.Labels = stoatLabels(instance.Name)
		mongoHost := svcDatabase
		redisHost := svcRedis
		rabbitHost := svcRabbit
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
			"Revolt.toml": renderRevoltTOML(domain, mongoHost, redisHost, rabbitHost, s3, secrets, liveKitEnabled(instance.Spec), lat, lon),
			"stoat.json":  fmt.Sprintf(`{"api":"https://%s/api"}`, domain),
		}
		return nil
	})
	return err
}

// reconcileGifboxEnvSecret copies the Gifbox Compatibility API key into stoat-secrets
// as REVOLT__API__SECURITY__TENOR_KEY (Stoat's historical field name for the upstream key).
func (r *StoatReconciler) reconcileGifboxEnvSecret(ctx context.Context, instance *stoatv1alpha1.Stoat) error {
	if instance.Spec.Gifbox == nil || strings.TrimSpace(instance.Spec.Gifbox.CredentialsSecret) == "" {
		return nil
	}
	srcName := strings.TrimSpace(instance.Spec.Gifbox.CredentialsSecret)
	src := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: srcName, Namespace: instance.Namespace}, src); err != nil {
		return fmt.Errorf("gifbox credentials secret %q: %w", srcName, err)
	}
	key := string(src.Data["apiKey"])
	if key == "" {
		key = string(src.Data["tenorKey"])
	}
	if key == "" {
		return fmt.Errorf("secret %q must contain apiKey (or tenorKey)", srcName)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretNameFor(instance),
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, secret, func() error {
		if err := controllerutil.SetControllerReference(instance, secret, r.Scheme); err != nil {
			return err
		}
		secret.Labels = stoatLabels(instance.Name)
		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		secret.Type = corev1.SecretTypeOpaque
		secret.Data[keyTenorKey] = []byte(key)
		return nil
	})
	return err
}

// reconcileS3EnvSecret writes Autumn S3 settings into stoat-secrets as REVOLT__FILES__S3__*
// env vars. Stoat's config crate applies REVOLT__ after Revolt.toml; without this, Autumn
// can keep the embedded default path_style_buckets=false and fail puts against RustFS
// (virtual-host NXDOMAIN on <bucket>.<host>).
func (r *StoatReconciler) reconcileS3EnvSecret(ctx context.Context, instance *stoatv1alpha1.Stoat, s3 s3Settings) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretNameFor(instance),
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, secret, func() error {
		if err := controllerutil.SetControllerReference(instance, secret, r.Scheme); err != nil {
			return err
		}
		secret.Labels = stoatLabels(instance.Name)
		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		secret.Type = corev1.SecretTypeOpaque
		pathStyle := "false"
		if s3.PathStyleBuckets {
			pathStyle = "true"
		}
		secret.Data[keyS3Endpoint] = []byte(s3.Endpoint)
		secret.Data[keyS3PathStyle] = []byte(pathStyle)
		secret.Data[keyS3Region] = []byte(s3.Region)
		secret.Data[keyS3AccessKey] = []byte(s3.AccessKeyID)
		secret.Data[keyS3SecretKey] = []byte(s3.SecretAccessKey)
		secret.Data[keyS3Bucket] = []byte(s3.Bucket)
		return nil
	})
	return err
}

func (r *StoatReconciler) resolveS3Settings(ctx context.Context, instance *stoatv1alpha1.Stoat) (s3Settings, error) {
	s3 := s3Settings{
		Endpoint:         fmt.Sprintf("http://%s:%d", svcMinio, portMinio),
		AccessKeyID:      minioUser,
		SecretAccessKey:  minioPass,
		Bucket:           objectStorageBucket(instance.Spec),
		Region:           objectStorageRegion(instance.Spec),
		PathStyleBuckets: objectStoragePathStyle(instance.Spec),
	}
	if !objectStorageExternal(instance.Spec) {
		return s3, nil
	}
	os := instance.Spec.Dependencies.ObjectStorage
	s3.Endpoint = strings.TrimRight(strings.TrimSpace(os.Endpoint), "/")
	if os.CredentialsSecret == "" {
		return s3Settings{}, fmt.Errorf("dependencies.objectStorage.credentialsSecret is required when endpoint is set")
	}
	sec := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: os.CredentialsSecret, Namespace: instance.Namespace}, sec); err != nil {
		return s3Settings{}, fmt.Errorf("objectStorage credentials secret %q: %w", os.CredentialsSecret, err)
	}
	ak := string(sec.Data["accessKeyId"])
	sk := string(sec.Data["secretAccessKey"])
	if ak == "" || sk == "" {
		return s3Settings{}, fmt.Errorf("secret %q must contain accessKeyId and secretAccessKey", os.CredentialsSecret)
	}
	s3.AccessKeyID = ak
	s3.SecretAccessKey = sk
	return s3, nil
}

func renderRevoltTOML(domain, mongoURL, redisURL, rabbitHost string, s3 s3Settings, secrets *stoatSecrets, withLiveKit bool, lat, lon float64) string {
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
	fmt.Fprintf(&b, "endpoint = %q\n", s3.Endpoint)
	fmt.Fprintf(&b, "path_style_buckets = %t\n", s3.PathStyleBuckets)
	fmt.Fprintf(&b, "region = %q\n", s3.Region)
	fmt.Fprintf(&b, "access_key_id = %q\n", s3.AccessKeyID)
	fmt.Fprintf(&b, "secret_access_key = %q\n", s3.SecretAccessKey)
	fmt.Fprintf(&b, "default_bucket = %q\n", s3.Bucket)
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
	// Use mutually exclusive handle blocks only. Mixing route + catch-all handle
	// let /api (and friends) fall through to the web SPA.
	var b strings.Builder
	b.WriteString(`:80 {
	handle /.well-known/stoat* {
		uri strip_prefix /.well-known/stoat
		header {
			Access-Control-Allow-Origin *
		}
		rewrite * /stoat.json
		file_server {
			root /config
		}
	}

	handle /stoat.json {
		header {
			Access-Control-Allow-Origin *
		}
		rewrite * /stoat.json
		file_server {
			root /config
		}
	}

`)
	fmt.Fprintf(&b, "\thandle /api* {\n\t\turi strip_prefix /api\n\t\treverse_proxy http://%s:%d\n\t}\n\n", svcAPI, portAPI)
	fmt.Fprintf(&b, "\thandle /ws {\n\t\turi strip_prefix /ws\n\t\treverse_proxy http://%s:%d\n\t}\n\n", svcEvents, portEvents)
	fmt.Fprintf(&b, "\thandle /autumn* {\n\t\turi strip_prefix /autumn\n\t\treverse_proxy http://%s:%d\n\t}\n\n", svcAutumn, portAutumn)
	fmt.Fprintf(&b, "\thandle /january* {\n\t\turi strip_prefix /january\n\t\treverse_proxy http://%s:%d\n\t}\n\n", svcJanuary, portJanuary)
	fmt.Fprintf(&b, "\thandle /gifbox* {\n\t\turi strip_prefix /gifbox\n\t\treverse_proxy http://%s:%d\n\t}\n\n", svcGifbox, portGifbox)
	if withLiveKit {
		fmt.Fprintf(&b, "\thandle /livekit* {\n\t\turi strip_prefix /livekit\n\t\treverse_proxy http://%s:%d\n\t}\n\n", svcLiveKit, portLiveKitHTTP)
		fmt.Fprintf(&b, "\thandle /ingress* {\n\t\turi strip_prefix /ingress\n\t\treverse_proxy http://%s:%d\n\t}\n\n", svcVoiceIngress, portVoiceIngress)
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
		gifbox := fmt.Sprintf("https://%s/gifbox", domain)
		cm.Data = map[string]string{
			"HOSTNAME":            ":80",
			"VITE_HOST":           domain,
			"VITE_API_URL":        fmt.Sprintf("https://%s/api", domain),
			"VITE_DEV_WS_URL":     fmt.Sprintf("wss://%s/ws", domain),
			"VITE_DEV_MEDIA_URL":  fmt.Sprintf("https://%s/autumn", domain),
			"VITE_DEV_PROXY_URL":  fmt.Sprintf("https://%s/january", domain),
			"VITE_DEV_GIFBOX_URL": gifbox,
			// Production for-web injects this (DEV_* is ignored outside Vite DEV).
			"VITE_GIFBOX_URL": gifbox,
		}
		return nil
	})
	return err
}
