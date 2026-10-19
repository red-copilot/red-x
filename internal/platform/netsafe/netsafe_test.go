package netsafe

import (
	"context"
	"net/netip"
	"testing"
)

func TestBlockedAddresses(t *testing.T) {
	p, err := NewPolicy([]string{"10.0.0.0/8"}, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Blocked(netip.MustParseAddr("10.1.2.3")) {
		t.Fatal("explicit black CIDR should block 10.1.2.3")
	}
	if !p.Blocked(netip.MustParseAddr("192.168.1.2")) {
		t.Fatal("private address should be blocked when allowPrivate is false")
	}
	if p.Blocked(netip.MustParseAddr("127.0.0.1")) {
		t.Fatal("loopback should be allowed when allowLoopback is true")
	}
	if !p.Blocked(netip.MustParseAddr("169.254.169.254")) {
		t.Fatal("metadata address must always be blocked")
	}
}

func TestValidateDomainAndHost(t *testing.T) {
	if _, err := ValidateDomain("Example.COM."); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateDomain("not a domain!"); err == nil {
		t.Fatal("invalid domain should fail")
	}
	p, _ := NewPolicy(nil, true, true)
	if err := p.ValidateHost(context.Background(), "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
}
