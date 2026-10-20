package discovery

import (
	"testing"

	retryabledns "github.com/projectdiscovery/retryabledns"
	"github.com/red-copilot/red-x/internal/platform/netsafe"
)

func TestDomainResultFromDNSData(t *testing.T) {
	data := &retryabledns.DNSData{
		A:        []string{"192.0.2.10"},
		AAAA:     []string{"2001:db8::10"},
		CNAME:    []string{"edge.example.com"},
		TTL:      300,
		Resolver: []string{"udp:1.1.1.1:53"},
	}
	got := domainResultFromDNSData("www.example.com", data, nil)
	if got.Domain != "www.example.com" || got.Type != "CNAME" {
		t.Fatalf("result = %+v", got)
	}
	if len(got.IPs) != 2 || len(got.CNAME) != 1 || got.TTL != 300 {
		t.Fatalf("result = %+v", got)
	}
	if len(got.Records) != 1 || got.Records[0] != "edge.example.com" {
		t.Fatalf("records = %v", got.Records)
	}
	if got.Resolver == nil || len(got.Resolver) != 1 {
		t.Fatalf("resolver = %v", got.Resolver)
	}
}

func TestFilterBlockedIPs(t *testing.T) {
	policy, err := netsafe.NewPolicy([]string{"203.0.113.0/24"}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	got := filterBlockedIPs([]string{"198.51.100.2", "127.0.0.1", "10.0.0.1", "203.0.113.9", "198.51.100.2"}, policy)
	if len(got) != 1 || got[0] != "198.51.100.2" {
		t.Fatalf("filtered IPs = %v", got)
	}
}
