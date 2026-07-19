package proxy

import "testing"

func TestNormalizeTarget(t *testing.T) {
	for _, test := range []struct {
		input, defaultPort, want string
	}{
		{"example.com", "80", "example.com:80"},
		{"example.com:8080", "80", "example.com:8080"},
		{"[2001:db8::1]", "443", "[2001:db8::1]:443"},
	} {
		got, err := normalizeTarget(test.input, test.defaultPort)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Fatalf("normalizeTarget(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}
