package proxy

import (
	"io"
	"log/slog"
	"net"
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
