package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// GPK-V0-880 (V1-3873): impact refuses a tracked directory and the Makefile
// with a code whose supported fixes name the transitive alternative,
// corvint affected --path, beside omitting the path.
func TestGPKV0880SuffixRefusalNamesAffectedForDirectoryAndMakefile(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte("all:\n\t@true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"add", "Makefile"}, {"commit", "-qm", "add Makefile"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	for _, path := range []string{"pkg", "Makefile"} {
		exit, stdout, stderr := runCLI(t, "--root", root, "impact", path)
		if exit != 2 || stdout != "" {
			t.Fatalf("impact %s: exit=%d stdout=%q", path, exit, stdout)
		}
		var envelope map[string]any
		if err := json.Unmarshal([]byte(stderr), &envelope); err != nil {
			t.Fatalf("impact %s stderr %q: %v", path, stderr, err)
		}
		if want := impactPathSuffixEnvelope(path); !reflect.DeepEqual(envelope, want) {
			t.Fatalf("impact %s refusal = %#v, want %#v", path, envelope, want)
		}
	}
}
