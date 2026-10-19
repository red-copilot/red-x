package discovery

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"
)

type Target struct {
	Raw    string
	Host   string
	Kind   string // domain, ip, cidr, url
	Prefix netip.Prefix
}

type DomainResult struct {
	Domain   string   `json:"domain"`
	Type     string   `json:"type"`
	Records  []string `json:"records"`
	IPs      []string `json:"ips"`
	Source   string   `json:"source"`
	TTL      uint32   `json:"ttl,omitempty"`
	Resolver []string `json:"resolver,omitempty"`
	CNAME    []string `json:"cname,omitempty"`
	CDN      bool     `json:"cdn,omitempty"`
	CDNName  string   `json:"cdnName,omitempty"`
	CDNType  string   `json:"cdnType,omitempty"`
}

type PortInfo struct {
	PortID      int      `json:"portId"`
	ServiceName string   `json:"serviceName"`
	Version     string   `json:"version,omitempty"`
	Product     string   `json:"product,omitempty"`
	Protocol    string   `json:"protocol"`
	Banner      string   `json:"banner,omitempty"`
	TLS         bool     `json:"tls,omitempty"`
	DeviceType  string   `json:"deviceType,omitempty"`
	OSType      string   `json:"osType,omitempty"`
	CPEs        []string `json:"cpes,omitempty"`
	Confidence  int      `json:"confidence,omitempty"`
}

type IPResult struct {
	IP         string            `json:"ip"`
	IPType     string            `json:"ipType"`
	Ports      []PortInfo        `json:"ports"`
	OSInfo     map[string]any    `json:"osInfo,omitempty"`
	GeoASN     map[string]any    `json:"geoAsn,omitempty"`
	GeoCity    map[string]any    `json:"geoCity,omitempty"`
	CDNName    string            `json:"cdnName,omitempty"`
	CDNType    string            `json:"cdnType,omitempty"`
	ASN        map[string]any    `json:"asn,omitempty"`
	Domains    []string          `json:"domains,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type CertResult struct {
	IP                string   `json:"ip"`
	Port              int      `json:"port"`
	Subject           string   `json:"subject"`
	Issuer            string   `json:"issuer"`
	NotBefore         string   `json:"notBefore"`
	NotAfter          string   `json:"notAfter"`
	DNSNames          []string `json:"dnsNames"`
	Serial            string   `json:"serial,omitempty"`
	FingerprintSHA256 string   `json:"fingerprintSha256,omitempty"`
	TLSVersion        string   `json:"tlsVersion,omitempty"`
	Cipher            string   `json:"cipher,omitempty"`
	SelfSigned        bool     `json:"selfSigned,omitempty"`
	Expired           bool     `json:"expired,omitempty"`
	Chain             []string `json:"chain,omitempty"`
	JARM              string   `json:"jarm,omitempty"`
}

type SiteResult struct {
	Site         string            `json:"site"`
	Hostname     string            `json:"hostname"`
	IP           string            `json:"ip"`
	Status       int               `json:"status"`
	Title        string            `json:"title"`
	Headers      map[string]string `json:"headers"`
	HTTPServer   string            `json:"httpServer"`
	BodyLength   int               `json:"bodyLength"`
	ContentType  string            `json:"contentType"`
	FaviconURL   string            `json:"faviconUrl,omitempty"`
	FaviconHash  int32             `json:"faviconHash,omitempty"`
	Fingerprints []string          `json:"fingerprints,omitempty"`
	Technologies []string          `json:"technologies,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	CDN          bool              `json:"cdn,omitempty"`
	CDNName      string            `json:"cdnName,omitempty"`
	CNAMEs       []string          `json:"cnames,omitempty"`
	FinalURL     string            `json:"finalUrl,omitempty"`
	TLS          map[string]any    `json:"tls,omitempty"`
	Body         string            `json:"-"`
}

type Resolver interface {
	Resolve(ctx context.Context, domain string) (DomainResult, error)
	WildcardIPs(ctx context.Context, base string) []string
	Brute(ctx context.Context, base string, words []string, wildcard []string, progress func(done, total int)) []DomainResult
}

type PassiveEnumerator interface {
	Enumerate(ctx context.Context, domain string) ([]DomainResult, error)
}

type PortScanner interface {
	ScanTargets(ctx context.Context, ips []string, ports []int, detectService bool, progress func(done, total int)) []IPResult
}

type TLSInspector interface {
	Inspect(ctx context.Context, host, ip string, port int) (*CertResult, error)
}

type HTTPProbe interface {
	Probe(ctx context.Context, targets []string) ([]SiteResult, []error)
}

func ParseTarget(raw string) (Target, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return Target{}, fmt.Errorf("empty target")
	}
	if strings.Contains(raw, "://") {
		u, err := netip.ParseAddr(raw)
		_ = u
		_ = err
		return Target{Raw: raw, Host: raw, Kind: "url"}, nil
	}
	if prefix, err := netip.ParsePrefix(raw); err == nil {
		return Target{Raw: raw, Host: prefix.Masked().String(), Kind: "cidr", Prefix: prefix}, nil
	}
	if addr, err := netip.ParseAddr(raw); err == nil {
		return Target{Raw: raw, Host: addr.String(), Kind: "ip"}, nil
	}
	if strings.Contains(raw, "/") {
		return Target{}, fmt.Errorf("invalid CIDR %q", raw)
	}
	return Target{Raw: raw, Host: raw, Kind: "domain"}, nil
}

