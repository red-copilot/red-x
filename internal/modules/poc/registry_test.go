package poc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/platform/netsafe"
)

func TestBuiltinRegistry(t *testing.T) {
	client := NewHTTPClient(netsafe.Policy{AllowLoopback: true}, "", time.Second)
	plugins := Builtin(client, "")
	counts := map[PluginType]int{}
	names := map[string]bool{}
	for _, p := range plugins {
		counts[p.Meta().Type]++
		if names[p.Meta().Name] {
			t.Fatalf("duplicate plugin name %s", p.Meta().Name)
		}
		names[p.Meta().Name] = true
	}
	if counts[TypePOC] != 40 || counts[TypeSniffer] != 28 || counts[TypeBrute] != 28 {
		t.Fatalf("unexpected plugin counts: %+v", counts)
	}
	if len(plugins) != 96 {
		t.Fatalf("expected 96 plugins, got %d", len(plugins))
	}
}

func TestRedisSniffer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 64)
		_, _ = conn.Read(buf)
		_, _ = conn.Write([]byte("+PONG\r\n"))
	}()
	addr := ln.Addr().(*net.TCPAddr)
	plugin := &SnifferPlugin{
		MetaData: Meta{Name: "Sniffer_Redis", Type: TypeSniffer, Schemes: []string{"redis"}, DefaultPorts: []int{addr.Port}},
		Probe:    []byte("PING\r\n"), Match: []string{"pong"}, Timeout: time.Second,
	}
	res, err := plugin.Run(context.Background(), Target{Host: "127.0.0.1", Port: addr.Port})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Hit {
		t.Fatal("redis sniffer should hit")
	}
}
