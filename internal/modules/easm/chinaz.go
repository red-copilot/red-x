package easm

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

var (
	ErrICPNoAttribution = errors.New("icp: no attribution found")
	ErrICPCookieExpired = errors.New("icp: cookie expired")
	ErrICPRateLimited   = errors.New("icp: rate limited")
	ErrICPUnavailable   = errors.New("icp: provider unavailable")
	ErrICPAccessDenied  = errors.New("icp: access denied")
)

type DomainICPResult struct {
	RawHTML   string `json:"-"`
	OrgName   string `json:"orgName"`
	OrgType   string `json:"orgType"`
	ICPNo     string `json:"icpNo"`
	AuditDate string `json:"auditDate"`
}

type ChinazProvider struct {
	Cookie    string
	UserAgent string
	BaseURL   string
	APIURL    string
	Client    *http.Client
}

func NewChinazProvider(cookie, userAgent, baseURL, apiURL string, client *http.Client) *ChinazProvider {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if client.Jar == nil {
		if jar, err := cookiejar.New(nil); err == nil {
			client.Jar = jar
		}
	}
	return &ChinazProvider{
		Cookie: cookie, UserAgent: userAgent,
		BaseURL: strings.TrimRight(baseURL, "/"), APIURL: apiURL, Client: client,
	}
}

var (
	chinazCompanyRe = regexp.MustCompile(`(?s)<a[^>]*id="companyName"[^>]*>(.*?)</a>`)
	chinazTypeRe    = regexp.MustCompile(`(?s)<div[^>]*class="[^"]*contactPhone[^"]*"[^>]*>(.*?)</div>`)
	chinazAuditRe   = regexp.MustCompile(`(?s)审核时间</div>\s*<div[^>]*>\s*<div[^>]*>(.*?)</div>`)
	chinazSKRe      = regexp.MustCompile(`var sk = '([0-9a-fA-F]{32,})';`)
	chinazTagRe     = regexp.MustCompile(`<[^>]+>`)
)

func (p *ChinazProvider) LookupByDomain(ctx context.Context, rootDomain string) (DomainICPResult, error) {
	root, err := NormalizeRootDomain(rootDomain)
	if err != nil {
		return DomainICPResult{}, fmt.Errorf("invalid root domain: %w", err)
	}
	pageURL := p.BaseURL + "/" + url.PathEscape(root)
	client := *p.Client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return DomainICPResult{}, err
	}
	p.setHeaders(req, pageURL, "")
	resp, err := client.Do(req)
	if err != nil {
		return DomainICPResult{}, fmt.Errorf("%w: %v", ErrICPUnavailable, err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect, http.StatusUnauthorized:
		return DomainICPResult{}, ErrICPCookieExpired
	case http.StatusTooManyRequests:
		return DomainICPResult{}, ErrICPRateLimited
	default:
		return DomainICPResult{}, fmt.Errorf("%w: status %d", ErrICPUnavailable, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return DomainICPResult{}, fmt.Errorf("%w: %v", ErrICPUnavailable, err)
	}
	out := DomainICPResult{RawHTML: truncateString(string(body), 64*1024)}
	out.OrgName = firstMatchText(chinazCompanyRe, body)
	if out.OrgName == "" {
		return out, ErrICPNoAttribution
	}
	out.OrgType = firstMatchText(chinazTypeRe, body)
	out.AuditDate = firstMatchText(chinazAuditRe, body)
	if m := chinazSKRe.FindSubmatch(body); len(m) == 2 {
		if icpNo, err := p.queryPermit(ctx, string(m[1]), root, pageURL); err == nil {
			out.ICPNo = icpNo
		}
	}
	return out, nil
}

type chinazPermitResponse struct {
	Code int    `json:"code"`
	Data string `json:"data"`
	Msg  string `json:"msg"`
}

func (p *ChinazProvider) queryPermit(ctx context.Context, sk, keyword, referer string) (string, error) {
	payload, err := json.Marshal(map[string]string{"keyword": keyword})
	if err != nil {
		return "", err
	}
	rnd := 100 + rand.IntN(900)
	hostKey := chinazHostKey(string(payload), rnd)
	ts := time.Now().UnixMilli()
	token := chinazToken(hostKey, ts)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.APIURL, strings.NewReader(string(payload)))
	if err != nil {
		return "", err
	}
	p.setHeaders(req, referer, sk)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("rd", strconv.Itoa(rnd))
	req.Header.Set("ts", strconv.FormatInt(ts, 10))
	req.Header.Set("token", token)
	client := *p.Client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrICPUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			return "", ErrICPRateLimited
		}
		return "", fmt.Errorf("%w: status %d", ErrICPUnavailable, resp.StatusCode)
	}
	var out chinazPermitResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return "", fmt.Errorf("%w: %v", ErrICPUnavailable, err)
	}
	switch out.Code {
	case 1:
		if strings.TrimSpace(out.Data) == "" {
			return "", ErrICPNoAttribution
		}
		return strings.TrimSpace(out.Data), nil
	case -1:
		return "", ErrICPAccessDenied
	default:
		return "", fmt.Errorf("%w: code %d %s", ErrICPUnavailable, out.Code, out.Msg)
	}
}

func (p *ChinazProvider) setHeaders(req *http.Request, referer, sk string) {
	if p.Cookie != "" {
		req.Header.Set("Cookie", p.Cookie)
	}
	ua := p.UserAgent
	if ua == "" {
		ua = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Origin", "https://icp.chinaz.com")
	req.Header.Set("Referer", referer)
	if sk != "" {
		req.Header.Set("sk", sk)
	}
}

// chinazHostKey reproduces the site's byte-sum request key.
func chinazHostKey(payload string, rnd int) string {
	parts := strings.Split(payload, ".")
	vals := make([]string, 0, len(parts))
	for i := len(parts) - 1; i >= 0; i-- {
		sum := 0
		for _, r := range strings.ToLower(parts[i]) {
			sum += int(r)
		}
		if i != 0 {
			sum += int('.')
		}
		vals = append(vals, strconv.Itoa(sum+rnd))
	}
	return strconv.Itoa(rnd) + "," + strings.Join(vals, ",")
}

func chinazToken(hostKey string, ts int64) string {
	sum := md5.Sum([]byte(hostKey + "*#|&" + strconv.FormatInt(ts, 10)))
	return hex.EncodeToString(sum[:])
}

func firstMatchText(re *regexp.Regexp, body []byte) string {
	m := re.FindSubmatch(body)
	if len(m) != 2 {
		return ""
	}
	text := chinazTagRe.ReplaceAllString(string(m[1]), "")
	return strings.TrimSpace(html.UnescapeString(text))
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
