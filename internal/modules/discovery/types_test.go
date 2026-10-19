package discovery

import "testing"

func TestIsRootDomain(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   bool
		wantOK bool
	}{
		{name: "registrable domain", input: "baidu.com", want: true, wantOK: true},
		{name: "registrable multi-label suffix", input: "guc.edu.cn", want: true, wantOK: true},
		{name: "registrable com cn", input: "foo.com.cn", want: true, wantOK: true},
		{name: "multi-label public suffix", input: "edu.cn", want: true, wantOK: true},
		{name: "multi-label public suffix com cn", input: "com.cn", want: true, wantOK: true},
		{name: "multi-label public suffix co uk", input: "co.uk", want: true, wantOK: true},
		{name: "subdomain", input: "www.baidu.com", want: false, wantOK: true},
		{name: "com cn subdomain", input: "www.foo.com.cn", want: false, wantOK: true},
		{name: "deep subdomain", input: "a.b.example.co.uk", want: false, wantOK: true},
		{name: "uppercase and trailing dot", input: "WWW.BAIDU.COM.", want: false, wantOK: true},
		{name: "idn root", input: "例子.中国", want: true, wantOK: true},
		{name: "idn subdomain", input: "www.例子.中国", want: false, wantOK: true},
		{name: "unmanaged internal tld", input: "intranet.corp", want: false, wantOK: true},
		{name: "unmanaged local tld", input: "foo.local", want: false, wantOK: true},
		{name: "private suffix", input: "foo.github.io", want: false, wantOK: true},
		{name: "private suffix subdomain", input: "www.foo.github.io", want: false, wantOK: true},
		{name: "single-label tld", input: "cn", want: false, wantOK: false},
		{name: "invalid domain", input: "not a domain", want: false, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsRootDomain(tt.input)
			if (err == nil) != tt.wantOK {
				t.Fatalf("IsRootDomain(%q) error = %v, wantOK = %v", tt.input, err, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("IsRootDomain(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTargets(t *testing.T) {
	targets, err := ParseTargets("example.com, 192.0.2.1\n10.0.0.0/24 example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 3 {
		t.Fatalf("expected 3 unique targets, got %d", len(targets))
	}
	if targets[0].Kind != "domain" || targets[1].Kind != "ip" || targets[2].Kind != "cidr" {
		t.Fatalf("unexpected target kinds: %+v", targets)
	}
}

func TestParsePortList(t *testing.T) {
	ports, err := ParsePortList("80,443,8000-8002,80")
	if err != nil {
		t.Fatal(err)
	}
	want := []int{80, 443, 8000, 8001, 8002}
	if len(ports) != len(want) {
		t.Fatalf("got %v", ports)
	}
	for i := range want {
		if ports[i] != want[i] {
			t.Fatalf("got %v", ports)
		}
	}
	if _, err := ParsePortList("70000"); err == nil {
		t.Fatal("invalid port should fail")
	}
}

func TestExpandCIDR(t *testing.T) {
	target, err := ParseTarget("192.0.2.0/30")
	if err != nil {
		t.Fatal(err)
	}
	ips := ExpandCIDR(target.Prefix, 10)
	if len(ips) != 4 {
		t.Fatalf("expected 4 addresses, got %v", ips)
	}
}

func TestPortTypes(t *testing.T) {
	ports, err := PortsForType("top1000", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) < 1000 {
		t.Fatalf("top1000 returned %d ports", len(ports))
	}
}
