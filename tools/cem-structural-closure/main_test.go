package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestClosureDigestBoundary: the digest covers the non-test files directly in
// each included package and nothing in an excluded package, a test file or a
// subdirectory (decision 0551).
func TestClosureDigestBoundary(t *testing.T) {
	base := map[string]string{
		"go/parser/parser.go":            "package parser\n",
		"go/parser/interface.go":         "package parser\n",
		"go/parser/parser_test.go":       "package parser\n",
		"go/parser/testdata/x.src":       "x\n",
		"strings/strings.go":             "package strings\n",
		"runtime/proc.go":                "package runtime\n",
		"internal/godebugs/table.go":     "package godebugs\n",
		"internal/runtime/sys/sys.go":    "package sys\n",
		"internal/bytealg/bytealg.go":    "package bytealg\n",
		"internal/bytealg/index_arm64.s": "TEXT\n",
	}
	deps := []string{"go/parser", "strings", "runtime", "internal/godebugs", "internal/runtime/sys", "internal/bytealg"}
	digestOf := func(t *testing.T, edit map[string]string) closure {
		root := t.TempDir()
		writeTree(t, root, base)
		writeTree(t, root, edit)
		c, err := digestClosure(root, "go0.0.0", deps)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	c := digestOf(t, nil)
	if c.PackageCount != 3 || c.FileCount != 5 {
		t.Fatalf("closure counts %d packages, %d files; want 3, 5: %+v", c.PackageCount, c.FileCount, c.Packages)
	}
	for _, edit := range []map[string]string{
		{"runtime/proc.go": "package runtime // changed\n"},
		{"internal/godebugs/table.go": "package godebugs // changed\n"},
		{"internal/runtime/sys/sys.go": "package sys // changed\n"},
		{"go/parser/parser_test.go": "package parser // changed\n"},
		{"go/parser/testdata/x.src": "y\n"},
	} {
		if got := digestOf(t, edit); got.ClosureSha256 != c.ClosureSha256 {
			t.Errorf("edit outside the closure %v changed the digest", edit)
		}
	}
	for _, edit := range []map[string]string{
		{"go/parser/parser.go": "package parser // changed\n"},
		{"internal/bytealg/index_arm64.s": "TEXT changed\n"},
		{"strings/builder.go": "package strings\n"},
	} {
		if got := digestOf(t, edit); got.ClosureSha256 == c.ClosureSha256 {
			t.Errorf("edit inside the closure %v kept the digest", edit)
		}
	}
}

func TestExcludedTreesMatchWholeSegments(t *testing.T) {
	for path, want := range map[string]bool{
		"runtime": true, "internal/godebug": true, "internal/godebugs": true,
		"internal/runtime/atomic": true, "internal/cpu": true, "internal/cpuid": false,
		"go/parser": false, "time": true, "text/tabwriter": false,
		"path/filepath": false, "internal/filepathlite": false,
	} {
		if got := excluded(path); got != want {
			t.Errorf("excluded(%q) = %v, want %v", path, got, want)
		}
	}
}

// TestRunningToolchainClosure: on an admitted structural tuple the running
// toolchain's parser closure equals the qualified one, and every sealed tuple
// packet records that digest (CEM-V1-050, decision 0551).
func TestRunningToolchainClosure(t *testing.T) {
	root := filepath.Join("..", "..", "protocol", "cem-1.0")
	var q struct {
		QualifiedTuple string   `json:"qualifiedTuple"`
		AdmittedTuples []string `json:"admittedTuples"`
		ParserClosure  struct {
			Profile       string            `json:"profile"`
			ClosureSha256 string            `json:"qualifiedClosureSha256"`
			Tuples        map[string]string `json:"tuples"`
		} `json:"parserClosure"`
	}
	raw, err := os.ReadFile(filepath.Join(root, "structural-corpus", "QUALIFICATION.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	if q.ParserClosure.Profile != profile || q.ParserClosure.Tuples[q.QualifiedTuple] != q.ParserClosure.ClosureSha256 {
		t.Fatalf("QUALIFICATION.json parser closure does not record %s under %s", q.QualifiedTuple, profile)
	}
	for _, tuple := range q.AdmittedTuples {
		if q.ParserClosure.Tuples[tuple] != q.ParserClosure.ClosureSha256 {
			t.Errorf("admitted tuple %s closure %q is not the qualified %s", tuple, q.ParserClosure.Tuples[tuple], q.ParserClosure.ClosureSha256)
		}
		if tuple == q.QualifiedTuple {
			continue
		}
		var packet struct {
			ParserClosure struct {
				ClosureSha256 string `json:"closureSha256"`
			} `json:"parserClosure"`
		}
		raw, err := os.ReadFile(filepath.Join(root, "structural-tuples", tuple, "TUPLE.json"))
		if err == nil {
			err = json.Unmarshal(raw, &packet)
		}
		if err != nil || packet.ParserClosure.ClosureSha256 != q.ParserClosure.ClosureSha256 {
			t.Errorf("structural-tuples/%s/TUPLE.json does not record the qualified closure: %v", tuple, err)
		}
	}
	admitted := false
	for _, tuple := range q.AdmittedTuples {
		admitted = admitted || tuple == runtime.Version()
	}
	if !admitted {
		t.Skipf("%s is not an admitted structural tuple", runtime.Version())
	}
	// go test exports the GOROOT of the toolchain that built this binary.
	goroot := os.Getenv("GOROOT")
	if goroot == "" {
		cmd := exec.Command("go", "env", "GOROOT")
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("locate the GOROOT of admitted %s: %v", runtime.Version(), err)
		}
		goroot = strings.TrimSpace(string(out))
	}
	c, err := digestToolchain(goroot)
	if err != nil {
		t.Fatal(err)
	}
	if c.GoVersion != runtime.Version() {
		t.Fatalf("GOROOT %s is %s, the test binary %s", goroot, c.GoVersion, runtime.Version())
	}
	if c.ClosureSha256 != q.ParserClosure.ClosureSha256 {
		t.Fatalf("%s parser closure %s, qualified %s", c.GoVersion, c.ClosureSha256, q.ParserClosure.ClosureSha256)
	}
}
