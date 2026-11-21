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
	s3 := s3Settings{
		Endpoint:         "http://minio:9000",
		AccessKeyID:      "minioautumn",
		SecretAccessKey:  "minioautumn",
		Bucket:           "revolt-uploads",
		Region:           "minio",
		PathStyleBuckets: true,
	}
	out := renderRevoltTOML("chat.example.com", "mongodb://database", "redis://redis/", defaultRabbitConn(), s3, s, true, 47.6, -122.3)
	for _, want := range []string{
		`app = "https://chat.example.com"`,
		`mongodb = "mongodb://database"`,
		`host = "rabbit"`,
		`encryption_key = "enc"`,
		`endpoint = "http://minio:9000"`,
		`path_style_buckets = true`,
		`lat = 47.6`,
		`lon = -122.3`,
		`[hosts.livekit]`,
		`[api.livekit.nodes.worldwide]`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}

	ext := parseRabbitConn("amqp://ha-user:s3cret@stoat-rabbit.stoat.svc:5672/")
	extOut := renderRevoltTOML("chat.example.com", "mongodb://ext", "redis://ext/", ext, s3, s, false, 0, 0)
	for _, want := range []string{
		`host = "stoat-rabbit.stoat.svc"`,
		`username = "ha-user"`,
		`password = "s3cret"`,
	} {
		if !strings.Contains(extOut, want) {
			t.Fatalf("missing %q in external rabbit toml:\n%s", want, extOut)
		}
	}

	off := renderRevoltTOML("chat.example.com", "mongodb://database", "redis://redis/", defaultRabbitConn(), s3, s, false, 0, 0)
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

func TestReplicaFor(t *testing.T) {
	two := int32(2)
	three := int32(3)
	spec := stoatv1alpha1.StoatSpec{Scaling: &stoatv1alpha1.ScalingSpec{Replicas: &two, API: &three}}
	if got := replicaFor(spec, "api"); got != 3 {
		t.Fatalf("api override: got %d want 3", got)
	}
	if got := replicaFor(spec, "web"); got != 2 {
		t.Fatalf("web default: got %d want 2", got)
	}
	if got := replicaFor(spec, "crond"); got != 1 {
		t.Fatalf("crond singleton: got %d want 1", got)
	}
	if got := replicaFor(stoatv1alpha1.StoatSpec{}, "api"); got != 1 {
		t.Fatalf("unset scaling: got %d want 1", got)
	}
}

func TestAdminEnabled(t *testing.T) {
	if adminEnabled(stoatv1alpha1.StoatSpec{}) {
		t.Fatal("expected admin disabled when unset")
	}
	off := false
	if adminEnabled(stoatv1alpha1.StoatSpec{Admin: &stoatv1alpha1.AdminSpec{Enabled: &off}}) {
		t.Fatal("expected admin disabled when enabled=false")
	}
	on := true
	if !adminEnabled(stoatv1alpha1.StoatSpec{Admin: &stoatv1alpha1.AdminSpec{Enabled: &on}}) {
		t.Fatal("expected admin enabled when enabled=true")
	}
}

func TestRenderLiveKitYAMLAddress(t *testing.T) {
	s := &stoatSecrets{LiveKitKey: "lk", LiveKitSec: "lks"}
	spec := stoatv1alpha1.StoatSpec{
		LiveKit: stoatv1alpha1.LiveKitSpec{Address: "192.168.14.189"},
	}
	out := renderLiveKitYAML(spec, s, "redis://redis/", 7881, 50000, 50100)
	if !strings.Contains(out, `use_external_ip: false`) || !strings.Contains(out, `node_ip: "192.168.14.189"`) {
		t.Fatalf("expected VIP node_ip:\n%s", out)
	}
	if !strings.Contains(out, "address: redis:6379") {
		t.Fatalf("expected default redis address:\n%s", out)
	}
}

func TestRenderLiveKitYAMLExternalRedis(t *testing.T) {
	s := &stoatSecrets{LiveKitKey: "lk", LiveKitSec: "lks"}
	spec := stoatv1alpha1.StoatSpec{LiveKit: stoatv1alpha1.LiveKitSpec{Address: "192.168.14.189"}}
	out := renderLiveKitYAML(spec, s, "redis://:s3cret@stoat-redis-master.stoat.svc.cluster.local:6379/", 7881, 50000, 50100)
	if !strings.Contains(out, "address: stoat-redis-master.stoat.svc.cluster.local:6379") {
		t.Fatalf("expected external redis host:\n%s", out)
	}
	if !strings.Contains(out, "password: s3cret") {
		t.Fatalf("expected redis password:\n%s", out)
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
