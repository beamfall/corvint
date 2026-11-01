package shell

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// TSP-V0-005: a verified foreign-caller declaration stops
// FrontierForeignInvocation bearing on its script and makes the declared
// callers' units its dependents, so a dynamic script's callers are selected on
// every plan and the plan stays BOUNDED; a declared mention adds no edge. A
// missing declaration keeps the frontier; a declaration that is invalid,
// misses a file naming the script or lists a file that no longer names it
// declares nothing, names FrontierForeignCallersInvalid and keeps the
// frontier bearing; a dirty declaration leaves the plan UNKNOWN.
// TSP-V0-820 (V1-4062): an over-long reason refusal names its length and the
// 1024-byte limit; a reason of exactly 1024 bytes reads.
func TestForeignCallerReasonLimit_TSPV0820(t *testing.T) {
	for _, tc := range []struct {
		size int
		want string
	}{
		{size: 1024},
		{size: 1025, want: `script "run.sh" reason has 1025 bytes; want non-empty valid UTF-8 without control characters, at most 1024 bytes`},
	} {
		root := t.TempDir()
		write(t, root, "run.sh", "#!/bin/sh\n")
		write(t, root, affected.ShellForeignCallersPath, `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run.sh":{"callers":["Makefile"],"reason":"`+strings.Repeat("r", tc.size)+`"}}}`)
		err := CheckForeignCallers(affected.DiskSourceContext(context.Background(), root))
		switch {
		case tc.want == "" && err != nil:
			t.Errorf("%d-byte reason refused: %v", tc.size, err)
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
			t.Errorf("%d-byte reason: error %v, want it to contain %q", tc.size, err, tc.want)
		}
	}
	if err := CheckForeignCallers(affected.DiskSourceContext(context.Background(), t.TempDir())); err != nil {
		t.Errorf("absent declaration: %v", err)
	}
}

