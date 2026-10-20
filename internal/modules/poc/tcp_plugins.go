package poc

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/red-copilot/red-x/internal/modules/poc/mongowire"
)

type TCPNoAuthPlugin struct {
	MetaData Meta
	Probe    []byte
	Check    func(ctx context.Context, target Target, response string) (bool, map[string]any)
}

func (p *TCPNoAuthPlugin) Meta() Meta { return p.MetaData }

func (p *TCPNoAuthPlugin) Run(ctx context.Context, target Target) (Result, error) {
	port := target.Port
	if port == 0 && len(p.MetaData.DefaultPorts) > 0 {
		port = p.MetaData.DefaultPorts[0]
	}
	d := net.Dialer{Timeout: 4 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(target.Host, fmt.Sprintf("%d", port)))
	if err != nil {
		return Result{}, nil
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if len(p.Probe) > 0 {
		_, _ = conn.Write(p.Probe)
	}
	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)
	resp := string(buf[:n])
	ok, data := p.Check(ctx, target, resp)
	if !ok {
		return Result{}, nil
	}
	uri := fmt.Sprintf("%s://%s:%d", p.MetaData.Schemes[0], target.Host, port)
	return Result{Hit: true, Target: uri, VerifyData: uri, Data: data}, nil
}

func BuiltinTCPNoAuth() []Plugin {
	redis := &TCPNoAuthPlugin{
		MetaData: Meta{Name: "Redis_noauth", Type: TypePOC, AppName: "Redis", VulName: "Redis 未授权访问", Category: "未授权访问", Schemes: []string{"redis"}, DefaultPorts: []int{6379}},
		Probe:    []byte("INFO\r\n"),
		Check: func(_ context.Context, _ Target, response string) (bool, map[string]any) {
			if strings.Contains(response, "redis_version:") {
				return true, map[string]any{"banner": sanitize(response), "protocol": "redis"}
			}
			return false, nil
		},
	}
	memcached := &TCPNoAuthPlugin{
		MetaData: Meta{Name: "Memcached_noauth", Type: TypePOC, AppName: "Memcached", VulName: "Memcached 未授权访问", Category: "未授权访问", Schemes: []string{"memcached"}, DefaultPorts: []int{11211}},
		Probe:    []byte("stats\r\n"),
		Check: func(_ context.Context, _ Target, response string) (bool, map[string]any) {
			if strings.Contains(response, "STAT ") {
				return true, map[string]any{"banner": sanitize(response), "protocol": "memcached"}
			}
			return false, nil
		},
	}
	zookeeper := &TCPNoAuthPlugin{
		MetaData: Meta{Name: "ZooKeeper_noauth", Type: TypePOC, AppName: "ZooKeeper", VulName: "ZooKeeper 未授权访问", Category: "未授权访问", Schemes: []string{"zookeeper"}, DefaultPorts: []int{2181}},
		Probe:    []byte("ruok"),
		Check: func(_ context.Context, _ Target, response string) (bool, map[string]any) {
			if strings.Contains(response, "imok") {
				return true, map[string]any{"banner": sanitize(response), "protocol": "zookeeper"}
			}
			return false, nil
		},
	}
	mongoPlugin := &MongoNoAuthPlugin{
		MetaData: Meta{Name: "Mongodb_noauth", Type: TypePOC, AppName: "MongoDB", VulName: "MongoDB 未授权访问", Category: "未授权访问", Schemes: []string{"mongodb"}, DefaultPorts: []int{27017}},
	}
	return []Plugin{redis, memcached, zookeeper, mongoPlugin}
}

type MongoNoAuthPlugin struct{ MetaData Meta }

func (p *MongoNoAuthPlugin) Meta() Meta { return p.MetaData }

func (p *MongoNoAuthPlugin) Run(ctx context.Context, target Target) (Result, error) {
	port := target.Port
	if port == 0 {
		port = 27017
	}
	address := net.JoinHostPort(target.Host, fmt.Sprintf("%d", port))
	if err := mongowire.ProbeUnauthenticated(ctx, address); err != nil {
		return Result{}, nil
	}
	uriOut := fmt.Sprintf("mongodb://%s:%d", target.Host, port)
	return Result{Hit: true, Target: uriOut, VerifyData: uriOut, Data: map[string]any{"protocol": "mongodb", "target": uriOut}}, nil
}
