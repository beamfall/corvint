// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x")
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const pinnedReaders = `{"profile":"corvint-runtime-readers/0","entries":[{"path":"a.json","reason":"r"}]}`

// AFP-V0-710 (V1-3600): against a pinned planner commit, a new entry of known shape and a new
// declaration file pass, a profile bump passes, and a new member at a closed position fails,
// naming the file, position, member and pin and the compatible alternatives; an unclassified
// declaration and a pin absent from the clone fail too.
func TestClosedDeclarationMemberAgainstPin_V13600(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-q")
	writeFile(t, root, ".corvint/runtime-readers.json", pinnedReaders)
	writeFile(t, root, ".corvint/dogfood-report.json", `{"anything":1}`)
	writeFile(t, root, ".github/workflows/ci.yml", "env:\n  CORVINT_PR_TOOL_SOURCE: \"\"\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "pin")
	pinned := git(t, root, "rev-parse", "HEAD")
	writeFile(t, root, ".github/workflows/ci.yml", "env:\n  CORVINT_PR_TOOL_SOURCE: \""+pinned+"\"\n  CORVINT_TEST_SHADOW_SOURCE: \""+pinned+"\"\n")
	result := func() (int, string) {
		var stdout, stderr bytes.Buffer
		code := run(root, ".github/workflows/ci.yml", &stdout, &stderr)
		return code, stdout.String() + stderr.String()
	}

	writeFile(t, root, ".corvint/runtime-readers.json", `{"profile":"corvint-runtime-readers/0","entries":[{"path":"a.json","reason":"r"},{"path":"b.json","reason":"s"}]}`)
	writeFile(t, root, ".corvint/unbounded-reader-exclusions.json", `{"profile":"corvint-unbounded-reader-exclusions/0","paths":["a.json"]}`)
	if code, out := result(); code != 0 || !strings.Contains(out, "declaration-compat-check: ok") {
		t.Fatalf("an additive entry and a new file: code %d\n%s", code, out)
	}

	writeFile(t, root, ".corvint/runtime-readers.json", `{"profile":"corvint-runtime-readers/0","entries":[{"path":"a.json","reason":"r","excludeUnbounded":true}]}`)
	code, out := result()
	if code != 1 || !strings.Contains(out, `.corvint/runtime-readers.json at /entries/*: member "excludeUnbounded" is absent at pin `+pinned+` (CORVINT_PR_TOOL_SOURCE+CORVINT_TEST_SHADOW_SOURCE)`) ||
		!strings.Contains(out, "put it in a new declaration file") || !strings.Contains(out, "new profile value") || strings.Count(out, "FAIL") != 1 {
		t.Fatalf("a new closed member: code %d\n%s", code, out)
	}

	writeFile(t, root, ".corvint/runtime-readers.json", `{"profile":"corvint-runtime-readers/1","entries":[{"path":"a.json","reason":"r","excludeUnbounded":true}]}`)
	if code, out := result(); code != 0 {
		t.Fatalf("a versioned profile: code %d\n%s", code, out)
	}

	writeFile(t, root, ".corvint/new-declaration.json", `{"profile":"x/0"}`)
	if code, out := result(); code != 0 {
		t.Fatalf("an untracked declaration is no CI input: code %d\n%s", code, out)
	}
	git(t, root, "add", ".corvint/new-declaration.json")
	if code, out := result(); code != 1 || !strings.Contains(out, ".corvint/new-declaration.json is neither a closed planner declaration") {
		t.Fatalf("an unclassified declaration: code %d\n%s", code, out)
	}
	git(t, root, "rm", "-q", "-f", ".corvint/new-declaration.json")

	writeFile(t, root, ".github/workflows/ci.yml", "env:\n  CORVINT_PR_TOOL_SOURCE: \""+strings.Repeat("e", 40)+"\"\n")
	if code, out := result(); code != 1 || !strings.Contains(out, "is not in this clone; fetch it") {
		t.Fatalf("a missing pin: code %d\n%s", code, out)
	}
}

// AFP-V0-710 (Codex P2): the change-evidence map a dogfood run tracks between bind and seal, and
// the ignored change-outcome maps, status and report it leaves, are no planner input, so an active
// CEM and OCM leave the check passing; a tracked look-alike still fails.
func TestDogfoodMapsAreNoPlannerInput_V13600(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-q")
	writeFile(t, root, ".corvint/runtime-readers.json", pinnedReaders)
	writeFile(t, root, ".github/workflows/ci.yml", "env:\n  CORVINT_PR_TOOL_SOURCE: \"\"\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "pin")
	pinned := git(t, root, "rev-parse", "HEAD")
	writeFile(t, root, ".github/workflows/ci.yml", "env:\n  CORVINT_PR_TOOL_SOURCE: \""+pinned+"\"\n")
	writeFile(t, root, ".corvint/change.cem.json", `{"schema":"cem","hunks":[]}`)
	git(t, root, "add", ".corvint/change.cem.json")
	writeFile(t, root, ".corvint/change.ocm.001.json", `{"schema":"ocm"}`)
	writeFile(t, root, ".corvint/change.ocm-status.json", `{"status":"ready"}`)
	writeFile(t, root, ".corvint/dogfood-report.json", `{"profile":"r"}`)
	var stdout, stderr bytes.Buffer
	if code := run(root, ".github/workflows/ci.yml", &stdout, &stderr); code != 0 {
		t.Fatalf("an active CEM and OCM: code %d\n%s%s", code, stdout.String(), stderr.String())
	}
	writeFile(t, root, ".corvint/change.other.json", `{}`)
	git(t, root, "add", ".corvint/change.other.json")
	stdout.Reset()
	stderr.Reset()
	if code := run(root, ".github/workflows/ci.yml", &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), ".corvint/change.other.json is neither") {
		t.Fatalf("a look-alike name was exempted: code %d\n%s", code, stderr.String())
	}
}
