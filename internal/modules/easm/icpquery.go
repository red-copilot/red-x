package easm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type OrgDomainRecord struct {
	Domain         string
	ServiceLicence string
	UnitName       string
	ServiceName    string
}

type ICPQueryProvider struct {
	BaseURL   string
	PageSize  int
	PageDelay time.Duration
	Client    *http.Client
}

func NewICPQueryProvider(baseURL string, pageSize int, pageDelay time.Duration, client *http.Client) *ICPQueryProvider {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	return &ICPQueryProvider{
		BaseURL: strings.TrimRight(baseURL, "/"), PageSize: pageSize, PageDelay: pageDelay, Client: client,
	}
}

const icpQueryMaxPages = 20

type icpQueryResponse struct {
	Code   int `json:"code"`
	Params struct {
		Total       int  `json:"total"`
		Pages       int  `json:"pages"`
		HasNextPage bool `json:"hasNextPage"`
		List        []struct {
			Domain         string `json:"domain"`
			ServiceLicence string `json:"serviceLicence"`
			UnitName       string `json:"unitName"`
			ServiceName    string `json:"serviceName"`
		} `json:"list"`
	} `json:"params"`
}

func (p *ICPQueryProvider) SearchByOrganization(ctx context.Context, name string) ([]OrgDomainRecord, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("ICP_Query: organization name is required")
	}
	var out []OrgDomainRecord
	seen := make(map[string]bool)
	pages := 0
	for page := 1; page <= icpQueryMaxPages; page++ {
		if page > 1 && p.PageDelay > 0 {
			timer := time.NewTimer(p.PageDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		resp, err := p.fetch(ctx, name, page)
		if err != nil {
			return nil, err
		}
		pages = resp.Params.Pages
		if pages > icpQueryMaxPages {
			pages = icpQueryMaxPages
		}
		for _, item := range resp.Params.List {
			root, err := NormalizeRootDomain(item.Domain)
			if err != nil || seen[root] {
				continue
			}
			seen[root] = true
			out = append(out, OrgDomainRecord{
				Domain: root, ServiceLicence: strings.TrimSpace(item.ServiceLicence),
				UnitName: strings.TrimSpace(item.UnitName), ServiceName: strings.TrimSpace(item.ServiceName),
			})
		}
		if pages > 0 {
			if page >= pages {
				break
			}
			continue
		}
		if !resp.Params.HasNextPage {
			break
		}
	}
	return out, nil
}

func (p *ICPQueryProvider) fetch(ctx context.Context, name string, page int) (*icpQueryResponse, error) {
	q := url.Values{}
	q.Set("search", name)
	q.Set("pageNum", strconv.Itoa(page))
	q.Set("pageSize", strconv.Itoa(p.PageSize))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.BaseURL+"/query/web?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrICPUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrICPUnavailable, resp.StatusCode)
	}
	var out icpQueryResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrICPUnavailable, err)
	}
	if out.Code != 200 {
		return nil, fmt.Errorf("%w: code %d", ErrICPUnavailable, out.Code)
	}
	return &out, nil
}

func (p *ICPQueryProvider) Probe(ctx context.Context) error {
	if p == nil || strings.TrimSpace(p.BaseURL) == "" {
		return ErrICPUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.BaseURL, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrICPUnavailable, err)
	}
	client := *p.Client
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrICPUnavailable, err)
	}
	_ = resp.Body.Close()
	return nil
}
