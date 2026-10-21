package tasking

import (
	"errors"
	"net/netip"
	"strings"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/discovery"
)

var ErrTargetOutsideScope = errors.New("target outside authorized scope")

func scopeAllowsTarget(scope *domain.Scope, target discovery.Target) bool {
	if scope == nil || len(scope.Targets) == 0 {
		return false
	}
	if scopeTargetMatches(scope.Type, scope.BlackScope, target) {
		return false
	}
	return scopeTargetMatches(scope.Type, scope.Targets, target)
}

func scopeTargetMatches(scopeType string, rules []string, target discovery.Target) bool {
	switch strings.ToLower(strings.TrimSpace(scopeType)) {
	case "domain":
		host := normalizeDomainRule(target.Host)
		if host == "" {
			return false
		}
		for _, rule := range rules {
			if domainRuleMatches(host, rule) {
				return true
			}
		}
	case "ip":
		addr, prefix, ok := targetNetwork(target)
		if !ok {
			return false
		}
		for _, rule := range rules {
			if networkRuleMatches(rule, addr, prefix) {
				return true
			}
		}
	}
	return false
}

func domainForbidden(host string, rules []string) bool {
	host = normalizeDomainRule(host)
	if host == "" {
		return false
	}
	for _, rule := range rules {
		if domainRuleMatches(host, rule) {
			return true
		}
	}
	return false
}

func domainRuleMatches(host, rule string) bool {
	host = normalizeDomainRule(host)
	rule = normalizeDomainRule(rule)
	if host == "" || rule == "" {
		return false
	}
	return host == rule || strings.HasSuffix(host, "."+rule)
}

func normalizeDomainRule(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, ".")
	value = strings.TrimPrefix(value, "*.")
	return value
}

func targetNetwork(target discovery.Target) (netip.Addr, netip.Prefix, bool) {
	switch target.Kind {
	case "cidr":
		if target.Prefix.IsValid() {
			return target.Prefix.Addr(), target.Prefix, true
		}
	case "ip", "url":
		if addr, err := netip.ParseAddr(target.Host); err == nil {
			return addr.Unmap(), netip.PrefixFrom(addr.Unmap(), addr.Unmap().BitLen()), true
		}
	}
	return netip.Addr{}, netip.Prefix{}, false
}

func networkRuleMatches(rule string, addr netip.Addr, target netip.Prefix) bool {
	rule = strings.TrimSpace(rule)
	if rule == "" || !addr.IsValid() {
		return false
	}
	if prefix, err := netip.ParsePrefix(rule); err == nil {
		addr = addr.Unmap()
		prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits())
		if addr.Is6() != prefix.Addr().Is6() {
			return false
		}
		return prefix.Bits() <= target.Bits() && prefix.Contains(addr)
	}
	ruleAddr, err := netip.ParseAddr(rule)
	if err != nil {
		return false
	}
	return addr.Unmap() == ruleAddr.Unmap()
}
