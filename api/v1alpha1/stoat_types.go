package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PhasePending = "Pending"
	PhaseRunning = "Running"
	PhaseFailed  = "Failed"
)

// ImageOverrides optionally pins component container images.
// Defaults follow ghcr.io/stoatchat/* tags from the upstream self-hosted compose stack.
type ImageOverrides struct {
	// API is the Stoat API server image.
	// +optional
	API string `json:"api,omitempty"`

	// Events is the Stoat events service image.
	// +optional
	Events string `json:"events,omitempty"`

	// Autumn is the file server (autumn) image.
	// +optional
	Autumn string `json:"autumn,omitempty"`

	// January is the metadata/image proxy image.
	// +optional
	January string `json:"january,omitempty"`

	// Gifbox is the Tenor proxy image.
	// +optional
	Gifbox string `json:"gifbox,omitempty"`

	// Crond is the task daemon image.
	// +optional
	Crond string `json:"crond,omitempty"`

	// Pushd is the push notification daemon image.
	// +optional
	Pushd string `json:"pushd,omitempty"`

	// VoiceIngress is the voice ingress daemon image.
	// +optional
	VoiceIngress string `json:"voiceIngress,omitempty"`

	// Web is the for-web front-end image.
	// +optional
	Web string `json:"web,omitempty"`

	// LiveKit is the LiveKit server image.
	// +optional
	LiveKit string `json:"livekit,omitempty"`

	// AdminPanel is the Stoat admin panel image (DataKnifeAI/stoat-admin).
	// +optional
	AdminPanel string `json:"adminPanel,omitempty"`
}

// ExternalDependencyRef points at an existing in-cluster or external service.
type ExternalDependencyRef struct {
	// URI is a connection string or host URL for the dependency.
	// +optional
	URI string `json:"uri,omitempty"`

	// SecretRef holds credentials (connection URI, passwords, keys).
	// +optional
	SecretRef *corev1.SecretKeySelector `json:"secretRef,omitempty"`
}

// ObjectStorageSpec configures Autumn's S3-compatible backend.
// Omit to use the embedded MinIO/Silo deployment (when dependencies.embed is true).
// Set endpoint + credentialsSecret to use external storage (e.g. RustFS).
type ObjectStorageSpec struct {
	// Endpoint is the S3 API URL (no trailing slash), e.g. https://rustfs.example:30292
	// or http://minio:9000 for the embedded service.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// CredentialsSecret names a Secret with keys accessKeyId and secretAccessKey.
	// Required when Endpoint is set to an external service.
	// +optional
	CredentialsSecret string `json:"credentialsSecret,omitempty"`

	// Bucket is the S3 bucket name for encrypted uploads.
	// +kubebuilder:default="revolt-uploads"
	// +optional
	Bucket string `json:"bucket,omitempty"`

	// Region is the S3 region string written into Revolt.toml.
	// +kubebuilder:default="minio"
	// +optional
	Region string `json:"region,omitempty"`

	// PathStyleBuckets enables path-style addressing (bucket in URL path).
	// Default true — required for Kubernetes and most S3-compatible stores
	// (RustFS, etc.). Docker Compose MinIO often uses virtual-host style instead.
	// +kubebuilder:default=true
	// +optional
	PathStyleBuckets *bool `json:"pathStyleBuckets,omitempty"`
}

// DependenciesSpec configures data-plane dependencies.
// When embed is true (default), the operator provisions bundled MongoDB,
// Valkey, RabbitMQ, and object storage. External refs override embed for that
// dependency.
type DependenciesSpec struct {
	// Embed provisions in-cluster dependency workloads when true.
	// +kubebuilder:default=true
	// +optional
	Embed *bool `json:"embed,omitempty"`

	// MongoDB external connection (overrides embed for MongoDB when set).
	// +optional
	MongoDB *ExternalDependencyRef `json:"mongodb,omitempty"`

	// Redis/Valkey external connection.
	// +optional
	Redis *ExternalDependencyRef `json:"redis,omitempty"`

	// RabbitMQ external connection.
	// +optional
	RabbitMQ *ExternalDependencyRef `json:"rabbitmq,omitempty"`

	// ObjectStorage selects Autumn's S3 backend.
	// Omit for embedded MinIO/Silo; set endpoint + credentialsSecret for
	// external S3 (e.g. RustFS on TrueNAS).
	// +optional
	ObjectStorage *ObjectStorageSpec `json:"objectStorage,omitempty"`
}

// IngressSpec configures HTTP(S) exposure for the web UI and API.
type IngressSpec struct {
	// ClassName is the IngressClass or GatewayClass name.
	// +kubebuilder:default="nginx"
	// +optional
	ClassName string `json:"className,omitempty"`

	// Annotations applied to the Ingress / HTTPRoute resources.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// TLSSecretName is an optional TLS Secret for HTTPS termination.
	// +optional
	TLSSecretName string `json:"tlsSecretName,omitempty"`
}

