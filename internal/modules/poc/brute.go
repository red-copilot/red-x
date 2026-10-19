package poc

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"golang.org/x/crypto/ssh"
)

type Dict struct {
	Users     []string
	Passwords []string
}

func LoadDict(dir, userFile, passFile string) (Dict, error) {
	users, err := readLines(filepath.Join(dir, userFile))
	if err != nil {
		return Dict{}, err
	}
	passwords, err := readLines(filepath.Join(dir, passFile))
	if err != nil {
		return Dict{}, err
	}
	return Dict{Users: users, Passwords: passwords}, nil
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		s := strings.TrimSpace(sc.Text())
		if s != "" && !strings.HasPrefix(s, "#") {
			out = append(out, strings.ReplaceAll(s, "%user%", "admin"))
		}
	}
	return out, sc.Err()
}

type BrutePlugin struct {
	MetaData Meta
	Dict     Dict
	Login    func(context.Context, Target, string, string) (bool, error)
	Timeout  time.Duration
}

func (p *BrutePlugin) Meta() Meta { return p.MetaData }

func (p *BrutePlugin) Run(ctx context.Context, target Target) (Result, error) {
	if p.Login == nil {
		return Result{}, ErrUnsupported
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	for _, user := range p.Dict.Users {
		for _, pass := range p.Dict.Passwords {
			select {
			case <-ctx.Done():
				return Result{}, ctx.Err()
			default:
			}
			attemptCtx, cancel := context.WithTimeout(ctx, timeout)
			ok, err := p.Login(attemptCtx, target, user, pass)
			cancel()
			if err == nil && ok {
				return Result{Hit: true, Target: target.URL, VerifyData: target.URL, Data: map[string]any{
					"username": user, "password": pass, "target": target.URL,
				}}, nil
			}
		}
	}
	return Result{}, nil
}

func ftpLogin(ctx context.Context, target Target, user, pass string) (bool, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port)))
	if err != nil {
		return false, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(6 * time.Second))
	r := bufio.NewReader(conn)
	if _, err := r.ReadString('\n'); err != nil {
		return false, err
	}
	fmt.Fprintf(conn, "USER %s\r\n", user)
	if _, err := r.ReadString('\n'); err != nil {
		return false, err
	}
	fmt.Fprintf(conn, "PASS %s\r\n", pass)
	line, err := r.ReadString('\n')
	if err != nil {
		return false, err
	}
	return strings.HasPrefix(line, "230"), nil
}

func redisLogin(ctx context.Context, target Target, user, pass string) (bool, error) {
	d := net.Dialer{Timeout: 4 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port)))
	if err != nil {
		return false, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	cmd := fmt.Sprintf("AUTH %s\r\n", pass)
	if user != "" && user != "default" {
		cmd = fmt.Sprintf("AUTH %s %s\r\n", user, pass)
	}
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return false, err
	}
	buf := make([]byte, 128)
	n, _ := conn.Read(buf)
	return strings.HasPrefix(string(buf[:n]), "+OK"), nil
}

func sshLogin(ctx context.Context, target Target, user, pass string) (bool, error) {
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port)))
	if err != nil {
		return false, err
	}
	defer conn.Close()
	c, chans, reqs, err := ssh.NewClientConn(conn, target.Host, cfg)
	if err != nil {
		return false, nil
	}
	client := ssh.NewClient(c, chans, reqs)
	defer client.Close()
	return true, nil
}

func smtpLogin(ctx context.Context, target Target, user, pass string) (bool, error) {
	addr := net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return false, err
	}
	client, err := smtp.NewClient(conn, target.Host)
	if err != nil {
		conn.Close()
		return false, nil
	}
	defer client.Close()
	if ok, _ := client.Extension("AUTH"); !ok {
		return false, nil
	}
	if err := client.Auth(smtp.PlainAuth("", user, pass, target.Host)); err != nil {
		return false, nil
	}
	return true, nil
}

func mysqlLogin(ctx context.Context, target Target, user, pass string) (bool, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=4s&readTimeout=4s", user, pass, target.Host, target.Port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return false, err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return false, nil
	}
	return true, nil
}

func postgresLogin(ctx context.Context, target Target, user, pass string) (bool, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/postgres?sslmode=disable&connect_timeout=4", user, pass, target.Host, target.Port)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return false, err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return false, nil
	}
	return true, nil
}

func mssqlLogin(ctx context.Context, target Target, user, pass string) (bool, error) {
	dsn := fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=master&encrypt=disable&connection+timeout=4", user, pass, target.Host, target.Port)
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return false, err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return false, nil
	}
	return true, nil
}

func mongoLogin(ctx context.Context, target Target, user, pass string) (bool, error) {
	uri := fmt.Sprintf("mongodb://%s:%s@%s:%d/?authSource=admin", urlQuery(user), urlQuery(pass), target.Host, target.Port)
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetServerSelectionTimeout(4 * time.Second))
	if err != nil {
		return false, err
	}
	defer client.Disconnect(ctx)
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return false, nil
	}
	return true, nil
}

