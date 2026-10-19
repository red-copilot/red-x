package httpapi

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/red-copilot/red-x/internal/modules/easm"
)

const (
	maxOrganizationImportRows = 1000
	maxOrganizationImportSize = 5 << 20
)

type organizationImportRow struct {
	Row           int      `json:"row"`
	CanonicalName string   `json:"canonicalName"`
	Aliases       []string `json:"aliases,omitempty"`
	Region        string   `json:"region,omitempty"`
	USCC          string   `json:"uscc,omitempty"`
	RootDomains   []string `json:"rootDomains,omitempty"`
}

type organizationImportError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

func parseOrganizationCSV(r io.Reader) ([]organizationImportRow, []organizationImportError, error) {
	reader := csv.NewReader(io.LimitReader(r, maxOrganizationImportSize+1))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	header, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read CSV header: %w", err)
	}
	cols := map[string]int{}
	for i, raw := range header {
		cols[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff")))] = i
	}
	required := []string{"canonicalname"}
	for _, name := range required {
		if _, ok := cols[name]; !ok {
			return nil, nil, fmt.Errorf("missing required CSV column %q", name)
		}
	}
	var rows []organizationImportRow
	var rowErrs []organizationImportError
	for n := 2; ; n++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("read CSV row %d: %w", n, err)
		}
		if len(rows) >= maxOrganizationImportRows {
			return nil, nil, fmt.Errorf("CSV exceeds %d data rows", maxOrganizationImportRows)
		}
		value := func(name string) string {
			i, ok := cols[name]
			if !ok || i >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[i])
		}
		row := organizationImportRow{
			Row: n, CanonicalName: value("canonicalname"), Region: value("region"), USCC: value("uscc"),
		}
		if row.CanonicalName == "" {
			rowErrs = append(rowErrs, organizationImportError{Row: n, Message: "canonicalName is required"})
			continue
		}
		for _, alias := range splitImportValue(value("aliases"), '|') {
			row.Aliases = append(row.Aliases, alias)
		}
		var badDomain string
		for _, raw := range splitImportValue(value("rootdomains"), ';') {
			root, err := easm.NormalizeRootDomain(raw)
			if err != nil {
				badDomain = raw
				break
			}
			row.RootDomains = appendUniqueString(row.RootDomains, root)
		}
		if badDomain != "" {
			rowErrs = append(rowErrs, organizationImportError{Row: n, Message: fmt.Sprintf("invalid root domain %q", badDomain)})
			continue
		}
		rows = append(rows, row)
	}
	return rows, rowErrs, nil
}

func splitImportValue(raw string, sep rune) []string {
	var out []string
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == sep || r == '\n' || r == '\r' }) {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func appendUniqueString(list []string, value string) []string {
	for _, item := range list {
		if item == value {
			return list
		}
	}
	return append(list, value)
}
