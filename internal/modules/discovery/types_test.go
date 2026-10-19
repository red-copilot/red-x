package discovery

import "testing"

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
