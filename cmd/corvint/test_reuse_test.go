// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func reuseFixtureWrite(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func reuseFixtureCommit(t *testing.T, root, message string) string {
	t.Helper()
	materializationGit(t, root, "add", "-A")
	materializationGit(t, root, "commit", "-qm", message)
	return strings.TrimSpace(materializationGit(t, root, "rev-parse", "HEAD"))
}

// TestTestReuseCLI is AFP-V0-2323 to AFP-V0-2325 end to end: a record from a
// go test -json stream, a decision that reuses the declared package after an
// unrelated commit and runs it after a change to its scope, and the usage
// refusals.
func TestTestReuseCLI(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOWORK", "off")
	root := materializationFixture(t)
	reuseFixtureWrite(t, root, map[string]string{
		"go.mod":                         "module example.com/m\n\ngo 1.27\n",
		"a/a.go":                         "package a\n",
		"a/a_test.go":                    "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n",
		"data/a.txt":                     "ok\n",
		"docs/readme.md":                 "# r\n",
		".corvint/test-read-scopes.json": `{"profile":"corvint-test-read-scopes/0","packages":{"a":["data/a.txt"]}}` + "\n",
	})
	reuseFixtureCommit(t, root, "base")
	scratch := t.TempDir()
	stream := filepath.Join(scratch, "run.json")
	if err := os.WriteFile(stream, []byte(`{"Action":"pass","Package":"example.com/m/a"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	invoke := func(wantCode int, arguments ...string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := run(append([]string{"--root", root, "test", "reuse"}, arguments...), nil, &stdout, &stderr)
		if code != wantCode {
			t.Fatalf("test reuse %v: exit %d, want %d\n%s%s", arguments, code, wantCode, stdout.String(), stderr.String())
		}
		return stdout.String() + stderr.String()
	}
	record := filepath.Join(scratch, "record.json")
	invoke(0, "record", "--go-test-json", stream, "--argv", "-count=1", "--out", record)
	decide := func() map[string]string {
		t.Helper()
		out := filepath.Join(scratch, "decision.json")
		invoke(0, "decide", "--argv", "-count=1", "--records", record, "--out", out, "--run-list", filepath.Join(scratch, "run.txt"))
		body, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		var decision struct {
			Units []struct {
				Package  string `json:"package"`
				Decision string `json:"decision"`
				Reason   string `json:"reason"`
			} `json:"units"`
		}
		if err := json.Unmarshal(body, &decision, json.RejectUnknownMembers(false)); err != nil {
			t.Fatal(err)
		}
		units := map[string]string{}
		for _, unit := range decision.Units {
			units[strings.TrimPrefix(unit.Package, "example.com/m/")] = unit.Decision + " " + unit.Reason
		}
		return units
	}
	reuseFixtureWrite(t, root, map[string]string{"docs/readme.md": "# changed\n"})
	reuseFixtureCommit(t, root, "docs")
	if got := decide(); got["a"] != "REUSE RECORD_DIGEST_EQUAL" {
		t.Fatalf("unrelated commit: %v", got)
	}
	reuseFixtureWrite(t, root, map[string]string{"data/a.txt": "changed\n"})
	reuseFixtureCommit(t, root, "scope")
	if got := decide(); got["a"] != "RUN DIGEST_CHANGED" {
		t.Fatalf("scope change: %v", got)
	}
	if body, _ := os.ReadFile(filepath.Join(scratch, "run.txt")); strings.TrimSpace(string(body)) != "example.com/m/a" {
		t.Fatalf("run list %q", body)
	}
	invoke(2, "record", "--go-test-json", stream, "--argv", "-count=1 -run TestA", "--out", record)
	invoke(2, "decide", "--argv", "-count=1", "--trust", "pull_request")
	invoke(2, "record", "--argv", "-count=1", "--out", record)
	invoke(2, "explain")
}
