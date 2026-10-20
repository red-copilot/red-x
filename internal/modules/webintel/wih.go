package webintel

import (
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/red-copilot/red-x/internal/modules/discovery"
	"gopkg.in/yaml.v3"
)

type WIHRule struct {
	ID      string `yaml:"id"`
	Enabled bool   `yaml:"enabled"`
	Pattern string `yaml:"pattern"`
}

type WIHExclude struct {
	Name      string `yaml:"name"`
	ID        string `yaml:"id"`
	Target    string `yaml:"target"`
	Content   string `yaml:"content"`
	SourceTag string `yaml:"source_tag"`
	Enabled   bool   `yaml:"enabled"`
}

type WIHConfig struct {
	Rules        []WIHRule    `yaml:"rules"`
	ExcludeRules []WIHExclude `yaml:"exclude_rules"`
}

type WIHRecord struct {
	RecordType string `json:"recordType"`
	Content    string `json:"content"`
	Source     string `json:"source"`
	Site       string `json:"site"`
	Hash       string `json:"hash"`
}

type WIHScanner struct {
	Client *http.Client
	Config WIHConfig
}

func LoadWIHConfig(path string) (WIHConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return WIHConfig{}, err
	}
	var cfg WIHConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (s *WIHScanner) Scan(ctx context.Context, site string) ([]WIHRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discovery.NormalizeURL(site), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	var scripts []string
	doc.Find("script").Each(func(_ int, sel *goquery.Selection) {
		if src, ok := sel.Attr("src"); ok {
			if u, err := url.Parse(discovery.NormalizeURL(site)); err == nil {
				if abs, err := u.Parse(src); err == nil {
					scripts = append(scripts, abs.String())
				}
			}
		} else if text := sel.Text(); strings.TrimSpace(text) != "" {
			scripts = append(scripts, text)
		}
	})
	if len(scripts) == 0 {
		scripts = append(scripts, string(body))
	}
	seen := map[string]bool{}
	var out []WIHRecord
	for _, script := range scripts {
		content := script
		source := "inline"
		if strings.HasPrefix(script, "http://") || strings.HasPrefix(script, "https://") {
			source = script
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, script, nil)
			resp, err := s.Client.Do(req)
			if err != nil {
				continue
			}
			data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
			resp.Body.Close()
			content = string(data)
		}
		for _, record := range s.extract(site, source, content) {
			if seen[record.Hash] {
				continue
			}
			seen[record.Hash] = true
			out = append(out, record)
		}
	}
	return out, nil
}

func (s *WIHScanner) extract(site, source, content string) []WIHRecord {
	var out []WIHRecord
	for _, rule := range s.Config.Rules {
		if !rule.Enabled {
			continue
		}
		values := extractRule(rule, site, content)
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" || s.excluded(rule.ID, site, value, source) {
				continue
			}
			h := fnv.New64a()
			_, _ = h.Write([]byte(rule.ID + "|" + value + "|" + source))
			out = append(out, WIHRecord{
				RecordType: rule.ID, Content: value, Source: source, Site: site,
				Hash: fmt.Sprintf("%d", h.Sum64()),
			})
		}
	}
	return out
}

func extractRule(rule WIHRule, site, content string) []string {
	switch rule.ID {
	case "domain", "domain_url":
		re := regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}\b`)
		return dedupe(re.FindAllString(content, 500))
	case "ip", "ip_url":
		re := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
		return dedupe(re.FindAllString(content, 200))
	case "path":
		re := regexp.MustCompile(`(?i)["'](/[a-z0-9_\-./]{2,120})["']`)
		matches := re.FindAllStringSubmatch(content, 300)
		var out []string
		for _, m := range matches {
			out = append(out, m[1])
		}
		return dedupe(out)
	case "secret_key":
		re := regexp.MustCompile(`(?i)(?:secret|secretkey|secret_key|access_key|private_key)\s*[:=]\s*["']([A-Za-z0-9/+=_\-]{8,160})["']`)
		matches := re.FindAllStringSubmatch(content, 100)
		var out []string
		for _, m := range matches {
			out = append(out, m[1])
		}
		return dedupe(out)
	default:
		if rule.Pattern == "" {
			return nil
		}
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return nil
		}
		return dedupe(re.FindAllString(content, 500))
	}
}

func (s *WIHScanner) excluded(id, site, content, source string) bool {
	for _, ex := range s.Config.ExcludeRules {
		if !ex.Enabled {
			continue
		}
		if ex.ID != "" && ex.ID != id {
			continue
		}
		if !matchExclude(ex.Target, site) || !matchExclude(ex.Content, content) {
			continue
		}
		if ex.SourceTag != "" && ex.SourceTag != source {
			continue
		}
		return true
	}
	return false
}

func matchExclude(pattern, value string) bool {
	if pattern == "" {
		return true
	}
	if strings.HasPrefix(pattern, "regex:") {
		re, err := regexp.Compile(strings.TrimPrefix(pattern, "regex:"))
		return err == nil && re.MatchString(value)
	}
	return pattern == value
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
