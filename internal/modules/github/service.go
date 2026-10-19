package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Aabyss-Team/ARL/internal/domain"
	"github.com/Aabyss-Team/ARL/internal/platform/security"
	"github.com/Aabyss-Team/ARL/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Service struct {
	Store  *store.Store
	Token  string
	Client *http.Client
	Log    *slog.Logger
}

type searchItem struct {
	GitURL     string `json:"git_url"`
	HTMLURL    string `json:"html_url"`
	Path       string `json:"path"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func (s *Service) Search(ctx context.Context, keyword string, limit int) ([]searchItem, error) {
	if s.Token == "" {
		return nil, fmt.Errorf("github token is not configured")
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	q := url.Values{}
	q.Set("q", keyword)
	q.Set("per_page", fmt.Sprintf("%d", limit))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/search/code?"+q.Encode(), nil)
	req.Header.Set("Authorization", "Bearer "+s.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("github search status %d: %s", resp.StatusCode, string(data))
	}
	var data struct {
		Items []searchItem `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data.Items, nil
}

func (s *Service) fetchContent(ctx context.Context, gitURL string) string {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, gitURL, nil)
	req.Header.Set("Authorization", "Bearer "+s.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := s.Client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	var data struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(data.Content, "\n", ""))
	if err != nil {
		return ""
	}
	return string(raw)
}

func (s *Service) CreateTask(ctx context.Context, keyword, monitorID string) (*domain.GitHubTask, error) {
	t := &domain.GitHubTask{Keyword: keyword, Status: domain.TaskQueued, MonitorID: monitorID}
	if err := s.Store.CreateGitHubTask(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) RunTask(ctx context.Context, id bson.ObjectID) error {
	t, err := s.Store.GetGitHubTask(ctx, id)
	if err != nil || t == nil {
		return fmt.Errorf("github task not found")
	}
	now := time.Now().UTC()
	t.Status = domain.TaskRunning
	t.StartedAt = &now
	_ = s.Store.UpdateGitHubTask(ctx, t)
	items, err := s.Search(ctx, t.Keyword, 50)
	if err != nil {
		t.Status = domain.TaskFailed
		t.Error = err.Error()
		fin := time.Now().UTC()
		t.FinishedAt = &fin
		_ = s.Store.UpdateGitHubTask(ctx, t)
		return err
	}
	count := 0
	for _, item := range items {
		content := s.fetchContent(ctx, item.GitURL)
		if content == "" || filtered(item.Path, content) {
			continue
		}
		hash := security.SHA256Hex(item.Repository.FullName + "/" + item.Path)
		added, err := s.Store.AddGitHubFinding(ctx, &domain.GitHubFinding{
			TaskID: t.ID.Hex(), MonitorID: t.MonitorID, Keyword: t.Keyword,
			Repo: item.Repository.FullName, Path: item.Path, HTMLURL: item.HTMLURL,
			GitURL: item.GitURL, HashMD5: hash, HumanContent: humanContent(content, t.Keyword),
		})
		if err == nil && added {
			count++
		}
	}
	t.ResultCount = count
	t.Status = domain.TaskSucceeded
	fin := time.Now().UTC()
	t.FinishedAt = &fin
	return s.Store.UpdateGitHubTask(ctx, t)
}

func filtered(path, content string) bool {
	pathFilters := []string{"open-app-filter/", "/adbyby", "/adblock", "luci-app-dnsfilter/", "Spider/", "/spider", "_files/", "alexa_10k.json"}
	for _, f := range pathFilters {
		if strings.Contains(path, f) {
			return true
		}
	}
	contentFilters := []string{"DOMAIN-SUFFIX", "HOST-SUFFIX", "name:[proto;sport;dport;host", `"websites": [`}
	for _, f := range contentFilters {
		if strings.Contains(content, f) {
			return true
		}
	}
	return false
}

func humanContent(content, keyword string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, keyword) {
			start := i - 4
			if start < 0 {
				start = 0
			}
			end := i + 8
			if end > len(lines) {
				end = len(lines)
			}
			return strings.Join(lines[start:end], "\n")
		}
	}
	if len(content) > 4000 {
		return content[:4000]
	}
	return content
}

func (s *Service) RunMonitor(ctx context.Context, m domain.GitHubMonitor) error {
	t, err := s.CreateTask(ctx, m.Keyword, m.ID.Hex())
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	m.LastRunAt = &now
	m.NextRunAt = now.Add(time.Duration(m.IntervalSec) * time.Second)
	_ = s.Store.UpdateGitHubMonitor(ctx, &m)
	return s.RunTask(ctx, t.ID)
}
