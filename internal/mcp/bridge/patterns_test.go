package bridge

import (
	"regexp"
	"strings"
	"testing"
)

// pathRuleHolds is the rule spelled as the lookahead patterns it replaced
// read: no leading slash, `//`, backslash, `.` or `..` segment, refused
// `.git` segment or refused character, and the trailing-slash and suffix
// options.
func pathRuleHolds(rule PathRule, refused func(rune) bool, path string) bool {
	if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "//") || strings.Contains(path, `\`) || strings.ContainsFunc(path, refused) {
		return false
	}
	if !rule.TrailingSlash && strings.HasSuffix(path, "/") {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "." || segment == ".." || (rule.GitSegment == "lower" && segment == ".git") || (rule.GitSegment == "fold" && strings.EqualFold(segment, ".git")) {
			return false
		}
	}
	if len(rule.Suffixes) == 0 {
		return true
	}
	for _, suffix := range rule.Suffixes {
		if strings.HasSuffix(path, "."+suffix) && len(path) > len(suffix)+1 {
			return true
		}
	}
	return false
}

// MCPV0-954: every advertised path rule compiles under RE2 (Go's regexp) and,
// over every string of up to five characters from an adversarial alphabet,
// accepts exactly what the replaced lookahead patterns accepted.
func TestMCPV0954PathPatternsAreRE2AndEquivalent(t *testing.T) {
	control := func(r rune) bool { return r <= 0x1f || r == 0x7f }
	for name, test := range map[string]struct {
		rule    PathRule
		refused func(rune) bool
	}{
		"relative (cem map, artifacts)": {PathRule{Excluded: `\x00-\x1f\x7f`, GitSegment: "fold", TrailingSlash: true}, control},
		"flows":                         {PathRule{Excluded: `\x00-\x1f\x7f-\x9f`, GitSegment: "fold"}, func(r rune) bool { return r <= 0x1f || (r >= 0x7f && r <= 0x9f) }},
		"subject and docs path":         {SubjectPathRule, func(r rune) bool { return r == '\n' || r == '\r' }},
		"impact path":                   {impactPathRule, func(r rune) bool { return r == '\n' || r == '\r' }},
		"test-validity receipt":         {PathRule{Excluded: `\n\r`, GitSegment: "lower", TrailingSlash: true}, func(r rune) bool { return r == '\n' || r == '\r' }},
	} {
		compiled, err := regexp.Compile(test.rule.Pattern())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		alphabet := []rune{'a', '.', '/', '\\', 'g', 'G', 'i', 'I', 't', 'T', 'o', '\n', 0x01, 0x85}
		var walk func(prefix []rune)
		walk = func(prefix []rune) {
			for _, extra := range []string{"", ".go", ".tsx"} {
				path := string(prefix) + extra
				if got, want := compiled.MatchString(path), pathRuleHolds(test.rule, test.refused, path); got != want {
					t.Fatalf("%s: %q matched=%v, rule=%v (pattern %s)", name, path, got, want, test.rule.Pattern())
				}
			}
			if len(prefix) == 5 {
				return
			}
			for _, next := range alphabet {
				walk(append(prefix, next))
			}
		}
		walk(nil)
	}
}

// MCPV0-954: no advertised schema repeats $schema below its root, and every
// pattern compiles under RE2.
func TestMCPV0954SchemasHaveOneRootSchemaKeywordAndRE2Patterns(t *testing.T) {
	var visit func(name string, node any, root bool)
	visit = func(name string, node any, root bool) {
		switch typed := node.(type) {
		case map[string]any:
			if _, nested := typed["$schema"]; nested && !root {
				t.Fatalf("%s: nested $schema", name)
			}
			if pattern, ok := typed["pattern"].(string); ok {
				if _, err := regexp.Compile(pattern); err != nil {
					t.Fatalf("%s: pattern %q is not RE2: %v", name, pattern, err)
				}
			}
			for _, child := range typed {
				visit(name, child, false)
			}
		case []any:
			for _, child := range typed {
				visit(name, child, false)
			}
		}
	}
	for _, profile := range []Profile{{}, {TaskReview: true}, {LSP: true}, {Flows: true}, {ImportGraph: true}} {
		for _, tool := range profile.Tools() {
			visit(tool.Name, tool.InputSchema, true)
		}
	}
}
