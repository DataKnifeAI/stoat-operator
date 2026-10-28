package controller

const (
	finalizerName = "stoat.dataknife.ai/finalizer"

	defaultVersion        = "v0.15.5"
	defaultIngressClass   = "envoy"
	defaultStorageSize    = "20Gi"
	defaultLiveKitRTCPort = int32(7881)
	defaultUDPPortStart   = int32(50000)
	defaultUDPPortEnd     = int32(50100)

	defaultAPIImage          = "ghcr.io/stoatchat/api"
	defaultEventsImage       = "ghcr.io/stoatchat/events"
	defaultAutumnImage       = "ghcr.io/stoatchat/file-server"
	defaultJanuaryImage      = "ghcr.io/stoatchat/proxy"
	defaultGifboxImage       = "ghcr.io/stoatchat/gifbox"
	defaultCrondImage        = "ghcr.io/stoatchat/crond"
	defaultPushdImage        = "ghcr.io/stoatchat/pushd"
	defaultVoiceIngressImage = "ghcr.io/stoatchat/voice-ingress"
	defaultWebImage          = "ghcr.io/stoatchat/for-web"
	defaultLiveKitImage      = "ghcr.io/stoatchat/livekit-server"
)