// LiveKitSpec configures voice/RTC exposure.
// The LiveKit pod uses hostNetwork so TCP 7881 and UDP 50000–50100 bind on the
// node. Signaling HTTP (7880) is also available via the in-cluster Service for
// Caddy's /livekit* route.
type LiveKitSpec struct {
	// Enabled toggles LiveKit / voice-ingress workloads and advertises voice
	// endpoints in Revolt.toml / Caddy. When false, voice config and routes are omitted.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// UseExternalIP discovers a public WAN IP via STUN for RTC candidates.
	// Leave false (default) for site-local access: clients on the LAN/VPN reach
	// LiveKit on the node (or VIP) address. This is not ClusterIP-only — media
	// still uses hostNetwork on the local network, not the pod network.
	// +kubebuilder:default=false
	// +optional
	UseExternalIP *bool `json:"useExternalIP,omitempty"`

	// Address is an optional stable LAN/VPN IP advertised to clients for RTC ICE
	// (typically a kube-vip / MetalLB VIP). When empty, LiveKit advertises the
	// scheduled node's LAN address (status.hostIP). Not a cluster-internal IP.
	// When set with serviceType=LoadBalancer, pin LiveKit onto the VIP host via
	// nodeName (or ensure kube-vip moves the VIP onto LiveKit's node) — otherwise
	// ICE advertises the VIP while UDP/TCP 7881 listen on a different node.
	// +optional
	Address string `json:"address,omitempty"`

	// NodeName pins the LiveKit hostNetwork pod to a specific node. Required when
	// Address is a kube-vip VIP with externalTrafficPolicy=Local: LiveKit must
	// run on the same node that holds the VIP or clients cannot complete ICE.
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// Latitude of this LiveKit node (used by Stoat for nearest-node selection).
	// +optional
	Latitude *float64 `json:"latitude,omitempty"`

	// Longitude of this LiveKit node (used by Stoat for nearest-node selection).
	// +optional
	Longitude *float64 `json:"longitude,omitempty"`

	// RTCPort is the LiveKit TCP RTC port (hostNetwork).
	// +kubebuilder:default=7881
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +optional
	RTCPort int32 `json:"rtcPort,omitempty"`

	// UDPPortStart is the first UDP port in the RTC media range (hostNetwork).
	// +kubebuilder:default=50000
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +optional
	UDPPortStart int32 `json:"udpPortStart,omitempty"`

	// UDPPortEnd is the last UDP port in the RTC media range (hostNetwork).
	// +kubebuilder:default=50100
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +optional
	UDPPortEnd int32 `json:"udpPortEnd,omitempty"`

	// ServiceType for the optional RTC exposure Service (livekit-rtc).
	// Signaling stays ClusterIP for Caddy. Use LoadBalancer with address set
	// to claim a kube-vip VIP for TCP 7881 + the UDP media range.
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +kubebuilder:default=ClusterIP
	// +optional
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`
}

// GifboxSpec configures Autumn's sibling GIF proxy (gifbox service).
type GifboxSpec struct {
	// CredentialsSecret names a Secret with key apiKey (Gifbox Compatibility API key).
	// Synced into stoat-secrets as REVOLT__API__SECURITY__TENOR_KEY for gifbox/api.
	// +optional
	CredentialsSecret string `json:"credentialsSecret,omitempty"`
}

// AdminSpec configures the optional Stoat admin panel.
// Image source: https://github.com/DataKnifeAI/stoat-admin
// Exposed on a dedicated Ingress host with Authentik SSO (not via Stoat Caddy).
type AdminSpec struct {
	// Enabled deploys the admin panel when true.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Domain is the public hostname for the admin Ingress
	// (e.g. stoat-admin.dataknife.net). Required when Enabled.
	// +optional
	Domain string `json:"domain,omitempty"`

	// CredentialsSecret names a Secret with Authentik + NextAuth keys:
	// authentikId, authentikSecret, authentikIssuer (no trailing slash),
	// and optional nextAuthSecret (generated into stoat-admin-env when missing).
	// +optional
	CredentialsSecret string `json:"credentialsSecret,omitempty"`

	// PlatformAccountId is the Stoat user ULID used as PLATFORM_ACCOUNT_ID
	// for admin-originated platform DMs / actions.
	// +optional
	PlatformAccountId string `json:"platformAccountId,omitempty"`

	// TLSSecretName optionally sets Ingress TLS for the admin host.
	// When empty, falls back to spec.ingress.tlsSecretName.
	// +optional
	TLSSecretName string `json:"tlsSecretName,omitempty"`
}

// StorageSpec configures persistent volumes for databases and object storage.
type StorageSpec struct {
	// Size is the default PVC request for embedded data volumes.
	// +kubebuilder:default="20Gi"
	// +optional
	Size string `json:"size,omitempty"`

	// StorageClassName selects the StorageClass for PVCs.
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`
}

