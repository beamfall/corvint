// SPDX-License-Identifier: Apache-2.0
package shell

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// TSP-V0-863: a file that runs a script by a computed path the resolver cannot
// fix keeps FrontierForeignInvocation on itself, bearing on every plan that
// reaches it, and no foreign-caller declaration clears it; a computed run that
// may name a declared script without the file's text naming it makes the
// declaration incomplete.
func TestComputedScriptRunKeepsTheFrontier_TSP_V0_863(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		computed   bool
	}{
		{"a wildcard name", "sh \"$dir/run-$kind.sh\"\n", true},
		{"a computed command word", "\"$tools/run-a.sh\" --check\n", true},
		{"a literal loop", "for k in a b; do\n  bash \"$here/run-$k.sh\"\ndone\n", true},
		{"a loop over names no script has", "for k in x y; do\n  bash \"$here/run-$k.sh\"\ndone\n", false},
		{"a loop variable read elsewhere", "read -r k\nfor k in x y; do\n  bash \"$here/run-$k.sh\"\ndone\n", true},
		{"one expansion alone", "\"$heavy\" go test ./...\n", false},
		{"the caller's arguments", "\"$@\"\nsh \"$1\"\n", false},
		{"a shape no script matches", "sh \"$dir/other-$kind.sh\"\n", false},
		{"a resolved literal", "sh run/run-a.sh\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "run/run-a.sh", "#!/bin/sh\necho a\n")
			write(t, root, "run/run-b.sh", "#!/bin/sh\necho b\n")
			write(t, root, "check/caller_test.sh", "#!/bin/sh\n"+tc.body)
			caller := unitOf(t, units(t, root), "check/caller_test.sh")
			if got := slices.Contains(caller.Frontier, FrontierForeignInvocation); got != tc.computed || caller.FrontierBears != "" {
				t.Fatalf("caller frontier %v bears %q, want computed %v (%+v)", caller.Frontier, caller.FrontierBears, tc.computed, caller)
			}
			if !tc.computed {
				return
			}
			graph, err := affected.Build(context.Background(), root, New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{"check/caller_test.sh"})
			if plan.Scope != affected.ScopeUnknown || !slices.Contains(plan.Unknown, affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: FrontierForeignInvocation}) {
				t.Fatalf("a plan reaching the caller: scope %s unknown %v", plan.Scope, plan.Unknown)
			}
		})
	}

	base := map[string]string{
		"go.mod":                "module example.test/m\n",
		"run/run.sh":            "#!/bin/sh\necho run\n",
		"caller/caller.go":      "package caller\n\nvar script = \"run/run.sh\"\n",
		"caller/caller_test.go": "package caller\n",
	}
	const declaration = `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run/run.sh":{"callers":["caller/caller.go"],"reason":"r"}}}`
	for _, tc := range []struct {
		name    string
		extra   map[string]string
		invalid bool
		bears   bool
	}{
		{name: "complete"},
		{name: "a wildcard that may name it", extra: map[string]string{"tool/use.sh": "sh \"$d/r$x.sh\"\n"}, invalid: true, bears: true},
		{name: "a loop that names it in parts", extra: map[string]string{"tool/use.sh": "for n in run; do\n  sh \"$d/$n.sh\"\ndone\n"}, invalid: true, bears: true},
		{name: "a wildcard naming another script", extra: map[string]string{"tool/use.sh": "sh \"$d/other-$x.sh\"\n", "tool/other-a.sh": "echo\n"}},
		// The declared script's own computed run keeps its frontier, though its callers verify.
		{name: "the declared script runs one", extra: map[string]string{"run/run.sh": "#!/bin/sh\nsh \"$d/other-$x.sh\"\n", "tool/other-a.sh": "echo\n"}, bears: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := maps.Clone(base)
			maps.Copy(files, tc.extra)
			files[affected.ShellForeignCallersPath] = declaration
			root := t.TempDir()
			for relative, content := range files {
				write(t, root, relative, content)
			}
			result := units(t, root)
			if got := slices.Contains(result.Frontier, FrontierForeignCallersInvalid); got != tc.invalid {
				t.Errorf("invalid frontier %v, want %v", got, tc.invalid)
			}
			script := unitOf(t, result, "run/run.sh")
			if got := slices.Contains(script.Frontier, FrontierForeignInvocation); got != tc.bears {
				t.Errorf("script frontier %v, want bearing %v", script.Frontier, tc.bears)
			}
			graph, err := affected.Build(context.Background(), root, New(), golang.New())
			if err != nil {
				t.Fatal(err)
			}
			if plan := affected.Select(graph, []string{"run/run.sh"}); (plan.Scope == affected.ScopeUnknown) != (tc.invalid || tc.bears) {
				t.Errorf("a change to the script: scope %s unknown %v", plan.Scope, plan.Unknown)
			}
		})
	}
}
