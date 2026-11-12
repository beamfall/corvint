// SPDX-License-Identifier: Apache-2.0
package clierrors_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
)

// The exit codes of the supported-command tables that the 1.x compatibility promise makes stable
// (CMP-V1-003 item 1, docs/COMPATIBILITY.md): every command node of the frozen command tree exits
// exitHelp for --help and exitUsage for an argument it does not define, and the root exits
// exitHelp for --help and --version and exitUsage for an unknown verb. Changing either code, or
// letting a node accept an undefined flag, is a breaking change under CMP-V1-004.
const (
	exitHelp  = 0
	exitUsage = 2
	// undefinedFlag is a flag no node defines.
	undefinedFlag = "--zz-cmp-undefined-flag"
)

// TestCMPV1003CorvintExitCodes (CMP-V1-003, V1-5434) pins the exit codes of every corvint node.
// corvint-docs is built beside corvint, so the docs and flows shims forward deterministically.
func TestCMPV1003CorvintExitCodes(t *testing.T) {
	exitCodeContract(t, "corvint", "./cmd/corvint", "./cmd/corvint-docs")
}

// TestCMPV1003CorvintDocsExitCodes (CMP-V1-003, V1-5434) pins the exit codes of every
// corvint-docs node.
func TestCMPV1003CorvintDocsExitCodes(t *testing.T) {
	exitCodeContract(t, "corvint-docs", "./cmd/corvint-docs")
}

func exitCodeContract(t *testing.T, binary string, packages ...string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the contract records the POSIX exits; Windows runs the commands' own tests")
	}
	tree, ok := capcatalog.TreeSnapshot(binary)
	if !ok || len(tree.Nodes) == 0 {
		t.Fatalf("%s: no frozen command tree", binary)
	}
	bin := t.TempDir()
	build := exec.Command("go", append([]string{"build", "-buildvcs=false", "-o", bin + string(filepath.Separator)}, packages...)...)
	build.Dir = filepath.Join("..", "..")
	build.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, output)
	}
	executable := filepath.Join(bin, binary)
	work := t.TempDir()
	expect := func(want int, args ...string) {
		t.Helper()
		if code, stdout, stderr := launch(t, executable, work, args...); code != want {
			t.Errorf("%s %v: exit %d, want %d (stdout %q stderr %q)", binary, args, code, want, stdout, stderr)
		}
	}
	expect(exitHelp, "--help")
	expect(exitHelp, "--version")
	expect(exitUsage, unknownVerb)
	for _, node := range tree.Nodes {
		expect(exitHelp, append(append([]string{}, node.Path...), "--help")...)
		expect(exitUsage, append(append([]string{}, node.Path...), undefinedFlag)...)
	}
}
