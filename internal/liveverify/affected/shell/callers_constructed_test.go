// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"context"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"slices"
	"testing"
)

// AZU-V0-8901: new literal/constructed foreign callers fail closed; listing
// them makes completeness hold. An unrelated directory is no caller.
func TestConstructedForeignCallers_AZU_V0_8901(t *testing.T) {
	for _, body := range []string{
		`var command = "run/" + name + ".sh"`,
		`cmd := filepath.Join("run", name + ".sh")`,
		"cmd := filepath.Join(`run`, name + \".sh\")",
		`subprocess.run([f"run/{name}.sh"])`,
		`exec("run/$name.sh")`,
		`exec("run/run.sh")`,
	} {
		t.Run(body, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "run/run.sh", "echo ok\n")
			write(t, root, "foreign/caller.go", body)
			declared := map[string]ForeignCallers{"run/run.sh": {Callers: []string{"foreign/caller.go"}, Reason: "computed caller"}}
			if !foreignCallersComplete(affected.DiskSource(root), declared, nil) {
				t.Fatal("listed constructed caller refused")
			}
			declared["run/run.sh"] = ForeignCallers{Callers: []string{"other.go"}, Reason: "missing caller"}
			if foreignCallersComplete(affected.DiskSource(root), declared, nil) {
				t.Fatal("undeclared caller accepted")
			}
		})
	}
	if constructedForeignReference([]byte(`filepath.Join("elsewhere",name)`), "run/run.sh") {
		t.Fatal("unrelated directory matched")
	}
}

// AZU-V0-8901: a root callee has computed callers too. An incomplete
// declaration must keep uncertainty even while its known callers stay linked.
func TestRootConstructedForeignCallers_AZU_V0_8901(t *testing.T) {
	for _, expression := range []string{
		`filepath.Join(".", name+".sh")`,
		`"./" + name + ".sh"`,
		`name + ".sh"`,
	} {
		t.Run(expression, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "go.mod", "module example.test/root\n\ngo 1.27\n")
			write(t, root, "run.sh", "echo ok\n")
			write(t, root, "caller/caller.go", "package caller\nvar script = \"run.sh\"\n")
			write(t, root, "caller/caller_test.go", "package caller\n")
			write(t, root, "extra/extra.go", "package extra\nimport (\"os/exec\"; \"path/filepath\")\nvar name string\nvar cmd = exec.Command(\"sh\", "+expression+")\n")
			write(t, root, affected.ShellForeignCallersPath, `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run.sh":{"callers":["caller/caller.go"],"reason":"literal caller"}}}`)
			result := units(t, root)
			unit := unitOf(t, result, "run.sh")
			if !slices.Contains(result.Frontier, FrontierForeignCallersInvalid) || !slices.Contains(unit.Frontier, FrontierForeignInvocation) {
				t.Fatalf("undeclared root caller lost uncertainty: unit=%+v frontier=%v", unit, result.Frontier)
			}
			graph, err := affected.Build(context.Background(), root, golang.New(), New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{"run.sh"})
			if plan.Scope != affected.ScopeUnknown || len(plan.Unknown) == 0 {
				t.Fatalf("undeclared root caller certified: %+v", plan)
			}
			if !slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == "go:example.test/root/caller" }) {
				t.Fatalf("known caller dropped: %+v", plan)
			}
		})
	}
}

// AZU-V0-8901: the wrapper's repository-root query resolves a known shell
// target, while querying another directory cannot certify the current root.
func TestDeclaredRootWrapper_AZU_V0_8901(t *testing.T) {
	for _, query := range []string{"git rev-parse --show-toplevel", "env -u GIT_CONFIG_GLOBAL git rev-parse --show-toplevel"} {
		root := t.TempDir()
		write(t, root, "script/worker.sh", "echo ok\n")
		write(t, root, "script/wrapper.sh", "top=$("+query+")\nscript=${top:+$top/script/worker.sh}\nexec \"$script\" \"$@\"\n")
		result := units(t, root)
		wrapper := unitOf(t, result, "script/wrapper.sh")
		if !slices.Contains(wrapper.Imports, "shell:script/worker.sh") {
			t.Fatalf("root target not linked: %+v", wrapper)
		}
	}
	if repositoryRootCommand("git -C elsewhere rev-parse --show-toplevel") {
		t.Fatal("foreign root accepted")
	}
}

// AZU-V0-8901: extending completeness can add an unknown, but must preserve
// caller edges already verified by the legacy literal/shell-expansion check.
func TestConstructedDriftKeepsVerifiedCallers_AZU_V0_8901(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.test/legacy\n\ngo 1.27\n")
	write(t, root, "run/run.sh", "echo ok\n")
	write(t, root, "caller/caller.go", "package caller\n\nvar script = \"run/run.sh\"\n")
	write(t, root, "caller/caller_test.go", "package caller\n")
	write(t, root, "extra/extra.go", "package extra\nimport \"path/filepath\"\nvar name string\nvar command = filepath.Join(\"run\", name)\n")
	write(t, root, affected.ShellForeignCallersPath, `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run/run.sh":{"callers":["caller/caller.go"],"reason":"legacy literal caller"}}}`)
	result := units(t, root)
	unit := unitOf(t, result, "run/run.sh")
	if !slices.Contains(result.Frontier, FrontierForeignCallersInvalid) || !slices.Contains(unit.Frontier, FrontierForeignInvocation) || !slices.Equal(unit.Callers, []string{"caller/caller.go"}) {
		t.Fatalf("lost verified callers or uncertainty: unit=%+v frontier=%v", unit, result.Frontier)
	}
	graph, err := affected.Build(context.Background(), root, golang.New(), New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"run/run.sh"})
	if !slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == "go:example.test/legacy/caller" }) {
		t.Fatalf("legacy caller dropped: %+v", plan)
	}
}

// AZU-V0-004, AZU-V0-8901: a Go raw-string directory join must not
// certify completeness while its undeclared caller's tests stay excluded.
func TestRawStringJoinRetainsUnknown_AZU_V0_8901(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.test/rawjoin\n\ngo 1.27\n")
	write(t, root, "run/run.sh", "echo ok\n")
	write(t, root, "caller/caller.go", "package caller\nvar script = \"run/run.sh\"\n")
	write(t, root, "caller/caller_test.go", "package caller\n")
	write(t, root, "extra/extra.go", "package extra\nimport (\"os/exec\"; \"path/filepath\")\nvar name string\nvar cmd = exec.Command(\"sh\", filepath.Join(`run`, name+\".sh\"))\n")
	write(t, root, "extra/extra_test.go", "package extra\n")
	write(t, root, affected.ShellForeignCallersPath, `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run/run.sh":{"callers":["caller/caller.go"],"reason":"literal caller"}}}`)
	graph, err := affected.Build(context.Background(), root, golang.New(), New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"run/run.sh"})
	if plan.Scope != affected.ScopeUnknown || !slices.ContainsFunc(plan.Unknown, func(u affected.Unknown) bool {
		return u.Detail == FrontierForeignInvocation
	}) {
		t.Fatalf("undeclared raw-string caller certified: %+v", plan)
	}
	if !slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == "go:example.test/rawjoin/caller" }) {
		t.Fatalf("known caller dropped: %+v", plan)
	}
}
