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

// DependenciesSpec configures data-plane dependencies.
// When embed is true (default), the operator will eventually provision bundled
// MongoDB, Valkey, RabbitMQ, and object storage. External refs override embed
// for that dependency.
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

	// ObjectStorage is an S3-compatible endpoint (MinIO/Silo or external S3).
	// +optional
	ObjectStorage *ExternalDependencyRef `json:"objectStorage,omitempty"`
}

// IngressSpec configures HTTP(S) exposure for the web UI and API.
type IngressSpec struct {
	// ClassName is the IngressClass or GatewayClass name.
	// +kubebuilder:default="envoy"
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
// LiveKit requires direct access to TCP 7881 and a UDP port range; it cannot
// sit behind a normal HTTP Ingress without special RTC handling.
type LiveKitSpec struct {
	// Enabled toggles LiveKit / voice-ingress workloads.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// RTCPort is the LiveKit TCP RTC port.
	// +kubebuilder:default=7881
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +optional
	RTCPort int32 `json:"rtcPort,omitempty"`

	// UDPPortStart is the first UDP port in the RTC media range.
	// +kubebuilder:default=50000
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +optional
	UDPPortStart int32 `json:"udpPortStart,omitempty"`

	// UDPPortEnd is the last UDP port in the RTC media range.
	// +kubebuilder:default=50100
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +optional
	UDPPortEnd int32 `json:"udpPortEnd,omitempty"`

	// ServiceType for LiveKit exposure (NodePort or LoadBalancer recommended).
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +kubebuilder:default=LoadBalancer
	// +optional
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`
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

// StoatSpec defines the desired state of a Stoat instance.
// Models the upstream self-hosted compose stack as a single umbrella CR:
// https://github.com/stoatchat/self-hosted
type StoatSpec struct {
	// Domain is the public hostname for this Stoat instance (e.g. chat.example.com).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Domain string `json:"domain"`

	// Version pins the Stoat component image tag family (e.g. v0.15.5).
	// +kubebuilder:default="v0.15.5"
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
