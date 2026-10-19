package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/red-copilot/red-x/internal/platform/config"
)

type Notifier struct {
	Config config.NotificationConfig
	Client *http.Client
	Log    *slog.Logger
}

func New(cfg config.NotificationConfig, log *slog.Logger) *Notifier {
	return &Notifier{Config: cfg, Client: &http.Client{Timeout: 15 * time.Second}, Log: log}
}

func (n *Notifier) Send(ctx context.Context, title, markdown, html string) error {
	var first error
	try := func(name string, fn func() error) {
		if err := fn(); err != nil {
			n.Log.Warn("notification failed", "channel", name, "error", err)
			if first == nil {
				first = err
			}
		}
	}
	if n.Config.DingTalk.AccessToken != "" && n.Config.DingTalk.Secret != "" {
		try("dingtalk", func() error { return n.dingtalk(ctx, title, markdown) })
	}
	if n.Config.Feishu.WebhookURL != "" {
		try("feishu", func() error { return n.feishu(ctx, title, markdown) })
	}
	if n.Config.WeCom.WebhookURL != "" {
		try("wecom", func() error { return n.wecom(ctx, title, markdown) })
	}
	if n.Config.SMTP.Host != "" && n.Config.SMTP.To != "" {
		try("email", func() error { return n.email(title, html) })
	}
	if n.Config.Webhook.URL != "" {
		try("webhook", func() error {
			return n.postJSON(ctx, n.Config.Webhook.URL, map[string]any{"title": title, "markdown": markdown}, map[string]string{"Token": n.Config.Webhook.Token})
		})
	}
	return first
}

func (n *Notifier) dingtalk(ctx context.Context, title, markdown string) error {
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	stringToSign := ts + "\n" + n.Config.DingTalk.Secret
	mac := hmac.New(sha256.New, []byte(n.Config.DingTalk.Secret))
	_, _ = mac.Write([]byte(stringToSign))
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	u := "https://oapi.dingtalk.com/robot/send?access_token=" + url.QueryEscape(n.Config.DingTalk.AccessToken) + "&timestamp=" + ts + "&sign=" + sign
	return n.postJSON(ctx, u, map[string]any{
		"msgtype": "markdown", "markdown": map[string]string{"title": title, "text": markdown},
	}, nil)
}

func (n *Notifier) feishu(ctx context.Context, title, markdown string) error {
	body := map[string]any{"msg_type": "text", "content": map[string]string{"text": title + "\n" + markdown}}
	if n.Config.Feishu.Secret != "" {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		mac := hmac.New(sha256.New, []byte(ts+"\n"+n.Config.Feishu.Secret))
		_, _ = mac.Write(nil)
		body["timestamp"] = ts
		body["sign"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	}
	return n.postJSON(ctx, n.Config.Feishu.WebhookURL, body, nil)
}

func (n *Notifier) wecom(ctx context.Context, title, markdown string) error {
	return n.postJSON(ctx, n.Config.WeCom.WebhookURL, map[string]any{
		"msgtype": "markdown", "markdown": map[string]string{"content": title + "\n" + markdown},
	}, nil)
}

func (n *Notifier) postJSON(ctx context.Context, target string, body any, headers map[string]string) error {
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := n.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http status %d", resp.StatusCode)
	}
	return nil
}

func (n *Notifier) email(title, html string) error {
	cfg := n.Config.SMTP
	if cfg.Port == 0 {
		cfg.Port = 465
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	headers := map[string]string{
		"From":         cfg.Username,
		"To":           cfg.To,
		"Subject":      "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(title)) + "?=",
		"MIME-Version": "1.0",
		"Content-Type": "text/html; charset=UTF-8",
	}
	var msg strings.Builder
	for k, v := range headers {
		fmt.Fprintf(&msg, "%s: %s\r\n", k, v)
	}
	msg.WriteString("\r\n")
	msg.WriteString(html)
	if cfg.Port == 465 {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12})
		if err != nil {
			return err
		}
		defer conn.Close()
		client, err := smtp.NewClient(conn, cfg.Host)
		if err != nil {
			return err
		}
		defer client.Close()
		if cfg.Username != "" {
			if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
				return err
			}
		}
		if err := client.Mail(cfg.Username); err != nil {
			return err
		}
		if err := client.Rcpt(cfg.To); err != nil {
			return err
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		_, _ = w.Write([]byte(msg.String()))
		return w.Close()
	}
	return smtp.SendMail(addr, smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host), cfg.Username, []string{cfg.To}, []byte(msg.String()))
}
