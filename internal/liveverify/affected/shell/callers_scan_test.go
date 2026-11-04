package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TSP-V0-943 (V1-3351): the completeness scan reads its candidates by a
// bounded pool on a disk source and serially on an FS source, with one
// verdict: complete when the named files are exactly the listed ones, not
// complete when a candidate past the first worker batch names the script
// unlisted, and not complete when a listed candidate is unreadable.
func TestForeignCallersScanVerdict_TSP_V0_943(t *testing.T) {
	write := func(t *testing.T, files map[string]string) string {
		dir := t.TempDir()
		for name, body := range files {
			full := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	files := map[string]string{"run/run.sh": "#!/bin/sh\n", "tool/other.sh": "#!/bin/sh\n"}
	var callers []string
	for index := range 3 * foreignCallerWorkers {
		name := fmt.Sprintf("c%02d/c.go", index)
		files[name] = "package c\n"
		if index%5 == 0 {
			files[name] = "package c\n\nvar s = \"run.sh\"\n"
			callers = append(callers, name)
		}
	}
	declared := map[string]ForeignCallers{
		"run/run.sh":    {Callers: callers, Reason: "computed"},
		"tool/other.sh": {Callers: []string{callers[len(callers)-1]}, Reason: "computed"},
	}
	files[callers[len(callers)-1]] += "var o = \"other.sh\"\n"
	sources := func(dir string) map[string]*affected.Source {
		return map[string]*affected.Source{"disk": affected.DiskSource(dir), "fs": affected.FSSource(os.DirFS(dir))}
	}
	for name, source := range sources(write(t, files)) {
		if !foreignCallersComplete(source, declared, nil) {
			t.Errorf("%s: a complete listing is not complete", name)
		}
	}
	unlisted := map[string]string{"z9/late.go": "package z\n\nvar s = \"run.sh\"\n"}
	for name, body := range files {
		unlisted[name] = body
	}
	for name, source := range sources(write(t, unlisted)) {
		if foreignCallersComplete(source, declared, nil) {
			t.Errorf("%s: an unlisted late candidate naming the script still verifies", name)
		}
	}
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-0 file")
	}
	dir := write(t, files)
	if err := os.Chmod(filepath.Join(dir, callers[1]), 0); err != nil {
		t.Fatal(err)
	}
	for name, source := range sources(dir) {
		if foreignCallersComplete(source, declared, nil) {
			t.Errorf("%s: an unreadable listed candidate still verifies", name)
		}
	}
}

// TSP-V0-7302: a comment still needs a mention declaration, but its refusal names file:line.
func TestForeignCallerCommentDiagnostic(t *testing.T) {
	root := t.TempDir()
	write(t, root, "run/run.sh", "#!/bin/sh\ntrue\n")
	write(t, root, "caller.go", "package c\nvar script = \"run.sh\"\n")
	write(t, root, "notes.go", "package c\n\n// run.sh is only documentation\n")
	write(t, root, affected.ShellForeignCallersPath, `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run/run.sh":{"callers":["caller.go"],"reason":"computed"}}}`)
	result := units(t, root)
	if !strings.Contains(strings.Join(result.Frontier, "\n"), "notes.go:3") {
		t.Fatalf("comment refusal does not identify caller: %v", result.Frontier)
	}
}
