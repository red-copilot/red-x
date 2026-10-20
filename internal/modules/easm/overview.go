package easm

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
)

var ErrSearchQueryTooShort = errors.New("search query must contain at least 2 characters")

type OverviewSummary struct {
	Organizations    int `json:"organizations"`
	ConfirmedDomains int `json:"confirmedDomains"`
	CandidateDomains int `json:"candidateDomains"`
	DomainAssets     int `json:"domainAssets"`
	IPAssets         int `json:"ipAssets"`
	Services         int `json:"services"`
	UnlocatedAssets  int `json:"unlocatedAssets"`
	CurrentFindings  int `json:"currentFindings"`
}

type SeverityCounts struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Info     int `json:"info"`
}

type OrganizationRisk struct {
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	Risk           string `json:"risk"`
	AssetCount     int    `json:"assetCount"`
	FindingCount   int    `json:"findingCount"`
}

type Overview struct {
	Summary          OverviewSummary    `json:"summary"`
	Severity         SeverityCounts     `json:"severity"`
	TopOrganizations []OrganizationRisk `json:"topOrganizations"`
	RecentFindings   []FindingItem      `json:"recentFindings"`
	Health           Health             `json:"health"`
	UpdatedAt        time.Time          `json:"updatedAt"`
	Truncated        bool               `json:"truncated"`
}

type SearchHit struct {
	Type           string `json:"type"`
	ID             string `json:"id"`
	Label          string `json:"label"`
	Detail         string `json:"detail,omitempty"`
	Risk           string `json:"risk,omitempty"`
	OrganizationID string `json:"organizationId,omitempty"`
}

type SearchResult struct {
	Organizations []SearchHit `json:"organizations"`
	Domains       []SearchHit `json:"domains"`
	Assets        []SearchHit `json:"assets"`
	Findings      []SearchHit `json:"findings"`
}

func (s *Service) Overview(ctx context.Context, since time.Time) (*Overview, error) {
	c, err := s.closure(ctx, nil)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if since.IsZero() {
		since = now.Add(-30 * 24 * time.Hour)
	}

	out := &Overview{
		TopOrganizations: []OrganizationRisk{},
		RecentFindings:   []FindingItem{},
		Health:           s.Health(ctx),
		UpdatedAt:        now,
		Truncated:        c.truncated,
	}
	out.Summary.Organizations = len(c.orgs)
	out.Summary.ConfirmedDomains = len(c.confirmed)
	out.Summary.CandidateDomains = len(c.candidates)
	out.Summary.DomainAssets = len(c.domains)
	out.Summary.IPAssets = len(c.ips)
	out.Summary.Services = len(c.services)
	out.Summary.UnlocatedAssets = len(c.unlocatedIPs)

	orgByID := make(map[string]domain.Organization, len(c.orgs))
	for _, org := range c.orgs {
		orgByID[org.ID.Hex()] = org
	}
	riskByOrg := make(map[string]string, len(c.orgs))
	findingsByOrg := make(map[string]int, len(c.orgs))
	current, occurrences, _ := currentFindings(c.findings, c)
	for _, f := range current {
		if f.CreatedAt.Before(since) {
			continue
		}
		out.Summary.CurrentFindings++
		switch strings.ToLower(f.Severity) {
		case "critical":
			out.Severity.Critical++
		case "high":
			out.Severity.High++
		case "medium":
			out.Severity.Medium++
		case "low":
			out.Severity.Low++
		default:
			out.Severity.Info++
		}
		for _, orgID := range orgsForFinding(c, f) {
			findingsByOrg[orgID]++
			if RiskRank(f.Severity) > RiskRank(riskByOrg[orgID]) {
				riskByOrg[orgID] = strings.ToLower(f.Severity)
			}
		}
		item := FindingItem{
			Finding:         f,
			Occurrences:     occurrences[findingGroupKey(f)],
			OrganizationIDs: orgsForFinding(c, f),
			RootDomains:     rootsForFinding(c, f),
		}
		out.RecentFindings = append(out.RecentFindings, item)
	}
	sort.Slice(out.RecentFindings, func(i, j int) bool {
		return out.RecentFindings[i].Finding.CreatedAt.After(out.RecentFindings[j].Finding.CreatedAt)
	})
	if len(out.RecentFindings) > 10 {
		out.RecentFindings = out.RecentFindings[:10]
	}

	assetCountByOrg := make(map[string]int, len(c.orgs))
	for _, orgIDs := range c.targetOrgs {
		for _, orgID := range orgIDs {
			assetCountByOrg[orgID]++
		}
	}
	for orgID, org := range orgByID {
		risk := riskByOrg[orgID]
		if risk == "" {
			risk = "info"
		}
		out.TopOrganizations = append(out.TopOrganizations, OrganizationRisk{
			OrganizationID: orgID,
			Name:           org.CanonicalName,
			Risk:           risk,
			AssetCount:     assetCountByOrg[orgID],
			FindingCount:   findingsByOrg[orgID],
		})
	}
	sort.Slice(out.TopOrganizations, func(i, j int) bool {
		a, b := out.TopOrganizations[i], out.TopOrganizations[j]
		if RiskRank(a.Risk) != RiskRank(b.Risk) {
			return RiskRank(a.Risk) > RiskRank(b.Risk)
		}
		if a.FindingCount != b.FindingCount {
			return a.FindingCount > b.FindingCount
		}
		return a.AssetCount > b.AssetCount
	})
	if len(out.TopOrganizations) > 10 {
		out.Truncated = true
		out.TopOrganizations = out.TopOrganizations[:10]
	}
	return out, nil
}

