package tasking

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/discovery"
	"github.com/red-copilot/red-x/internal/modules/poc"
	"github.com/red-copilot/red-x/internal/modules/webintel"
	"github.com/red-copilot/red-x/internal/platform/pdtools"
	"github.com/red-copilot/red-x/internal/store"
)

func (e *Engine) executeStep(ctx context.Context, task *domain.Task, step *domain.TaskStep) error {
	switch step.Kind {
	case "dns":
		return e.stepDNS(ctx, task, step)
	case "port":
		return e.stepPort(ctx, task, step)
	case "http":
		return e.stepHTTP(ctx, task, step)
	case "webintel":
		return e.stepWebIntel(ctx, task, step)
	case "wih":
		return e.stepWIH(ctx, task, step)
	case "poc":
		return e.stepPOC(ctx, task, step)
	case "fofa":
		return e.stepFofa(ctx, task, step)
	case "github":
		return fmt.Errorf("github step requires github integration")
	case "stats":
		stats, err := e.Store.TaskStatistics(ctx, task.ID)
		if err != nil {
			return err
		}
		return e.Store.UpdateTaskFields(ctx, task.ID, store.TaskPatch{Statistics: &stats})
	default:
		return fmt.Errorf("unknown step %q", step.Kind)
	}
}

func (e *Engine) stepDNS(ctx context.Context, task *domain.Task, step *domain.TaskStep) error {
	base := strings.ToLower(strings.TrimSpace(task.Target))
	var passive []discovery.DomainResult
	if task.Options.DNSQueryPlugin && e.Passive != nil {
		var err error
		var enumeration discovery.PassiveEnumeration
		enumeration, err = e.Passive.Enumerate(ctx, base)
		if err != nil {
			e.recordDNSWarning(ctx, task, fmt.Sprintf("subfinder 枚举失败: %v", err))
		} else {
			passive = enumeration.Domains
			limit := e.hostBudget()
			if enumeration.Truncated {
				e.recordDNSLimit(ctx, task, "subfinder", fmt.Sprintf("subfinder 达到 maxHosts=%d 上限并停止枚举", limit))
			}
			if len(passive) > limit {
				if !enumeration.Truncated {
					e.recordDNSLimit(ctx, task, "subfinder", fmt.Sprintf("subfinder 返回 %d 个子域，按 maxHosts=%d 截断", len(passive), limit))
				}
				passive = passive[:limit]
			}
			e.recordDNSDiscovery(ctx, task, step, len(passive))
		}
	}
	res, err := e.DNS.Resolve(ctx, base)
	if err != nil {
		return err
	}
	e.observeDomain(ctx, task, res, "dns")
	e.resolvePassiveDomains(ctx, task, passive)
	wildcard := e.DNS.WildcardIPs(ctx, base)
	if task.Options.DomainBrute {
		dictPath := e.dictPath(task.Options.DomainBruteType)
		words, err := discovery.LoadWords(dictPath)
		if err != nil {
			return fmt.Errorf("load domain dictionary: %w", err)
		}
		if limit := e.hostBudget(); len(words) > limit {
			e.recordDNSLimit(ctx, task, "domain_brute", fmt.Sprintf("域名爆破字典 %d 项，按 maxHosts=%d 截断", len(words), limit))
			words = words[:limit]
		}
		results := e.DNS.Brute(ctx, base, words, wildcard, func(done, total int) {
			p := 0
			if total > 0 {
				p = done * 100 / total
			}
			_ = e.Store.UpdateStepFields(ctx, step.ID, store.StepPatch{Progress: &p, Message: ptr(fmt.Sprintf("DNS %d/%d", done, total))})
		})
		for _, r := range results {
			e.observeDomain(ctx, task, r, "domain_brute")
		}
	}
	if task.Options.ARLSearch {
		limit := e.hostBudget()
		history, _ := e.Store.SearchHistoryDomains(ctx, base, limit+1)
		if len(history) > limit {
			e.recordDNSLimit(ctx, task, "arl", fmt.Sprintf("ARL 历史域名超过 maxHosts=%d，按上限截断", limit))
			history = history[:limit]
		}
		for _, d := range history {
			rr, err := e.DNS.Resolve(ctx, d)
			if err == nil {
				rr.Source = "arl"
				e.observeDomain(ctx, task, rr, "arl")
			}
		}
	}
	if task.Options.AltDNS {
		words, _ := discovery.LoadWords(e.dictPath("test"))
		subs, _ := e.taskAssetKeys(ctx, task.ID, domain.AssetDomain)
		alts := discovery.AltDNSWords(subs, base, words)
		if limit := e.hostBudget(); len(alts) > limit {
			e.recordDNSLimit(ctx, task, "alt_dns", fmt.Sprintf("AltDNS 生成 %d 个候选，按 maxHosts=%d 截断", len(alts), limit))
			alts = alts[:limit]
		}
		if len(alts) > 0 {
			results := e.DNS.Brute(ctx, base, alts, wildcard, nil)
			for _, r := range results {
				e.observeDomain(ctx, task, r, "alt_dns")
			}
		}
	}
	return nil
}

