// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-9600 (V1-5071): the internal/closureguard shape. Several tests in one
// file each derive the module root from their own runtime.Caller(0) names,
// capture stderr in a buffer, and set a literal environment. That package is a
// proven reader: an import-set change selects it and a body-only change does
// not. Each variant below that observes more keeps the unproven fallback.
func TestClosureGuardShapeIsProven_AFP_V0_9600(t *testing.T) {
	const header = "package probe\nimport (\"bytes\"; \"context\"; \"errors\"; \"os\"; \"os/exec\"; \"path/filepath\"; \"runtime\"; \"strings\"; \"testing\"; \"time\")\n" +
		"var _ = errors.New\nvar _ = strings.Fields\n"
	guardWith := func(name, prefix, environment string) string {
		return "func " + name + "(t *testing.T) {\n" +
			"_, " + prefix + "File, _, ok := runtime.Caller(0)\nif !ok { t.Fatal(\"caller\") }\n" +
			prefix + "Root := filepath.Dir(filepath.Dir(" + prefix + "File))\n" +
			"ctx, cancel := context.WithTimeout(context.Background(), time.Minute)\ndefer cancel()\n" +
			"command := exec.CommandContext(ctx, \"go\", \"list\", \"-deps\", \"./cmd/tool\")\n" +
			"command.Dir = " + prefix + "Root\n" +
			environment + "\n" +
			"var stderr bytes.Buffer\ncommand.Stderr = &stderr\n" +
			"out, err := command.Output()\nif err != nil { t.Fatalf(\"%v: %s\", err, stderr.String()) }\n_ = out\n}\n"
	}
	guardEnv := func(name, prefix, environment string) string {
		return guardWith(name, prefix, "command.Env = append(os.Environ(), "+environment+")")
	}
	// reassigned sets the environment twice, as a later edit might.
	reassigned := func(first, second string) string {
		return guardWith("TestA", "a", "command.Env = append(os.Environ(), "+first+")\ncommand.Env = append(os.Environ(), "+second+")")
	}
	guard := func(name, prefix string) string {
		return guardEnv(name, prefix, `"GOTOOLCHAIN=local", "GOPROXY=off"`)
	}
	shared := func(name string) string {
		return "func " + name + "(t *testing.T) {\n" +
			"_, file, _, _ := runtime.Caller(0)\nroot := filepath.Dir(filepath.Dir(file))\n" +
			"command := exec.Command(\"go\", \"list\", \"-deps\", \"./cmd/tool\")\ncommand.Dir = root\n_, _ = command.Output()\n}\n"
	}
	for _, test := range []struct {
		name, body string
		unproven   bool
		unbounded  bool
	}{
		{"distinct roots per test", guard("TestA", "a") + guard("TestB", "b"), false, false},
		// An empty flags reset only removes ambient flags (an inherited
		// -overlay or -e), so it stays proven; any other value does not.
		{"empty GOFLAGS reset", guardEnv("TestA", "a", `"GOTOOLCHAIN=local", "GOFLAGS="`) + guardEnv("TestB", "b", `"GOFLAGS=", "GOWORK=off"`), false, false},
		{"non-empty GOFLAGS literal", guardEnv("TestA", "a", `"GOTOOLCHAIN=local", "GOFLAGS=-mod=mod"`), true, false},
		{"dynamic GOFLAGS value", guardEnv("TestA", "a", `"GOTOOLCHAIN=local", "GOFLAGS="+os.Getenv("PROBE_FLAGS")`), true, false},
		{"empty GOFLAGS reset outside an Env assignment", guardEnv("TestA", "a", `"GOTOOLCHAIN=local"`) + "var flags = []string{\"GOFLAGS=\"}\n", true, false},
		// Every Env assignment of a package that uses the reset must carry
		// it: a later one without it brings the ambient flags back.
		{"reset dropped by a later Env assignment", reassigned(`"GOFLAGS="`, `"GOWORK=off"`), true, false},
		{"reset carried by every Env assignment", reassigned(`"GOFLAGS="`, `"GOWORK=off", "GOFLAGS="`), false, false},
		{"reset in one guard, none in another", guardEnv("TestA", "a", `"GOFLAGS="`) + guardEnv("TestB", "b", `"GOWORK=off"`), true, false},
		{"Env assigned from a variable", guardWith("TestA", "a", "environment := append(os.Environ(), \"GOFLAGS=\")\ncommand.Env = environment"), true, false},
		{"Setenv of an empty GOFLAGS", guardEnv("TestA", "a", `"GOFLAGS="`) + "func TestC(t *testing.T) { t.Setenv(\"GOFLAGS\", \"\") }\n", true, false},
		{"Setenv of a computed name", guard("TestA", "a") + "func init() { _ = os.Setenv(\"GO\"+\"FLAGS\", \"\") }\n", true, false},
		// A proven listing whose root is spelled with a `..` literal still
		// names the repository root to rule (d), so every edit selects it.
		{"root climbed with a .. literal", "func TestA(t *testing.T) {\n" +
			"_, file, _, _ := runtime.Caller(0)\nroot := filepath.Join(filepath.Dir(file), \"..\")\n" +
			"command := exec.Command(\"go\", \"list\", \"-deps\", \"./cmd/tool\")\ncommand.Dir = root\n_, _ = command.Output()\n}\n", false, true},
		{"one Caller root shared by name across tests", shared("TestA") + shared("TestB"), true, false},
		{"one root reused by two listings", "func TestA(t *testing.T) {\n" +
			"_, file, _, _ := runtime.Caller(0)\nroot := filepath.Dir(filepath.Dir(file))\n" +
			"first := exec.Command(\"go\", \"list\", \"-deps\", \"./cmd/tool\")\nfirst.Dir = root\nout, err := first.Output()\n" +
			"second := exec.Command(\"go\", \"list\", \"-deps\", \"./cmd/other\")\nsecond.Dir = root\nout, err = second.Output()\n_, _ = out, err\n}\n", true, false},
		{"stderr recovered from exec.ExitError", "func TestA(t *testing.T) {\n" +
			"_, file, _, _ := runtime.Caller(0)\nroot := filepath.Dir(filepath.Dir(file))\n" +
			"command := exec.Command(\"go\", \"list\", \"-deps\", \"./cmd/tool\")\ncommand.Dir = root\n" +
			"if _, err := command.Output(); err != nil {\nvar exit *exec.ExitError\nif errors.As(err, &exit) { t.Fatalf(\"%s\", exit.Stderr) }\n}\n}\n", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := importGraphRepository(t)
			writeFiles(t, root, map[string]string{"probe/probe.go": "package probe\n", "probe/probe_test.go": header + test.body})
			graph, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			unit, ok := graph.Unit(importGraphModule + "probe")
			reader := unit.ImportGraphReadsAll || len(unit.ImportGraphReads) != 0
			if !ok || !reader || unit.ImportGraphReadsFiles != test.unproven {
				t.Fatalf("files=%v all=%v reads=%v, want unproven=%v", unit.ImportGraphReadsFiles, unit.ImportGraphReadsAll, unit.ImportGraphReads, test.unproven)
			}
			if unbounded := unit.UnboundedReads != ""; unbounded != test.unbounded {
				t.Fatalf("unbounded reads %q, want unbounded=%v", unit.UnboundedReads, test.unbounded)
			}
			witness, hit := probeSelectedForBodyEdit(graph)
			switch {
			case test.unproven && hit && witness.Kind == affected.WitnessBinaryExec:
				// An unproven listing keeps its literal target as a binary
				// edge, so a body edit selects it by that stronger witness.
				return
			case test.unproven:
				assertReaderSelectedForLibEdit(t, graph, false)
			case test.unbounded:
				if !hit || witness.Kind != affected.WitnessUnboundedReader {
					t.Fatalf("a body-only lib edit selected the probe by %+v (hit=%v), want %s", witness, hit, affected.WitnessUnboundedReader)
				}
				return
			case hit:
				t.Fatalf("a body-only lib edit selected the proven closure guard by %+v", witness)
			}
			assertReaderSelectedForLibEdit(t, graph, true)
		})
	}
}

// probeSelectedForBodyEdit reports whether a body-only edit of lib, compared
// with no import-set change (as in AFP-V0-2201), selects the probe unit.
func probeSelectedForBodyEdit(graph *affected.Graph) (affected.Witness, bool) {
	changes := &affected.ImportSetChanges{Changed: map[string]string{}}
	for _, selection := range affected.Select(graph.WithImportSetChanges(changes), []string{"lib/lib.go"}).Selected {
		if selection.UnitID == importGraphModule+"probe" {
			return selection.Witness, true
		}
	}
	return affected.Witness{}, false
}