func urlQuery(s string) string {
	r := strings.NewReplacer("@", "%40", ":", "%3A", "/", "%2F", "?", "%3F", "#", "%23")
	return r.Replace(s)
}

func imapLogin(ctx context.Context, target Target, user, pass string) (bool, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port)))
	if err != nil {
		return false, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(6 * time.Second))
	r := bufio.NewReader(conn)
	if _, err := r.ReadString('\n'); err != nil {
		return false, err
	}
	fmt.Fprintf(conn, "a1 LOGIN %q %q\r\n", user, pass)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return false, nil
		}
		if strings.HasPrefix(line, "a1 OK") {
			return true, nil
		}
		if strings.HasPrefix(line, "a1 NO") || strings.HasPrefix(line, "a1 BAD") {
			return false, nil
		}
	}
}

func pop3Login(ctx context.Context, target Target, user, pass string) (bool, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port)))
	if err != nil {
		return false, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(6 * time.Second))
	r := bufio.NewReader(conn)
	if _, err := r.ReadString('\n'); err != nil {
		return false, err
	}
	fmt.Fprintf(conn, "USER %s\r\n", user)
	if line, err := r.ReadString('\n'); err != nil || !strings.HasPrefix(line, "+OK") {
		return false, nil
	}
	fmt.Fprintf(conn, "PASS %s\r\n", pass)
	line, err := r.ReadString('\n')
	if err != nil {
		return false, nil
	}
	return strings.HasPrefix(line, "+OK"), nil
}

type HTTPLogin struct {
	Path        string
	Method      string
	BodyFormat  string
	SuccessCode []int
	SuccessBody []string
	Headers     map[string]string
}

func (h HTTPLogin) Login(ctx context.Context, client *HTTPClient, target Target, user, pass string) (bool, error) {
	method := h.Method
	if method == "" {
		method = http.MethodPost
	}
	body := strings.ReplaceAll(h.BodyFormat, "{{user}}", user)
	body = strings.ReplaceAll(body, "{{pass}}", pass)
	u := JoinURL(target.URL, h.Path)
	resp, err := client.Do(ctx, method, u, body, h.Headers)
	if err != nil {
		return false, err
	}
	if len(h.SuccessCode) > 0 && containsInt(h.SuccessCode, resp.Status) {
		return true, nil
	}
	if len(h.SuccessBody) > 0 && containsAny(resp.Body, h.SuccessBody) {
		return true, nil
	}
	return false, nil
}