func (s *Service) Search(ctx context.Context, query string, limit int) (*SearchResult, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if len([]rune(query)) < 2 {
		return nil, ErrSearchQueryTooShort
	}
	if limit < 1 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	c, err := s.closure(ctx, nil)
	if err != nil {
		return nil, err
	}
	out := &SearchResult{
		Organizations: []SearchHit{},
		Domains:       []SearchHit{},
		Assets:        []SearchHit{},
		Findings:      []SearchHit{},
	}
	orgByID := make(map[string]domain.Organization, len(c.orgs))
	for _, org := range c.orgs {
		orgByID[org.ID.Hex()] = org
		if containsSearch(query, org.CanonicalName, strings.Join(org.Aliases, " "), org.USCC) {
			out.Organizations = appendLimited(out.Organizations, SearchHit{
				Type: "organization", ID: org.ID.Hex(), Label: org.CanonicalName, Detail: org.Region,
			}, limit)
		}
	}
	for _, d := range c.confirmed {
		if containsSearch(query, d.RootDomain, d.SiteName) {
			out.Domains = appendLimited(out.Domains, SearchHit{
				Type: "domain", ID: d.ID.Hex(), Label: d.RootDomain, Detail: d.SiteName,
				OrganizationID: d.OrganizationID.Hex(),
			}, limit)
		}
	}
	risks := assetRisks(c)
	for _, assets := range [][]domain.Asset{c.domains, c.ips, c.services} {
		for _, asset := range assets {
			if containsSearch(query, asset.Key, stringValue(asset.Data["title"])) {
				orgIDs := c.orgsForTarget(asset.Key)
				orgID := ""
				if len(orgIDs) > 0 {
					orgID = orgIDs[0]
				}
				out.Assets = appendLimited(out.Assets, SearchHit{
					Type: string(asset.Kind), ID: asset.ID.Hex(), Label: asset.Key,
					Detail: stringValue(asset.Data["title"]), Risk: risks[asset.Key], OrganizationID: orgID,
				}, limit)
			}
		}
	}
	current, _, _ := currentFindings(c.findings, c)
	for _, finding := range current {
		if containsSearch(query, finding.Title, finding.Target, finding.PluginName, finding.DedupeKey) {
			orgIDs := orgsForFinding(c, finding)
			orgID := ""
			if len(orgIDs) > 0 {
				orgID = orgIDs[0]
			}
			out.Findings = appendLimited(out.Findings, SearchHit{
				Type: "finding", ID: finding.ID.Hex(), Label: finding.Title,
				Detail: finding.Target, Risk: strings.ToLower(finding.Severity), OrganizationID: orgID,
			}, limit)
		}
	}
	return out, nil
}

func containsSearch(query string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func appendLimited(items []SearchHit, item SearchHit, limit int) []SearchHit {
	if len(items) >= limit {
		return items
	}
	return append(items, item)
}
