package controller

import (
	"fmt"
	"strings"

	stoatv1alpha1 "github.com/DataKnifeAI/stoat-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func boolDefault(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

func versionOrDefault(spec stoatv1alpha1.StoatSpec) string {
	if spec.Version != "" {
		return spec.Version
	}
	return defaultVersion
}

func publicURL(domain string) string {
	if domain == "" {
		return ""
	}
	return fmt.Sprintf("https://%s", domain)
}

func imageRef(override, repo, version string) string {
	if override != "" {
		return override
	}
	return fmt.Sprintf("%s:%s", repo, version)
}

func resolveImages(spec stoatv1alpha1.StoatSpec) map[string]string {
	v := versionOrDefault(spec)
	img := spec.Images
	web := imageRef(img.Web, defaultWebImage, defaultWebImageTag)
	return map[string]string{
		"api":           imageRef(img.API, defaultAPIImage, v),
		"events":        imageRef(img.Events, defaultEventsImage, v),
		"autumn":        imageRef(img.Autumn, defaultAutumnImage, v),
		"january":       imageRef(img.January, defaultJanuaryImage, v),
		"gifbox":        imageRef(img.Gifbox, defaultGifboxImage, v),
		"crond":         imageRef(img.Crond, defaultCrondImage, v),
		"pushd":         imageRef(img.Pushd, defaultPushdImage, v),
		"voice-ingress": imageRef(img.VoiceIngress, defaultVoiceIngressImage, v),
		"web":           web,
		"livekit":       imageRef(img.LiveKit, defaultLiveKitImage, defaultLiveKitImageTag),
		"admin-panel":   imageRef(img.AdminPanel, defaultAdminPanelImage, defaultAdminPanelTag),
	}
}

func adminEnabled(spec stoatv1alpha1.StoatSpec) bool {
	return spec.Admin != nil && boolDefault(spec.Admin.Enabled, false)
}

func stoatLabels(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "stoat",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/managed-by": "stoat-operator",
		"stoat.dataknife.ai/instance":  name,
	}
}

func componentLabels(instance, component string) map[string]string {
	labels := stoatLabels(instance)
	labels["app.kubernetes.io/component"] = component
	return labels
}

func storageSizeOrDefault(spec stoatv1alpha1.StoatSpec) resource.Quantity {
	size := defaultStorageSize
	if spec.Storage.Size != "" {
		size = spec.Storage.Size
	}
	q, err := resource.ParseQuantity(size)
	if err != nil {
		return resource.MustParse(defaultStorageSize)
	}
	return q
}

func pullPolicy(spec stoatv1alpha1.StoatSpec) corev1.PullPolicy {
	if spec.ImagePullPolicy == "" {
		return corev1.PullIfNotPresent
	}
	return spec.ImagePullPolicy
}

func secretNameFor(instance *stoatv1alpha1.Stoat) string {
	if instance.Spec.SecretsRef != "" {
		return instance.Spec.SecretsRef
	}
	return secretName
}

func int64Ptr(v int64) *int64 { return &v }

func boolPtr(v bool) *bool { return &v }

func tcpProbe(port int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt32(port),
			},
		},
		InitialDelaySeconds: 10,
		PeriodSeconds:       15,
		TimeoutSeconds:      5,
	}
}

func liveKitEnabled(spec stoatv1alpha1.StoatSpec) bool {
	return boolDefault(spec.LiveKit.Enabled, true)
}

func objectStorageExternal(spec stoatv1alpha1.StoatSpec) bool {
	os := spec.Dependencies.ObjectStorage
	return os != nil && strings.TrimSpace(os.Endpoint) != ""
}

func objectStorageBucket(spec stoatv1alpha1.StoatSpec) string {
	if spec.Dependencies.ObjectStorage != nil && spec.Dependencies.ObjectStorage.Bucket != "" {
		return spec.Dependencies.ObjectStorage.Bucket
	}
	return "revolt-uploads"
}

func objectStorageRegion(spec stoatv1alpha1.StoatSpec) string {
	if spec.Dependencies.ObjectStorage != nil && spec.Dependencies.ObjectStorage.Region != "" {
		return spec.Dependencies.ObjectStorage.Region
	}
	return "minio"
}

func objectStoragePathStyle(spec stoatv1alpha1.StoatSpec) bool {
	if spec.Dependencies.ObjectStorage != nil && spec.Dependencies.ObjectStorage.PathStyleBuckets != nil {
		return *spec.Dependencies.ObjectStorage.PathStyleBuckets
	}
	return true
}

func liveKitUseExternalIP(spec stoatv1alpha1.StoatSpec) bool {
	return boolDefault(spec.LiveKit.UseExternalIP, false)
}

func liveKitCoords(spec stoatv1alpha1.StoatSpec) (lat, lon float64) {
	if spec.LiveKit.Latitude != nil {
		lat = *spec.LiveKit.Latitude
	}
	if spec.LiveKit.Longitude != nil {
		lon = *spec.LiveKit.Longitude
	}
	return lat, lon
}

func liveKitPorts(spec stoatv1alpha1.StoatSpec) (rtc, udpStart, udpEnd int32) {
	rtc = defaultLiveKitRTCPort
	udpStart = defaultUDPPortStart
	udpEnd = defaultUDPPortEnd
	if spec.LiveKit.RTCPort != 0 {
		rtc = spec.LiveKit.RTCPort
	}
	if spec.LiveKit.UDPPortStart != 0 {
		udpStart = spec.LiveKit.UDPPortStart
	}
	if spec.LiveKit.UDPPortEnd != 0 {
		udpEnd = spec.LiveKit.UDPPortEnd
	}
	return rtc, udpStart, udpEnd
}

func ingressClass(spec stoatv1alpha1.StoatSpec) string {
	if spec.Ingress.ClassName != "" {
		return spec.Ingress.ClassName
	}
	return defaultIngressClass
}
