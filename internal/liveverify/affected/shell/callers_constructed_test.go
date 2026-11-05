// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"slices"
	"testing"
)

// AZU-V0-8901: new literal/constructed foreign callers fail closed; listing
// them makes completeness hold. An unrelated directory is no caller.
func TestConstructedForeignCallers_AZU_V0_8901(t *testing.T) {
	for _, body := range []string{
		`var command = "run/" + name + ".sh"`,
		`cmd := filepath.Join("run", name + ".sh")`,
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
