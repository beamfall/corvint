package doccorpus

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func fixture(t *testing.T) (string, Manifest) {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Corpus test")
	git(t, root, "config", "user.email", "corpus@example.invalid")
	files := map[string]string{"go.mod": "module example.invalid/corpus\n\ngo 1.27.1\n", "src/value.go": "package value\n\n// Count returns one.\nfunc Count() int { return 1 }\n", "src/value_test.go": "package value\nimport \"testing\"\nfunc TestCount(t *testing.T) { if Count()!=1 { t.Fatal(\"bad\") } }\n", "src/view.ts": "export function showCount() { return 1; }\n", "src/report.py": "def report_count():\n    return 1\n", "src/readme.md": "# Count\n\nCounts a value.\n"}
	for p, data := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "source")
	rev := git(t, root, "rev-parse", "HEAD")
	m, err := Inventory(context.Background(), root, rev, "src", "2026-09-19T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return root, m
}
func TestCorpusDeterministicRoundTrip(t *testing.T) {
	t.Run("DCP-V1-001 DCP-V1-002 DCP-V1-011 identity", func(t *testing.T) {
		root, m := fixture(t)
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		data, err := Encode(a)
		if err != nil {
			t.Fatal(err)
		}
		again, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		other, _ := Encode(again)
		if !bytes.Equal(data, other) {
			t.Fatal("DCP-V1-001 nondeterministic bytes")
		}
		receipt, err := ReadQuery(context.Background(), root, data, Request{Operation: "search", Query: "Count"})
		if err != nil {
			t.Fatal(err)
		}
		if len(receipt.Results) == 0 || receipt.Freshness != "fresh" {
			t.Fatalf("missing real result: %+v", receipt)
		}
		absent, _ := Query(a, Request{Operation: "journey", ID: "missing"}, "fresh", nil)
		if absent.Miss != "capability-absent" {
			t.Fatalf("absent capability masqueraded as empty: %+v", absent)
		}
		miss, _ := Query(a, Request{Operation: "search", Query: "nonexistentquokka"}, "fresh", nil)
		if miss.Miss != "no-match" {
			t.Fatal(miss)
		}
		if err := os.WriteFile(filepath.Join(root, "src/value.go"), []byte("package value\nfunc Count() int { return 99 }\n"), 0600); err != nil {
			t.Fatal(err)
		}
		stale, err := ReadQuery(context.Background(), root, data, Request{Operation: "info"})
		if err != nil {
			t.Fatal(err)
		}
		if stale.Freshness != "stale" {
			t.Fatal("dirty source hidden")
		}
	})
}
func TestCorpusClosedInputsAndTamper(t *testing.T) {
	t.Run("DCP-V1-003 DCP-V1-004 DCP-V1-018 closure", func(t *testing.T) {
		root, m := fixture(t)
		t.Run("missing declared", func(t *testing.T) {
			bad := m
			bad.Inputs = append([]Input{}, m.Inputs...)
			bad.Inputs[0].Path = "src/missing.go"
			if _, err := Build(context.Background(), root, bad); err == nil {
				t.Fatal("missing input accepted")
			}
		})
		t.Run("undeclared", func(t *testing.T) {
			bad := m
			bad.Inputs = m.Inputs[1:]
			if _, err := Build(context.Background(), root, bad); err == nil {
				t.Fatal("undeclared input accepted")
			}
		})
		t.Run("duplicate", func(t *testing.T) {
			bad := m
			bad.Inputs = append(append([]Input{}, m.Inputs...), m.Inputs[0])
			if _, err := Build(context.Background(), root, bad); err == nil {
				t.Fatal("duplicate input accepted")
			}
		})
		t.Run("provider unavailable", func(t *testing.T) {
			bad := m
			bad.Providers = append([]Provider{}, m.Providers...)
			bad.Providers[0].Kind = "imaginary"
			if _, err := Build(context.Background(), root, bad); err == nil {
				t.Fatal("unavailable provider accepted")
			}
		})
		t.Run("tamper", func(t *testing.T) {
			a, err := Build(context.Background(), root, m)
			if err != nil {
				t.Fatal(err)
			}
			a.Claims[0].Text = "invented behavior"
			a.SHA256 = ""
			a.SHA256 = testHash(t, a)
			data, _ := Encode(a)
			if _, err := Open(context.Background(), root, data); err == nil {
				t.Fatal("rehashing tampered claims accepted")
			}
		})
		t.Run("duplicate key", func(t *testing.T) {
			if _, err := ParseManifest([]byte(`{"schema":"x","schema":"y"}`)); err == nil {
				t.Fatal("duplicate key accepted")
			}
		})
	})
}

func testHash(t *testing.T, value any) string {
	t.Helper()
	digest, err := hashValue(value)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func TestHashValueRefusesOversizedValue(t *testing.T) {
	if _, err := hashValue(strings.Repeat("a", MaxBytes)); err == nil || !strings.Contains(err.Error(), "output bound exceeded") {
		t.Fatalf("oversized value hashed instead of refused: %v", err)
	}
}
