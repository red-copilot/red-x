package webintel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/projectdiscovery/goflags"
	nuclei "github.com/projectdiscovery/nuclei/v3/lib"
	nucleioutput "github.com/projectdiscovery/nuclei/v3/pkg/output"
	templatetypes "github.com/projectdiscovery/nuclei/v3/pkg/templates/types"
	nucleitypes "github.com/projectdiscovery/nuclei/v3/pkg/types"
)

type NucleiFinding struct {
	TemplateID       string         `json:"templateId"`
	Name             string         `json:"name"`
	Severity         string         `json:"severity"`
	Target           string         `json:"target"`
	URL              string         `json:"url"`
	Status           int            `json:"status,omitempty"`
	TemplatePath     string         `json:"templatePath,omitempty"`
	MatcherName      string         `json:"matcherName,omitempty"`
	ExtractedResults []string       `json:"extractedResults,omitempty"`
	CURLCommand      string         `json:"curlCommand,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type NucleiScanner interface {
	Scan(ctx context.Context, targets []string) ([]NucleiFinding, error)
}

type NucleiSDKScanner struct {
	engine *nuclei.ThreadSafeNucleiEngine
	err    error
}

func NewNucleiSDKScanner(templatesDir, proxyURL, interactshURL string, excludeTargets []string, restrictLocalNetwork bool, timeout, rateLimit, bulkSize, templateThreads int) *NucleiSDKScanner {
	if timeout <= 0 {
		timeout = 10
	}
	if rateLimit <= 0 {
		rateLimit = 100
	}
	if bulkSize <= 0 {
		bulkSize = 10
	}
	if templateThreads <= 0 {
		templateThreads = 10
	}
	options := nucleitypes.DefaultOptions()
	options.Templates = []string{templatesDir}
	options.Protocols = templatetypes.ProtocolTypes{templatetypes.HTTPProtocol}
	options.ExcludeProtocols = templatetypes.ProtocolTypes{
		templatetypes.CodeProtocol, templatetypes.FileProtocol, templatetypes.HeadlessProtocol,
	}
	options.Timeout = timeout
	options.RateLimit = rateLimit
	options.RateLimitDuration = time.Second
	options.BulkSize = bulkSize
	options.TemplateThreads = templateThreads
	options.NoInteractsh = strings.TrimSpace(interactshURL) == ""
	options.InteractshURL = interactshURL
	options.AllowLocalFileAccess = false
	options.RestrictLocalNetworkAccess = restrictLocalNetwork
	options.EnableCodeTemplates = false
	options.EnableFileTemplates = false
	options.Headless = false
	options.Silent = true
	options.NoColor = true
	options.ExcludeTargets = goflags.StringSlice(excludeTargets)
	if proxyURL != "" {
		options.Proxy = goflags.StringSlice{proxyURL}
	}

	scanner := &NucleiSDKScanner{}
	engine, err := nuclei.NewThreadSafeNucleiEngine(
		nuclei.WithOptions(options),
		nuclei.WithTemplateFilters(nuclei.TemplateFilters{ProtocolTypes: "http"}),
		nuclei.WithResultCallback(func(event *nucleioutput.ResultEvent) {}),
		nuclei.DisableUpdateCheck(),
	)
	if err != nil {
		scanner.err = fmt.Errorf("create nuclei engine: %w", err)
		return scanner
	}
	scanner.engine = engine
	if err := engine.GlobalLoadAllTemplates(); err != nil {
		scanner.err = fmt.Errorf("load nuclei templates: %w", err)
	}
	return scanner
}

func (s *NucleiSDKScanner) Scan(ctx context.Context, targets []string) ([]NucleiFinding, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.engine == nil {
		return nil, fmt.Errorf("nuclei engine is not initialized")
	}
	if len(targets) == 0 {
		return nil, nil
	}
	var (
		mu       sync.Mutex
		findings []NucleiFinding
		seen     = map[string]bool{}
	)
	err := s.engine.ExecuteNucleiWithOptsCtx(ctx, targets, nuclei.WithResultCallback(func(event *nucleioutput.ResultEvent) {
		finding := nucleiFindingFromEvent(event)
		key := finding.TemplateID + "\x00" + finding.URL + "\x00" + finding.MatcherName
		mu.Lock()
		defer mu.Unlock()
		if !seen[key] {
			seen[key] = true
			findings = append(findings, finding)
		}
	}))
	if err != nil {
		return nil, err
	}
	return findings, nil
}

func (s *NucleiSDKScanner) Close() {
	if s.engine != nil {
		s.engine.Close()
	}
}

func nucleiFindingFromEvent(event *nucleioutput.ResultEvent) NucleiFinding {
	if event == nil {
		return NucleiFinding{}
	}
	url := event.URL
	if url == "" {
		url = event.Matched
	}
	severity := event.Info.SeverityHolder.Severity.String()
	if severity == "" {
		severity = "info"
	}
	return NucleiFinding{
		TemplateID: event.TemplateID, Name: event.Info.Name, Severity: severity,
		Target: event.Host, URL: url, TemplatePath: event.TemplatePath,
		MatcherName: event.MatcherName, ExtractedResults: append([]string(nil), event.ExtractedResults...),
		CURLCommand: event.CURLCommand, Metadata: event.Metadata,
	}
}

var _ NucleiScanner = (*NucleiSDKScanner)(nil)
