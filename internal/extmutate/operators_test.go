package extmutate

import (
	"fmt"
	"reflect"
	"testing"
)

// CCE-V0-170: only Ruby and TS/JS application files are candidates; tests, vendored and
// generated trees, and declaration files are not.
func TestCCEV0170ApplicationLanguage(t *testing.T) {
	for name, want := range map[string]Language{
		"app/models/user.rb": Ruby, "lib/tasks/sync.rake": Ruby, "src/price.ts": JavaScript, "src/view.tsx": JavaScript,
		"web/app.mjs": JavaScript, "server.cjs": JavaScript, "src/util.js": JavaScript,
		"spec/models/user_spec.rb": "", "test/user_test.rb": "", "app/user_test.rb": "", "src/price.test.ts": "", "src/price.spec.ts": "",
		"e2e/login.ts": "", "src/__tests__/a.ts": "", "node_modules/x/index.js": "", "vendor/gem/a.rb": "", "dist/app.js": "",
		"types/global.d.ts": "", "README.md": "", "app/main.go": "", "features/step.rb": "",
	} {
		got, ok := ApplicationLanguage(name)
		if got != want || ok != (want != "") {
			t.Errorf("%s: got %q %v, want %q", name, got, ok, want)
		}
	}
}

func sitesOf(language Language, lines ...string) []string {
	var out []string
	block := false
	for i, line := range lines {
		for _, s := range PlanLine(language, i+1, line, &block) {
			out = append(out, fmt.Sprintf("%s@%02d:%02d", s.Operator, s.Line, s.Column))
		}
	}
	return out
}

// CCE-V0-171: the closed operator set matches only code tokens.
func TestCCEV0171OperatorSites(t *testing.T) {
	for _, c := range []struct {
		name     string
		language Language
		lines    []string
		want     []string
	}{
		{"ruby comparisons", Ruby, []string{"age >= 18 && admin == true"}, []string{"ge-boundary@01:05", "and-or@01:11", "eq-negate@01:20", "true-false@01:23"}},
		{"ruby boundaries", Ruby, []string{"a < b || c > d", "x <= y", "z != w"}, []string{"lt-boundary@01:03", "or-and@01:07", "gt-boundary@01:12", "le-boundary@02:03", "ne-negate@03:03"}},
		{"ruby strings and comments", Ruby, []string{`puts "a == b" # x == y`, `name = 'true'`}, nil},
		{"ruby non-operators", Ruby, []string{"x ||= 1", "items << item", "a <=> b", "h = { a => 1 }", "case x when String === y", "x =~ /a/"}, nil},
		{"ruby keywords", Ruby, []string{"if ready", "unless done", "  if nested", "return x if y", "elsif z", "obj.true", ":false", "true?(x)", "opts = { true: 1 }", "@true = 1"}, []string{"if-unless@01:01", "unless-if@02:01", "if-unless@03:03"}},
		{"ts strict", JavaScript, []string{"const free = (p: number): boolean => p === 0 && true;"}, []string{"strict-eq-negate@01:40", "and-or@01:46", "true-false@01:49"}},
		{"ts negation", JavaScript, []string{"if (a !== b || c != d) return false;"}, []string{"strict-ne-negate@01:07", "or-and@01:13", "ne-negate@01:18", "false-true@01:31"}},
		{"ts generics arrows shifts", JavaScript, []string{"const xs: Array<string> = [];", "const f = (x) => x >> 1;", "let a = b ?? c;", "x ||= y; z &&= w;"}, nil},
		{"ts comments and templates", JavaScript, []string{"// a === b", "const s = `x === ${y}`;", "/* a < b", " c > d */ e > f"}, []string{"gt-boundary@04:13"}},
		{"ts keywords are words", JavaScript, []string{"const trueish = isTrue; x.true; const o = { false: 1 };"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := sitesOf(c.language, c.lines...); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
	if got := Operators(Ruby); len(got) != 12 || got[0] != "eq-negate" {
		t.Fatalf("ruby operators %v", got)
	}
	if got := Operators(JavaScript); len(got) != 12 || got[0] != "strict-eq-negate" {
		t.Fatalf("ts-js operators %v", got)
	}
}
