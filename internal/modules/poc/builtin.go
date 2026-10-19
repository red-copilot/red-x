package poc

import (
	"path/filepath"

	"github.com/Aabyss-Team/ARL/internal/platform/netsafe"
)

func Builtin(client *HTTPClient, dictDir string) []Plugin {
	var out []Plugin
	out = append(out, BuiltinHTTPPlugins(client)...)
	out = append(out, BuiltinTCPNoAuth()...)
	out = append(out, BuiltinSniffers()...)
	out = append(out, BuiltinBruites(dictDir, client)...)
	return out
}

func DefaultDictDir(assetsDir string) string {
	return filepath.Join(assetsDir, "poc", "dicts")
}

func PolicyClient(policy netsafe.Policy, proxyURL string) *HTTPClient {
	return NewHTTPClient(policy, proxyURL, 0)
}
