package geoip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"

	"github.com/oschwald/geoip2-golang"
	"github.com/red-copilot/red-x/internal/domain"
)

var (
	ErrIPUnlocated    = errors.New("geoip: ip not located")
	ErrGeoUnavailable = errors.New("geoip: unavailable")
)

type Resolver interface {
	Resolve(ctx context.Context, ip netip.Addr) (*domain.IPLocation, error)
}

type citySource interface {
	City(net.IP) (*geoip2.City, error)
}

type asnSource interface {
	ASN(net.IP) (*geoip2.ASN, error)
}

type MaxMindResolver struct {
	city    citySource
	asn     asnSource
	closers []io.Closer
}

func NewMaxMind(cityPath, asnPath string) (*MaxMindResolver, error) {
	if cityPath == "" || asnPath == "" {
		return nil, ErrGeoUnavailable
	}
	city, err := geoip2.Open(cityPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGeoUnavailable, err)
	}
	asn, err := geoip2.Open(asnPath)
	if err != nil {
		_ = city.Close()
		return nil, fmt.Errorf("%w: %v", ErrGeoUnavailable, err)
	}
	return &MaxMindResolver{city: city, asn: asn, closers: []io.Closer{city, asn}}, nil
}

func (r *MaxMindResolver) Close() error {
	var first error
	for _, c := range r.closers {
		if err := c.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (r *MaxMindResolver) Resolve(ctx context.Context, ip netip.Addr) (*domain.IPLocation, error) {
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
		return nil, ErrIPUnlocated
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	city, err := r.city.City(net.IP(ip.AsSlice()))
	if err != nil || city == nil {
		return nil, ErrIPUnlocated
	}
	loc := &domain.IPLocation{
		IP: ip.String(), Lat: city.Location.Latitude, Lon: city.Location.Longitude,
		Country: city.Country.IsoCode, City: pickName(city.City.Names),
		Source: "maxmind", UpdatedAt: time.Now().UTC(),
	}
	if len(city.Subdivisions) > 0 {
		loc.Province = pickName(city.Subdivisions[len(city.Subdivisions)-1].Names)
	}
	if r.asn != nil {
		if a, err := r.asn.ASN(net.IP(ip.AsSlice())); err == nil && a != nil {
			loc.ASN = a.AutonomousSystemNumber
			loc.ASNOrg = a.AutonomousSystemOrganization
		}
	}
	return loc, nil
}

func pickName(names map[string]string) string {
	if v := names["zh-CN"]; v != "" {
		return v
	}
	if v := names["en"]; v != "" {
		return v
	}
	for _, v := range names {
		return v
	}
	return ""
}
