package reporting

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/easm"
	"github.com/xuri/excelize/v2"
)

type EASMReportProvider interface {
	Summary(ctx context.Context, orgID string) (*easm.OrganizationSummary, error)
	Assets(ctx context.Context, orgID *string, filter easm.AssetFilter) (*easm.AssetPage, error)
	Findings(ctx context.Context, orgID *string, filter easm.FindingFilter) (*easm.FindingPage, error)
}

type eASMAssetRow struct {
	Kind      string
	Key       string
	Title     string
	Risk      string
	OrgIDs    []string
	Roots     []string
	FirstSeen time.Time
	LastSeen  time.Time
}

type eASMFindingRow struct {
	Kind        string
	Title       string
	Target      string
	Severity    string
	PluginName  string
	Occurrences int
	Roots       []string
	OrgIDs      []string
	CreatedAt   time.Time
}

type eASMReportData struct {
	Summary      *easm.OrganizationSummary
	Organization domain.Organization
	Domains      []domain.OrganizationDomain
	Assets       []eASMAssetRow
	Findings     []eASMFindingRow
	Tasks        []domain.Task
}

func (e *Exporter) ExportOrganization(ctx context.Context, provider EASMReportProvider, org domain.Organization, domains []domain.OrganizationDomain, tasks []domain.Task, format string) (string, error) {
	if provider == nil {
		return "", fmt.Errorf("EASM report provider is not configured")
	}
	summary, err := provider.Summary(ctx, org.ID)
	if err != nil {
		return "", err
	}
	data := eASMReportData{Summary: summary, Organization: org, Domains: domains, Tasks: tasks}
	for page := 1; ; page++ {
		out, err := provider.Assets(ctx, &org.ID, easm.AssetFilter{Page: page, PageSize: 200})
		if err != nil {
			return "", err
		}
		for _, item := range out.Items {
			data.Assets = append(data.Assets, eASMAssetRow{
				Kind: item.Kind, Key: item.Key, Title: item.Title, Risk: item.Risk,
				OrgIDs: item.OrganizationIDs, Roots: item.RootDomains, FirstSeen: item.FirstSeen, LastSeen: item.LastSeen,
			})
		}
		if len(out.Items) == 0 || page*out.PageSize >= out.Total {
			break
		}
	}
	for page := 1; ; page++ {
		out, err := provider.Findings(ctx, &org.ID, easm.FindingFilter{State: "current", Page: page, PageSize: 200})
		if err != nil {
			return "", err
		}
		for _, item := range out.Items {
			f := item.Finding
			data.Findings = append(data.Findings, eASMFindingRow{
				Kind: string(f.Kind), Title: f.Title, Target: f.Target, Severity: f.Severity,
				PluginName: f.PluginName, Occurrences: item.Occurrences, Roots: item.RootDomains,
				OrgIDs: item.OrganizationIDs, CreatedAt: f.CreatedAt,
			})
		}
		if len(out.Items) == 0 || page*out.PageSize >= out.Total {
			break
		}
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "xlsx"
	}
	if err := os.MkdirAll(e.Dir, 0o750); err != nil {
		return "", err
	}
	name := fmt.Sprintf("EASM_%s_%d.%s", org.ID, time.Now().Unix(), format)
	path := filepath.Join(e.Dir, name)
	switch format {
	case "csv":
		err = writeEASMCSV(path, data)
	case "xlsx":
		err = writeEASMXLSX(path, data)
	default:
		return "", fmt.Errorf("unsupported format %q", format)
	}
	if err != nil {
		return "", err
	}
	return path, nil
}

