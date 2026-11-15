package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func impactDirectoryEnvelope(directory string) map[string]any {
	return refusalEnvelope("unsupported-impact-directory", "the directory holds no tracked file with a suffix admitted by the repository index; subdirectories are not read", subjectOf("value", directory), evidenceOf(), fixesOf("impact.omit-unadmitted-path", "affected.use-path"), "")
}

// impactDirectoryRepository adds, to the impact fixture, a tracked directory
// holding only an unadmitted suffix, one holding only a subdirectory, and an
// untracked empty directory.
func impactDirectoryRepository(t *testing.T) string {
	t.Helper()
	root := impactCLIRepository(t)
	for relative, content := range map[string]string{
		"assets/table.csv":      "a,b\n",
		"nested/inner/inner.go": "package inner\n",
	} {
		file := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "add directory fixtures")
	if err := os.Mkdir(filepath.Join(root, "hollow"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// GPK-V0-883 (V1-5436): a directory operand reads byte-for-byte as naming the
// admitted files directly inside it, alone or mixed with file operands.
func TestGPKV0883ImpactDirectoryReadsAsItsAdmittedFiles(t *testing.T) {
	t.Parallel()
	root := impactDirectoryRepository(t)
	for _, test := range []struct {
		name            string
		operands, files []string
	}{
		{"directory", []string{"pkg"}, []string{"pkg/main.go", "pkg/main_test.go"}},
		{"trailing-slash", []string{"pkg/"}, []string{"pkg/main.go", "pkg/main_test.go"}},
		{"mixed-file-and-directory", []string{"testing/features.yaml", "pkg"}, []string{"testing/features.yaml", "pkg/main.go", "pkg/main_test.go"}},
		{"file-inside-its-directory", []string{"pkg/main.go", "pkg"}, []string{"pkg/main.go", "pkg/main_test.go"}},
		{"two-directories", []string{"pkg", "nested/inner"}, []string{"pkg/main.go", "pkg/main_test.go", "nested/inner/inner.go"}},
		{"require-clean", []string{"--require-clean", "pkg"}, []string{"--require-clean", "pkg/main.go", "pkg/main_test.go"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			exit, stdout, stderr := runCLI(t, append([]string{"--root", root, "impact"}, test.operands...)...)
			wantExit, want, wantStderr := runCLI(t, append([]string{"--root", root, "impact"}, test.files...)...)
			if wantExit != 0 || want == "" {
				t.Fatalf("file impact: exit=%d stderr=%q", wantExit, wantStderr)
			}
			if exit != wantExit || stdout != want || stderr != wantStderr {
				t.Fatalf("impact %v: exit=%d stderr=%q\n got %s\nwant %s", test.operands, exit, stderr, stdout, want)
			}
		})
	}
}

// GPK-V0-883: a directory with no admitted file directly inside it is refused
// by name with its own code, and an unadmitted file keeps the suffix refusal.
func TestGPKV0883ImpactRefusesADirectoryWithoutAdmittedFiles(t *testing.T) {
	t.Parallel()
	root := impactDirectoryRepository(t)
	for _, test := range []struct {
		name     string
		operands []string
		want     map[string]any
	}{
		{"unadmitted-suffix-only", []string{"assets"}, impactDirectoryEnvelope("assets")},
		{"subdirectory-only", []string{"nested"}, impactDirectoryEnvelope("nested")},
		{"untracked-empty", []string{"hollow"}, impactDirectoryEnvelope("hollow")},
		{"mixed-with-admitted", []string{"pkg", "assets", "pkg/main.go"}, impactDirectoryEnvelope("assets")},
		{"unadmitted-file-in-directory", []string{"assets/table.csv"}, impactPathSuffixEnvelope("assets/table.csv")},
		{"unadmitted-file-beside-directory", []string{"pkg", "assets/table.csv"}, impactPathSuffixEnvelope("assets/table.csv")},
	} {
		t.Run(test.name, func(t *testing.T) {
			exit, stdout, stderr := runCLI(t, append([]string{"--root", root, "impact"}, test.operands...)...)
			if exit != 2 || stdout != "" {
				t.Fatalf("impact %v: exit=%d stdout=%q", test.operands, exit, stdout)
			}
			var envelope map[string]any
			if err := json.Unmarshal([]byte(stderr), &envelope); err != nil {
				t.Fatalf("impact %v stderr %q: %v", test.operands, stderr, err)
			}
			if !reflect.DeepEqual(envelope, test.want) {
				t.Fatalf("impact %v refusal = %#v, want %#v", test.operands, envelope, test.want)
			}
		})
	}
}

// GPK-V0-883: the capability listing states the directory expansion and its
// refusal, beside the suffix tables a consumer already reads (GPK-V0-084).
func TestGPKV0883ImpactCapabilitiesStateDirectoryExpansion(t *testing.T) {
	t.Parallel()
	exit, stdout, stderr := runCLI(t, "impact", "--capabilities")
	if exit != 0 {
		t.Fatalf("capabilities: exit=%d stderr=%q", exit, stderr)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"expansion": "admitted-files-directly-inside", "recursive": false, "empty": "unsupported-impact-directory"}
	if !reflect.DeepEqual(decoded["directories"], want) {
		t.Fatalf("directories = %#v, want %#v", decoded["directories"], want)
	}
}
