package poc

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/red-copilot/red-x/internal/platform/netsafe"
)

type HTTPClient struct {
	Client *http.Client
	Policy netsafe.Policy
}

func NewHTTPClient(policy netsafe.Policy, proxyURL string, timeout time.Duration) *HTTPClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	tr := &http.Transport{
		DialContext:         policy.DialContext,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, // scanning arbitrary endpoints
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 4,
	}
	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	return &HTTPClient{
		Client: &http.Client{
			Transport: tr, Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return policy.ValidateHost(req.Context(), req.URL.Hostname())
			},
		},
		Policy: policy,
	}
}

type HTTPResponse struct {
	Status int
	Body   string
	Header http.Header
	URL    string
}

func (c *HTTPClient) Do(ctx context.Context, method, target string, body string, headers map[string]string) (*HTTPResponse, error) {
	var rdr io.Reader
	if body != "" {
		rdr = bytes.NewBufferString(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return &HTTPResponse{Status: resp.StatusCode, Body: string(data), Header: resp.Header, URL: resp.Request.URL.String()}, nil
}

func JoinURL(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}
