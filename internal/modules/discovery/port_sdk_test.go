package discovery

import (
	"testing"
	"time"

	"github.com/projectdiscovery/naabu/v2/pkg/port"
	"github.com/projectdiscovery/naabu/v2/pkg/protocol"
	"github.com/projectdiscovery/tlsx/pkg/tlsx/clients"
)

func TestPortInfoFromNaabu(t *testing.T) {
	p := &port.Port{
		Port:     8443,
		Protocol: protocol.TCP,
		TLS:      true,
		Service: &port.Service{
			Name: "https", Product: "nginx", Version: "1.25", DeviceType: "web",
			OSType: "linux", CPEs: []string{"cpe:/a:nginx:nginx"}, Confidence: 95,
		},
	}
	got := portInfoFromNaabu(p)
	if got.PortID != 8443 || got.ServiceName != "https" || got.Product != "nginx" || got.Version != "1.25" {
		t.Fatalf("port info = %+v", got)
	}
	if got.DeviceType != "web" || got.OSType != "linux" || len(got.CPEs) != 1 || got.Confidence != 95 {
		t.Fatalf("service metadata = %+v", got)
	}
	if !got.TLS {
		t.Fatal("expected TLS to be mapped")
	}
}

func TestCertResultFromTLSX(t *testing.T) {
	resp := &clients.Response{
		IP: "192.0.2.1", Port: "443", Version: "tls13", Cipher: "TLS_AES_128_GCM_SHA256",
		CertificateResponse: &clients.CertificateResponse{
			SubjectCN: "example.com", IssuerCN: "Test CA",
			NotBefore:  time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
			NotAfter:   time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			SubjectAN:  []string{"example.com", "www.example.com"},
			SelfSigned: true, Expired: true, Serial: "01:02",
			FingerprintHash: clients.CertificateResponseFingerprintHash{SHA256: "abc"},
		},
	}
	got := certResultFromTLSX(resp, "example.com", "192.0.2.1", 443)
	if got.Subject != "example.com" || got.Issuer != "Test CA" || got.Port != 443 {
		t.Fatalf("cert result = %+v", got)
	}
	if got.Subject != "example.com" || got.FingerprintSHA256 != "abc" || got.TLSVersion != "tls13" {
		t.Fatalf("cert metadata = %+v", got)
	}
	if !got.SelfSigned || !got.Expired || len(got.DNSNames) != 2 {
		t.Fatalf("cert flags = %+v", got)
	}
}
