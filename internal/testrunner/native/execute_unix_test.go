//go:build darwin || linux

package native

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// The tool and source are real local bytes, so this catches the shared executor
// ABI regression that parser-only fixture tests cannot detect.
func TestNativeGoBuildExecuteParse(t *testing.T) {
	executable, err := exec.LookPath("go")
	if err != nil {
		t.Skip("Go runtime unavailable")
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal(err)
	}
	toolBytes, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	inputs := map[string][]byte{
		"go.mod":        []byte("module example.com/nativeprobe\n\ngo 1.22\n"),
		"probe_test.go": []byte("package nativeprobe\nimport \"testing\"\nfunc TestPass(t *testing.T) {}\nfunc TestFail(t *testing.T) { t.Fatal(\"deliberate\") }\nfunc TestSkip(t *testing.T) { t.Skip(\"deliberate\") }\n"),
	}
	r := tr.Request{Runner: "go-test", Executable: executable, ExecutableSha256: tr.Digest(toolBytes), Root: root, Project: "example.com/nativeprobe", ReportDir: filepath.Join(t.TempDir(), "reports"), TimeoutSeconds: 120, InputFiles: map[string]string{}, ExpectedTests: []string{"example.com/nativeprobe::TestPass", "example.com/nativeprobe::TestFail", "example.com/nativeprobe::TestSkip"}}
	for name, data := range inputs {
		if err = os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatal(err)
		}
		r.InputFiles[name] = tr.Digest(data)
	}
	inv, err := Build(r)
	if err != nil {
		t.Fatal(err)
	}
	result, err := tr.Execute(context.Background(), r, inv)
	if err != nil {
		t.Fatalf("execution: %v; input=%+v", err, result.Input)
	}
	observation, err := Parse(result.Input)
	if err != nil {
		t.Fatalf("parse: %v; stderr=%s; stdout=%s", err, result.Input.Stderr, result.Input.Stdout)
	}
	observation = tr.Normalize(result.Input, observation)
	if !observation.Complete || len(observation.Tests) != 3 || result.Input.ExitCode != 1 {
		t.Fatalf("observation=%+v; stderr=%s", observation, result.Input.Stderr)
	}
	counts := map[string]int{}
	for _, row := range observation.Tests {
		counts[row.State]++
	}
	if counts[tr.Passed] != 1 || counts[tr.Failed] != 1 || counts[tr.Skipped] != 1 {
		t.Fatal(counts)
	}
}
