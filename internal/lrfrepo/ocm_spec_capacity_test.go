// SPDX-License-Identifier: Apache-2.0
package lrfrepo

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// SRG-V0-744/745: each lane is admitted by OCM and the generator on its own,
// but their union refuses with every added ID attributed to the supplied base.
func TestSpecCapacityCombinedLanes(t *testing.T) {
	a, err := os.ReadFile("testdata/spec-capacity/lane-a.md")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("testdata/spec-capacity/lane-b.md")
	if err != nil {
		t.Fatal(err)
	}
	marker := []byte("## Requirements\n\n")
	combined := append(bytes.Clone(a), bytes.SplitN(b, marker, 2)[1]...)
	for name, data := range map[string][]byte{"a": a, "b": b, "combined": combined} {
		want := ""
		if name == "combined" {
			want = "too-many-obligations"
		}
		if got := IntentRequirementsRefusal(data); got != want {
			t.Fatalf("%s: refusal %q, want %q", name, got, want)
		}
	}

	root := t.TempDir()
	run := func(env []string, wantOK bool, command string, args ...string) string {
		t.Helper()
		cmd := exec.Command(command, args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), env...)
		out, err := cmd.CombinedOutput()
		if (err == nil) != wantOK {
			t.Fatalf("%s %v: %v\n%s", command, args, err, out)
		}
		return string(out)
	}
	write := func(path string, data []byte) {
		t.Helper()
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	script, err := os.ReadFile("../../script/gen-spec-requirements.sh")
	if err != nil {
		t.Fatal(err)
	}
	write("script/gen-spec-requirements.sh", script)
	run(nil, true, "git", "init", "-q")
	stage := func(data []byte) {
		t.Helper()
		write("docs/specs/combined.md", data)
		run(nil, true, "git", "add", "docs/specs/combined.md")
	}
	for _, data := range [][]byte{a, b} {
		stage(data)
		run(nil, true, "sh", "script/gen-spec-requirements.sh", "--capacity")
		run(nil, true, "sh", "script/gen-spec-requirements.sh")
	}
	// An immutable tree is enough for the base commit; no branch or user Git config changes.
	stage(a)
	snapshot := func() string {
		t.Helper()
		tree := strings.TrimSpace(run(nil, true, "git", "write-tree"))
		return strings.TrimSpace(run([]string{"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid"}, true,
			"git", "commit-tree", tree, "-m", "fixture base"))
	}
	base := snapshot()
	stage(combined)
	for _, mode := range []string{"", "--capacity"} {
		args := []string{"script/gen-spec-requirements.sh"}
		if mode != "" {
			args = append(args, mode)
		}
		out := run([]string{"BASE=" + base}, false, "sh", args...)
		for _, want := range []string{"docs/specs/combined.md", "257 obligations; the limit is 256", "companion spec with its own requirement prefix", "requirement IDs added since BASE=" + base} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: missing %q in %s", mode, want, out)
			}
		}
		added := strings.SplitN(out, "requirement IDs added since BASE=", 2)[1]
		if strings.Contains(added, "CAPACITY-V0-001") || !strings.Contains(added, "CAPACITY-V0-129") || !strings.Contains(added, "CAPACITY-V0-257") {
			t.Errorf("base difference lost lane attribution: %s", added)
		}
	}
	// 256 remains an admitted OCM input. Capacity refuses only when IDs were added since BASE.
	full := combined[:bytes.LastIndex(combined, []byte("- `CAPACITY-V0-257`"))]
	if got := IntentRequirementsRefusal(full); got != "" {
		t.Fatalf("OCM must still admit 256: %s", got)
	}
	stage(full)
	run(nil, true, "sh", "script/gen-spec-requirements.sh")
	out := run([]string{"BASE=" + base}, false, "sh", "script/gen-spec-requirements.sh", "--capacity")
	for _, want := range []string{"256 obligations; the limit is 256", "requirement IDs added since BASE=" + base, "CAPACITY-V0-256"} {
		if !strings.Contains(out, want) {
			t.Errorf("newly filled spec: missing %q in %s", want, out)
		}
	}
	fullBase := snapshot()
	for _, env := range [][]string{nil, {"BASE=" + fullBase}} {
		out = run(env, true, "sh", "script/gen-spec-requirements.sh", "--capacity")
		for _, want := range []string{"warning:", "256 of the 256 obligations", "companion spec with its own requirement prefix"} {
			if !strings.Contains(out, want) {
				t.Errorf("unchanged or unattributed full spec: missing %q in %s", want, out)
			}
		}
		if env != nil && !strings.Contains(out, "requirement IDs added since BASE="+fullBase+" ("+fullBase+"): (none)") {
			t.Errorf("unchanged full spec lost base attribution: %s", out)
		}
	}
	// Capacity is a generator mode: it must retain the shared duplicate-definition refusal.
	stage(append(bytes.Clone(a), []byte("- `CAPACITY-V0-001`: (proposed) Duplicate.\n")...))
	out = run(nil, false, "sh", "script/gen-spec-requirements.sh", "--capacity")
	if !strings.Contains(out, "CAPACITY-V0-001: defined more than once") {
		t.Errorf("capacity bypassed shared definition validation: %s", out)
	}

}
