package poc

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

type SnifferPlugin struct {
	MetaData Meta
	Probe    []byte
	Match    []string
	Regex    string
	Scheme   string
	Timeout  time.Duration
}

func (p *SnifferPlugin) Meta() Meta { return p.MetaData }

func (p *SnifferPlugin) Run(ctx context.Context, target Target) (Result, error) {
	port := target.Port
	if port == 0 && len(p.MetaData.DefaultPorts) > 0 {
		port = p.MetaData.DefaultPorts[0]
	}
	dialer := net.Dialer{Timeout: p.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(target.Host, fmt.Sprintf("%d", port)))
	if err != nil {
		return Result{}, nil
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(p.Timeout))
	if len(p.Probe) > 0 {
		_, _ = conn.Write(p.Probe)
	}
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if n <= 0 && err != nil {
		return Result{}, nil
	}
	text := string(buf[:n])
	matched := false
	if p.Regex != "" {
		if re, err := regexp.Compile(p.Regex); err == nil && re.MatchString(text) {
			matched = true
		}
	}
	if len(p.Match) > 0 {
		lower := strings.ToLower(text)
		for _, m := range p.Match {
			if strings.Contains(lower, strings.ToLower(m)) {
				matched = true
				break
			}
		}
	}
	if !matched {
		return Result{}, nil
	}
	scheme := p.Scheme
	if scheme == "" {
		scheme = p.MetaData.Schemes[0]
	}
	uri := fmt.Sprintf("%s://%s:%d", scheme, target.Host, port)
	return Result{Hit: true, Target: uri, VerifyData: uri, Data: map[string]any{"scheme": scheme, "host": target.Host, "port": port, "banner": sanitize(text)}}, nil
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 500 {
		s = s[:500]
	}
	return strings.TrimSpace(s)
}

func BuiltinSniffers() []Plugin {
	type def struct {
		name, app, scheme string
		port              int
		probe             string
		match             []string
		regex             string
	}
	defs := []def{
		{"Sniffer_SSH", "SSH", "ssh", 22, "", []string{"ssh-"}, `(?i)^SSH-`},
		{"Sniffer_FTP", "FTP", "ftp", 21, "", []string{"ftp", "220"}, `(?i)^220`},
		{"Sniffer_MySQL", "MySQL", "mysql", 3306, "", []string{"mysql", "mariadb"}, `(?i)mysql|mariadb`},
		{"Sniffer_Redis", "Redis", "redis", 6379, "PING\r\n", []string{"pong", "-noauth", "-err"}, `(?i)\+PONG|-NOAUTH|-ERR`},
		{"Sniffer_Mongo", "MongoDB", "mongodb", 27017, "", []string{"ismaster", "mongodb"}, `(?i)ismaster|mongodb`},
		{"Sniffer_Postgresql", "PostgreSQL", "psql", 5432, "", []string{"postgresql", "pg"}, `(?i)postgres`},
		{"Sniffer_SMTP", "SMTP", "smtp", 25, "", []string{"smtp", "220"}, `(?i)^220`},
		{"Sniffer_POP3", "POP3", "pop3", 110, "", []string{"+ok", "pop3"}, `(?i)^\+OK`},
		{"Sniffer_IMAP", "IMAP", "imap", 143, "", []string{"imap", "* ok"}, `(?i)^\* OK`},
		{"Sniffer_LDAP", "LDAP", "ldap", 389, "", []string{"ldap"}, `(?i)ldap`},
		{"Sniffer_RDP", "RDP", "rdp", 3389, "\x03\x00\x00\x13\x0e\xe0\x00\x00\x00\x00\x00\x01\x00\x08\x00\x03\x00\x00\x00", []string{"cookie", "rdp"}, `(?i)rdp|cookie`},
		{"Sniffer_MemCache", "Memcached", "memcached", 11211, "stats\r\n", []string{"stat ", "version"}, `(?i)STAT `},
		{"Sniffer_ZooKeeper", "ZooKeeper", "zookeeper", 2181, "ruok", []string{"imok"}, `(?i)imok`},
		{"Sniffer_Socks4", "SOCKS4", "socks4", 1080, "\x04\x01\x00\x50\x7f\x00\x00\x01\x00", []string{"\x00"}, ""},
		{"Sniffer_Socks5", "SOCKS5", "socks5", 1080, "\x05\x01\x00", []string{"\x05"}, ""},
		{"Sniffer_ProxyHTTPS", "HTTPS Proxy", "proxy_https", 8080, "CONNECT example.com:443 HTTP/1.1\r\nHost: example.com:443\r\n\r\n", []string{"200", "http/"}, `(?i)HTTP/1\.[01] 200`},
		{"Sniffer_Rsync", "Rsync", "rsync", 873, "", []string{"@RSYNCD:"}, `(?i)@RSYNCD:`},
		{"Sniffer_AJP", "AJP", "ajp", 8009, "", []string{"ajp"}, `(?i)ajp`},
		{"Sniffer_CobaltStrike", "CobaltStrike", "csts", 50050, "", []string{"cobalt", "\x00\x00\x00\x00"}, ""},
		{"Sniffer_Dubbo", "Dubbo", "dubbo", 20880, "", []string{"dubbo"}, `(?i)dubbo`},
		{"Sniffer_HRPC", "Hadoop RPC", "hrpc", 8020, "", []string{"hrpc", "hadoop"}, `(?i)hrpc|hadoop`},
		{"Sniffer_IIOP", "IIOP", "iiop", 7001, "", []string{"iiop", "giop"}, `(?i)iiop|giop`},
		{"Sniffer_JDWP", "JDWP", "jdwp", 8000, "JDWP-Handshake", []string{"JDWP"}, `JDWP`},
		{"Sniffer_NFS", "NFS", "nfs", 2049, "", []string{"nfs"}, `(?i)nfs`},
		{"Sniffer_Oracle", "Oracle", "oracle", 1521, "", []string{"oracle", "tns"}, `(?i)oracle|tns`},
		{"Sniffer_RMI", "RMI", "rmi", 1099, "JRMI\x00\x02K", []string{"JRMI"}, `JRMI`},
		{"Sniffer_T3", "T3", "t3", 7001, "t3 12.2.1\nAS:255\nHL:19\nMS:10000000\nPU:t3://localhost:7001\n\n", []string{"HELO"}, `(?i)HELO`},
		{"Sniffer_Zmtp", "ZeroMQ", "zmtp", 5555, "", []string{"zmtp"}, `(?i)zmtp`},
	}
	var out []Plugin
	for _, d := range defs {
		meta := Meta{Name: d.name, Type: TypeSniffer, AppName: d.app, VulName: d.app + " 协议识别", Category: "协议识别", Schemes: []string{d.scheme}, DefaultPorts: []int{d.port}}
		out = append(out, &SnifferPlugin{MetaData: meta, Probe: []byte(d.probe), Match: d.match, Regex: d.regex, Scheme: d.scheme, Timeout: 4 * time.Second})
	}
	return out
}
