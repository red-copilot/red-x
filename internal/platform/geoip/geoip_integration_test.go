package geoip

import (
	"context"
	"net/netip"
	"os"
	"testing"
)

func TestMaxMindFilesResolvePublicIP(t *testing.T) {
	city := os.Getenv("ARL_TEST_GEOIP_CITY")
	asn := os.Getenv("ARL_TEST_GEOIP_ASN")
	if city == "" || asn == "" {
		t.Skip("set ARL_TEST_GEOIP_CITY and ARL_TEST_GEOIP_ASN to run GeoIP integration tests")
	}
	resolver, err := NewMaxMind(city, asn)
	if err != nil {
		t.Fatal(err)
	}
	defer resolver.Close()
	loc, err := resolver.Resolve(context.Background(), netip.MustParseAddr("1.1.1.1"))
	if err != nil {
		t.Fatal(err)
	}
	if loc.Lat == 0 && loc.Lon == 0 {
		t.Fatalf("unexpected location: %+v", loc)
	}
}