func BuiltinBruites(dictDir string, client *HTTPClient) []Plugin {
	type def struct {
		name, app, scheme string
		port              int
		user, pass        string
		login             func(context.Context, Target, string, string) (bool, error)
	}
	httpLogin := func(path, body string, codes []int, needles []string) func(context.Context, Target, string, string) (bool, error) {
		h := HTTPLogin{Path: path, BodyFormat: body, SuccessCode: codes, SuccessBody: needles}
		return func(ctx context.Context, t Target, u, p string) (bool, error) {
			return h.Login(ctx, client, t, u, p)
		}
	}
	defs := []def{
		{"FTPBrute", "FTP", "ftp", 21, "username_ftp.txt", "password_ftp.txt", ftpLogin},
		{"RedisBrute", "Redis", "redis", 6379, "username_redis.txt", "password_redis.txt", redisLogin},
		{"SSHBrute", "SSH", "ssh", 22, "username_ssh.txt", "password_ssh.txt", sshLogin},
		{"SMTPBrute", "SMTP", "smtp", 25, "username_smtp.txt", "password_smtp.txt", smtpLogin},
		{"MysqlBrute", "MySQL", "mysql", 3306, "username_mysql.txt", "password_mysql.txt", mysqlLogin},
		{"JenkinsBrute", "Jenkins", "http", 8080, "username_jenkins.txt", "password_jenkins.txt", nil},
		{"GitlabBrute", "GitLab", "http", 80, "username_gitlab.txt", "password_gitlab.txt", nil},
		{"GrafanaBrute", "Grafana", "http", 3000, "username_grafana.txt", "password_grafana.txt", nil},
		{"HarborBrute", "Harbor", "http", 80, "username_harbor.txt", "password_harbor.txt", nil},
		{"NacosBrute", "Nacos", "http", 8848, "username_nacos.txt", "password_nacos.txt", nil},
		{"NexusBrute", "Nexus", "http", 8081, "username_nexus.txt", "password_nexus.txt", nil},
		{"TomcatBrute", "Tomcat", "http", 8080, "username_tomcat.txt", "password_tomcat.txt", nil},
		{"AlibabaDruidBrute", "Druid", "http", 8080, "username_alibaba-druid.txt", "password_alibaba-druid.txt", nil},
		{"ActiveMQBrute", "ActiveMQ", "http", 8161, "username_activemq.txt", "password_activemq.txt", nil},
		{"APISIXBrute", "APISIX", "http", 9080, "username_apisix.txt", "password_apisix.txt", nil},
		{"ClickhouseBrute", "ClickHouse", "http", 8123, "username_clickhouse.txt", "password_clickhouse.txt", nil},
		{"ExchangeBrute", "Exchange", "http", 443, "username_exchange.txt", "password_exchange.txt", nil},
		{"OpenfireBrute", "Openfire", "http", 9090, "username_openfire.txt", "password_openfire.txt", nil},
		{"ManageIQBrute", "ManageIQ", "http", 443, "username_manageiq.txt", "password_manageiq.txt", nil},
		{"CobaltStrikeBrute", "CobaltStrike", "csts", 50050, "username_csts.txt", "password_csts.txt", nil},
		{"Shiro_CBC_Brute", "Shiro", "http", 8080, "username_shiro.txt", "password_shiro.txt", nil},
		{"Shiro_GCM_Brute", "Shiro", "http", 8080, "username_shiro.txt", "password_shiro.txt", nil},
		{"IMAPBrute", "IMAP", "imap", 143, "username_imap.txt", "password_imap.txt", imapLogin},
		{"POP3Brute", "POP3", "pop3", 110, "username_pop3.txt", "password_pop3.txt", pop3Login},
		{"MongoDBBrute", "MongoDB", "mongodb", 27017, "username_mongodb.txt", "password_mongodb.txt", mongoLogin},
		{"PostgreSQLBrute", "PostgreSQL", "psql", 5432, "username_postgresql.txt", "password_postgresql.txt", postgresLogin},
		{"RDPBrute", "RDP", "rdp", 3389, "username_rdp.txt", "password_rdp.txt", nil},
		{"SQLServerBrute", "MSSQL", "mssql", 1433, "username_sqlserver.txt", "password_sqlserver.txt", mssqlLogin},
	}
	var out []Plugin
	for _, d := range defs {
		login := d.login
		if login == nil {
			switch d.name {
			case "JenkinsBrute":
				login = httpLogin("/j_spring_security_check", "j_username={{user}}&j_password={{pass}}", []int{302}, nil)
			case "GitlabBrute":
				login = httpLogin("/users/sign_in", "user[login]={{user}}&user[password]={{pass}}", []int{302}, nil)
			case "GrafanaBrute":
				login = httpLogin("/login", "user={{user}}&password={{pass}}", []int{200}, []string{`"message":"Logged in"`})
			case "HarborBrute":
				login = httpLogin("/c/login", "principal={{user}}&password={{pass}}", []int{302}, nil)
			case "NacosBrute":
				login = httpLogin("/nacos/v1/auth/login", "username={{user}}&password={{pass}}", []int{200}, []string{"accessToken"})
			case "NexusBrute":
				login = httpLogin("/service/rapture/session", "username={{user}}&password={{pass}}", []int{204}, nil)
			case "TomcatBrute":
				login = httpLogin("/manager/html", "", []int{200}, []string{"Tomcat Web Application Manager"})
			case "AlibabaDruidBrute":
				login = httpLogin("/druid/submitLogin", "loginUsername={{user}}&loginPassword={{pass}}", []int{200}, []string{`"Result":"success"`})
			case "ActiveMQBrute":
				login = httpLogin("/admin/index.jsp", "", []int{200}, []string{"ActiveMQ"})
			case "APISIXBrute":
				login = httpLogin("/apisix/admin/routes", "", []int{200}, []string{"total"})
			case "ClickhouseBrute":
				login = httpLogin("/?user={{user}}&password={{pass}}", "", []int{200}, []string{"Ok."})
			case "ExchangeBrute":
				login = httpLogin("/owa/auth.owa", "username={{user}}&password={{pass}}", []int{302}, nil)
			case "OpenfireBrute":
				login = httpLogin("/login.jsp", "username={{user}}&password={{pass}}", []int{302}, nil)
			case "ManageIQBrute":
				login = httpLogin("/dashboard/authenticate", "user_name={{user}}&user_password={{pass}}", []int{302}, nil)
			case "Shiro_CBC_Brute", "Shiro_GCM_Brute":
				login = httpLogin("/login.jsp", "username={{user}}&password={{pass}}", []int{302}, nil)
			default:
				login = func(context.Context, Target, string, string) (bool, error) { return false, ErrUnsupported }
			}
		}
		dict := Dict{}
		if d.user != "" {
			dict, _ = LoadDict(dictDir, d.user, d.pass)
		}
		meta := Meta{Name: d.name, Type: TypeBrute, AppName: d.app, VulName: d.app + " 弱口令", Category: "弱口令", Schemes: []string{d.scheme}, DefaultPorts: []int{d.port}}
		out = append(out, &BrutePlugin{MetaData: meta, Dict: dict, Login: login, Timeout: 5 * time.Second})
	}
	return out
}
