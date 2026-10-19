package discovery

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/red-copilot/red-x/internal/platform/netsafe"
	"github.com/red-copilot/red-x/internal/platform/pdtools"
	"github.com/projectdiscovery/goflags"
	"github.com/projectdiscovery/httpx/runner"
)

type HTTPXProbe struct {
	Threads      int
	Timeout      int
	Retries      int
	MaxRedirects int
	MaxBodyRead  int
	ProxyURL     string
	Policy       netsafe.Policy
}

func NewHTTPXProbe(policy netsafe.Policy, proxyURL string, threads, timeout, retries, maxRedirects, maxBodyRead int) *HTTPXProbe {
	if threads <= 0 {
		threads = 12
	}
	if timeout <= 0 {
		timeout = 10
	}
	if retries < 0 {
		retries = 0
	}
	if maxRedirects <= 0 {
		maxRedirects = 5
	}
	if maxBodyRead <= 0 {
		maxBodyRead = 2 << 20
	}
	return &HTTPXProbe{
		Threads: threads, Timeout: timeout, Retries: retries, MaxRedirects: maxRedirects,
		MaxBodyRead: maxBodyRead, ProxyURL: proxyURL, Policy: policy,
	}
}

func (p *HTTPXProbe) Probe(ctx context.Context, targets []string) ([]SiteResult, []error) {
	if len(targets) == 0 {
		return nil, nil
	}
	np, err := pdtools.NewNetworkPolicy(p.Policy)
	if err != nil {
		return nil, []error{err}
	}
	var (
		results []SiteResult
		errs    []error
		mu      sync.Mutex
	)
	options := &runner.Options{
		InputTargetHost:           goflags.StringSlice(targets),
		Methods:                   "GET",
		Threads:                   p.Threads,
		Timeout:                   p.Timeout,
		Retries:                   p.Retries,
		MaxRedirects:              p.MaxRedirects,
		FollowRedirects:           true,
		ExtractTitle:              true,
		StatusCode:                true,
		ContentLength:             true,
		OutputServerHeader:        true,
		OutputContentType:         true,
		OutputIP:                  true,
		OutputCName:               true,
		Favicon:                   true,
		TLSGrab:                   true,
		TechDetect:                true,
		ResponseHeadersInStdout:   true,
		ResponseInStdout:          true,
		DisableStdout:             true,
		Silent:                    true,
		NoColor:                   true,
		DisableUpdateCheck:        true,
		MaxResponseBodySizeToRead: p.MaxBodyRead,
		MaxResponseBodySizeToSave: p.MaxBodyRead,
		Networkpolicy:             np,
		Proxy:                     p.ProxyURL,
		OnResult: func(result runner.Result) {
			if result.Err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", result.Input, result.Err))
				mu.Unlock()
				return
			}
			mu.Lock()
			results = append(results, siteResultFromHTTPX(&result))
			mu.Unlock()
		},
	}
	if err := options.ValidateOptions(); err != nil {
		return nil, []error{fmt.Errorf("validate httpx options: %w", err)}
	}
	httpRunner, err := runner.New(options)
	if err != nil {
		return nil, []error{fmt.Errorf("create httpx runner: %w", err)}
	}
	defer httpRunner.Close()
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			httpRunner.Interrupt()
		case <-done:
		}
	}()
	httpRunner.RunEnumeration()
	close(done)
	if err := ctx.Err(); err != nil {
		mu.Lock()
		errs = append(errs, err)
		mu.Unlock()
	}
	return results, errs
}

func siteResultFromHTTPX(r *runner.Result) SiteResult {
	if r == nil {
		return SiteResult{}
	}
	site := strings.TrimSpace(r.URL)
	if site == "" {
		site = strings.TrimSpace(r.Input)
	}
	out := SiteResult{
		Site:         site,
		Hostname:     r.Host,
		IP:           r.HostIP,
		Status:       r.StatusCode,
		Title:        r.Title,
		Headers:      headersToStrings(r.ResponseHeaders),
		HTTPServer:   r.WebServer,
		BodyLength:   r.ContentLength,
		ContentType:  r.ContentType,
		FaviconURL:   r.FaviconURL,
		FaviconHash:  parseFaviconHash(r.FavIconMMH3),
		Technologies: append([]string(nil), r.Technologies...),
		CDN:          r.CDN,
		CDNName:      r.CDNName,
		CNAMEs:       append([]string(nil), r.CNAMEs...),
		FinalURL:     r.FinalURL,
		Body:         r.ResponseBody,
	}
	if out.FinalURL == "" {
		out.FinalURL = out.Site
	}
	if r.TLSData != nil {
		tls := map[string]any{
			"version": r.TLSData.Version,
			"cipher":  r.TLSData.Cipher,
			"jarm":    r.TLSData.JarmHash,
		}
		if r.TLSData.CertificateResponse != nil {
			tls["subject"] = r.TLSData.SubjectCN
			tls["issuer"] = r.TLSData.IssuerCN
			tls["expired"] = r.TLSData.Expired
		}
		out.TLS = tls
	}
	return out
}

func headersToStrings(in map[string]interface{}) map[string]string {
	out := map[string]string{}
	for key, value := range in {
		out[strings.ToLower(key)] = fmt.Sprint(value)
	}
	return out
}

func parseFaviconHash(raw string) int32 {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0
	}
	return int32(value)
}

func NormalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		return "http://" + raw
	}
	return raw
}

var _ HTTPProbe = (*HTTPXProbe)(nil)