func (e *Engine) hostBudget() int {
	if e.Config.Scan.MaxHosts > 0 {
		return e.Config.Scan.MaxHosts
	}
	return 4096
}

func (e *Engine) resolvePassiveDomains(ctx context.Context, task *domain.Task, domains []discovery.DomainResult) {
	if len(domains) == 0 {
		return
	}
	concurrency := e.Config.Scan.Concurrency
	if concurrency <= 0 {
		concurrency = 8
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, item := range domains {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(item discovery.DomainResult) {
			defer wg.Done()
			defer func() { <-sem }()
			result, err := e.DNS.Resolve(ctx, item.Domain)
			if err != nil {
				return
			}
			result.Source = item.Source
			e.observeDomain(ctx, task, result, item.Source)
		}(item)
	}
	wg.Wait()
}

func (e *Engine) dictPath(kind string) string {
	switch kind {
	case "big":
		return e.AssetsDir + "/dicts/domain_2w.txt"
	default:
		return e.AssetsDir + "/dicts/domain_dict_test.txt"
	}
}

func (e *Engine) observeDomain(ctx context.Context, task *domain.Task, res discovery.DomainResult, source string) {
	if res.Domain == "" {
		return
	}
	if e.Store == nil {
		return
	}
	data := map[string]any{
		"domain": res.Domain, "type": res.Type, "record": res.Records, "ips": res.IPs,
		"source": source, "taskId": task.ID, "ttl": res.TTL, "resolver": res.Resolver,
		"cname": res.CNAME, "cdn": res.CDN, "cdnName": res.CDNName, "cdnType": res.CDNType,
		"engine": "projectdiscovery/dnsx", "engineVersion": pdtools.DNSXVersion,
	}
	asset, err := e.Store.UpsertAsset(ctx, domain.AssetDomain, res.Domain, data, task.ID, task.ScopeID)
	if err != nil {
		return
	}
	_ = e.Store.ObserveAsset(ctx, task.ID, asset, source, data)
	e.linkScope(ctx, task, asset)
	for _, ip := range res.IPs {
		ipData := map[string]any{"ip": ip, "ipType": "PUBLIC", "ports": []any{}, "domains": []string{res.Domain}}
		ipAsset, err := e.Store.UpsertAsset(ctx, domain.AssetIP, ip, ipData, task.ID, task.ScopeID)
		if err == nil {
			_ = e.Store.ObserveAsset(ctx, task.ID, ipAsset, source, ipData)
			e.linkScope(ctx, task, ipAsset)
		}
	}
}

func (e *Engine) recordDNSWarning(ctx context.Context, task *domain.Task, message string) {
	e.recordDNSIssue(ctx, task, "subfinder", message, "failed", "subfinder enumeration failed")
}

func (e *Engine) recordDNSLimit(ctx context.Context, task *domain.Task, kind, message string) {
	e.recordDNSIssue(ctx, task, kind, message, "truncated", "dns discovery truncated")
}

func (e *Engine) recordDNSIssue(ctx context.Context, task *domain.Task, kind, message, status, logMessage string) {
	task.Warnings = append(task.Warnings, message)
	if e.Log != nil {
		e.Log.Warn(logMessage, "taskId", task.ID, "warning", message)
	}
	if e.Store == nil {
		return
	}
	_ = e.Store.UpdateTaskFields(ctx, task.ID, store.TaskPatch{Warnings: ptr(task.Warnings)})
	_ = e.Store.AppendTaskEvent(ctx, &domain.TaskEvent{
		TaskID: task.ID, Type: "task.warning", Message: message,
		Payload: map[string]any{"kind": kind, "status": status},
	})
}

func (e *Engine) recordDNSDiscovery(ctx context.Context, task *domain.Task, step *domain.TaskStep, count int) {
	if e.Store == nil {
		return
	}
	_ = e.Store.AppendTaskEvent(ctx, &domain.TaskEvent{
		TaskID: task.ID, Type: "step.status", Message: fmt.Sprintf("subfinder 发现 %d 个子域候选", count),
		Payload: map[string]any{"stepId": step.ID, "kind": step.Kind, "status": "running", "discovery": "subfinder", "count": count},
	})
}

func (e *Engine) stepPort(ctx context.Context, task *domain.Task, step *domain.TaskStep) error {
	ips, err := e.taskAssetKeys(ctx, task.ID, domain.AssetIP)
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		for _, raw := range task.Targets {
			t, err := discovery.ParseTarget(raw)
			if err != nil {
				continue
			}
			if t.Kind == "cidr" {
				ips = append(ips, discovery.ExpandCIDR(t.Prefix, e.Config.Scan.MaxHosts)...)
			} else if t.Kind == "ip" {
				ips = append(ips, t.Host)
			}
		}
	}
	ports, err := discovery.PortsForType(task.Options.PortScanType, task.Options.PortCustom)
	if err != nil {
		return err
	}
	if len(ports) > e.Config.Scan.MaxPorts {
		ports = ports[:e.Config.Scan.MaxPorts]
	}
	results := e.Ports.ScanTargets(ctx, ips, ports, task.Options.ServiceDetect, func(done, total int) {
		p := 0
		if total > 0 {
			p = done * 100 / total
		}
		_ = e.Store.UpdateStepFields(ctx, step.ID, store.StepPatch{Progress: &p, Message: ptr(fmt.Sprintf("端口扫描 %d/%d", done, total))})
	})
	for _, ipRes := range results {
		data := map[string]any{
			"ip": ipRes.IP, "ipType": ipRes.IPType, "ports": portMaps(ipRes.Ports),
			"osInfo": ipRes.OSInfo, "geoAsn": ipRes.GeoASN, "geoCity": ipRes.GeoCity,
			"cdnName": ipRes.CDNName, "cdnType": ipRes.CDNType, "asn": ipRes.ASN,
			"domains": ipRes.Domains, "engine": "projectdiscovery/naabu", "engineVersion": pdtools.NaabuVersion,
		}
		asset, err := e.Store.UpsertAsset(ctx, domain.AssetIP, ipRes.IP, data, task.ID, task.ScopeID)
		if err != nil {
			continue
		}
		_ = e.Store.ObserveAsset(ctx, task.ID, asset, "port_scan", data)
		e.linkScope(ctx, task, asset)
		for _, port := range ipRes.Ports {
			svcData := map[string]any{
				"ip": ipRes.IP, "port": port.PortID, "serviceName": port.ServiceName, "version": port.Version,
				"product": port.Product, "banner": port.Banner, "deviceType": port.DeviceType,
				"osType": port.OSType, "cpes": port.CPEs, "confidence": port.Confidence,
				"engine": "projectdiscovery/naabu", "engineVersion": pdtools.NaabuVersion,
			}
			key := fmt.Sprintf("%s:%d/%s", ipRes.IP, port.PortID, port.Protocol)
			svc, err := e.Store.UpsertAsset(ctx, domain.AssetService, key, svcData, task.ID, task.ScopeID)
			if err == nil {
				_ = e.Store.ObserveAsset(ctx, task.ID, svc, "service_detection", svcData)
				e.linkScope(ctx, task, svc)
			}
			if task.Options.SSLCert && e.TLS != nil && (port.PortID == 443 || port.PortID == 8443 || port.TLS) {
				if cert, err := e.TLS.Inspect(ctx, ipRes.IP, ipRes.IP, port.PortID); err == nil {
					cd := map[string]any{"ip": cert.IP, "port": cert.Port, "subject": cert.Subject, "issuer": cert.Issuer, "notBefore": cert.NotBefore, "notAfter": cert.NotAfter, "dnsNames": cert.DNSNames}
					cd["serial"] = cert.Serial
					cd["fingerprintSha256"] = cert.FingerprintSHA256
					cd["tlsVersion"] = cert.TLSVersion
					cd["cipher"] = cert.Cipher
					cd["selfSigned"] = cert.SelfSigned
					cd["expired"] = cert.Expired
					cd["chain"] = cert.Chain
					cd["jarm"] = cert.JARM
					cd["engine"] = "projectdiscovery/tlsx"
					cd["engineVersion"] = pdtools.TLSXVersion
					ck := fmt.Sprintf("%s:%d", cert.IP, cert.Port)
					ca, err := e.Store.UpsertAsset(ctx, domain.AssetCert, ck, cd, task.ID, task.ScopeID)
					if err == nil {
						_ = e.Store.ObserveAsset(ctx, task.ID, ca, "tls_cert", cd)
						e.linkScope(ctx, task, ca)
					}
				}
			}
		}
	}
	return nil
}

