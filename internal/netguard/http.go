// Package netguard supplies an outbound transport that pins validated DNS answers.
package netguard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var blocked = prefixes("0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4")
var v6global = netip.MustParsePrefix("2000::/3")
var v6blocked = prefixes("2001::/23", "2001:db8::/32", "2002::/16", "3fff::/20")

func prefixes(ss ...string) []netip.Prefix {
	p := make([]netip.Prefix, len(ss))
	for i, s := range ss {
		p[i] = netip.MustParsePrefix(s)
	}
	return p
}

func PublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	if ip.Is4() {
		for _, p := range blocked {
			if p.Contains(ip) {
				return false
			}
		}
		return true
	}
	if !v6global.Contains(ip) {
		return false
	}
	for _, p := range v6blocked {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

func URL(raw string) (*url.URL, error) {
	u, e := url.Parse(raw)
	if e != nil || u == nil || len(raw) > 2048 || u.User != nil || u.Fragment != "" || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("use an HTTP(S) URL without credentials or fragment")
	}
	if p := u.Port(); p != "" && p != "80" && p != "443" {
		return nil, errors.New("only ports 80 and 443 are allowed")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.Contains(host, "%") || !strings.Contains(host, ".") && !strings.Contains(host, ":") {
		return nil, errors.New("public hostname required")
	}
	if strings.Trim(host, "0123456789.") == "" {
		if _, e := netip.ParseAddr(host); e != nil {
			return nil, errors.New("noncanonical IP address")
		}
	}
	if ip, e := netip.ParseAddr(host); e == nil && !PublicIP(ip) {
		return nil, errors.New("destination is not public")
	}
	return u, nil
}

type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

func DialContext(resolver Resolver) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		ips, e := resolver.LookupNetIP(ctx, "ip", host)
		if e != nil || len(ips) == 0 {
			return nil, errors.New("DNS lookup failed")
		}
		// Reject mixed public/private answers as well as all-private destinations.
		for _, ip := range ips {
			if !PublicIP(ip) {
				return nil, errors.New("destination is not public")
			}
		}
		d := net.Dialer{Timeout: 5 * time.Second, KeepAlive: -1}
		for _, ip := range ips {
			conn, e := d.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), port))
			if e == nil {
				return conn, nil
			}
		}
		return nil, errors.New("connection failed")
	}
}
func Client(timeout time.Duration) *http.Client {
	tr := &http.Transport{Proxy: nil, DialContext: DialContext(net.DefaultResolver), TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: timeout, MaxResponseHeaderBytes: 32 << 10, DisableKeepAlives: true, ForceAttemptHTTP2: false}
	return &http.Client{Transport: tr, Timeout: timeout, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("redirect limit exceeded")
		}
		if _, e := URL(r.URL.String()); e != nil {
			return e
		}
		if len(via) > 0 && r.URL.Host != via[0].URL.Host {
			return errors.New("cross-origin redirects are not allowed")
		}
		if len(via) > 0 && via[0].URL.Scheme == "https" && r.URL.Scheme != "https" {
			return errors.New("HTTPS downgrade prohibited")
		}
		return nil
	}}
}
