package discovery

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"net/netip"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/red-copilot/red-x/internal/platform/netsafe"
	"github.com/red-copilot/red-x/internal/platform/pdtools"
	mdns "github.com/miekg/dns"
	"github.com/projectdiscovery/dnsx/libs/dnsx"
	retryabledns "github.com/projectdiscovery/retryabledns"
)

type PDDNSResolver struct {
	client      *dnsx.DNSX
	policy      netsafe.Policy
	concurrency int
	timeout     time.Duration
}

func NewPDDNSResolver(servers []string, concurrency, retries int, timeout time.Duration, policy netsafe.Policy) (*PDDNSResolver, error) {
	if concurrency <= 0 {
		concurrency = 100
	}
	if retries <= 0 {
		retries = 3
	}
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	resolvers := pdtools.NormalizeResolvers(servers)
	if len(resolvers) == 0 {
		resolvers = dnsx.DefaultResolvers
	}
	client, err := dnsx.New(dnsx.Options{
		BaseResolvers: resolvers,
		MaxRetries:    retries,
		QuestionTypes: []uint16{mdns.TypeA, mdns.TypeAAAA, mdns.TypeCNAME},
		Timeout:       timeout,
		Hostsfile:     false,
		OutputCDN:     true,
	})
	if err != nil {
		return nil, fmt.Errorf("create dnsx resolver: %w", err)
	}
	return &PDDNSResolver{client: client, policy: policy, concurrency: concurrency, timeout: timeout}, nil
}

func (r *PDDNSResolver) Resolve(ctx context.Context, domain string) (DomainResult, error) {
	domain = normalizeDomain(domain)
	res := DomainResult{Domain: domain, Source: "dns"}
	if domain == "" {
		return res, fmt.Errorf("empty domain")
	}
	if err := ctx.Err(); err != nil {
		return res, err
	}
	data, err := r.client.QueryMultiple(domain)
	if err != nil {
		return res, err
	}
	res = domainResultFromDNSData(domain, data, r.client)
	res.IPs = filterBlockedIPs(res.IPs, r.policy)
	if len(res.IPs) == 0 && len(res.CNAME) == 0 {
		res.Type = "NONE"
		res.Records = nil
	}
	return res, nil
}

func domainResultFromDNSData(domain string, data *retryabledns.DNSData, client *dnsx.DNSX) DomainResult {
	res := DomainResult{Domain: normalizeDomain(domain), Source: "dns"}
	if data == nil {
		res.Type = "NONE"
		return res
	}
	res.IPs = uniqueStrings(append(append([]string(nil), data.A...), data.AAAA...))
	res.CNAME = uniqueStrings(append([]string(nil), data.CNAME...))
	res.TTL = data.TTL
	res.Resolver = uniqueStrings(append([]string(nil), data.Resolver...))
	if len(res.CNAME) > 0 {
		res.Type = "CNAME"
		res.Records = append([]string(nil), res.CNAME...)
	} else if len(res.IPs) > 0 {
		res.Type = "A"
		res.Records = append([]string(nil), res.IPs...)
	} else {
		res.Type = "NONE"
	}
	if client != nil {
		if matched, name, itemType, err := client.CdnCheckRespData(data); err == nil {
			res.CDN, res.CDNName, res.CDNType = matched, name, itemType
		}
	}
	return res
}

func (r *PDDNSResolver) WildcardIPs(ctx context.Context, base string) []string {
	seen := map[string]bool{}
	var out []string
	for i := 0; i < 3; i++ {
		label := fmt.Sprintf("arl-wildcard-%d-%d", time.Now().UnixNano(), rand.Int63())
		res, err := r.Resolve(ctx, label+"."+normalizeDomain(base))
		if err != nil {
			continue
		}
		for _, ip := range res.IPs {
			if !seen[ip] {
				seen[ip] = true
				out = append(out, ip)
			}
		}
	}
	return out
}

func (r *PDDNSResolver) Brute(ctx context.Context, base string, words []string, wildcard []string, progress func(done, total int)) []DomainResult {
	if len(words) == 0 {
		return nil
	}
	wild := map[string]bool{}
	for _, ip := range wildcard {
		wild[ip] = true
	}
	sem := make(chan struct{}, r.concurrency)
	var wg sync.WaitGroup
	var done atomic.Int64
	var mu sync.Mutex
	var out []DomainResult
	base = normalizeDomain(base)
	for _, word := range words {
		word = strings.TrimSpace(strings.ToLower(word))
		if word == "" {
			continue
		}
		name := word
		if strings.Contains(base, "{fuzz}") {
			name = strings.ReplaceAll(base, "{fuzz}", word)
		} else {
			name = word + "." + base
		}
		if err := ctx.Err(); err != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(name string) {
			defer wg.Done()
			defer func() { <-sem }()
			n := int(done.Add(1))
			if progress != nil && (n%50 == 0 || n == len(words)) {
				progress(n, len(words))
			}
			res, err := r.Resolve(ctx, name)
			if err != nil || len(res.IPs) == 0 {
				return
			}
			for _, ip := range res.IPs {
				if wild[ip] {
					return
				}
			}
			res.Source = "domain_brute"
			mu.Lock()
			out = append(out, res)
			mu.Unlock()
		}(name)
	}
	wg.Wait()
	return out
}

func filterBlockedIPs(ips []string, policy netsafe.Policy) []string {
	out := make([]string, 0, len(ips))
	seen := map[string]bool{}
	for _, raw := range ips {
		addr, err := netip.ParseAddr(strings.TrimSpace(raw))
		if err != nil || policy.Blocked(addr) || seen[addr.String()] {
			continue
		}
		seen[addr.String()] = true
		out = append(out, addr.String())
	}
	return out
}

func normalizeDomain(domain string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
}

func LoadWords(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for sc.Scan() {
		w := strings.TrimSpace(sc.Text())
		if w != "" && !strings.HasPrefix(w, "#") {
			out = append(out, w)
		}
	}
	return out, sc.Err()
}

func AltDNSWords(subdomains []string, base string, words []string) []string {
	if len(subdomains) == 0 || len(words) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, sub := range subdomains {
		prefix := strings.TrimSuffix(sub, "."+base)
		if prefix == sub {
			continue
		}
		parts := strings.Split(prefix, ".")
		for _, w := range words {
			for i := 0; i < len(parts); i++ {
				cp := append([]string(nil), parts...)
				cp[i] = w + cp[i]
				add(strings.Join(cp, ".") + "." + base)
				cp = append([]string(nil), parts...)
				cp[i] = cp[i] + w
				add(strings.Join(cp, ".") + "." + base)
				cp = append([]string(nil), parts...)
				cp[i] = w + "-" + cp[i]
				add(strings.Join(cp, ".") + "." + base)
				cp = append([]string(nil), parts...)
				cp[i] = cp[i] + "-" + w
				add(strings.Join(cp, ".") + "." + base)
			}
			add(w + "." + strings.Join(parts, ".") + "." + base)
			for n := 1; n <= 3; n++ {
				cp := append([]string(nil), parts...)
				cp[len(cp)-1] = fmt.Sprintf("%s%d", cp[len(cp)-1], n)
				add(strings.Join(cp, ".") + "." + base)
			}
		}
	}
	return out
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