func (e *Engine) stepHTTP(ctx context.Context, task *domain.Task, step *domain.TaskStep) error {
	var candidates []string
	if task.Kind == domain.TaskRiskCruising || task.Kind == domain.TaskAssetSiteAdd || task.Kind == domain.TaskAssetSiteSync {
		for _, t := range task.Targets {
			candidates = append(candidates, strings.TrimSpace(t))
		}
	} else {
		ips, _ := e.taskAssetKeys(ctx, task.ID, domain.AssetIP)
		for _, ip := range ips {
			sites, _ := e.sitesForIP(ctx, task.ID, ip)
			candidates = append(candidates, sites...)
		}
		if len(candidates) == 0 {
			for _, t := range task.Targets {
				target, err := discovery.ParseTarget(t)
				if err != nil || target.Kind != "domain" {
					continue
				}
				candidates = append(candidates, "http://"+target.Host, "https://"+target.Host)
			}
		}
	}
	candidates = unique(candidates)
	if len(candidates) == 0 {
		return nil
	}
	sites, probeErrs := e.HTTP.Probe(ctx, candidates)
	for _, err := range probeErrs {
		if e.Log != nil {
			e.Log.Warn("http probe failed", "taskId", task.ID, "error", err)
		}
	}
	var firstErr error
	for _, site := range sites {
		if task.Options.SiteIdentify {
			site.Fingerprints = e.Matcher.Identify(webintel.FingerprintVariables{
				Body: site.Body, Header: headerText(site.Headers), Title: site.Title,
				IconHash: strconv.FormatInt(int64(site.FaviconHash), 10),
			})
		}
		webintel.AutoTag(&site)
		data := map[string]any{
			"site": site.Site, "hostname": site.Hostname, "ip": site.IP, "status": site.Status,
			"title": site.Title, "headers": site.Headers, "httpServer": site.HTTPServer,
			"bodyLength": site.BodyLength, "contentType": site.ContentType,
			"favicon": map[string]any{"url": site.FaviconURL, "hash": site.FaviconHash},
			"finger":  site.Fingerprints, "tag": site.Tags, "technologies": site.Technologies,
			"cdn": site.CDN, "cdnName": site.CDNName, "cnames": site.CNAMEs,
			"finalUrl": site.FinalURL, "tls": site.TLS,
			"engine": "projectdiscovery/httpx", "engineVersion": pdtools.HTTPXVersion,
		}
		asset, err := e.Store.UpsertAsset(ctx, domain.AssetSite, site.Site, data, task.ID, task.ScopeID)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		_ = e.Store.ObserveAsset(ctx, task.ID, asset, "http_probe", data)
		e.linkScope(ctx, task, asset)
	}
	return firstErr
}

