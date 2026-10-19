package webintel

import "testing"

func TestFilterCrawlURLs(t *testing.T) {
	got := filterCrawlURLs("https://example.com/start", []string{
		"https://example.com/a",
		"http://example.com/b",
		"https://sub.example.com/c",
		"https://other.example/d",
		"mailto:test@example.com",
		"https://example.com/a",
	})
	if len(got) != 2 || got[0] != "https://example.com/a" || got[1] != "http://example.com/b" {
		t.Fatalf("crawl URLs = %v", got)
	}
}
