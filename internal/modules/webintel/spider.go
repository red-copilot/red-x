package webintel

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/Aabyss-Team/ARL/internal/modules/discovery"
	"github.com/projectdiscovery/katana/pkg/engine/standard"
	katanaoutput "github.com/projectdiscovery/katana/pkg/output"
	katanatypes "github.com/projectdiscovery/katana/pkg/types"
)

type Crawler interface {
	Crawl(ctx context.Context, site string) ([]string, error)
}

type KatanaCrawler struct {
	MaxDepth    int
	MaxPages    int
	RateLimit   int
	Timeout     int
	Concurrency int
}

func NewKatanaCrawler(maxDepth, maxPages, rateLimit, timeout, concurrency int) *KatanaCrawler {
	if maxDepth <= 0 {
		maxDepth = 2
	}
	if maxPages <= 0 {
		maxPages = 100
	}
	if rateLimit <= 0 {
		rateLimit = 50
	}
	if timeout <= 0 {
		timeout = 10
	}
	if concurrency <= 0 {
		concurrency = 10
	}
	return &KatanaCrawler{
		MaxDepth: maxDepth, MaxPages: maxPages, RateLimit: rateLimit,
		Timeout: timeout, Concurrency: concurrency,
	}
}

func (c *KatanaCrawler) Crawl(ctx context.Context, site string) ([]string, error) {
	site = discovery.NormalizeURL(site)
	var (
		mu   sync.Mutex
		urls []string
	)
	options := &katanatypes.Options{
		URLs:                []string{site},
		Context:             ctx,
		MaxDepth:            c.MaxDepth,
		MaxDomainPages:      c.MaxPages,
		BodyReadSize:        2 << 20,
		Timeout:             c.Timeout,
		Concurrency:         c.Concurrency,
		Parallelism:         c.Concurrency,
		RateLimit:           c.RateLimit,
		Strategy:            "depth-first",
		FieldScope:          "rdn",
		DisableRedirects:    true,
		NoColors:            true,
		Silent:              true,
		DisableUpdateCheck:  true,
		OmitRaw:             true,
		OmitBody:            true,
		DisableUniqueFilter: false,
		OnResult: func(result katanaoutput.Result) {
			if result.Request == nil || result.Request.URL == "" {
				return
			}
			mu.Lock()
			urls = append(urls, result.Request.URL)
			mu.Unlock()
		},
	}
	crawlerOptions, err := katanatypes.NewCrawlerOptions(options)
	if err != nil {
		return nil, fmt.Errorf("create katana options: %w", err)
	}
	defer crawlerOptions.Close()
	crawler, err := standard.New(crawlerOptions)
	if err != nil {
		return nil, fmt.Errorf("create katana crawler: %w", err)
	}
	defer crawler.Close()
	crawlErr := crawler.Crawl(site)
	mu.Lock()
	defer mu.Unlock()
	filtered := filterCrawlURLs(site, urls)
	if crawlErr != nil && len(filtered) == 0 {
		return nil, crawlErr
	}
	return filtered, nil
}

func filterCrawlURLs(root string, urls []string) []string {
	rootURL, err := url.Parse(discovery.NormalizeURL(root))
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(urls))
	for _, raw := range urls {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.EqualFold(u.Hostname(), rootURL.Hostname()) {
			continue
		}
		u.Fragment = ""
		target := u.String()
		if target == rootURL.String() || seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}
	return out
}

var _ Crawler = (*KatanaCrawler)(nil)
