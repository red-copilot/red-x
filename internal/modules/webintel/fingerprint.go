package webintel

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

type FingerprintVariables struct {
	Body     string
	Header   string
	Title    string
	IconHash string
}

type FingerprintMatcher struct {
	mu      sync.RWMutex
	builtin map[string]BuiltinRule
	dbRules []DBRule
}

type BuiltinRule struct {
	HTML        []string `json:"html"`
	Headers     []string `json:"headers"`
	Title       []string `json:"title"`
	FaviconHash []any    `json:"favicon_hash"`
}

type DBRule struct {
	Name string
	Rule string
}

func NewFingerprintMatcher() *FingerprintMatcher {
	return &FingerprintMatcher{builtin: map[string]BuiltinRule{}}
}

func (m *FingerprintMatcher) LoadBuiltin(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var rules map[string]BuiltinRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return err
	}
	m.mu.Lock()
	m.builtin = rules
	m.mu.Unlock()
	return nil
}

func (m *FingerprintMatcher) SetDBRules(rules []DBRule) {
	m.mu.Lock()
	m.dbRules = append([]DBRule(nil), rules...)
	m.mu.Unlock()
}

func (m *FingerprintMatcher) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.builtin) + len(m.dbRules)
}

func (m *FingerprintMatcher) Identify(v FingerprintVariables) []string {
	m.mu.RLock()
	builtin := m.builtin
	dbRules := append([]DBRule(nil), m.dbRules...)
	m.mu.RUnlock()
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	headerLower := strings.ToLower(v.Header)
	for name, rule := range builtin {
		matched := false
		for _, s := range rule.HTML {
			if s != "" && strings.Contains(v.Body, s) {
				matched = true
				break
			}
		}
		if !matched {
			for _, s := range rule.Headers {
				if s != "" && strings.Contains(headerLower, strings.ToLower(s)) {
					matched = true
					break
				}
			}
		}
		if !matched {
			for _, s := range rule.Title {
				if s != "" && strings.Contains(v.Title, s) {
					matched = true
					break
				}
			}
		}
		if !matched {
			for _, raw := range rule.FaviconHash {
				switch x := raw.(type) {
				case float64:
					if strconv.FormatInt(int64(x), 10) == v.IconHash {
						matched = true
					}
				case string:
					if x == v.IconHash {
						matched = true
					}
				}
			}
		}
		if matched {
			add(name)
		}
	}
	for _, rule := range dbRules {
		expr, err := ParseFingerprint(rule.Rule)
		if err != nil {
			continue
		}
		if expr.Eval(v) {
			add(rule.Name)
		}
	}
	return out
}

type Expr interface {
	Eval(FingerprintVariables) bool
}

type compareExpr struct {
	Var string
	Op  string
	Val string
}

func (e compareExpr) Eval(v FingerprintVariables) bool {
	var actual string
	switch strings.ToLower(e.Var) {
	case "body", "html":
		actual = v.Body
	case "header", "headers":
		actual = v.Header
	case "title":
		actual = v.Title
	case "icon_hash", "favicon_hash":
		actual = v.IconHash
	default:
		return false
	}
	switch e.Op {
	case "=":
		return strings.Contains(actual, e.Val)
	case "==":
		return actual == e.Val
	case "!=":
		return !strings.Contains(actual, e.Val)
	default:
		return false
	}
}

type binaryExpr struct {
	Op    string
	Left  Expr
	Right Expr
}

func (e binaryExpr) Eval(v FingerprintVariables) bool {
	if e.Op == "&&" {
		return e.Left.Eval(v) && e.Right.Eval(v)
	}
	return e.Left.Eval(v) || e.Right.Eval(v)
}

type notExpr struct{ Inner Expr }

func (e notExpr) Eval(v FingerprintVariables) bool { return !e.Inner.Eval(v) }

type token struct {
	kind string
	text string
}

