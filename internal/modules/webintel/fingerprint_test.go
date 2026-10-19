package webintel

import (
	"path/filepath"
	"testing"
)

func TestFingerprintExpression(t *testing.T) {
	expr, err := ParseFingerprint(`body="hello" && (title="world" || header="server: nginx")`)
	if err != nil {
		t.Fatal(err)
	}
	v := FingerprintVariables{Body: "say hello", Title: "world", Header: "server: nginx"}
	if !expr.Eval(v) {
		t.Fatal("expression should match")
	}
	v.Title = "different"
	v.Header = "server: nginx"
	if !expr.Eval(v) {
		t.Fatal("header branch should match")
	}
	v.Header = "server: apache"
	if expr.Eval(v) {
		t.Fatal("expression should not match")
	}
}

func TestLoadBuiltinFingerprints(t *testing.T) {
	m := NewFingerprintMatcher()
	path := filepath.Join("..", "..", "..", "assets", "dicts", "webapp.json")
	if err := m.LoadBuiltin(path); err != nil {
		t.Fatal(err)
	}
	if m.Count() == 0 {
		t.Fatal("expected built-in fingerprints")
	}
}

func TestWIHConfig(t *testing.T) {
	cfg, err := LoadWIHConfig(filepath.Join("..", "..", "..", "assets", "rules", "wih_rules.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Rules) == 0 {
		t.Fatal("expected WIH rules")
	}
}
