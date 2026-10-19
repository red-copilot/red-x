package easm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestICPQueryPaginates(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.URL.Path != "/query/web" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("search") != "四川信息职业技术学院" {
			t.Errorf("unexpected search: %q", r.URL.Query().Get("search"))
		}
		switch r.URL.Query().Get("pageNum") {
		case "1":
			fmt.Fprint(w, `{"code":200,"params":{"total":3,"pages":2,"hasNextPage":true,"list":[{"domain":"A.Example.com","serviceLicence":"蜀ICP备1号","unitName":"四川信息职业技术学院","serviceName":"站点A"},{"domain":"b.example.org","serviceLicence":"蜀ICP备2号","unitName":"四川信息职业技术学院","serviceName":"站点B"}]}}`)
		case "2":
			fmt.Fprint(w, `{"code":200,"params":{"total":3,"pages":2,"hasNextPage":false,"list":[{"domain":"c.example.net","serviceLicence":"蜀ICP备3号","unitName":"四川信息职业技术学院","serviceName":"站点C"},{"domain":"a.example.com","serviceLicence":"蜀ICP备1号","unitName":"四川信息职业技术学院","serviceName":"站点A"}]}}`)
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("pageNum"))
		}
	}))
	defer srv.Close()
	p := NewICPQueryProvider(srv.URL, 50, 0, srv.Client())
	rows, err := p.SearchByOrganization(context.Background(), "四川信息职业技术学院")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 deduped rows, got %d: %+v", len(rows), rows)
	}
	if rows[0].Domain != "example.com" || rows[0].ServiceLicence != "蜀ICP备1号" {
		t.Fatalf("unexpected first row: %+v", rows[0])
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestICPQueryStopsAtMaxPages(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		page := r.URL.Query().Get("pageNum")
		fmt.Fprintf(w, `{"code":200,"params":{"total":30,"pages":30,"hasNextPage":true,"list":[{"domain":"p%s.example%s.com","serviceLicence":"蜀ICP备%s号","unitName":"某单位","serviceName":"站点"}]}}`, page, page, page)
	}))
	defer srv.Close()
	p := NewICPQueryProvider(srv.URL, 50, 0, srv.Client())
	rows, err := p.SearchByOrganization(context.Background(), "某单位")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 20 || len(rows) != 20 {
		t.Fatalf("expected 20 pages, got calls=%d rows=%d", calls, len(rows))
	}
}

func TestICPQueryNon200IsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	defer srv.Close()
	p := NewICPQueryProvider(srv.URL, 50, 0, srv.Client())
	_, err := p.SearchByOrganization(context.Background(), "某单位")
	if !errors.Is(err, ErrICPUnavailable) {
		t.Fatalf("expected ErrICPUnavailable, got %v", err)
	}
}

func TestICPQueryMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{not json`)
	}))
	defer srv.Close()
	p := NewICPQueryProvider(srv.URL, 50, 0, srv.Client())
	_, err := p.SearchByOrganization(context.Background(), "某单位")
	if !errors.Is(err, ErrICPUnavailable) {
		t.Fatalf("expected ErrICPUnavailable, got %v", err)
	}
}

func TestICPQueryHonorsPageDelay(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pageNum") == "1" {
			fmt.Fprint(w, `{"code":200,"params":{"total":2,"pages":2,"hasNextPage":true,"list":[{"domain":"a.example.com","serviceLicence":"蜀ICP备1号","unitName":"某单位","serviceName":"A"}]}}`)
			return
		}
		fmt.Fprint(w, `{"code":200,"params":{"total":2,"pages":2,"hasNextPage":false,"list":[{"domain":"b.example.org","serviceLicence":"蜀ICP备2号","unitName":"某单位","serviceName":"B"}]}}`)
	}))
	defer srv.Close()
	p := NewICPQueryProvider(srv.URL, 50, 10*time.Millisecond, srv.Client())
	start := time.Now()
	if _, err := p.SearchByOrganization(context.Background(), "某单位"); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 10*time.Millisecond {
		t.Fatalf("expected page delay, elapsed %v", elapsed)
	}
}
