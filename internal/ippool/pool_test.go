package ippool

import (
	"net"
	"testing"
)

func TestRoundRobinExplicitIPv6(t *testing.T) {
	p, err := New([]string{"2001:db8::1", "2001:db8::2"}, nil, "round_robin")
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Next().String(); got != "2001:db8::1" {
		t.Fatalf("first address = %s", got)
	}
	if got := p.Next().String(); got != "2001:db8::2" {
		t.Fatalf("second address = %s", got)
	}
	if got := p.Next().String(); got != "2001:db8::1" {
		t.Fatalf("wrapped address = %s", got)
	}
}

func TestRandomAddressStaysInPrefix(t *testing.T) {
	p, err := New(nil, []string{"2001:db8:1234:5678::/64"}, "random")
	if err != nil {
		t.Fatal(err)
	}
	network := &net.IPNet{IP: net.ParseIP("2001:db8:1234:5678::").To16(), Mask: net.CIDRMask(64, 128)}
	for i := 0; i < 20; i++ {
		if got := p.Next(); !network.Contains(got) {
			t.Fatalf("generated address %s is outside %s", got, network)
		}
	}
}

func TestRejectsIPv4Sources(t *testing.T) {
	if _, err := New([]string{"192.0.2.1"}, nil, "random"); err == nil {
		t.Fatal("expected IPv4 source to be rejected")
	}
	if _, err := New(nil, []string{"192.0.2.0/24"}, "random"); err == nil {
		t.Fatal("expected IPv4 source CIDR to be rejected")
	}
}
