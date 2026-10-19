package discovery

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Aabyss-Team/ARL/internal/platform/netsafe"
	"github.com/Aabyss-Team/ARL/internal/platform/pdtools"
	"github.com/projectdiscovery/naabu/v2/pkg/port"
	naaburesult "github.com/projectdiscovery/naabu/v2/pkg/result"
	naaburunner "github.com/projectdiscovery/naabu/v2/pkg/runner"
	"github.com/projectdiscovery/tlsx/pkg/tlsx"
	"github.com/projectdiscovery/tlsx/pkg/tlsx/clients"
)

type NaabuPortScanner struct {
	Threads          int
	Timeout          time.Duration
	Rate             int
	ScanType         string
	ServiceDiscovery bool
	ServiceVersion   bool
	ExcludeCDN       bool
	Policy           netsafe.Policy
}

func NewNaabuPortScanner(threads, rate, timeout int, scanType string, serviceDiscovery, serviceVersion, excludeCDN bool, policy netsafe.Policy) *NaabuPortScanner {
	if threads <= 0 {
		threads = 100
	}
	if rate <= 0 {
		rate = 1000
	}
	if timeout <= 0 {
		timeout = 3
	}
	scanType = strings.ToLower(strings.TrimSpace(scanType))
	if scanType != "syn" {
		scanType = "connect"
	}
	return &NaabuPortScanner{
		Threads: threads, Rate: rate, Timeout: time.Duration(timeout) * time.Second, ScanType: scanType,
		ServiceDiscovery: serviceDiscovery, ServiceVersion: serviceVersion, ExcludeCDN: excludeCDN, Policy: policy,
	}
}

func (s *NaabuPortScanner) ScanTargets(ctx context.Context, ips []string, ports []int, detectService bool, progress func(done, total int)) []IPResult {
	if len(ips) == 0 || len(ports) == 0 {
		return nil
	}
	var (
		mu      sync.Mutex
		byIP    = map[string][]*port.Port{}
		seen    = map[string]bool{}
		done    int
		results []IPResult
	)
	portSpec := joinPorts(ports)
	policy, err := pdtools.NewNetworkPolicy(s.Policy)
	if err != nil {
		return nil
	}
	options := &naaburunner.Options{
		Host:                 ips,
		Ports:                portSpec,
		ScanType:             s.ScanType,
		Threads:              s.Threads,
		Rate:                 s.Rate,
		Timeout:              s.Timeout,
		Retries:              1,
		ExcludeCDN:           s.ExcludeCDN,
		OutputCDN:            true,
		ServiceDiscovery:     detectService && s.ServiceDiscovery,
		ServiceVersion:       detectService && s.ServiceVersion,
		DisableStdout:        true,
		Silent:               true,
		NoColor:              true,
		DisableUpdateCheck:   true,
		DisableStdin:         true,
		NetworkPolicyOptions: policy.Options,
		OnReceive: func(hostResult *naaburesult.HostResult) {
			if hostResult == nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			byIP[hostResult.IP] = append(byIP[hostResult.IP], hostResult.Ports...)
			key := hostResult.IP
			if !seen[key] {
				seen[key] = true
				done++
			}
			if progress != nil {
				progress(done, len(ips))
			}
		},
	}
	runner, err := naaburunner.NewRunner(options)
	if err != nil {
		return nil
	}
	defer runner.Close()
	if err := runner.RunEnumeration(ctx); err != nil {
		return nil
	}

	mu.Lock()
	defer mu.Unlock()
	for _, ip := range ips {
		portsForIP := uniquePorts(byIP[ip])
		info := IPResult{IP: ip, IPType: ipType(ip), Ports: make([]PortInfo, 0, len(portsForIP))}
		for _, p := range portsForIP {
			info.Ports = append(info.Ports, portInfoFromNaabu(p))
		}
		sortPorts(info.Ports)
		if osInfo := osInfoFromPorts(info.Ports); osInfo != nil {
			info.OSInfo = osInfo
		}
		results = append(results, info)
	}
	return results
}