func (e *Engine) stepWebIntel(ctx context.Context, task *domain.Task, step *domain.TaskStep) error {
	sites, err := e.taskSiteSnapshots(ctx, task.ID)
	if err != nil {
		return err
	}
	if len(sites) == 0 {
		return nil
	}
	if task.Options.WebInfoHunter {
		if err := e.runWIH(ctx, task, step, sites); err != nil {
			return err
		}
	}
	if task.Options.SiteCapture {
		for _, site := range sites {
			raw, _ := site["site"].(string)
			if raw == "" {
				continue
			}
			if path, err := e.Screenshot.Capture(ctx, raw, task.ID); err == nil {
				_ = e.Store.UpdateTaskFields(ctx, task.ID, store.TaskPatch{LastScreenshot: &path})
			}
		}
	}
	if task.Options.SiteSpider {
		for _, site := range sites {
			raw, _ := site["site"].(string)
			if raw == "" {
				continue
			}
			urls, err := e.Spider.Crawl(ctx, raw)
			if err != nil && e.Log != nil {
				e.Log.Warn("katana crawl failed", "taskId", task.ID, "site", raw, "error", err)
			}
			for _, u := range urls {
				data := map[string]any{"site": raw, "url": u, "source": "site_spider"}
				asset, err := e.Store.UpsertAsset(ctx, domain.AssetURL, u, data, task.ID, task.ScopeID)
				if err == nil {
					_ = e.Store.ObserveAsset(ctx, task.ID, asset, "site_spider", data)
					e.linkScope(ctx, task, asset)
				}
			}
		}
	}
	if task.Options.FileLeak {
		words, err := discovery.LoadWords(e.AssetsDir + "/dicts/file_top_200.txt")
		if err == nil {
			for _, site := range sites {
				raw, _ := site["site"].(string)
				if raw == "" {
					continue
				}
				for _, r := range e.FileLeak.Scan(ctx, raw, words) {
					_, _ = e.addFinding(ctx, task, domain.FindingFileLeak, "file_leak", "文件泄漏: "+r.URL, r.URL, "medium", map[string]any{
						"url": r.URL, "status": r.Status, "contentLength": r.ContentLength,
					})
				}
			}
		}
	}
	if task.Options.NucleiScan && e.Nuclei != nil {
		var targets []string
		for _, site := range sites {
			raw, _ := site["site"].(string)
			if raw == "" {
				continue
			}
			targets = append(targets, raw)
		}
		results, err := e.Nuclei.Scan(ctx, unique(targets))
		if err != nil {
			return err
		}
		for _, result := range results {
			_, _ = e.addFinding(ctx, task, domain.FindingNuclei, "nuclei:"+result.TemplateID+":"+result.URL, result.Name, result.URL, result.Severity, map[string]any{
				"templateId": result.TemplateID, "name": result.Name, "severity": result.Severity,
				"url": result.URL, "status": result.Status, "templatePath": result.TemplatePath,
				"matcher": result.MatcherName, "extractedResults": result.ExtractedResults,
				"curlCommand": result.CURLCommand, "metadata": result.Metadata,
				"engine": "projectdiscovery/nuclei", "engineVersion": pdtools.NucleiVersion,
			})
		}
	}
	return nil
}

