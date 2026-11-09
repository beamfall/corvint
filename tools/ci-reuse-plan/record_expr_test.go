package main

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// actionsExpr evaluates the subset of the GitHub Actions expression syntax a
// job if: uses here: string literals, context references, cancelled(), !,
// ==, != (case-insensitive on strings), && and || (which return an operand,
// as Actions does), and parentheses. An unknown reference is ”.
type actionsExpr struct {
	tokens []string
	at     int
	ctx    map[string]string
}

var actionsToken = regexp.MustCompile(`\s*('(?:[^']|'')*'|&&|\|\||==|!=|!|\(|\)|[A-Za-z_][A-Za-z0-9_.\-]*)`)

func evalActionsIf(t *testing.T, expr string, ctx map[string]string) bool {
	t.Helper()
	expr = strings.TrimSpace(expr)
	expr = strings.TrimSuffix(strings.TrimPrefix(expr, "${{"), "}}")
	e := &actionsExpr{ctx: ctx}
	for rest := strings.TrimSpace(expr); rest != ""; rest = strings.TrimSpace(rest) {
		m := actionsToken.FindStringSubmatchIndex(rest)
		if m == nil || m[0] != 0 {
			t.Fatalf("expression: cannot read %q", rest)
		}
		e.tokens = append(e.tokens, rest[m[2]:m[3]])
		rest = rest[m[1]:]
	}
	v := e.or(t)
	if e.at != len(e.tokens) {
		t.Fatalf("expression: trailing %q", e.tokens[e.at:])
	}
	return v != ""
}

func (e *actionsExpr) peek() string {
	if e.at < len(e.tokens) {
		return e.tokens[e.at]
	}
	return ""
}

func (e *actionsExpr) next(t *testing.T) string {
	t.Helper()
	if e.at == len(e.tokens) {
		t.Fatal("expression: unexpected end")
	}
	e.at++
	return e.tokens[e.at-1]
}

// Values are strings; "" is falsy and "true" stands for a true boolean.
func (e *actionsExpr) or(t *testing.T) string {
	v := e.and(t)
	for e.peek() == "||" {
		e.next(t)
		r := e.and(t)
		if v == "" {
			v = r
		}
	}
	return v
}

func (e *actionsExpr) and(t *testing.T) string {
	v := e.unary(t)
	for e.peek() == "&&" {
		e.next(t)
		r := e.unary(t)
		if v != "" {
			v = r
		}
	}
	return v
}

func (e *actionsExpr) unary(t *testing.T) string {
	if e.peek() == "!" {
		e.next(t)
		return boolValue(e.unary(t) == "")
	}
	v := e.primary(t)
	if op := e.peek(); op == "==" || op == "!=" {
		e.next(t)
		r := e.primary(t)
		return boolValue(strings.EqualFold(v, r) == (op == "=="))
	}
	return v
}

func (e *actionsExpr) primary(t *testing.T) string {
	switch tok := e.next(t); {
	case tok == "(":
		v := e.or(t)
		if e.next(t) != ")" {
			t.Fatal("expression: unbalanced parentheses")
		}
		return v
	case strings.HasPrefix(tok, "'"):
		return strings.ReplaceAll(tok[1:len(tok)-1], "''", "'")
	case e.peek() == "(":
		e.next(t)
		if e.next(t) != ")" {
			t.Fatalf("expression: %s takes no arguments here", tok)
		}
		v, ok := e.ctx[tok+"()"]
		if !ok {
			t.Fatalf("expression: unknown function %s()", tok)
		}
		return v
	default:
		return e.ctx[tok]
	}
}

func boolValue(b bool) string {
	if b {
		return "true"
	}
	return ""
}

func TestActionsExprEvaluator(t *testing.T) {
	ctx := map[string]string{"a": "x", "cancelled()": ""}
	for expr, want := range map[string]bool{
		"${{ a == 'X' }}": true, "a != 'x'": false, "!cancelled() && a": true, "'' || a == 'y'": false,
		"(a == 'y' || a == 'x') && !(b == 'z')": true, "b": false, "b == ''": true,
	} {
		if got := evalActionsIf(t, expr, ctx); got != want {
			t.Errorf("%s = %v, want %v", expr, got, want)
		}
	}
}

// AFP-V0-170, GLS-V0-042: the line-level step narrows a shard under any
// narrowing verdict (CORVINT_LINECOV_NARROWING is independent of the
// AFP-V0-098 verdict), so the record job needs every shard's FULL output
// whatever the verdict says. Each case leaves one shard's output empty, as a
// line-narrowed shard does; none may record a tested tree.
func TestAFPV0170RecordNeedsEveryShardFullUnderAnyVerdict(t *testing.T) {
	record := jobBlock(t, readWorkflow(t, "ci.yml"), "ci-tested-tree-record")
	m := regexp.MustCompile(`(?m)^    if: (.*)$`).FindStringSubmatch(record)
	if m == nil {
		t.Fatal("ci-tested-tree-record has no if:")
	}
	const shards = 16
	context := func(verdict string, empty int) map[string]string {
		ctx := map[string]string{
			"cancelled()": "", "github.event_name": "pull_request",
			"needs.go-product-shard.result": "success", "needs.docs-plan.result": "success",
			"needs.docs-plan.outputs.mode": "FULL", "needs.narrowing-verdict.outputs.verdict": verdict,
		}
		for i := 0; i < shards; i++ {
			if i != empty {
				ctx[fmt.Sprintf("needs.go-product-shard.outputs.full%d", i)] = "FULL"
			}
		}
		return ctx
	}
	for _, verdict := range []string{"MET", "NOT_MET", "UNKNOWN", ""} {
		if !evalActionsIf(t, m[1], context(verdict, -1)) {
			t.Errorf("verdict %q with every shard FULL records no tree", verdict)
		}
		for empty := 0; empty < shards; empty++ {
			if evalActionsIf(t, m[1], context(verdict, empty)) {
				t.Errorf("verdict %q records a tree although shard %d is not FULL", verdict, empty)
			}
		}
	}
}
