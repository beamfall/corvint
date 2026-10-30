package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// treeDigest lists every path under root with its size and mode, so a read
// that writes, renames or removes anything is observed.
func treeDigest(t *testing.T, root string) map[string]string {
	t.Helper()
	digest := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		digest[path] = fmt.Sprintf("%v %v %d", info.Mode(), info.ModTime(), info.Size())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

// externalMap copies the fixture's sealed map into a fresh directory outside
// the repository and returns its absolute path.
func externalMap(t *testing.T, f stableReadFixture) string {
	t.Helper()
	data, err := os.ReadFile(f.mapPath)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bundle", "cem.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// CEM-V1-315: status and verify read an absolute map outside the repository
// and report exactly what verify-stable reports for the same map, writing
// nothing in the repository or beside the map.
func TestCEMV1315StatusVerifyReadOutOfTreeMap(t *testing.T) {
	f := materializeStableRead(t)
	outside := externalMap(t, f)
	external := stableReadFixture{repo: f.repo, artifacts: f.artifacts, mapPath: outside,
		expectedBase: f.expectedBase, target: f.target, acceptGo: f.acceptGo}
	wantExit, want := runVerifyStable(t, external, f.artifacts)
	repoBefore, mapBefore := treeDigest(t, f.repo), treeDigest(t, filepath.Dir(outside))
	for _, action := range []string{"status", "verify"} {
		exit, envelope, refusal := runCoreRead(t, f.repo, action, "--map", outside,
			"--expected-base", f.expectedBase, "--target", f.target, "--artifacts", f.artifacts)
		if refusal != nil {
			t.Fatalf("%s refused an out-of-tree map: %v", action, refusal)
		}
		if exit != wantExit {
			t.Errorf("%s exit = %d, verify-stable exit = %d", action, exit, wantExit)
		}
		got, _ := envelope["stableVerification"].(map[string]any)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s result differs from verify-stable over the same map:\n got %v\nwant %v", action, got, want)
		}
		if envelope["mutates"] != false {
			t.Errorf("%s mutates = %v", action, envelope["mutates"])
		}
		if action == "status" && envelope["map"] != outside {
			t.Errorf("status map = %v, want the absolute path read %s", envelope["map"], outside)
		}
	}
	if !reflect.DeepEqual(treeDigest(t, f.repo), repoBefore) || !reflect.DeepEqual(treeDigest(t, filepath.Dir(outside)), mapBefore) {
		t.Error("an out-of-tree read changed the repository or the map's directory")
	}
}

// CEM-V1-316: an absolute map inside the worktree or its Git directory, one
// reached through a symlinked parent directory, and a symlinked map are
// refused before the verifier runs; a mutator writes an
// out-of-tree map only to a repository-relative --output (CEM-V1-647).
func TestCEMV1316OutOfTreeMapRefusals(t *testing.T) {
	f := materializeStableRead(t)
	outside := externalMap(t, f)
	gitMap := filepath.Join(f.repo, ".git", "cem.json")
	data, err := os.ReadFile(f.mapPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gitMap, data, 0o644); err != nil {
		t.Fatal(err)
	}
	scratch, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	linkedDir := filepath.Join(scratch, "into-repo")
	if err := os.Symlink(f.repo, linkedDir); err != nil {
		t.Fatal(err)
	}
	linkedMap := filepath.Join(scratch, "linked.cem.json")
	if err := os.Symlink(f.mapPath, linkedMap); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, action, mapPath, code, message string
	}{
		{"inside worktree", "status", f.mapPath, "invalid-arguments", "must lie outside the repository"},
		{"inside git dir", "verify", gitMap, "invalid-arguments", "must lie outside the repository"},
		{"symlinked parent", "verify", filepath.Join(linkedDir, "change.cem.json"), "invalid-arguments", "must lie outside the repository"},
		{"symlinked map", "status", linkedMap, "map-unavailable", ""},
		{"missing map", "verify", filepath.Join(scratch, "absent.json"), "map-unavailable", ""},
		{"mark needs a relative output", "mark", outside, "invalid-arguments", "only with a repository-relative --output"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			arguments := []string{c.action, "--map", c.mapPath,
				"--expected-base", f.expectedBase, "--target", f.target, "--artifacts", f.artifacts}
			if c.action == "mark" {
				arguments = []string{"mark", "--map", c.mapPath, "--hunk", "1", "--disposition", "unknown", "--reason", "insufficient-evidence"}
			}
			exit, envelope, refusal := runCoreRead(t, f.repo, arguments...)
			if envelope != nil || exit == 0 {
				t.Fatalf("exit %d envelope %v, want a refusal", exit, envelope)
			}
			if refusal["code"] != c.code || !strings.Contains(refusal["error"].(string), c.message) {
				t.Errorf("refusal = %v, want code %s containing %q", refusal, c.code, c.message)
			}
		})
	}
}

// CEM-V1-640: cem report reads an absolute map outside the repository like
// status and verify, with the same embedded verifier result as the
// repository-relative read; its review file may sit beside the map but never
// replaces it, and the map's bytes are unchanged.
func TestCEMV1640ReportReadsOutOfTreeMap(t *testing.T) {
	f := materializeStableRead(t)
	outside := externalMap(t, f)
	mapBytes, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	wantExit, want, _ := runCoreRead(t, f.repo, append(coreStableArguments("report", f, f.artifacts), "--format", "json")...)
	report := func(extra ...string) []string {
		return append([]string{"report", "--map", outside, "--expected-base", f.expectedBase, "--target", f.target, "--artifacts", f.artifacts}, extra...)
	}
	exit, envelope, refusal := runCoreRead(t, f.repo, report("--format", "json")...)
	if refusal != nil || exit != wantExit {
		t.Fatalf("json report over an out-of-tree map: exit %d (want %d) refusal %v", exit, wantExit, refusal)
	}
	if !reflect.DeepEqual(envelope["stableVerification"], want["stableVerification"]) {
		t.Fatalf("out-of-tree report differs from the repository-relative read:\n got %v\nwant %v", envelope["stableVerification"], want["stableVerification"])
	}
	beside := filepath.Join(filepath.Dir(outside), "review.md")
	exit, _, refusal = runCoreRead(t, f.repo, report("--output", beside)...)
	if refusal != nil || exit != wantExit {
		t.Fatalf("markdown report beside the map: exit %d refusal %v", exit, refusal)
	}
	if data, err := os.ReadFile(beside); err != nil || len(data) == 0 {
		t.Fatalf("review beside the map was not written: %v", err)
	}
	exit, envelope, refusal = runCoreRead(t, f.repo, report("--output", outside)...)
	if exit != 2 || envelope != nil || refusal["code"] != "invalid-arguments" || !strings.Contains(refusal["error"].(string), "report and map paths must differ") {
		t.Fatalf("report over its own map: exit %d envelope %v refusal %v", exit, envelope, refusal)
	}
	if after, err := os.ReadFile(outside); err != nil || string(after) != string(mapBytes) {
		t.Fatalf("the out-of-tree map changed: %v", err)
	}
}