// ScalingSpec controls horizontal replicas for stateless Stoat components.
// Embedded Mongo/Redis/Rabbit/MinIO and LiveKit stay single-replica (use
// external managed deps for true data-plane HA).
type ScalingSpec struct {
	// Replicas is the default replica count for scalable app components
	// (api, events, autumn, january, gifbox, web, caddy, pushd, admin-panel).
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// API overrides replicas for the API deployment.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	API *int32 `json:"api,omitempty"`

	// Events overrides replicas for the events deployment.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	Events *int32 `json:"events,omitempty"`

	// Autumn overrides replicas for the file server.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	Autumn *int32 `json:"autumn,omitempty"`

	// January overrides replicas for the proxy.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	January *int32 `json:"january,omitempty"`

	// Gifbox overrides replicas for gifbox.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	Gifbox *int32 `json:"gifbox,omitempty"`

	// Web overrides replicas for the for-web front-end.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	Web *int32 `json:"web,omitempty"`

	// Caddy overrides replicas for the edge proxy.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	Caddy *int32 `json:"caddy,omitempty"`

	// Pushd overrides replicas for push notifications.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	Pushd *int32 `json:"pushd,omitempty"`

	// AdminPanel overrides replicas for the admin panel.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	AdminPanel *int32 `json:"adminPanel,omitempty"`
}

// StoatSpec defines the desired state of a Stoat instance.
// Models the upstream self-hosted compose stack as a single umbrella CR:
// https://github.com/stoatchat/self-hosted
type StoatSpec struct {
	// Domain is the public hostname for this Stoat instance (e.g. chat.example.com).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Domain string `json:"domain"`

	// Version pins the Stoat component image tag family (e.g. v0.16.2).
	// +kubebuilder:default="v0.16.2"
	// +optional
	Version string `json:"version,omitempty"`

	// Images optionally overrides individual component images.
	// +optional
	Images ImageOverrides `json:"images,omitempty"`

	// GenerateSecrets when true creates an owned Secret with Stoat encryption
	// keys and service passwords. Losing these secrets can permanently lock
	// file access — back them up.
	// +kubebuilder:default=true
	// +optional
	GenerateSecrets *bool `json:"generateSecrets,omitempty"`

	// SecretsRef names an existing Secret with Stoat keys (skips generation).
	// Expected keys align with upstream secrets.env.
	// +optional
	SecretsRef string `json:"secretsRef,omitempty"`

	// Dependencies configures MongoDB, Valkey/Redis, RabbitMQ, and object storage.
	// +optional
	Dependencies DependenciesSpec `json:"dependencies,omitempty"`

	// Ingress configures HTTP(S) exposure for web and API.
	// +optional
	Ingress IngressSpec `json:"ingress,omitempty"`

	// LiveKit configures voice/RTC networking.
	// +optional
	LiveKit LiveKitSpec `json:"livekit,omitempty"`

	// Gifbox configures the GIF picker upstream (Gifbox Compatibility API).
	// Without an API key, gifbox panics on search/trending against compat.gifbox.me.
	// Create a key at https://gifbox.me/settings after signing in.
	// +optional
	Gifbox *GifboxSpec `json:"gifbox,omitempty"`

	// Admin configures the optional admin panel (Authentik SSO, separate Ingress).
	// +optional
	Admin *AdminSpec `json:"admin,omitempty"`

	// Scaling configures horizontal replicas for stateless components.
	// Set replicas: 2 (or higher) for app-tier HA behind Services/Ingress.
	// +optional
	Scaling *ScalingSpec `json:"scaling,omitempty"`

	// Storage configures PVC defaults for embedded data services.
	// +optional
	Storage StorageSpec `json:"storage,omitempty"`

	// ImagePullPolicy for Stoat component containers.
	// +kubebuilder:default=IfNotPresent
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ImagePullSecrets for private registries.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// NodeSelector pins Stoat pods to specific nodes.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Resources applied as defaults to app containers when set.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// StoatStatus defines the observed state of Stoat.
type StoatStatus struct {
	// Phase summarizes high-level lifecycle state.
	// +optional
	Phase string `json:"phase,omitempty"`

	// Ready indicates the instance is serving traffic.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// URL is the public web URL derived from spec.domain.
	// +optional
	URL string `json:"url,omitempty"`

	// ObservedVersion is the Stoat version currently reconciled.
	// +optional
	ObservedVersion string `json:"observedVersion,omitempty"`

	// Message provides human-readable status details.
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=st
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Domain",type=string,JSONPath=`.spec.domain`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.observedVersion`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Stoat is the Schema for the stoats API — an umbrella CR for a self-hosted
// Stoat chat instance (API, web, files, push, voice, and dependencies).
type Stoat struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StoatSpec   `json:"spec,omitempty"`
	Status StoatStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StoatList contains a list of Stoat.
type StoatList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Stoat `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Stoat{}, &StoatList{})
}
