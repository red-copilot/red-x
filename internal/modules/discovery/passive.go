package discovery

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	subfinderresolve "github.com/projectdiscovery/subfinder/v2/pkg/resolve"
	subfinderrunner "github.com/projectdiscovery/subfinder/v2/pkg/runner"
)

type SubfinderEnumerator struct {
	Sources            []string
	Threads            int
	Timeout            int
	MaxEnumerationTime int
	MaxResults         int
	ProviderConfig     string
}

func NewSubfinderEnumerator(sources []string, threads, timeout, maxEnumerationTime int, providerConfig string) *SubfinderEnumerator {
	return &SubfinderEnumerator{
		Sources: sources, Threads: threads, Timeout: timeout,
		MaxEnumerationTime: maxEnumerationTime, ProviderConfig: providerConfig,
	}
}

func (s *SubfinderEnumerator) Enumerate(ctx context.Context, domain string) (PassiveEnumeration, error) {
	domain = normalizeDomain(domain)
	limit := s.MaxResults
	if limit <= 0 {
		limit = 4096
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		mu           sync.Mutex
		entries      []*subfinderresolve.HostEntry
		limitReached bool
	)
	options := &subfinderrunner.Options{
		Domain:             []string{domain},
		Sources:            s.Sources,
		Threads:            s.Threads,
		Timeout:            s.Timeout,
		MaxEnumerationTime: s.MaxEnumerationTime,
		MaxResults:         limit + 1,
		ProviderConfig:     s.ProviderConfig,
		Output:             io.Discard,
		Silent:             true,
		NoColor:            true,
		DisableUpdateCheck: true,
		ResultCallback: func(entry *subfinderresolve.HostEntry) {
			if entry == nil {
				return
			}
			mu.Lock()
			if !appendSubfinderEntry(&entries, entry, limit) {
				limitReached = true
				cancel()
			}
			mu.Unlock()
		},
	}
	if options.Threads <= 0 {
		options.Threads = 10
	}
	if options.Timeout <= 0 {
		options.Timeout = 30
	}
	if options.MaxEnumerationTime <= 0 {
		options.MaxEnumerationTime = 5
	}
	runner, err := subfinderrunner.NewRunner(options)
	if err != nil {
		return PassiveEnumeration{}, fmt.Errorf("create subfinder runner: %w", err)
	}
	if _, err := runner.EnumerateSingleDomainWithCtx(runCtx, domain, nil); err != nil {
		mu.Lock()
		reached := limitReached
		mu.Unlock()
		if !reached {
			return PassiveEnumeration{}, err
		}
	}
	mu.Lock()
	defer mu.Unlock()
	return PassiveEnumeration{Domains: passiveDomainResults(entries, domain), Truncated: limitReached}, nil
}

func appendSubfinderEntry(entries *[]*subfinderresolve.HostEntry, entry *subfinderresolve.HostEntry, limit int) bool {
	if entry == nil || limit <= 0 || len(*entries) >= limit {
		return false
	}
	*entries = append(*entries, entry)
	return true
}

func passiveDomainResults(entries []*subfinderresolve.HostEntry, base string) []DomainResult {
	base = normalizeDomain(base)
	seen := map[string]bool{}
	out := make([]DomainResult, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		host := normalizeDomain(entry.Host)
		if host == "" || host == base || !strings.HasSuffix(host, "."+base) || seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, DomainResult{Domain: host, Source: entry.Source})
	}
	return out
}

type FofaSource struct {
	Client   *http.Client
	Key      string
	BaseURL  string
	MaxPage  int
	PageSize int
}

func (f FofaSource) Name() string { return "fofa" }

func (f FofaSource) Query(ctx context.Context, domain string) ([]string, error) {
	if f.Key == "" {
		return nil, fmt.Errorf("fofa key not configured")
	}
	q := `domain="` + domain + `"`
	qb64 := base64.StdEncoding.EncodeToString([]byte(q))
	base := strings.TrimRight(f.BaseURL, "/")
	if base == "" {
		base = "https://fofa.info"
	}
	u := fmt.Sprintf("%s/api/v1/search/all?email=&key=%s&qbase64=%s&size=%d&page=1&fields=host",
		base, url.QueryEscape(f.Key), url.QueryEscape(qb64), f.PageSize)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fofa status %d", resp.StatusCode)
	}
	var data struct {
		Results [][]string `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	var out []string
	for _, row := range data.Results {
		if len(row) > 0 {
			out = append(out, row[0])
		}
	}
	return uniqueStrings(out), nil
}

var _ PassiveEnumerator = (*SubfinderEnumerator)(nil)
