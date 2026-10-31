package shell

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TSP-V0-960: a computed command word is expanded through the file's own
// values. One whose value is a literal path is an edge ("$heavy" with a
// literal default). One that builds a script name it cannot resolve keeps
// FrontierForeignInvocation on a non-test caller, even a declared one, and
// names every script its pattern may be in the completeness scan, so an
// unlisted builder leaves the declaration invalid.
func TestComputedInvocation_TSP_V0_960(t *testing.T) {
	t.Run("literal value is an edge", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "run/run.sh", "echo run\n")
		write(t, root, "use/use.sh", "tool=${USE_TOOL-run/run.sh}\nenv A=1 \"$tool\" x\n")
		use := unitOf(t, units(t, root), "use/use.sh")
		if !slices.Equal(use.Imports, []string{"shell:run/run.sh"}) || len(use.Frontier) != 0 {
			t.Errorf("use imports %v frontier %v, want the run.sh edge and no frontier", use.Imports, use.Frontier)
		}
	})
	t.Run("built name keeps the caller frontier", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "run/run.sh", "echo run\n")
		write(t, root, "use/use.sh", "\"$here/$name.sh\" x\n")
		write(t, root, "use/use_test.sh", "\"$here/$name.sh\" x\n")
		result := units(t, root)
		use := unitOf(t, result, "use/use.sh")
		if !slices.Equal(use.Frontier, []string{FrontierForeignInvocation}) || use.FrontierBears != affected.FrontierBearsOnDependents {
			t.Errorf("use frontier %v bears %q, want the foreign frontier on dependents", use.Frontier, use.FrontierBears)
		}
		if test := unitOf(t, result, "use/use_test.sh"); len(test.Frontier) != 0 {
			t.Errorf("test frontier %v, want none: a test is a leaf", test.Frontier)
		}
		if other := unitOf(t, result, "run/run.sh"); len(other.Frontier) != 0 {
			t.Errorf("run frontier %v, want none in a shell-only repository", other.Frontier)
		}
	})

	declaration := func(callers string) string {
		return `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run/run.sh":{"callers":[` + callers + `],"reason":"computed paths"}}}`
	}
	for _, tc := range []struct {
		name, script, builder, callers string
		declared, keeps                bool
	}{
		{name: "declared script that builds a name keeps the frontier", script: "for n in helper; do \"$here/$n.sh\"; done\n", builder: "sh run/run.sh\n", callers: `"Makefile","tool/use.sh"`, declared: true, keeps: true},
		{name: "unlisted literal builder", script: "echo run\n", builder: "n=ru\n\"$here/${n}n.sh\"\n", callers: `"Makefile"`},
		{name: "unlisted pattern builder", script: "echo run\n", builder: "\"$here/r$x.sh\"\n", callers: `"Makefile"`},
		{name: "unlisted interpreter operand", script: "echo run\n", builder: "bash \"$here/$x.sh\"\n", callers: `"Makefile"`},
		{name: "listed pattern builder", script: "echo run\n", builder: "\"$here/r$x.sh\"\n", callers: `"Makefile","tool/use.sh"`, declared: true},
		{name: "builder of another name", script: "echo run\n", builder: "\"$here/x$x.sh\"\n", callers: `"Makefile"`, declared: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "go.mod", "module example.test/m\n")
			write(t, root, "m.go", "package m\n")
			write(t, root, "Makefile", "check:\n\tsh run/run.sh\n")
			write(t, root, "run/run.sh", tc.script)
			write(t, root, "tool/use.sh", tc.builder)
			write(t, root, affected.ShellForeignCallersPath, declaration(tc.callers))
			result := units(t, root)
			if invalid := slices.Contains(result.Frontier, FrontierForeignCallersInvalid); invalid == tc.declared {
				t.Fatalf("invalid %v, want declared %v (frontier %v)", invalid, tc.declared, result.Frontier)
			}
			script := unitOf(t, result, "run/run.sh")
			if bears := slices.Contains(script.Frontier, FrontierForeignInvocation); bears != (tc.keeps || !tc.declared) {
				t.Errorf("script frontier %v, want bearing %v", script.Frontier, tc.keeps || !tc.declared)
			}
		})
	}
}

// TSP-V0-960: a computed run target whose values exceed the 64-value bound
// is not silently cut. Two nine-word loops build 81 names; the kept values
// stay edges and the test becomes an unbounded reader, so a change to a
// dropped script still selects it. Eight by eight fits and stays bounded.
func TestComputedInvocationValueBoundIsUnbounded_TSP_V0_960(t *testing.T) {
	for _, tc := range []struct {
		name    string
		letters string
		cut     bool
	}{
		{name: "81 values exceed the bound", letters: "abcdefghi", cut: true},
		{name: "64 values fit the bound", letters: "abcdefgh"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			words := strings.Join(strings.Split(tc.letters, ""), " ")
			for _, x := range tc.letters {
				for _, y := range tc.letters {
					write(t, root, "scripts/"+string(x)+string(y)+".sh", "echo ok\n")
				}
			}
			write(t, root, "use_test.sh", "for x in "+words+"\ndo\n  for y in "+words+"\n  do\n    sh \"./scripts/${x}${y}.sh\"\n  done\ndone\n")
			unit := unitOf(t, units(t, root), "use_test.sh")
			if got := unit.UnboundedReads == UnboundedDynamicSource; got != tc.cut {
				t.Errorf("unbounded reads %q, want unbounded %v", unit.UnboundedReads, tc.cut)
			}
			if len(unit.Imports) != 64 {
				t.Errorf("imports %d, want the 64 kept edges", len(unit.Imports))
			}
		})
	}
}