func TestForeignCallerDeclaration_TSP_V0_005(t *testing.T) {
	base := map[string]string{
		"go.mod":                "module example.test/m\n",
		"run/run.sh":            "#!/bin/sh\n. \"$ENVFILE\"\n",
		"caller/caller.go":      "package caller\n\nvar script = \"run\" + \"/run.sh\"\n",
		"caller/caller_test.go": "package caller\n",
		"direct/direct.go":      "package direct\n",
		"direct/direct_test.go": "package direct\n\nvar script = \"../run/\" + \"run.sh\"\n",
		"Makefile":              "check:\n\tsh run/$(RUN).sh || sh run/run.sh\n",
		"other/other.go":        "package other\n",
		"other/other_test.go":   "package other\n",
		"docs/notes.md":         "run.sh is described here, and prose is no caller\n",
		"upper/upper.go":        "package upper\n\nimport _ \"example.test/m/caller\"\n",
		"upper/upper_test.go":   "package upper\n",
		"beside/beside.go":      "package beside\n\nimport _ \"example.test/m/direct\"\n",
		"beside/beside_test.go": "package beside\n",
	}
	const listed = `"callers":["Makefile","caller/caller.go","direct/direct_test.go"]`
	// notes names the script in prose only; listed as a mention it adds no
	// edge, so its test is never selected for the script.
	notes := map[string]string{"notes/notes.go": "package notes\n\n// run.sh is only named here.\n", "notes/notes_test.go": "package notes\n"}
	declare := func(callers string) string {
		return `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run/run.sh":{` + callers + `,"reason":"computed paths"}}}`
	}
	for _, tc := range []struct {
		name        string
		declaration string
		extra       map[string]string
		declared    bool
		invalid     bool
	}{
		{name: "undeclared"},
		{name: "declared", declaration: declare(listed), declared: true},
		{name: "mention", declaration: declare(listed + `,"mentions":["notes/notes.go"]`), extra: notes, declared: true},
		{name: "mention also a caller", declaration: declare(`"callers":["Makefile","caller/caller.go","direct/direct_test.go","notes/notes.go"],"mentions":["notes/notes.go"]`), extra: notes, invalid: true},
		{name: "stale mention", declaration: declare(listed + `,"mentions":["other/other.go"]`), invalid: true},
		{name: "unlisted mention", declaration: declare(listed), extra: notes, invalid: true},
		{name: "unlisted go caller", declaration: declare(listed), extra: map[string]string{"extra/extra.go": "package extra\n\nvar s = \"run.sh\"\n"}, invalid: true},
		{name: "unlisted shell caller", declaration: declare(listed), extra: map[string]string{"tool/use.sh": "sh \"$here/run.sh\"\n"}, invalid: true},
		{name: "unlisted make caller", declaration: declare(listed), extra: map[string]string{"sub/rules.mk": "x:\n\tsh run/run.sh\n"}, invalid: true},
		{name: "unlisted python caller", declaration: declare(listed), extra: map[string]string{"lane.py": "import subprocess\nsubprocess.run(['run/run.sh'])\n"}, invalid: true},
		{name: "unlisted workflow caller", declaration: declare(listed), extra: map[string]string{"ci/flow.yml": "run: run/run.sh\n"}, invalid: true},
		{name: "stale caller", declaration: declare(`"callers":["Makefile","caller/caller.go","direct/direct_test.go","other/other.go"]`), invalid: true},
		{name: "unsorted callers", declaration: declare(`"callers":["caller/caller.go","Makefile","direct/direct_test.go"]`), invalid: true},
		{name: "no script unit", declaration: `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run/run.sh":{` + listed + `,"reason":"r"},"gone.sh":{"callers":["Makefile"],"reason":"r"}}}`, invalid: true},
		{name: "empty reason", declaration: `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run/run.sh":{` + listed + `,"reason":""}}}`, invalid: true},
		{name: "bad profile", declaration: `{"profile":"corvint-shell-foreign-callers/9","scripts":{"run/run.sh":{` + listed + `,"reason":"r"}}}`, invalid: true},
		{name: "unknown member", declaration: `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run/run.sh":{` + listed + `,"reason":"r","tests":[]}}}`, invalid: true},
		{name: "unparsed", declaration: `{`, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := maps.Clone(base)
			maps.Copy(files, tc.extra)
			if tc.declaration != "" {
				files[affected.ShellForeignCallersPath] = tc.declaration
			}
			root := t.TempDir()
			for relative, content := range files {
				write(t, root, relative, content)
			}
			result := units(t, root)
			script := unitOf(t, result, "run/run.sh")
			if bears := slices.Contains(script.Frontier, FrontierForeignInvocation); bears == tc.declared {
				t.Errorf("foreign frontier on the script %v, want %v (%+v)", bears, !tc.declared, script)
			}
			if got := len(script.Callers) > 0; got != tc.declared {
				t.Errorf("script callers %v, want declared %v", script.Callers, tc.declared)
			}
			if got := slices.Contains(result.Frontier, FrontierForeignCallersInvalid); got != tc.invalid {
				t.Errorf("invalid frontier %v, want %v (frontier %v)", got, tc.invalid, result.Frontier)
			}

			graph, err := affected.Build(context.Background(), root, New(), golang.New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{"other/other.go"})
			if tc.declared {
				if plan.Scope != affected.ScopeBounded {
					t.Errorf("scope %s (unknown %v), want BOUNDED", plan.Scope, plan.Unknown)
				}
				// The dynamic script stays an unbounded reader, so its declared
				// callers' tests are selected on every plan: a source caller is
				// a dependent whose own dependents follow (upper), a test-file
				// caller is one test-user hop whose package's dependents do not
				// (beside).
				want := []string{"caller/caller_test.go", "direct/direct_test.go", "other/other_test.go", "upper/upper_test.go"}
				if got := plan.SelectedTests(); !slices.Equal(got, want) {
					t.Errorf("selected %v, want %v", got, want)
				}
			} else {
				if plan.Scope != affected.ScopeUnknown || !slices.Contains(plan.Unknown, affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: FrontierForeignInvocation}) {
					t.Errorf("scope %s, unknown %v; want the foreign frontier to bear", plan.Scope, plan.Unknown)
				}
			}
			if tc.invalid && !slices.Contains(plan.Unknown, affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: FrontierForeignCallersInvalid}) {
				t.Errorf("unknown %v lacks %s", plan.Unknown, FrontierForeignCallersInvalid)
			}
			if tc.declaration == "" {
				return
			}
			changed := affected.Select(graph, []string{affected.ShellForeignCallersPath})
			if changed.Scope != affected.ScopeUnknown || !slices.Contains(changed.Unknown, affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: affected.FrontierForeignCallersChanged}) {
				t.Errorf("dirty declaration: scope %s, unknown %v; want the changed frontier", changed.Scope, changed.Unknown)
			}
		})
	}
}
