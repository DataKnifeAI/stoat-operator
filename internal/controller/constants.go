package controller

const (
	finalizerName = "stoat.dataknife.ai/finalizer"

	defaultVersion        = "v0.16.2"
	// compose pins for-web:f080e84 (arm64-only today). :main publishes amd64.
	defaultWebImageTag = "main"
	defaultWebImage    = "ghcr.io/stoatchat/for-web"
	defaultIngressClass   = "nginx"
	defaultStorageSize    = "20Gi"
	defaultLiveKitRTCPort = int32(7881)
	defaultUDPPortStart   = int32(50000)
	defaultUDPPortEnd     = int32(50100)
	defaultLiveKitImageTag = "v1.9.14"

	defaultAPIImage          = "ghcr.io/stoatchat/api"
	defaultEventsImage       = "ghcr.io/stoatchat/events"
	defaultAutumnImage       = "ghcr.io/stoatchat/file-server"
	defaultJanuaryImage      = "ghcr.io/stoatchat/proxy"
	defaultGifboxImage       = "ghcr.io/stoatchat/gifbox"
	defaultCrondImage        = "ghcr.io/stoatchat/crond"
	defaultPushdImage        = "ghcr.io/stoatchat/pushd"
	defaultVoiceIngressImage = "ghcr.io/stoatchat/voice-ingress"
	defaultLiveKitImage      = "ghcr.io/stoatchat/livekit-server"

	mongoImage   = "docker.io/mongo:8"
	redisImage   = "valkey/valkey:9-alpine"
	rabbitImage  = "docker.io/rabbitmq:4"
	minioImage   = "docker.io/pgsty/silo"
	mcImage      = "docker.io/pgsty/mc"
	caddyImage   = "docker.io/caddy:2"

	// Compose-compatible Service names so Stoat defaults resolve in-cluster.
	// One Stoat CR per namespace is recommended.
	svcDatabase     = "database"
	svcRedis        = "redis"
	svcRabbit       = "rabbit"
	svcMinio        = "minio"
	svcAPI          = "api"
	svcEvents       = "events"
	svcAutumn       = "autumn"
	svcJanuary      = "january"
	svcGifbox       = "gifbox"
	svcCrond        = "crond"
	svcPushd        = "pushd"
	svcVoiceIngress = "voice-ingress"
	svcLiveKit      = "livekit"
	svcLiveKitRTC   = "livekit-rtc"
	svcWeb          = "web"
	svcCaddy        = "caddy"

	secretName     = "stoat-secrets"
	configName     = "stoat-config"
	livekitCfgName = "stoat-livekit"
	caddyCfgName   = "stoat-caddy"
	webEnvName     = "stoat-web-env"

	minioUser = "minioautumn"
	minioPass = "minioautumn"
	rabbitUser = "rabbituser"
	rabbitPass = "rabbitpass"

	portAPI          int32 = 14702
	portEvents       int32 = 14703
	portAutumn       int32 = 14704
	portJanuary      int32 = 14705
	portGifbox       int32 = 14706
	portWeb          int32 = 5000
	portCaddy        int32 = 80
	portLiveKitHTTP  int32 = 7880
	portVoiceIngress int32 = 8500
	portMongo        int32 = 27017
	portRedis        int32 = 6379
	portRabbit       int32 = 5672
	portMinio        int32 = 9000

	keyVapidPrivate = "REVOLT__PUSHD__VAPID__PRIVATE_KEY"
	keyVapidPublic  = "REVOLT__PUSHD__VAPID__PUBLIC_KEY"
	keyFilesEnc     = "REVOLT__FILES__ENCRYPTION_KEY"
	keyLiveKitKey   = "REVOLT__API__LIVEKIT__NODES__WORLDWIDE__KEY"
	keyLiveKitSec   = "REVOLT__API__LIVEKIT__NODES__WORLDWIDE__SECRET"

	// Autumn reads these via the config crate's REVOLT__ env overlay (wins over Revolt.toml).
	// Required so path_style_buckets is not left at Stoat's embedded default (false).
	keyS3Endpoint   = "REVOLT__FILES__S3__ENDPOINT"
	keyS3PathStyle  = "REVOLT__FILES__S3__PATH_STYLE_BUCKETS"
	keyS3Region     = "REVOLT__FILES__S3__REGION"
	keyS3AccessKey  = "REVOLT__FILES__S3__ACCESS_KEY_ID"
	keyS3SecretKey  = "REVOLT__FILES__S3__SECRET_ACCESS_KEY"
	keyS3Bucket     = "REVOLT__FILES__S3__DEFAULT_BUCKET"
)
