package proxy

import (
	"encoding/base64"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"

	"vefr/internal/config"
	"vefr/internal/ippool"
)

func TestPrivateAndLocalAddressDetection(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "::1", "fc00::1", "fe80::1"} {
		if !isPrivateOrLocal(net.ParseIP(raw)) {
			t.Fatalf("%s should be blocked", raw)
		}
	}
	if isPrivateOrLocal(net.ParseIP("2001:db8::1")) {
		t.Fatal("documentation IPv6 address should not be classified as private")
	}
}

func TestTargetPortAllowList(t *testing.T) {
	p, err := ippool.New([]string{"2001:db8::1"}, nil, "round_robin")
	if err != nil {
		t.Fatal(err)
	}
	secure := true
	s := NewServer(config.Config{AllowPorts: []int{80}, BlockPrivate: &secure}, p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := s.validateTarget("example.com:22"); err == nil {
		t.Fatal("expected disallowed port")
	}
}

func TestProxyBasicAuth(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("user:pass")))
	user, pass, ok := proxyBasicAuth(req)
	if !ok || user != "user" || pass != "pass" {
		t.Fatalf("proxyBasicAuth() = %q, %q, %v", user, pass, ok)
	}

	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("wrong:header")))
	user, pass, ok = proxyBasicAuth(req)
	if !ok || user != "user" || pass != "pass" {
		t.Fatalf("proxyBasicAuth() should ignore Authorization header, got %q, %q, %v", user, pass, ok)
	}
}

func TestFailedSourceIsSkipped(t *testing.T) {
	p, err := ippool.New([]string{"2001:db8::1", "2001:db8::2"}, nil, "round_robin")
	if err != nil {
		t.Fatal(err)
	}
	secure := false
	s := NewServer(config.Config{BlockPrivate: &secure}, p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	failed := net.ParseIP("2001:db8::1")
	s.markSourceFailure(failed)
	s.markSourceFailure(failed)
	if got := s.nextSource(nil).String(); got != "2001:db8::2" {
		t.Fatalf("nextSource() = %s, want failed source skipped", got)
	}
}
