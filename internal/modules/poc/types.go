package poc

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type PluginType string

const (
	TypePOC     PluginType = "poc"
	TypeSniffer PluginType = "sniffer"
	TypeBrute   PluginType = "brute"
)

type Target struct {
	Raw    string
	Scheme string
	Host   string
	Port   int
	URL    string
}

type Meta struct {
	Name         string
	Type         PluginType
	AppName      string
	VulName      string
	Category     string
	Schemes      []string
	DefaultPorts []int
}

type Result struct {
	Hit        bool
	Target     string
	VerifyData string
	Data       map[string]any
}

type Plugin interface {
	Meta() Meta
	Run(context.Context, Target) (Result, error)
}

var ErrUnsupported = errors.New("plugin is registered but not supported by the native implementation")

func ParseTarget(raw string, defaultScheme string, defaultPort int) (Target, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Target{}, fmt.Errorf("empty target")
	}
	if !strings.Contains(raw, "://") {
		if defaultScheme == "http" || defaultScheme == "https" {
			raw = defaultScheme + "://" + raw
		} else {
			raw = "tcp://" + raw
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Target{}, err
	}
	host := u.Hostname()
	port := defaultPort
	if p := u.Port(); p != "" {
		_, _ = fmt.Sscanf(p, "%d", &port)
	}
	scheme := u.Scheme
	if scheme == "tcp" {
		scheme = defaultScheme
	}
	return Target{Raw: raw, Scheme: scheme, Host: host, Port: port, URL: raw}, nil
}