func safeCSVCell(value string) string {
	if value == "" {
		return ""
	}
	switch value[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}

func writeEASMCSV(path string, data eASMReportData) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"organization", "rootDomain", "recordType", "kind", "key", "target", "severity", "title", "source", "firstSeen", "lastSeen", "detail"})
	orgName := safeCSVCell(data.Organization.CanonicalName)
	for _, d := range data.Domains {
		_ = w.Write([]string{orgName, safeCSVCell(d.RootDomain), "domain", "domain", d.RootDomain, "", "", "", safeCSVCell(d.Source), d.FirstSeen.Format(time.RFC3339), d.LastSeen.Format(time.RFC3339), string(d.Status)})
	}
	for _, a := range data.Assets {
		_ = w.Write([]string{orgName, safeCSVCell(strings.Join(a.Roots, ";")), "asset", safeCSVCell(a.Kind), safeCSVCell(a.Key), "", safeCSVCell(a.Risk), safeCSVCell(a.Title), "", a.FirstSeen.Format(time.RFC3339), a.LastSeen.Format(time.RFC3339), strings.Join(a.OrgIDs, ";")})
	}
	for _, x := range data.Findings {
		_ = w.Write([]string{orgName, safeCSVCell(strings.Join(x.Roots, ";")), "finding", safeCSVCell(x.Kind), "", safeCSVCell(x.Target), safeCSVCell(x.Severity), safeCSVCell(x.Title), safeCSVCell(x.PluginName), x.CreatedAt.Format(time.RFC3339), x.CreatedAt.Format(time.RFC3339), fmt.Sprintf("occurrences=%d", x.Occurrences)})
	}
	for _, t := range data.Tasks {
		_ = w.Write([]string{orgName, safeCSVCell(t.RootDomain), "task", safeCSVCell(string(t.Kind)), safeCSVCell(t.ID), safeCSVCell(t.Target), "", safeCSVCell(t.Name), safeCSVCell(t.CreatedBy), t.CreatedAt.Format(time.RFC3339), t.CreatedAt.Format(time.RFC3339), string(t.Status)})
	}
	return w.Error()
}

func writeEASMXLSX(path string, data eASMReportData) error {
	f := excelize.NewFile()
	defer f.Close()
	summary := f.GetSheetName(0)
	_ = f.SetSheetName(summary, "Summary")
	rows := [][]any{
		{"Organization", data.Organization.CanonicalName},
		{"Confirmed Domains", data.Summary.ConfirmedDomains},
		{"Candidate Domains", data.Summary.CandidateDomains},
		{"Domain Assets", data.Summary.DomainAssets},
		{"IP Assets", data.Summary.IPAssets},
		{"Services", data.Summary.Services},
		{"Current Findings", data.Summary.CurrentFindings},
		{"Critical", data.Summary.CriticalFindings},
		{"High", data.Summary.HighFindings},
		{"Medium", data.Summary.MediumFindings},
		{"Low", data.Summary.LowFindings},
	}
	for i, row := range rows {
		for j, value := range row {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+1)
			_ = f.SetCellValue("Summary", cell, value)
		}
	}
	_, _ = f.NewSheet("Domains")
	_ = f.SetSheetRow("Domains", "A1", &[]any{"Root Domain", "Status", "Source", "Confidence", "ICP No", "Decided By", "Decision Reason"})
	for i, d := range data.Domains {
		_ = f.SetSheetRow("Domains", fmt.Sprintf("A%d", i+2), &[]any{d.RootDomain, string(d.Status), d.Source, string(d.Confidence), d.ICPNo, d.DecidedBy, d.DecisionReason})
	}
	_, _ = f.NewSheet("Assets")
	_ = f.SetSheetRow("Assets", "A1", &[]any{"Kind", "Key", "Title", "Risk", "Organizations", "Root Domains", "First Seen", "Last Seen"})
	for i, a := range data.Assets {
		_ = f.SetSheetRow("Assets", fmt.Sprintf("A%d", i+2), &[]any{a.Kind, a.Key, a.Title, a.Risk, strings.Join(a.OrgIDs, ";"), strings.Join(a.Roots, ";"), a.FirstSeen.Format(time.RFC3339), a.LastSeen.Format(time.RFC3339)})
	}
	_, _ = f.NewSheet("Current Findings")
	_ = f.SetSheetRow("Current Findings", "A1", &[]any{"Kind", "Title", "Target", "Severity", "Plugin", "Occurrences", "Root Domains", "Last Seen"})
	for i, x := range data.Findings {
		_ = f.SetSheetRow("Current Findings", fmt.Sprintf("A%d", i+2), &[]any{x.Kind, x.Title, x.Target, x.Severity, x.PluginName, x.Occurrences, strings.Join(x.Roots, ";"), x.CreatedAt.Format(time.RFC3339)})
	}
	_, _ = f.NewSheet("Tasks")
	_ = f.SetSheetRow("Tasks", "A1", &[]any{"ID", "Name", "Root Domain", "Target", "Status", "Progress", "Created At"})
	for i, t := range data.Tasks {
		_ = f.SetSheetRow("Tasks", fmt.Sprintf("A%d", i+2), &[]any{t.ID, t.Name, t.RootDomain, t.Target, string(t.Status), t.Progress, t.CreatedAt.Format(time.RFC3339)})
	}
	return f.SaveAs(path)
}
