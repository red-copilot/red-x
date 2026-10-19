package discovery

import (
	"testing"

	"github.com/projectdiscovery/httpx/runner"
	"github.com/projectdiscovery/tlsx/pkg/tlsx/clients"
)

func TestSiteResultFromHTTPX(t *testing.T) {
	r := &runner.Result{
		URL:             "https://example.com/",
		Host:            "example.com",
		HostIP:          "192.0.2.10",
		StatusCode:      200,
		Title:           "Example",
		WebServer:       "nginx",
		ContentLength:   1234,
		ContentType:     "text/html",
		ResponseBody:    "<html>example</html>",
		FavIconMMH3:     "12345",
		FaviconURL:      "https://example.com/favicon.ico",
		Technologies:    []string{"nginx"},
		CDN:             true,
		CDNName:         "example-cdn",
		CNAMEs:          []string{"edge.example.com"},
		FinalURL:        "https://example.com/home",
		ResponseHeaders: map[string]interface{}{"server": "nginx"},
		TLSData: &clients.Response{
			Version: "tls13", Cipher: "TLS_AES_128_GCM_SHA256",
			CertificateResponse: &clients.CertificateResponse{SubjectCN: "example.com"},
		},
	}
	got := siteResultFromHTTPX(r)
	if got.Site != "https://example.com/" || got.Status != 200 || got.Title != "Example" {
		t.Fatalf("site = %+v", got)
	}
	if got.FaviconHash != 12345 || got.Body != "<html>example</html>" || got.HTTPServer != "nginx" {
		t.Fatalf("metadata = %+v", got)
	}
	if got.CDNName != "example-cdn" || len(got.Technologies) != 1 || got.FinalURL != "https://example.com/home" {
		t.Fatalf("enrichment = %+v", got)
	}
	if got.TLS == nil || got.TLS["version"] != "tls13" {
		t.Fatalf("tls = %+v", got.TLS)
	}
}
