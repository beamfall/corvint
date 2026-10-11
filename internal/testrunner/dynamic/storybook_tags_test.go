package dynamic

import (
	"slices"
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// TestStorybookSkipTagsFixedArgv binds TRE-V0-044: Target carries the exact
// native skip-tag set as one final argv element after --skipTags; an absent
// Target keeps the historical argv; anything outside the closed tag grammar
// refuses before launch instead of being injected or silently dropped.
func TestStorybookSkipTagsFixedArgv(t *testing.T) {
	base := tr.Request{Runner: "storybook-test-runner", Executable: "/trusted/test-storybook", ReportDir: "/report"}
	v, e := Build(base)
	if e != nil || !slices.Equal(v.Argv, []string{"--json", "--outputFile", "/report/storybook.json"}) {
		t.Fatalf("historical argv changed: %v %v", v.Argv, e)
	}
	for target, want := range map[string][]string{
		"skip-test":     {"--json", "--outputFile", "/report/storybook.json", "--skipTags", "skip-test"},
		"skip-test,a_1": {"--json", "--outputFile", "/report/storybook.json", "--skipTags", "skip-test,a_1"},
	} {
		r := base
		r.Target = target
		if v, e = Build(r); e != nil || !slices.Equal(v.Argv, want) {
			t.Fatalf("%q: %v %v", target, v.Argv, e)
		}
	}
	r := base
	r.Project, r.Target = "http://127.0.0.1:6006", "skip-test"
	if v, e = Build(r); e != nil || !slices.Equal(v.Argv[len(v.Argv)-4:], []string{"--url", "http://127.0.0.1:6006", "--skipTags", "skip-test"}) {
		t.Fatalf("skip tags must be the final option: %v %v", v.Argv, e)
	}
	many := make([]string, 17)
	for i := range many {
		many[i] = "t" + strings.Repeat("x", i)
	}
	for _, target := range []string{"-x", "--json", "a b", " a", "a,,b", ",a", "a,", "a,a", "a=b", "../x", "a;b", "a$b", "é", "a\nb", strings.Repeat("a", 65), strings.Join(many, ",")} {
		r := base
		r.Target = target
		if _, e := Build(r); e == nil {
			t.Fatalf("skip tag %q admitted", target)
		}
	}
	r = base
	r.Target = strings.Join(many[:16], ",")
	if _, e = Build(r); e != nil {
		t.Fatalf("sixteen distinct tags refused: %v", e)
	}
}