func lexFingerprint(s string) ([]token, error) {
	var out []token
	for i := 0; i < len(s); {
		c := s[i]
		if unicode.IsSpace(rune(c)) {
			i++
			continue
		}
		switch c {
		case '(':
			out = append(out, token{"(", "("})
			i++
		case ')':
			out = append(out, token{")", ")"})
			i++
		case '!':
			if i+1 < len(s) && s[i+1] == '=' {
				out = append(out, token{"op", "!="})
				i += 2
			} else {
				out = append(out, token{"!", "!"})
				i++
			}
		case '=':
			if i+1 < len(s) && s[i+1] == '=' {
				out = append(out, token{"op", "=="})
				i += 2
			} else {
				out = append(out, token{"op", "="})
				i++
			}
		case '&':
			if i+1 < len(s) && s[i+1] == '&' {
				out = append(out, token{"op", "&&"})
				i += 2
			} else {
				return nil, fmt.Errorf("unexpected & at %d", i)
			}
		case '|':
			if i+1 < len(s) && s[i+1] == '|' {
				out = append(out, token{"op", "||"})
				i += 2
			} else {
				return nil, fmt.Errorf("unexpected | at %d", i)
			}
		case '"':
			j := i + 1
			var b strings.Builder
			for j < len(s) {
				if s[j] == '\\' && j+1 < len(s) {
					j++
					switch s[j] {
					case 'n':
						b.WriteByte('\n')
					case 't':
						b.WriteByte('\t')
					case 'r':
						b.WriteByte('\r')
					default:
						b.WriteByte(s[j])
					}
					j++
					continue
				}
				if s[j] == '"' {
					break
				}
				b.WriteByte(s[j])
				j++
			}
			if j >= len(s) {
				return nil, fmt.Errorf("unterminated string")
			}
			out = append(out, token{"string", b.String()})
			i = j + 1
		default:
			j := i
			for j < len(s) && (unicode.IsLetter(rune(s[j])) || unicode.IsDigit(rune(s[j])) || s[j] == '_') {
				j++
			}
			if j == i {
				return nil, fmt.Errorf("unexpected character %q at %d", c, i)
			}
			out = append(out, token{"ident", s[i:j]})
			i = j
		}
	}
	return out, nil
}

type fpParser struct {
	tokens []token
	pos    int
}

func (p *fpParser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{"eof", ""}
	}
	return p.tokens[p.pos]
}

func (p *fpParser) next() token {
	t := p.peek()
	p.pos++
	return t
}

func (p *fpParser) parseExpr() (Expr, error) { return p.parseOr() }

func (p *fpParser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().text == "||" {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = binaryExpr{Op: "||", Left: left, Right: right}
	}
	return left, nil
}

func (p *fpParser) parseAnd() (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.peek().text == "&&" {
		p.next()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = binaryExpr{Op: "&&", Left: left, Right: right}
	}
	return left, nil
}

func (p *fpParser) parseUnary() (Expr, error) {
	if p.peek().text == "!" {
		p.next()
		inner, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return notExpr{Inner: inner}, nil
	}
	if p.peek().text == "(" {
		p.next()
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.next().text != ")" {
			return nil, fmt.Errorf("missing )")
		}
		return inner, nil
	}
	return p.parseCompare()
}

func (p *fpParser) parseCompare() (Expr, error) {
	v := p.next()
	if v.kind != "ident" {
		return nil, fmt.Errorf("expected variable, got %q", v.text)
	}
	if p.peek().text == "!" && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].kind == "op" {
		// ! followed by an identifier is a unary expression; handled above.
	}
	op := p.next()
	if op.kind != "op" || (op.text != "=" && op.text != "==" && op.text != "!=") {
		return nil, fmt.Errorf("expected comparison operator after %s", v.text)
	}
	val := p.next()
	if val.kind != "string" && val.kind != "ident" {
		return nil, fmt.Errorf("expected value after %s", op.text)
	}
	return compareExpr{Var: v.text, Op: op.text, Val: val.text}, nil
}

func ParseFingerprint(s string) (Expr, error) {
	tokens, err := lexFingerprint(s)
	if err != nil {
		return nil, err
	}
	p := &fpParser{tokens: tokens}
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.peek().kind != "eof" {
		return nil, fmt.Errorf("unexpected token %q", p.peek().text)
	}
	return expr, nil
}
