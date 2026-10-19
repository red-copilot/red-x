package webintel

import (
	"testing"

	"github.com/projectdiscovery/nuclei/v3/pkg/model"
	"github.com/projectdiscovery/nuclei/v3/pkg/model/types/severity"
	nucleioutput "github.com/projectdiscovery/nuclei/v3/pkg/output"
)

func TestNucleiFindingFromEvent(t *testing.T) {
	event := &nucleioutput.ResultEvent{
		TemplateID:       "exposed-config",
		TemplatePath:     "/tmp/templates/exposed-config.yaml",
		URL:              "https://example.com/config.json",
		Matched:          "https://example.com/config.json",
		MatcherName:      "word",
		ExtractedResults: []string{"secret"},
		CURLCommand:      "curl https://example.com/config.json",
		Metadata:         map[string]interface{}{"product": "demo"},
		Info: model.Info{
			Name:           "Exposed config",
			SeverityHolder: severity.Holder{Severity: severity.High},
		},
	}
	got := nucleiFindingFromEvent(event)
	if got.TemplateID != "exposed-config" || got.Name != "Exposed config" || got.Severity != "high" {
		t.Fatalf("finding = %+v", got)
	}
	if got.URL != "https://example.com/config.json" || got.MatcherName != "word" {
		t.Fatalf("finding URL/matcher = %+v", got)
	}
	if len(got.ExtractedResults) != 1 || got.CURLCommand == "" || got.Metadata["product"] != "demo" {
		t.Fatalf("finding details = %+v", got)
	}
}
