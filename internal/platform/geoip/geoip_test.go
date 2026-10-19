package geoip

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/oschwald/geoip2-golang"
)

type fakeCity struct {
	city *geoip2.City
	err  error
}

func (f fakeCity) City(net.IP) (*geoip2.City, error) { return f.city, f.err }

type fakeASN struct {
	asn *geoip2.ASN
	err error
}

func (f fakeASN) ASN(net.IP) (*geoip2.ASN, error) { return f.asn, f.err }

func TestMaxMindMapsCityAndASN(t *testing.T) {
	city := &geoip2.City{}
	city.Country.IsoCode = "CN"
	city.City.Names = map[string]string{"zh-CN": "北京", "en": "Beijing"}
	city.Subdivisions = append(city.Subdivisions, struct {
		Names     map[string]string `maxminddb:"names"`
		IsoCode   string            `maxminddb:"iso_code"`
		GeoNameID uint              `maxminddb:"geoname_id"`
	}{Names: map[string]string{"zh-CN": "北京"}, IsoCode: "BJ"})
	city.Location.Latitude = 39.9042
	city.Location.Longitude = 116.4074
	r := &MaxMindResolver{
		city: fakeCity{city: city},
		asn:  fakeASN{asn: &geoip2.ASN{AutonomousSystemNumber: 4134, AutonomousSystemOrganization: "CHINANET"}},
	}
	loc, err := r.Resolve(context.Background(), netip.MustParseAddr("8.8.8.8"))
	if err != nil {
		t.Fatal(err)
	}
	if loc.City != "北京" || loc.Province != "北京" || loc.Country != "CN" {
		t.Fatalf("unexpected location: %+v", loc)
	}
	if loc.Lat != 39.9042 || loc.Lon != 116.4074 || loc.ASN != 4134 || loc.ASNOrg != "CHINANET" {
		t.Fatalf("unexpected geo fields: %+v", loc)
	}
}

func TestMaxMindPrivateIPIsUnlocated(t *testing.T) {
	r := &MaxMindResolver{city: fakeCity{}, asn: fakeASN{}}
	for _, raw := range []string{"10.0.0.1", "127.0.0.1", "169.254.1.1", "::1", "0.0.0.0"} {
		_, err := r.Resolve(context.Background(), netip.MustParseAddr(raw))
		if !errors.Is(err, ErrIPUnlocated) {
			t.Fatalf("%s: expected ErrIPUnlocated, got %v", raw, err)
		}
	}
}

func TestMaxMindMissingDBIsUnavailable(t *testing.T) {
	if _, err := NewMaxMind("", ""); !errors.Is(err, ErrGeoUnavailable) {
		t.Fatalf("expected ErrGeoUnavailable, got %v", err)
	}
}

func TestMaxMindLookupErrorIsUnlocated(t *testing.T) {
	r := &MaxMindResolver{city: fakeCity{err: errors.New("not found")}, asn: fakeASN{}}
	_, err := r.Resolve(context.Background(), netip.MustParseAddr("8.8.8.8"))
	if !errors.Is(err, ErrIPUnlocated) {
		t.Fatalf("expected ErrIPUnlocated, got %v", err)
	}
}
