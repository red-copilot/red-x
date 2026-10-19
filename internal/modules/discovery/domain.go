package discovery

import (
	"strings"

	"github.com/red-copilot/red-x/internal/platform/netsafe"
	"golang.org/x/net/publicsuffix"
)

// IsRootDomain reports whether domain is an ICANN-managed registrable domain
// or multi-label public suffix. Only public roots are eligible for automatic
// passive enumeration; subdomains and unmanaged/private suffixes stay on-host.
func IsRootDomain(domain string) (bool, error) {
	normalized, err := netsafe.ValidateDomain(domain)
	if err != nil {
		return false, err
	}
	suffix, icann := publicsuffix.PublicSuffix(normalized)
	if !icann {
		return false, nil
	}
	if normalized == suffix {
		return strings.Contains(normalized, "."), nil
	}
	root, err := publicsuffix.EffectiveTLDPlusOne(normalized)
	if err != nil {
		return false, err
	}
	return normalized == root, nil
}
