package controller

import (
	"fmt"

	stoatv1alpha1 "github.com/DataKnifeAI/stoat-operator/api/v1alpha1"
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
	return map[string]string{
		"api":           imageRef(img.API, defaultAPIImage, v),
		"events":        imageRef(img.Events, defaultEventsImage, v),
		"autumn":        imageRef(img.Autumn, defaultAutumnImage, v),
		"january":       imageRef(img.January, defaultJanuaryImage, v),
		"gifbox":        imageRef(img.Gifbox, defaultGifboxImage, v),
		"crond":         imageRef(img.Crond, defaultCrondImage, v),
		"pushd":         imageRef(img.Pushd, defaultPushdImage, v),
		"voice-ingress": imageRef(img.VoiceIngress, defaultVoiceIngressImage, v),
		"web":           imageRef(img.Web, defaultWebImage, v),
		"livekit":       imageRef(img.LiveKit, defaultLiveKitImage, "v1.9.14"),
	}
}

func stoatLabels(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "stoat",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/managed-by": "stoat-operator",
		"stoat.dataknife.ai/instance":  name,
	}
}
