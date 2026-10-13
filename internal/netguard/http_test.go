package netguard

import (
	"context"
	"net/netip"
	"testing"
)

func TestAddresses(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "::1", "::ffff:127.0.0.1", "10.1.2.3", "169.254.169.254", "100.100.100.200", "192.0.2.1", "fc00::1", "fe80::1", "64:ff9b::a00:1", "2002:7f00:1::", "2001:db8::1", "224.0.0.1", "198.18.0.1"} {
		if PublicIP(netip.MustParseAddr(s)) {
			t.Errorf("allowed %s", s)
		}
	}
	for _, s := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"} {
		if !PublicIP(netip.MustParseAddr(s)) {
			t.Errorf("blocked %s", s)
		}
	}
}
func TestURLs(t *testing.T) {
	for _, s := range []string{"file:///etc/passwd", "http://localhost", "http://127.1", "http://2130706433", "http://user:pass@example.com", "http://example.com:8080", "http://[::1]", "http://example.com/#x"} {
		if _, e := URL(s); e == nil {
			t.Errorf("allowed %s", s)
		}
	}
}

type fakeResolver struct{}

func (fakeResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("127.0.0.1")}, nil
}
func TestMixedDNS(t *testing.T) {
	c, e := DialContext(fakeResolver{})(context.Background(), "tcp", "example.com:80")
	if c != nil {
		c.Close()
	}
	if e == nil {
		t.Fatal("mixed DNS answers allowed")
	}
}
