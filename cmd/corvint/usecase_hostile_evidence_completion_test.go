package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const completionMap = ".corvint/change.cem.json"

// completionCase is one hostile input to the UC-EVIDENCE-CARRYING-COMPLETION
// entrypoints (`corvint cem`, `corvint dogfood`). Category is the UCV0-006
// case category and class is the V1-0188 hostile class it exercises.
type completionCase struct {
	category, class, name string
	run                   func(t *testing.T)
}

// TestUseCaseHostileEvidenceCompletion retains the hostile-tests evidence for
// UC-EVIDENCE-CARRYING-COMPLETION (V1-0188). Negative rows are invalid inputs
// refused with a named code; hostile rows are adversarial repository or map
// states that are refused or leave the verdict unchanged; abstention rows show
// that missing evidence never yields a complete verdict (DCW-V0-005/015).
// The dirty-worktree refusal of the dogfood loop lives in
// script/dogfood-check.sh, outside the Go CLI, so the CLI rows here pin the
// closest contracted behaviour: status is derived from commits only and
// `cem anchor` refuses a dirty or uncommitted map.
func TestUseCaseHostileEvidenceCompletion(t *testing.T) {
	t.Parallel()
	cases := []completionCase{
		// negative: invalid inputs refused with a named code.
		{"negative", "missing-anchors", "citation of an absent evidence path is refused", func(t *testing.T) {
			root, base, target := cemRepo(t)
			completionPrepare(t, root, base, target)
			completionRefusal(t, 2, "missing-evidence", "--root", root, "cem", "cite", "--map", completionMap, "--hunk", "1",
				"--evidence-path", "docs/missing.txt", "--lines", "1:1", "--relation", "specification")
		}},
		{"negative", "missing-anchors", "status against the wrong base is invalid", func(t *testing.T) {
			root, base, target := cemRepo(t)
			completionCandidate(t, root, base, target)
			out := completionRefusal(t, 1, "base-revision-mismatch", "--root", root, "cem", "status", "--map", completionMap,
				"--expected-base", target, "--target", "HEAD")
			if state := decodeObject(t, []byte(out))["state"]; state != "invalid" {
				t.Fatalf("state = %v, want invalid: %s", state, out)
			}
		}},
		{"negative", "malformed-provider-records", "map with an unsupported spec is refused", func(t *testing.T) {
			root, base, target := cemRepo(t)
			completionPrepare(t, root, base, target)
			rewriteRecord(t, filepath.Join(root, completionMap), `"spec": "cem/0.2"`, `"spec": "cem/9.9"`)
			completionRefusal(t, 2, "unsupported-spec", "--root", root, "cem", "verify", "--map", completionMap)
		}},
		{"negative", "dirty-worktree", "anchoring an uncommitted map is refused", func(t *testing.T) {
			root, base, target := cemRepo(t)
			completionPrepare(t, root, base, target)
			completionRefusal(t, 2, "anchor-map-uncommitted", "--root", root, "cem", "anchor", "--map", completionMap)
		}},
		// hostile: adversarial states are refused or leave the verdict unchanged.
		{"hostile", "stale-index", "candidate map behind a later commit is invalid", func(t *testing.T) {
			root, base, target := cemRepo(t)
			completionCandidate(t, root, base, target)
			cemWrite(t, root, "src/app.txt", "alpha\nBETA\ngamma\n")
			cemGit(t, root, "commit", "-qam", "later")
			out := completionRefusal(t, 1, "patch-digest-mismatch", "--root", root, "cem", "status", "--map", completionMap,
				"--expected-base", base, "--target", "HEAD")
			if state := decodeObject(t, []byte(out))["state"]; state != "invalid" {
				t.Fatalf("state = %v, want invalid: %s", state, out)
			}
		}},
		{"hostile", "dirty-worktree", "status is derived from commits, not the worktree", func(t *testing.T) {
			root, base, target := cemRepo(t)
			completionCandidate(t, root, base, target)
			arguments := []string{"--root", root, "cem", "status", "--map", completionMap, "--expected-base", base, "--target", "HEAD"}
			code, clean, stderr := runCLI(t, arguments...)
			if code != 0 {
				t.Fatalf("status exit %d: %s", code, stderr)
			}
			cemWrite(t, root, "src/app.txt", "alpha\nBETA\ndirty\n")
			if code, dirty, _ := runCLI(t, arguments...); code != 0 || dirty != clean {
				t.Fatalf("a worktree edit changed the status (exit %d):\n%s\n%s", code, clean, dirty)
			}
		}},
		{"hostile", "dirty-worktree", "anchoring a dirty committed map is refused", func(t *testing.T) {
			root, base, target := cemRepo(t)
			completionCandidate(t, root, base, target)
			appendFile(t, filepath.Join(root, completionMap), "\n")
			completionRefusal(t, 2, "anchor-map-dirty", "--root", root, "cem", "anchor", "--map", completionMap)
		}},
		{"hostile", "malformed-provider-records", "truncated map is invalid JSON", func(t *testing.T) {
			root, base, target := cemRepo(t)
			completionPrepare(t, root, base, target)
			body, err := os.ReadFile(filepath.Join(root, completionMap))
			if err != nil {
				t.Fatal(err)
			}
			cemWrite(t, root, completionMap, string(body[:len(body)/2]))
			completionRefusal(t, 2, "invalid-json", "--root", root, "cem", "verify", "--map", completionMap)
		}},
		{"hostile", "malformed-provider-records", "map with a duplicate key is invalid JSON", func(t *testing.T) {
			root, base, target := cemRepo(t)
			completionPrepare(t, root, base, target)
			rewriteRecord(t, filepath.Join(root, completionMap), `"spec": "cem/0.2"`, `"spec": "cem/0.2", "spec": "cem/0.2"`)
			completionRefusal(t, 2, "invalid-json", "--root", root, "cem", "verify", "--map", completionMap)
		}},
		{"hostile", "symlinked-or-relocated-subjects", "symlinked evidence path is not evidence", func(t *testing.T) {
			root, _, _ := cemRepo(t)
			if err := os.Symlink("rule.txt", filepath.Join(root, "docs", "alias.txt")); err != nil {
				t.Fatal(err)
			}
			cemGit(t, root, "add", ".")
			cemGit(t, root, "commit", "-qm", "alias")
			base := cemGit(t, root, "rev-parse", "HEAD")
			cemWrite(t, root, "src/app.txt", "alpha\nGAMMA\n")
			cemGit(t, root, "commit", "-qam", "change")
			completionPrepare(t, root, base, cemGit(t, root, "rev-parse", "HEAD"))
			completionRefusal(t, 2, "missing-evidence", "--root", root, "cem", "cite", "--map", completionMap, "--hunk", "1",
				"--evidence-path", "docs/alias.txt", "--lines", "1:1", "--relation", "specification")
		}},
		{"hostile", "symlinked-or-relocated-subjects", "symlinked map is not read", func(t *testing.T) {
			root, base, target := cemRepo(t)
			completionPrepare(t, root, base, target)
			if err := os.Symlink("change.cem.json", filepath.Join(root, ".corvint", "link.json")); err != nil {
				t.Fatal(err)
			}
			completionRefusal(t, 2, "cannot read CEM map", "--root", root, "cem", "verify", "--map", ".corvint/link.json")
		}},
		{"hostile", "symlinked-or-relocated-subjects", "symlinked .git marker is refused", func(t *testing.T) {
			root, base, target := cemRepo(t)
			completionCandidate(t, root, base, target)
			gitDirectory := filepath.Join(t.TempDir(), "git")
			if err := os.Rename(filepath.Join(root, ".git"), gitDirectory); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(gitDirectory, filepath.Join(root, ".git")); err != nil {
				t.Fatal(err)
			}
			completionRefusal(t, 2, "repository-object-unavailable", "--root", root, "cem", "status", "--map", completionMap,
				"--expected-base", base, "--target", "HEAD")
		}},
		{"hostile", "symlinked-or-relocated-subjects", "map relocated below a subdirectory root is refused", func(t *testing.T) {
			root, base, target := cemRepo(t)
			completionPrepare(t, root, base, target)
			body, err := os.ReadFile(filepath.Join(root, completionMap))
			if err != nil {
				t.Fatal(err)
			}
			cemWrite(t, root, "docs/"+completionMap, string(body))
			completionRefusal(t, 2, "repository-object-unavailable", "--root", filepath.Join(root, "docs"), "cem", "status",
				"--map", completionMap, "--expected-base", base, "--target", "HEAD")
		}},
		// abstention: missing evidence is counted unknown, never complete.
		{"abstention", "missing-anchors", "uncited hunk stays unknown", func(t *testing.T) {
			root, base, target := cemRepo(t)
			completionCandidate(t, root, base, target)
			code, out, stderr := runCLI(t, "--root", root, "cem", "status", "--map", completionMap, "--expected-base", base, "--target", "HEAD")
			if code != 0 {
				t.Fatalf("status exit %d: %s", code, stderr)
			}
			counts := decodeObject(t, []byte(out))["counts"].(map[string]any)
			if counts["unknown"] != 1.0 || counts["supported"] != 0.0 {
				t.Fatalf("an uncited hunk must be unknown, never supported: %v", counts)
			}
		}},
		{"abstention", "missing-anchors", "uncited hunk fails a zero-unknown policy", func(t *testing.T) {
			root, base, target := cemRepo(t)
			completionCandidate(t, root, base, target)
			completionRefusal(t, 1, "max-unknown-exceeded", "--root", root, "cem", "status", "--map", completionMap,
				"--expected-base", base, "--target", "HEAD", "--max-unknown", "0")
		}},
		{"abstention", "missing-anchors", "unenrolled worktree is not satisfied", func(t *testing.T) {
			root, _, _ := cemRepo(t)
			code, out, stderr := runCLI(t, "--root", root, "dogfood", "status", "--session-key", strings.Repeat("ab", 32))
			if code != 0 {
				t.Fatalf("dogfood status exit %d: %s", code, stderr)
			}
			var status struct {
				Policy struct {
					Lifecycle string `json:"lifecycle"`
					Satisfied *bool  `json:"satisfied"`
				} `json:"policy"`
			}
			if err := json.Unmarshal([]byte(out), &status); err != nil {
				t.Fatal(err)
			}
			if status.Policy.Lifecycle != "inactive" || status.Policy.Satisfied == nil || *status.Policy.Satisfied {
				t.Fatalf("an unenrolled worktree must be inactive and unsatisfied: %s", out)
			}
		}},
	}
	for _, test := range cases {
		t.Run(test.category+"/"+test.class+"/"+test.name, func(t *testing.T) {
			t.Parallel()
			test.run(t)
		})
	}
}

func completionPrepare(t *testing.T, root, base, target string) {
	t.Helper()
	if code, _, stderr := runCLI(t, "--root", root, "cem", "prepare", "--base", base, "--target", target); code != 0 {
		t.Fatalf("prepare exit %d: %s", code, stderr)
	}
}

// completionCandidate prepares the map and commits it as the candidate, so
// status resolves the change from base to HEAD with one uncited hunk.
func completionCandidate(t *testing.T, root, base, target string) {
	t.Helper()
	completionPrepare(t, root, base, target)
	cemGit(t, root, "add", completionMap)
	cemGit(t, root, "commit", "-qm", "candidate")
}

func completionRefusal(t *testing.T, exit int, reason string, arguments ...string) string {
	t.Helper()
	code, out, stderr := runCLI(t, arguments...)
	if code != exit || !strings.Contains(out+stderr, reason) {
		t.Fatalf("exit %d, want %d with %q: %s %s", code, exit, reason, out, stderr)
	}
	return out
}