func (e *Engine) stepWIH(ctx context.Context, task *domain.Task, step *domain.TaskStep) error {
	sites, err := e.taskSiteSnapshots(ctx, task.ID)
	if err != nil {
		return err
	}
	return e.runWIH(ctx, task, step, sites)
}

func (e *Engine) runWIH(ctx context.Context, task *domain.Task, step *domain.TaskStep, sites []map[string]any) error {
	for _, site := range sites {
		raw, _ := site["site"].(string)
		if raw == "" {
			continue
		}
		records, err := e.WIH.Scan(ctx, raw)
		if err != nil {
			continue
		}
		for _, r := range records {
			_, _ = e.addFinding(ctx, task, domain.FindingWIH, "wih:"+r.Hash, r.RecordType+": "+r.Content, raw, "info", map[string]any{
				"recordType": r.RecordType, "content": r.Content, "source": r.Source, "hash": r.Hash,
			})
		}
	}
	return nil
}

func (e *Engine) stepPOC(ctx context.Context, task *domain.Task, step *domain.TaskStep) error {
	e.InitPlugins()
	var siteTargets []poc.Target
	var serviceTargets []poc.Target
	if task.Kind == domain.TaskRiskCruising {
		for _, raw := range task.Targets {
			if t, err := poc.ParseTarget(raw, "http", 80); err == nil {
				siteTargets = append(siteTargets, t)
			}
		}
	} else {
		sites, _ := e.taskSiteSnapshots(ctx, task.ID)
		for _, site := range sites {
			raw, _ := site["site"].(string)
			if raw == "" {
				continue
			}
			if t, err := poc.ParseTarget(raw, "http", 80); err == nil {
				siteTargets = append(siteTargets, t)
			}
		}
		ips, _ := e.taskAssetKeys(ctx, task.ID, domain.AssetIP)
		for _, ip := range ips {
			ports, _ := e.portsForIP(ctx, task.ID, ip)
			for _, p := range ports {
				serviceTargets = append(serviceTargets, poc.Target{Raw: ip, Scheme: "tcp", Host: ip, Port: p, URL: fmt.Sprintf("%s:%d", ip, p)})
			}
		}
	}
	var selected []poc.Plugin
	for _, name := range task.Options.POCPlugins {
		if p := e.pluginByID[name]; p != nil && p.Meta().Type == poc.TypePOC {
			selected = append(selected, p)
		}
	}
	for _, name := range task.Options.BrutePlugins {
		if p := e.pluginByID[name]; p != nil && p.Meta().Type == poc.TypeBrute {
			selected = append(selected, p)
		}
	}
	if task.Options.ServiceDetect {
		for _, p := range e.plugins {
			if p.Meta().Type == poc.TypeSniffer {
				selected = append(selected, p)
			}
		}
	}
	if len(selected) == 0 {
		return nil
	}
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	maxFindings := task.Options.MaxFindings
	if maxFindings <= 0 {
		maxFindings = 10000
	}
	var count int
	var mu sync.Mutex
	for _, p := range selected {
		targets := siteTargets
		if p.Meta().Type == poc.TypeSniffer || !usesHTTP(p.Meta()) {
			targets = serviceTargets
		}
		for _, target := range targets {
			mu.Lock()
			if count >= maxFindings {
				mu.Unlock()
				continue
			}
			mu.Unlock()
			wg.Add(1)
			sem <- struct{}{}
			go func(target poc.Target, p poc.Plugin) {
				defer wg.Done()
				defer func() { <-sem }()
				res, err := p.Run(ctx, target)
				if err != nil || !res.Hit {
					return
				}
				kind := domain.FindingVuln
				if p.Meta().Type == poc.TypeSniffer {
					kind = domain.FindingProtocol
				}
				title := p.Meta().VulName
				if title == "" {
					title = p.Meta().Name
				}
				data := map[string]any{"plugin": p.Meta().Name, "appName": p.Meta().AppName, "category": p.Meta().Category, "verifyData": res.VerifyData}
				for k, v := range res.Data {
					data[k] = v
				}
				add, err := e.addFinding(ctx, task, kind, p.Meta().Name+":"+res.VerifyData, title, res.Target, "medium", data)
				if err == nil && add {
					mu.Lock()
					count++
					mu.Unlock()
				}
			}(target, p)
		}
	}
	wg.Wait()
	return nil
}

