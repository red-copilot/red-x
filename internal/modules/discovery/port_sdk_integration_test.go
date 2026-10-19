package discovery

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/platform/netsafe"
)

func TestNaabuPortScannerFindsLocalPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	_, portText, _ := net.SplitHostPort(listener.Addr().String())
	portNumber, _ := strconv.Atoi(portText)

	scanner := NewNaabuPortScanner(10, 50, 2, "connect", false, false, false, netsafe.Policy{AllowLoopback: true})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	results := scanner.ScanTargets(ctx, []string{"127.0.0.1"}, []int{portNumber}, false, nil)
	if len(results) != 1 || len(results[0].Ports) != 1 || results[0].Ports[0].PortID != portNumber {
		t.Fatalf("port results = %+v", results)
	}
}

func TestTLSXInspectorReadsSelfSignedCertificate(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	defer srv.Close()
	_, portText, _ := net.SplitHostPort(srv.Listener.Addr().String())
	portNumber, _ := strconv.Atoi(portText)
	inspector, err := NewTLSXInspector("ctls", 3, 1)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cert, err := inspector.Inspect(ctx, "127.0.0.1", "127.0.0.1", portNumber)
	if err != nil {
		t.Fatal(err)
	}
	if cert.Port != portNumber || cert.NotAfter == "" || !cert.SelfSigned {
		t.Fatalf("cert = %+v", cert)
	}
}