func ParseTargets(raw string) ([]Target, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	seen := map[string]bool{}
	var out []Target
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		t, err := ParseTarget(p)
		if err != nil {
			return nil, err
		}
		if seen[t.Raw] {
			continue
		}
		seen[t.Raw] = true
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("target is empty")
	}
	return out, nil
}

func ExpandCIDR(prefix netip.Prefix, max int) []string {
	if max <= 0 {
		max = 4096
	}
	var out []string
	for addr := prefix.Masked().Addr(); prefix.Contains(addr); addr = addr.Next() {
		if len(out) >= max {
			break
		}
		out = append(out, addr.String())
		if !addr.Next().IsValid() {
			break
		}
	}
	return out
}

var top10 = []int{80, 443, 8443, 8080, 8081, 8888, 8089, 5000, 5001, 8085, 800, 81, 9000, 88, 8001, 8090}

var top100 = []int{
	1000, 10000, 10001, 10030, 1024, 10250, 10255, 10256, 10443, 1080, 1099, 110, 111, 11211, 123, 1234,
	12345, 135, 137, 138, 139, 143, 1433, 1434, 1443, 15000, 1521, 16030, 18001, 18080, 1883, 2000,
	20000, 2049, 2080, 2081, 2082, 2083, 2086, 2087, 20880, 2096, 21, 2181, 22, 2222, 2323, 25,
	2525, 25672, 27017, 3000, 30000, 3001, 3306, 3307, 3389, 389, 4000, 4430, 4443, 4444, 445, 465,
	4848, 500, 5000, 5001, 5002, 5003, 5005, 50050, 5006, 50060, 5007, 50070, 50075, 50090, 5050, 5060,
	5222, 5357, 5432, 554, 5555, 5601, 5672, 5801, 587, 5985, 5986, 6000, 6060, 6080, 6379, 6443,
	6666, 7001, 7002, 7003, 7005, 7007, 7070, 7071, 7080, 7180, 7443, 7777, 8000, 8001, 8002, 8003,
	8005, 8006, 8008, 8009, 8010, 8012, 8020, 8025, 8030, 8031, 8032, 8060, 8069, 8080, 8081, 8082,
	8083, 8084, 8085, 8086, 8087, 8088, 8089, 8090, 8091, 8092, 8093, 8098, 8099, 8100, 8123, 8150,
	8161, 8180, 8181, 8191, 8288, 8443, 8480, 8686, 873, 880, 8800, 8848, 888, 8880, 8883, 8887,
	8888, 8889, 8890, 8899, 8983, 8989, 8999, 9000, 9001, 9002, 9003, 9080, 9083, 9090, 9091, 9099,
	9100, 9200, 9300, 9443, 9527, 9876, 9944, 9998, 9999, 80, 443,
}

var top1000 = buildTop1000()

func buildTop1000() []int {
	seen := map[int]bool{}
	var out []int
	add := func(p int) {
		if p > 0 && p <= 65535 && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, p := range top100 {
		add(p)
	}
	for p := 1; p <= 1024; p++ {
		add(p)
	}
	for _, p := range []int{1433, 1521, 1723, 1883, 2049, 2181, 2375, 2376, 3000, 3128, 3260, 3306, 3389, 4444, 4567, 5000, 5432, 5601, 5672, 5900, 5984, 6379, 6443, 7001, 7002, 8000, 8008, 8080, 8081, 8088, 8443, 8888, 9000, 9042, 9090, 9200, 9300, 9443, 11211, 27017} {
		add(p)
	}
	sort.Ints(out)
	return out
}

func PortsForType(kind, custom string) ([]int, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "test":
		return append([]int(nil), top10...), nil
	case "top100":
		return append([]int(nil), top100...), nil
	case "top1000":
		return append([]int(nil), top1000...), nil
	case "all":
		out := make([]int, 0, 65535)
		for p := 1; p <= 65535; p++ {
			out = append(out, p)
		}
		return out, nil
	case "custom":
		return ParsePortList(custom)
	default:
		return nil, fmt.Errorf("unknown port scan type %q", kind)
	}
}

func ParsePortList(spec string) ([]int, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("empty port list")
	}
	seen := map[int]bool{}
	var out []int
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			start, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err1 != nil || err2 != nil || start < 1 || end > 65535 || start > end {
				return nil, fmt.Errorf("invalid port range %q", part)
			}
			for p := start; p <= end; p++ {
				if !seen[p] {
					seen[p] = true
					out = append(out, p)
				}
			}
			continue
		}
		p, err := strconv.Atoi(part)
		if err != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("invalid port %q", part)
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Ints(out)
	return out, nil
}
