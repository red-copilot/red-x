package easm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const testChinazSK = "700b875cbd1ce28299b62dca97869f442d494e8f23f4026b9ebb3e632270db059afeb50f0610262e99dce95864ef949a"

func TestChinazHostKeyVectors(t *testing.T) {
	if got := chinazHostKey(`{"keyword":"四川吉利学院"}`, 209); got != "209,152130" {
		t.Fatalf("single segment host key = %q", got)
	}
	if got := chinazHostKey(`{"keyword":"guc.edu.cn"}`, 737); got != "737,1151,1101,2112" {
		t.Fatalf("multi segment host key = %q", got)
	}
}

func TestChinazTokenUsesSalt(t *testing.T) {
	if got := chinazToken("209,152130", 1792409644543); got != "f2b6c801e563652cbc5ae35ec0a09519" {
		t.Fatalf("token = %q", got)
	}
}

func TestChinazParsesDomainPage(t *testing.T) {
	fixture, err := os.ReadFile("testdata/chinaz_domain.html")
	if err != nil {
		t.Fatal(err)
	}
	var sawPermit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/guc.edu.cn") {
			_, _ = w.Write(fixture)
			return
		}
		if r.URL.Path == "/api" {
			sawPermit = true
			if r.Header.Get("sk") != testChinazSK {
				t.Errorf("sk header not forwarded")
			}
			if len(r.Header.Get("token")) != 32 || len(r.Header.Get("rd")) < 3 {
				t.Errorf("missing signed headers: %v", r.Header)
			}
			_, _ = w.Write([]byte(`{"code":1,"data":"蜀ICP备2024069804号-2","msg":"成功"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	p := NewChinazProvider("session=1", "test-agent", srv.URL, srv.URL+"/api", srv.Client())
	out, err := p.LookupByDomain(context.Background(), "GUC.EDU.CN.")
	if err != nil {
		t.Fatal(err)
	}
	if !sawPermit {
		t.Fatal("permit endpoint was not called")
	}
	if out.OrgName != "四川吉利学院" || out.OrgType != "民办非企业单位" || out.AuditDate != "2024-05-08" {
		t.Fatalf("unexpected result: %+v", out)
	}
	if out.ICPNo != "蜀ICP备2024069804号-2" {
		t.Fatalf("unexpected icpNo: %q", out.ICPNo)
	}
}

func TestChinazRefreshesExpiredCookie(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	}))
	defer srv.Close()
	p := NewChinazProvider("session=1", "test-agent", srv.URL, srv.URL+"/api", srv.Client())
	_, err := p.LookupByDomain(context.Background(), "guc.edu.cn")
	if !errors.Is(err, ErrICPCookieExpired) {
		t.Fatalf("expected ErrICPCookieExpired, got %v", err)
	}
}

func TestChinazRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	defer srv.Close()
	p := NewChinazProvider("session=1", "test-agent", srv.URL, srv.URL+"/api", srv.Client())
	_, err := p.LookupByDomain(context.Background(), "guc.edu.cn")
	if !errors.Is(err, ErrICPRateLimited) {
		t.Fatalf("expected ErrICPRateLimited, got %v", err)
	}
}

func TestChinazNoAttribution(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<html><body>no record</body></html>`)
	}))
	defer srv.Close()
	p := NewChinazProvider("session=1", "test-agent", srv.URL, srv.URL+"/api", srv.Client())
	_, err := p.LookupByDomain(context.Background(), "guc.edu.cn")
	if !errors.Is(err, ErrICPNoAttribution) {
		t.Fatalf("expected ErrICPNoAttribution, got %v", err)
	}
}

func TestChinazPermitFailureKeepsHTMLResult(t *testing.T) {
	fixture, err := os.ReadFile("testdata/chinaz_domain.html")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/guc.edu.cn") {
			_, _ = w.Write(fixture)
			return
		}
		_, _ = w.Write([]byte(`{"code":-1,"msg":"无权限"}`))
	}))
	defer srv.Close()
	p := NewChinazProvider("session=1", "test-agent", srv.URL, srv.URL+"/api", srv.Client())
	out, err := p.LookupByDomain(context.Background(), "guc.edu.cn")
	if err != nil {
		t.Fatal(err)
	}
	if out.OrgName != "四川吉利学院" || out.ICPNo != "" {
		t.Fatalf("unexpected result: %+v", out)
	}
}

func TestChinazLiveLookup(t *testing.T) {
	if os.Getenv("ARL_ICP_LIVE") == "" {
		t.Skip("set ARL_ICP_LIVE=1 and ARL_ICP_CHINAZ_COOKIE to run the live ICP test")
	}
	p := NewChinazProvider(os.Getenv("ARL_ICP_CHINAZ_COOKIE"), "", "https://icp.chinaz.com", "https://tooldata.chinaz.com/icp/api/website/queryPermit", nil)
	out, err := p.LookupByDomain(context.Background(), "guc.edu.cn")
	if err != nil {
		t.Fatal(err)
	}
	if out.OrgName != "四川吉利学院" || out.ICPNo == "" {
		t.Fatalf("unexpected live result: %+v", out)
	}
}
