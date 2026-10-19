package easm

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/red-copilot/red-x/internal/domain"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

var (
	orgBracketSuffix = regexp.MustCompile(`[（(][^）)]*[）)]`)
	licenseRegionRe  = regexp.MustCompile(`^([\p{Han}])ICP`)
)

// NormalizeRootDomain returns the registrable domain (eTLD+1) for a host or URL.
func NormalizeRootDomain(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errors.New("domain is empty")
	}
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil {
			return "", err
		}
		s = u.Hostname()
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	s = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
	ascii, err := idna.Lookup.ToASCII(s)
	if err != nil {
		return "", err
	}
	root, err := publicsuffix.EffectiveTLDPlusOne(ascii)
	if err != nil {
		return "", err
	}
	return strings.ToLower(root), nil
}

// NormalizeOrgName removes parenthesized branches and legal-form suffixes.
func NormalizeOrgName(name string) string {
	s := strings.ReplaceAll(name, "\u3000", " ")
	s = orgBracketSuffix.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), "")
	for _, suffix := range []string{"股份有限公司", "有限责任公司", "有限公司"} {
		s = strings.TrimSuffix(s, suffix)
	}
	return strings.TrimSpace(s)
}

// LicenseRegion extracts the province prefix from an ICP licence number.
func LicenseRegion(icpNo string) string {
	m := licenseRegionRe.FindStringSubmatch(strings.TrimSpace(icpNo))
	if len(m) != 2 {
		return ""
	}
	return m[1]
}

// MatchConfidence scores a candidate organization against a queried one.
func MatchConfidence(queryName, queryLicense, candidateName, candidateLicense string) domain.Confidence {
	qn, cn := NormalizeOrgName(queryName), NormalizeOrgName(candidateName)
	if qn == "" || qn != cn {
		return domain.ConfidenceLow
	}
	ql := strings.ToUpper(strings.TrimSpace(queryLicense))
	cl := strings.ToUpper(strings.TrimSpace(candidateLicense))
	if ql != "" && ql == cl {
		return domain.ConfidenceHigh
	}
	qr, cr := LicenseRegion(ql), LicenseRegion(cl)
	if qr != "" && qr == cr {
		return domain.ConfidenceMedium
	}
	return domain.ConfidenceLow
}

// RiskRank orders finding severities. Unknown severities rank 0.
func RiskRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 5
	case "high":
		return 4
	case "medium":
		return 3
	case "low":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

// LatestRisk returns the highest severity among the newest finding per dedupe key.
// Info-level findings are ignored; an empty string means no risk-bearing finding.
func LatestRisk(findings []domain.Finding) string {
	latest := make(map[string]domain.Finding, len(findings))
	for _, f := range findings {
		cur, ok := latest[f.DedupeKey]
		if !ok || f.CreatedAt.After(cur.CreatedAt) {
			latest[f.DedupeKey] = f
		}
	}
	best, bestRank := "", 0
	for _, f := range latest {
		r := RiskRank(f.Severity)
		if r <= RiskRank("info") {
			continue
		}
		if r > bestRank {
			best, bestRank = strings.ToLower(strings.TrimSpace(f.Severity)), r
		}
	}
	return best
}
