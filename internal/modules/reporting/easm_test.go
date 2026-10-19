package reporting

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/red-copilot/red-x/internal/domain"
)

func TestSafeCSVCellEscapesFormulaPrefix(t *testing.T) {
	for _, input := range []string{"=cmd", "+cmd", "-cmd", "@cmd"} {
		if got := safeCSVCell(input); !strings.HasPrefix(got, "'") {
			t.Fatalf("safeCSVCell(%q) = %q", input, got)
		}
	}
	if got := safeCSVCell("normal"); got != "normal" {
		t.Fatalf("unexpected normal cell %q", got)
	}
}

func TestWriteEASMCSVUsesLongFormat(t *testing.T) {
	path := t.TempDir() + "/report.csv"
	data := eASMReportData{
		Organization: domain.Organization{CanonicalName: "=单位A"},
		Domains:      []domain.OrganizationDomain{{RootDomain: "a.edu.cn", Source: "manual", Status: domain.DomainStatusConfirmed}},
		Assets:       []eASMAssetRow{{Kind: "domain", Key: "a.edu.cn", Risk: "high"}},
		Findings:     []eASMFindingRow{{Title: "漏洞", Target: "a.edu.cn", Severity: "high"}},
		Tasks:        []domain.Task{{Name: "EASM 单位A a.edu.cn", Target: "a.edu.cn", Status: domain.TaskSucceeded}},
	}
	if err := writeEASMCSV(path, data); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("rows = %d: %#v", len(rows), rows)
	}
	if rows[0][0] != "organization" || rows[1][0] != "'=单位A" {
		t.Fatalf("missing formula escaping or header: %#v", rows)
	}
	if rows[2][2] != "asset" || rows[3][2] != "finding" || rows[4][2] != "task" {
		t.Fatalf("unexpected long-format rows: %#v", rows)
	}
}
