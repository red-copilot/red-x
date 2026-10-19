package discovery

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/Aabyss-Team/ARL/internal/platform/netsafe"
	mdns "github.com/miekg/dns"
)

func TestPDDNSResolverQueriesRecords(t *testing.T) {
	packetConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mux := mdns.NewServeMux()
	mux.HandleFunc(".", func(w mdns.ResponseWriter, req *mdns.Msg) {
		resp := new(mdns.Msg)
		resp.SetReply(req)
		if len(req.Question) > 0 {
			question := req.Question[0]
			switch {
			case question.Qtype == mdns.TypeA && question.Name == "www.example.test.":
				resp.Answer = append(resp.Answer, &mdns.A{
					Hdr: mdns.RR_Header{Name: question.Name, Rrtype: mdns.TypeA, Class: mdns.ClassINET, Ttl: 120},
					A:   net.ParseIP("192.0.2.10"),
				})
			case question.Qtype == mdns.TypeCNAME && question.Name == "www.example.test.":
				resp.Answer = append(resp.Answer, &mdns.CNAME{
					Hdr:    mdns.RR_Header{Name: question.Name, Rrtype: mdns.TypeCNAME, Class: mdns.ClassINET, Ttl: 120},
					Target: "edge.example.test.",
				})
			}
		}
		_ = w.WriteMsg(resp)
	})
	server := &mdns.Server{PacketConn: packetConn, Handler: mux}
	go func() { _ = server.ActivateAndServe() }()
	defer server.Shutdown()

	resolver, err := NewPDDNSResolver(
		[]string{packetConn.LocalAddr().String()}, 2, 1, 2*time.Second,
		netsafe.Policy{AllowLoopback: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := resolver.Resolve(ctx, "www.example.test")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.IPs) != 1 || got.IPs[0] != "192.0.2.10" {
		t.Fatalf("resolved IPs = %v", got.IPs)
	}
	if len(got.CNAME) != 1 || got.CNAME[0] != "edge.example.test" {
		t.Fatalf("CNAME = %v", got.CNAME)
	}
}
