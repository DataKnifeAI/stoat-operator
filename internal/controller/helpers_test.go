package controller

import (
	"testing"

	stoatv1alpha1 "github.com/DataKnifeAI/stoat-operator/api/v1alpha1"
)

func TestVersionOrDefault(t *testing.T) {
	if got := versionOrDefault(stoatv1alpha1.StoatSpec{}); got != defaultVersion {
		t.Fatalf("expected default %q, got %q", defaultVersion, got)
	}
	if got := versionOrDefault(stoatv1alpha1.StoatSpec{Version: "v0.16.0"}); got != "v0.16.0" {
		t.Fatalf("expected pin, got %q", got)
	}
}

func TestPublicURL(t *testing.T) {
	if got := publicURL("chat.example.com"); got != "https://chat.example.com" {
		t.Fatalf("unexpected url %q", got)
	}
	if got := publicURL(""); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestResolveImages(t *testing.T) {
	images := resolveImages(stoatv1alpha1.StoatSpec{
		Version: "v0.15.5",
		Images:  stoatv1alpha1.ImageOverrides{API: "example.com/api:custom"},
	})
	if images["api"] != "example.com/api:custom" {
		t.Fatalf("api override missing: %q", images["api"])
	}
	if images["events"] != "ghcr.io/stoatchat/events:v0.15.5" {
		t.Fatalf("events image unexpected: %q", images["events"])
	}
}

func TestBoolDefault(t *testing.T) {
	if !boolDefault(nil, true) {
		t.Fatal("nil should use default true")
	}
	f := false
	if boolDefault(&f, true) {
		t.Fatal("explicit false should win")
	}
}
