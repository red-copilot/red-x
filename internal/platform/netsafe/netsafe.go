package netsafe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/idna"
)

type Policy struct {
	BlockedPrefixes []netip.Prefix
	AllowPrivate    bool
	AllowLoopback   bool
}

var domainRE = regexp.MustCompile(`^(?i)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func NewPolicy(cidrs []string, allowPrivate bool, allowLoopback ...bool) (Policy, error) {
	p := Policy{AllowPrivate: allowPrivate}
	if len(allowLoopback) > 0 {
		p.AllowLoopback = allowLoopback[0]
	}
	for _, raw := range cidrs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return p, fmt.Errorf("invalid black CIDR %q: %w", raw, err)
		}
		p.BlockedPrefixes = append(p.BlockedPrefixes, prefix)
	}
	return p, nil
}

func (p Policy) Blocked(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() {
		return true
	}
	if addr.IsLoopback() {
		return !p.AllowLoopback
	}
	if addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsUnspecified() || addr.IsMulticast() {
		return true
	}
	if addr == netip.MustParseAddr("169.254.169.254") {
		return true
	}
	if !p.AllowPrivate && addr.IsPrivate() {
		return true
	}
	for _, prefix := range p.BlockedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func (p Policy) ValidateHost(ctx context.Context, host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return errors.New("empty host")
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if p.Blocked(addr) {
			return fmt.Errorf("address %s is blocked", host)
		}
		return nil
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", host, err)
	}
	allowed := 0
	for _, ip := range ips {
		if !p.Blocked(ip) {
			allowed++
		}
	}
	if allowed == 0 {
		return fmt.Errorf("host %s resolves only to blocked addresses", host)
	}
	return nil
}

func (p Policy) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if p.Blocked(addr) {
			return nil, fmt.Errorf("address %s is blocked", addr)
		}
		return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(addr.String(), port))
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	var last error
	for _, ip := range ips {
		if p.Blocked(ip) {
			continue
		}
		conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("all addresses for %s are blocked", host)
	}
	return nil, last
}

func ValidateDomain(raw string) (string, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	raw = strings.TrimSuffix(raw, ".")
	if raw == "" || len(raw) > 253 {
		return "", errors.New("invalid domain length")
	}
	ascii, err := idna.Lookup.ToASCII(raw)
	if err != nil {
		return "", fmt.Errorf("invalid IDN domain: %w", err)
	}
	if !domainRE.MatchString(ascii) {
		return "", errors.New("invalid domain syntax")
	}
	return ascii, nil
}

func ValidateURL(raw string, p Policy) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("only http/https URLs are allowed")
	}
	if u.Hostname() == "" {
		return nil, errors.New("missing URL host")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.ValidateHost(ctx, u.Hostname()); err != nil {
		return nil, err
	}
	return u, nil
}

func IsFuzzDomain(s string) bool {
	return strings.Contains(strings.ToLower(s), "{fuzz}")
}
