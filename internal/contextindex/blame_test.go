package contextindex

import (
	"strings"
	"testing"
)

func TestCodeOwnersPatternFollowsGitHubSyntax(t *testing.T) {
	t.Run("TCP-V0-037", func(t *testing.T) {
		cases := []struct {
			pattern string
			match   []string
			miss    []string
		}{
			{"*", []string{"a.go", "x/y/z.md"}, nil},
			{"*.js", []string{"app.js", "web/src/app.js"}, []string{"app.jsx"}},
			{"/build/logs/", []string{"build/logs/a.txt", "build/logs/x/y"}, []string{"src/build/logs/a.txt", "build/logs"}},
			{"apps/", []string{"apps/a.go", "web/apps/b.go"}, []string{"apps"}},
			{"docs/*", []string{"docs/a.md"}, []string{"docs/sub/a.md", "x/docs/a.md"}},
			{"**/logs", []string{"logs/a", "deep/x/logs/b"}, []string{"logsx/a"}},
			{"/src/**/api.go", []string{"src/api.go", "src/a/b/api.go"}, []string{"lib/src/api.go"}},
			{"Makefile", []string{"Makefile", "tools/Makefile"}, []string{"Makefile.am"}},
		}
		for _, item := range cases {
			pattern := codeOwnersPattern(item.pattern)
			for _, candidate := range item.match {
				if !pattern.MatchString(candidate) {
					t.Errorf("%q must match %q", item.pattern, candidate)
				}
			}
			for _, candidate := range item.miss {
				if pattern.MatchString(candidate) {
					t.Errorf("%q must not match %q", item.pattern, candidate)
				}
			}
		}
		rules := parseCodeOwners("# comment\n\n*.go @go-team # inline\n!negated @x\n[ab].go @y\n/cmd/ owner@example.test\ncmd/tool.go\n")
		if len(rules) != 3 || rules[0].line != 3 || strings.Join(rules[0].owners, ",") != "@go-team" || len(rules[2].owners) != 0 {
			t.Fatalf("rules = %+v", rules)
		}
		if rule, _ := (&codeOwners{rules: rules}).owning("cmd/tool.go"); rule.line != 7 {
			t.Fatalf("the last matching rule must win, got line %d", rule.line)
		}
	})
}

func TestParseBlamePorcelainCountsLinesPerCommit(t *testing.T) {
	t.Run("TCP-V0-036", func(t *testing.T) {
		old := strings.Repeat("a", 40)
		recent := strings.Repeat("b", 40)
		raw := old + " 1 1 2\nauthor A\nauthor-mail <A@Example.test>\ncommitter-time 100\nboundary\nfilename f.go\n\tline one\n" +
			old + " 2 2\n\tline two\n" +
			recent + " 3 3 1\nauthor B\nauthor-mail <b@example.test>\ncommitter-time 1000\nfilename f.go\n\tline three\n"
		commits, lines := parseBlame([]byte(raw))
		if lines[old] != 2 || lines[recent] != 1 || !commits[old].boundary || commits[recent].boundary || commits[recent].when != 1000 {
			t.Fatalf("commits %+v lines %v", commits, lines)
		}
		touch := (&contextRecency{indexedAt: 1000}).touch(commits, lines)
		if touch.lines != 3 || touch.inWindow != 1 || touch.authors["b@example.test"] != 1 || touch.newestDays != 0 {
			t.Fatalf("touch = %+v", touch)
		}
		if touch.freshness < 0.33 || touch.freshness > 0.34 {
			t.Fatalf("freshness = %f, want one fresh line of three", touch.freshness)
		}
	})
}

// TestContextRecencyReportsCodeOwnersBlameDisagreement: an owner who wrote
// no in-window line is reported, never resolved; a team owner is
// unverifiable; a matching owner reports nothing.
func TestContextRecencyReportsCodeOwnersBlameDisagreement(t *testing.T) {
	t.Run("TCP-V0-037", func(t *testing.T) {
		root := recencyRepository(t)
		body := "package fixture\n\nfunc Flange() string { return \"flange\" }\n"
		for _, name := range []string{"email/a.go", "team/b.go", "agree/c.go"} {
			writeTestFile(t, root, name, body)
		}
		writeTestFile(t, root, ".github/CODEOWNERS", "* corvint@example.test\n/email/ someone@example.test\n/team/ @org/team @someone\n")
		recencyCommit(t, root, recencyOldDate, "start")
		for _, name := range []string{"email/a.go", "team/b.go", "agree/c.go"} {
			writeTestFile(t, root, name, body+"\n// flange tuned\n")
		}
		recencyCommit(t, root, recencyNewDate, "tune flanges")
		t.Setenv("CORVINT_CONTEXT_RECENCY", "on")
		packet := recencyPacket(t, root, "tune the flange", "")
		summary := packet["coverage"].(map[string]any)["recency"].(map[string]any)
		if summary["codeowners"] != ".github/CODEOWNERS" {
			t.Fatalf("coverage.recency = %v", summary)
		}
		states := map[string]string{}
		for _, entry := range mapsFromAny(summary["ownership"]) {
			states[entry["path"].(string)] = entry["state"].(string)
			if entry["blame_author"] != "corvint@example.test" || entry["reason"] == "" {
				t.Fatalf("ownership entry = %v", entry)
			}
		}
		if len(states) != 2 || states["email/a.go"] != "disagrees" || states["team/b.go"] != "unverifiable" {
			t.Fatalf("ownership states = %v, want email disagrees, team unverifiable, agree absent", states)
		}
		if reason := rowReason(t, packet, "email/a.go"); !strings.Contains(reason, "ownership disagrees: CODEOWNERS names someone@example.test, blame names corvint@example.test") {
			t.Fatalf("row reason %q must name the disagreement", reason)
		}
		if reason := rowReason(t, packet, "agree/c.go"); strings.Contains(reason, "ownership") {
			t.Fatalf("an agreeing owner must report nothing: %q", reason)
		}
	})
}