func usesHTTP(meta poc.Meta) bool {
	for _, scheme := range meta.Schemes {
		if scheme == "http" || scheme == "https" {
			return true
		}
	}
	return false
}

func (e *Engine) stepFofa(ctx context.Context, task *domain.Task, step *domain.TaskStep) error {
	if e.Config.Fofa.Key == "" {
		return fmt.Errorf("fofa key is not configured")
	}
	if e.ControlClient == nil {
		return fmt.Errorf("control HTTP client is not configured")
	}
	src := discovery.FofaSource{Client: e.ControlClient, Key: e.Config.Fofa.Key, BaseURL: e.Config.Fofa.URL, PageSize: e.Config.Fofa.PageSize}
	domains, err := src.Query(ctx, task.Target)
	if err != nil {
		return err
	}
	for _, raw := range domains {
		for _, t := range strings.Split(raw, ",") {
			host := strings.TrimSpace(t)
			if host == "" {
				continue
			}
			target, err := discovery.ParseTarget(host)
			if err != nil || target.Kind != "ip" {
				continue
			}
			data := map[string]any{"ip": target.Host, "source": "fofa", "ports": []any{}}
			asset, err := e.Store.UpsertAsset(ctx, domain.AssetIP, target.Host, data, task.ID, task.ScopeID)
			if err == nil {
				_ = e.Store.ObserveAsset(ctx, task.ID, asset, "fofa", data)
				e.linkScope(ctx, task, asset)
			}
		}
	}
	return nil
}

