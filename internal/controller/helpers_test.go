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
	out := renderRevoltTOML("chat.example.com", "mongodb://database", "redis://redis/", "rabbit", "http://minio:9000", s, true, 47.6, -122.3)
	for _, want := range []string{
		`app = "https://chat.example.com"`,
		`mongodb = "mongodb://database"`,
		`host = "rabbit"`,
		`encryption_key = "enc"`,
		`endpoint = "http://minio:9000"`,
		`lat = 47.6`,
		`lon = -122.3`,
		`[hosts.livekit]`,
		`[api.livekit.nodes.worldwide]`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}

	off := renderRevoltTOML("chat.example.com", "mongodb://database", "redis://redis/", "rabbit", "http://minio:9000", s, false, 0, 0)
	for _, ban := range []string{`[hosts.livekit]`, `[api.livekit.nodes.worldwide]`} {
		if strings.Contains(off, ban) {
			t.Fatalf("livekit disabled but found %q in:\n%s", ban, off)
		}
	}
}

func TestRenderCaddyfile(t *testing.T) {
	on := renderCaddyfile(true)
	for _, want := range []string{"handle /api*", "handle /livekit*", "handle /ingress*", "handle /assets*", "Cache-Control"} {
		if !strings.Contains(on, want) {
			t.Fatalf("missing %q in:\n%s", want, on)
		}
	}
	if strings.Contains(on, "\troute ") {
		t.Fatalf("expected handle-only Caddyfile, found route:\n%s", on)
	}
	off := renderCaddyfile(false)
	if strings.Contains(off, "/livekit*") || strings.Contains(off, "/ingress*") {
		t.Fatalf("livekit disabled but routes present:\n%s", off)
	}
	if !strings.Contains(off, "handle /api*") {
		t.Fatalf("expected api handle when livekit off:\n%s", off)
	}
}

func TestIngressClassDefault(t *testing.T) {
	if got := ingressClass(stoatv1alpha1.StoatSpec{}); got != "nginx" {
		t.Fatalf("expected nginx default, got %q", got)
	}
}

func TestRenderLiveKitYAMLAddress(t *testing.T) {
	s := &stoatSecrets{LiveKitKey: "lk", LiveKitSec: "lks"}
	spec := stoatv1alpha1.StoatSpec{
		LiveKit: stoatv1alpha1.LiveKitSpec{Address: "192.168.14.189"},
	}
	out := renderLiveKitYAML(spec, s, 7881, 50000, 50100)
	if !strings.Contains(out, `use_external_ip: false`) || !strings.Contains(out, `node_ip: "192.168.14.189"`) {
		t.Fatalf("expected VIP node_ip:\n%s", out)
	}
}

func TestLiveKitNodeIPEnv(t *testing.T) {
	vip := liveKitNodeIPEnv(stoatv1alpha1.StoatSpec{LiveKit: stoatv1alpha1.LiveKitSpec{Address: "192.168.14.189"}})
	if vip.Value != "192.168.14.189" {
		t.Fatalf("expected static VIP, got %#v", vip)
	}
	host := liveKitNodeIPEnv(stoatv1alpha1.StoatSpec{})
	if host.ValueFrom == nil || host.ValueFrom.FieldRef.FieldPath != "status.hostIP" {
		t.Fatalf("expected hostIP fieldRef, got %#v", host)
	}
}
