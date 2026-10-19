package easm

import (
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
)

func TestNormalizeRootDomain(t *testing.T) {
	cases := map[string]string{
		"a.b.example.com":          "example.com",
		"WWW.SCITC.EDU.CN":         "scitc.edu.cn",
		"scitc.edu.cn.":            "scitc.edu.cn",
		"https://a.scitc.edu.cn/x": "scitc.edu.cn",
		"scitc.edu.cn:443":         "scitc.edu.cn",
	}
	for in, want := range cases {
		got, err := NormalizeRootDomain(in)
		if err != nil {
			t.Fatalf("NormalizeRootDomain(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("NormalizeRootDomain(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"", "localhost", "not a domain", "192.168.1.1"} {
		if _, err := NormalizeRootDomain(bad); err == nil {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
}

func TestNormalizeOrgName(t *testing.T) {
	cases := map[string]string{
		"四川信息职业技术学院（广元无线电技工学校）": "四川信息职业技术学院",
		" 四川信息职业技术学院 ":          "四川信息职业技术学院",
		"四川某某科技有限公司":            "四川某某科技",
		"四川某某股份有限公司":            "四川某某",
	}
	for in, want := range cases {
		if got := NormalizeOrgName(in); got != want {
			t.Fatalf("NormalizeOrgName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLicenseRegion(t *testing.T) {
	cases := map[string]string{
		"蜀ICP备12031012号-1": "蜀",
		"京ICP备12345678号":   "京",
		"":                 "",
		"ABC":              "",
	}
	for in, want := range cases {
		if got := LicenseRegion(in); got != want {
			t.Fatalf("LicenseRegion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchConfidence(t *testing.T) {
	if got := MatchConfidence("四川信息职业技术学院", "蜀ICP备12031012号-1", "四川信息职业技术学院（广元）", "蜀ICP备12031012号-1"); got != domain.ConfidenceHigh {
		t.Fatalf("expected high, got %s", got)
	}
	if got := MatchConfidence("四川信息职业技术学院", "蜀ICP备12031012号-1", "四川信息职业技术学院", "蜀ICP备99999999号-x"); got != domain.ConfidenceMedium {
		t.Fatalf("expected medium, got %s", got)
	}
	if got := MatchConfidence("四川信息职业技术学院", "蜀ICP备12031012号-1", "四川信息技术职业学院", "蜀ICP备12031012号-1"); got != domain.ConfidenceLow {
		t.Fatalf("expected low, got %s", got)
	}
	if got := MatchConfidence("四川某学院", "", "四川某学院", ""); got != domain.ConfidenceLow {
		t.Fatalf("expected low without region, got %s", got)
	}
}

func TestRiskRank(t *testing.T) {
	if !(RiskRank("critical") > RiskRank("high") && RiskRank("high") > RiskRank("medium") && RiskRank("medium") > RiskRank("low") && RiskRank("low") > RiskRank("info")) {
		t.Fatal("risk ranks are not ordered")
	}
	if RiskRank("unknown") != 0 {
		t.Fatal("unknown severity should rank 0")
	}
}

func TestLatestRisk(t *testing.T) {
	base := time.Now().UTC()
	findings := []domain.Finding{
		{DedupeKey: "a", Severity: "high", CreatedAt: base.Add(-time.Hour)},
		{DedupeKey: "a", Severity: "low", CreatedAt: base},
		{DedupeKey: "b", Severity: "critical", CreatedAt: base.Add(-2 * time.Hour)},
		{DedupeKey: "c", Severity: "info", CreatedAt: base},
	}
	if got := LatestRisk(findings); got != "critical" {
		t.Fatalf("expected critical, got %q", got)
	}
	if got := LatestRisk(nil); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