func (e *Engine) addFinding(ctx context.Context, task *domain.Task, kind domain.FindingKind, dedupe, title, target, severity string, data map[string]any) (bool, error) {
	if target == "" {
		target = task.Target
	}
	return e.Store.AddFinding(ctx, &domain.Finding{
		TaskID: task.ID, ScopeID: task.ScopeID, Kind: kind, DedupeKey: dedupe,
		Title: title, Severity: severity, Target: target, Data: data,
	})
}

func (e *Engine) linkScope(ctx context.Context, task *domain.Task, asset *domain.Asset) {
	scopeID := task.ScopeID
	if scopeID == "" {
		scopeID = task.Options.RelatedScopeID
	}
	if scopeID == "" {
		return
	}
	id, err := domain.ParseID(scopeID)
	if err != nil {
		return
	}
	_ = e.Store.LinkAssetToScope(ctx, id, asset, nil)
}

func (e *Engine) taskAssetKeys(ctx context.Context, taskID string, kind domain.AssetKind) ([]string, error) {
	var out []string
	page := 1
	for {
		obs, total, err := e.Store.ListTaskObservations(ctx, taskID, string(kind), store.PageQuery{Page: page, PageSize: 200})
		if err != nil {
			return nil, err
		}
		for _, o := range obs {
			out = append(out, o.Key)
		}
		if len(obs) == 0 || int64(page*200) >= total {
			break
		}
		page++
	}
	return unique(out), nil
}

func (e *Engine) taskSiteSnapshots(ctx context.Context, taskID string) ([]map[string]any, error) {
	var out []map[string]any
	page := 1
	for {
		obs, total, err := e.Store.ListTaskObservations(ctx, taskID, string(domain.AssetSite), store.PageQuery{Page: page, PageSize: 200})
		if err != nil {
			return nil, err
		}
		for _, o := range obs {
			out = append(out, o.Snapshot)
		}
		if len(obs) == 0 || int64(page*200) >= total {
			break
		}
		page++
	}
	return out, nil
}

func (e *Engine) sitesForIP(ctx context.Context, taskID string, ip string) ([]string, error) {
	ports, err := e.portsForIP(ctx, taskID, ip)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range ports {
		switch p {
		case 80:
			out = append(out, "http://"+ip)
		case 443:
			out = append(out, "https://"+ip)
		default:
			out = append(out, fmt.Sprintf("http://%s:%d", ip, p), fmt.Sprintf("https://%s:%d", ip, p))
		}
	}
	if len(out) == 0 {
		out = append(out, "http://"+ip, "https://"+ip)
	}
	return out, nil
}

func (e *Engine) portsForIP(ctx context.Context, taskID string, ip string) ([]int, error) {
	assets, _, err := e.Store.ListAssets(ctx, string(domain.AssetIP), taskID, "", store.PageQuery{Page: 1, PageSize: 200})
	if err != nil {
		return nil, err
	}
	for _, a := range assets {
		if a.Key != ip {
			continue
		}
		raw, ok := a.Data["ports"]
		if !ok {
			return nil, nil
		}
		b, _ := json.Marshal(raw)
		var ports []map[string]any
		_ = json.Unmarshal(b, &ports)
		out := make([]int, 0, len(ports))
		for _, p := range ports {
			switch v := p["portId"].(type) {
			case float64:
				out = append(out, int(v))
			case int:
				out = append(out, v)
			}
		}
		return out, nil
	}
	return nil, nil
}

func headerText(h map[string]string) string {
	var b strings.Builder
	for k, v := range h {
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(v)
		b.WriteString("\n")
	}
	return b.String()
}

func portMaps(ports []discovery.PortInfo) []map[string]any {
	out := make([]map[string]any, 0, len(ports))
	for _, p := range ports {
		out = append(out, map[string]any{
			"portId": p.PortID, "serviceName": p.ServiceName, "version": p.Version,
			"product": p.Product, "protocol": p.Protocol, "banner": p.Banner, "tls": p.TLS,
			"deviceType": p.DeviceType, "osType": p.OSType, "cpes": p.CPEs, "confidence": p.Confidence,
		})
	}
	return out
}

func unique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
