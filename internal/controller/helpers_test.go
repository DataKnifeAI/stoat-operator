package controller

import (
	"strings"
	"testing"

	stoatv1alpha1 "github.com/DataKnifeAI/stoat-operator/api/v1alpha1"
)

func TestVersionOrDefault(t *testing.T) {
	if got := versionOrDefault(stoatv1alpha1.StoatSpec{}); got != defaultVersion {
		t.Fatalf("expected default %q, got %q", defaultVersion, got)
	}
	if got := versionOrDefault(stoatv1alpha1.StoatSpec{Version: "v0.17.0"}); got != "v0.17.0" {
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
		Version: "v0.16.2",
		Images:  stoatv1alpha1.ImageOverrides{API: "example.com/api:custom"},
	})
	if images["api"] != "example.com/api:custom" {
		t.Fatalf("api override missing: %q", images["api"])
	}
	if images["events"] != "ghcr.io/stoatchat/events:v0.16.2" {
		t.Fatalf("events image unexpected: %q", images["events"])
	}
	if images["web"] != "ghcr.io/stoatchat/for-web:main" {
		t.Fatalf("web image unexpected: %q", images["web"])
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

func TestGenerateStoatSecrets(t *testing.T) {
	s, err := generateStoatSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if s.VapidPrivate == "" || s.VapidPublic == "" || s.FilesEncKey == "" || s.LiveKitKey == "" || s.LiveKitSec == "" {
		t.Fatalf("incomplete secrets: %+v", s)
	}
}

func TestRenderRevoltTOML(t *testing.T) {
	s := &stoatSecrets{
		VapidPrivate: "priv",
		VapidPublic:  "pub",
		FilesEncKey:  "enc",
		LiveKitKey:   "lk",
		LiveKitSec:   "lks",
	}
	out := renderRevoltTOML("chat.example.com", "mongodb://database", "redis://redis/", "rabbit", "http://minio:9000", s)
	for _, want := range []string{
		`app = "https://chat.example.com"`,
		`mongodb = "mongodb://database"`,
		`host = "rabbit"`,
		`encryption_key = "enc"`,
		`endpoint = "http://minio:9000"`,
		`lat = 0.0`,
		`lon = 0.0`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