func portInfoFromNaabu(p *port.Port) PortInfo {
	info := PortInfo{PortID: p.Port, Protocol: p.Protocol.String(), TLS: p.TLS}
	if p.Service != nil {
		info.ServiceName = p.Service.Name
		info.Product = p.Service.Product
		info.Version = p.Service.Version
		info.DeviceType = p.Service.DeviceType
		info.OSType = p.Service.OSType
		info.CPEs = append([]string(nil), p.Service.CPEs...)
		info.Confidence = p.Service.Confidence
	}
	return info
}

func uniquePorts(in []*port.Port) []*port.Port {
	seen := map[string]bool{}
	out := make([]*port.Port, 0, len(in))
	for _, p := range in {
		if p == nil {
			continue
		}
		key := fmt.Sprintf("%d/%s", p.Port, p.Protocol.String())
		if !seen[key] {
			seen[key] = true
			out = append(out, p)
		}
	}
	return out
}

func joinPorts(ports []int) string {
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		parts = append(parts, strconv.Itoa(p))
	}
	return strings.Join(parts, ",")
}

func osInfoFromPorts(ports []PortInfo) map[string]any {
	for _, p := range ports {
		if p.OSType != "" {
			return map[string]any{"name": p.OSType, "confidence": p.Confidence, "method": "naabu-service"}
		}
	}
	return nil
}

func ipType(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return "UNKNOWN"
	}
	if addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return "PRIVATE"
	}
	return "PUBLIC"
}

func sortPorts(ports []PortInfo) {
	sort.Slice(ports, func(i, j int) bool { return ports[i].PortID < ports[j].PortID })
}

type TLSXInspector struct {
	Service *tlsx.Service
	Timeout time.Duration
}

func NewTLSXInspector(scanMode string, timeout, retries int) (*TLSXInspector, error) {
	if scanMode == "" {
		scanMode = "ctls"
	}
	if timeout <= 0 {
		timeout = 5
	}
	if retries <= 0 {
		retries = 1
	}
	service, err := tlsx.New(&clients.Options{
		ScanMode: scanMode, Timeout: timeout, Retries: retries,
		TLSVersion: true, Expired: true, SelfSigned: true, CN: true, SAN: true, Jarm: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create tlsx service: %w", err)
	}
	return &TLSXInspector{Service: service, Timeout: time.Duration(timeout) * time.Second}, nil
}

func (t *TLSXInspector) Inspect(ctx context.Context, host, ip string, portNumber int) (*CertResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resp, err := t.Service.Connect(host, ip, strconv.Itoa(portNumber))
	if err != nil {
		return nil, err
	}
	return certResultFromTLSX(resp, host, ip, portNumber), nil
}

func certResultFromTLSX(resp *clients.Response, host, ip string, portNumber int) *CertResult {
	out := &CertResult{IP: ip, Port: portNumber, Subject: host}
	if resp == nil {
		return out
	}
	if resp.CertificateResponse == nil {
		out.TLSVersion = resp.Version
		out.Cipher = resp.Cipher
		out.JARM = resp.JarmHash
		return out
	}
	out.Subject = resp.SubjectCN
	if out.Subject == "" {
		out.Subject = resp.SubjectDN
	}
	out.Issuer = resp.IssuerCN
	if out.Issuer == "" {
		out.Issuer = resp.IssuerDN
	}
	if !resp.NotBefore.IsZero() {
		out.NotBefore = resp.NotBefore.UTC().Format(time.RFC3339)
	}
	if !resp.NotAfter.IsZero() {
		out.NotAfter = resp.NotAfter.UTC().Format(time.RFC3339)
	}
	out.DNSNames = uniqueStrings(append([]string(nil), resp.SubjectAN...))
	out.Serial = resp.Serial
	out.FingerprintSHA256 = resp.FingerprintHash.SHA256
	out.TLSVersion = resp.Version
	out.Cipher = resp.Cipher
	out.SelfSigned = resp.SelfSigned
	out.Expired = resp.Expired
	out.JARM = resp.JarmHash
	for _, cert := range resp.Chain {
		if cert != nil {
			out.Chain = append(out.Chain, cert.SubjectCN)
		}
	}
	return out
}

var _ PortScanner = (*NaabuPortScanner)(nil)
var _ TLSInspector = (*TLSXInspector)(nil)
