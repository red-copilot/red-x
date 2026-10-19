package webintel

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/red-copilot/red-x/internal/modules/discovery"
)

type FileLeakResult struct {
	URL           string `json:"url"`
	Status        int    `json:"status"`
	ContentLength int    `json:"contentLength"`
	Title         string `json:"title"`
}

type FileLeakScanner struct {
	Client      *http.Client
	Concurrency int
}

func NewFileLeakScanner(client *http.Client, concurrency int) *FileLeakScanner {
	if concurrency <= 0 {
		concurrency = 6
	}
	return &FileLeakScanner{Client: client, Concurrency: concurrency}
}

func (s *FileLeakScanner) Scan(ctx context.Context, site string, words []string) []FileLeakResult {
	base := strings.TrimRight(discovery.NormalizeURL(site), "/") + "/"
	baselineURL := base + "arl-not-found-9f3c2a1b"
	baseStatus, baseLen, baseTitle := s.fetch(ctx, baselineURL)
	sem := make(chan struct{}, s.Concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var out []FileLeakResult
	for _, word := range words {
		word = strings.TrimSpace(strings.TrimPrefix(word, "/"))
		if word == "" || strings.HasPrefix(word, "#") {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(word string) {
			defer wg.Done()
			defer func() { <-sem }()
			target := base + word
			status, length, title := s.fetch(ctx, target)
			if status < 200 || status >= 400 {
				return
			}
			if status == baseStatus && abs(length-baseLen) <= 20 {
				return
			}
			if title != "" && title == baseTitle && abs(length-baseLen) <= 100 {
				return
			}
			mu.Lock()
			out = append(out, FileLeakResult{URL: target, Status: status, ContentLength: length, Title: title})
			mu.Unlock()
		}(word)
	}
	wg.Wait()
	return out
}

func (s *FileLeakScanner) fetch(ctx context.Context, target string) (int, int, string) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, 0, ""
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return 0, 0, ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	title := ""
	if u, err := url.Parse(target); err == nil {
		title = u.Path
	}
	return resp.StatusCode, len(body), title
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
