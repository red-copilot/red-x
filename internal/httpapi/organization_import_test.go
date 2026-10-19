package httpapi

import (
	"strings"
	"testing"
)

func TestParseOrganizationCSVExpandsAliasesAndDomains(t *testing.T) {
	input := "canonicalName,aliases,region,uscc,rootDomains\n四川某学院,川某学院|某学院,蜀,12510000ABC,scitc.edu.cn;guc.edu.cn\n"
	rows, rowErrs, err := parseOrganizationCSV(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(rowErrs) != 0 || len(rows) != 1 {
		t.Fatalf("rows=%+v errors=%+v", rows, rowErrs)
	}
	row := rows[0]
	if len(row.Aliases) != 2 || row.Aliases[1] != "某学院" {
		t.Fatalf("aliases = %v", row.Aliases)
	}
	if len(row.RootDomains) != 2 || row.RootDomains[0] != "scitc.edu.cn" {
		t.Fatalf("domains = %v", row.RootDomains)
	}
}

func TestParseOrganizationCSVReportsBadRowsAndKeepsGoodRows(t *testing.T) {
	input := "canonicalName,aliases,region,uscc,rootDomains\n,alias,蜀,,example.com\n单位B,alias,蜀,,https://guc.edu.cn/path\n"
	rows, rowErrs, err := parseOrganizationCSV(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(rowErrs) != 1 || rowErrs[0].Row != 2 {
		t.Fatalf("row errors = %+v", rowErrs)
	}
	if len(rows) != 1 || rows[0].CanonicalName != "单位B" || rows[0].RootDomains[0] != "guc.edu.cn" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestParseOrganizationCSVRequiresHeaderAndCanonicalName(t *testing.T) {
	_, _, err := parseOrganizationCSV(strings.NewReader("name,domains\nA,example.com\n"))
	if err == nil {
		t.Fatal("expected missing canonicalName header error")
	}
}
