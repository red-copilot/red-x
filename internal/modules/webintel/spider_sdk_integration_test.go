package webintel

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestKatanaCrawlerStaysInScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body><a href="/a">A</a><a href="http://example.invalid/b">B</a></body></html>`))
		case "/a":
			_, _ = w.Write([]byte(`<html><body><a href="/c">C</a></body></html>`))
		case "/c":
			_, _ = w.Write([]byte(`<html><body>done</body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	crawler := NewKatanaCrawler(2, 5, 20, 5, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	urls, err := crawler.Crawl(ctx, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	wantA := srv.URL + "/a"
	wantC := srv.URL + "/c"
	foundA, foundC := false, false
	for _, u := range urls {
		if u == wantA {
			foundA = true
		}
		if u == wantC {
			foundC = true
		}
		if u == "http://example.invalid/b" {
			t.Fatal("crawler returned out-of-scope URL")
		}
	}
	if !foundA || !foundC {
		t.Fatalf("crawl URLs = %v (%s, %s)", urls, fmt.Sprint(foundA), fmt.Sprint(foundC))
	}
}
