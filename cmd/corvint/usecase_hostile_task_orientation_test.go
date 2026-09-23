package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

const orientationTask = "does `Split` keep empty keys"

// orientationCase is one hostile input to the UC-TASK-ORIENTATION
// entrypoints (`corvint context`, `corvint query`). Category is the UCV0-006
// case category and class is the V1-0188 hostile class it exercises.
type orientationCase struct {
	category, class, name string
	run                   func(t *testing.T, root string)
}

// TestUseCaseHostileTaskOrientation retains the hostile-tests evidence for
// UC-TASK-ORIENTATION (V1-0188). Each row asserts the contracted outcome: a
// refusal with its named reason (negative), a miss or degradation that never
// changes the committed answer (hostile), or an explicit abstention.
// The cases are serial because the malformed-provider row sets PATH.
func TestUseCaseHostileTaskOrientation(t *testing.T) {
	cases := []orientationCase{
		// negative: refusals with a named reason.
		{"negative", "missing-anchors", "untracked subject is refused", func(t *testing.T, root string) {
			cemWrite(t, root, "untracked.go", "package main\n")
			orientationRefusal(t, root, "untracked.go", "subject path is not tracked at revision")
		}},
		{"negative", "missing-anchors", "absent subject is refused", func(t *testing.T, root string) {
			orientationRefusal(t, root, "missing.go", "subject path is not tracked at revision")
		}},
		{"negative", "symlinked-or-relocated-subjects", "subject outside the repository is refused", func(t *testing.T, root string) {
			orientationRefusal(t, root, "../outside.go", "impact path must be normalized and repository-relative")
		}},
		{"negative", "symlinked-or-relocated-subjects", "uncommitted relocation is refused at its new path", func(t *testing.T, root string) {
			gitFixture(t, root, "mv", "cache/demux.go", "cache/moved.go")
			orientationRefusal(t, root, "cache/moved.go", "subject path is not tracked at revision")
		}},
		// hostile: the answer stays pinned to the committed tree.
		{"hostile", "stale-index", "snapshot behind HEAD is a miss", func(t *testing.T, root string) {
			runIndexForTest(t, root, false)
			cemWrite(t, root, "cache/fresh.go", "package cache\n\nfunc Fresh() string { return \"\" }\n")
			gitFixture(t, root, "add", "-A")
			gitFixture(t, root, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "fresh")
			assertSnapshotIsAMiss(t, root, "where is `Fresh` defined", "context", "query")
		}},
		{"hostile", "stale-index", "corrupt snapshot is a miss", func(t *testing.T, root string) {
			runIndexForTest(t, root, false)
			entries, err := os.ReadDir(contextindex.SnapshotDirectory(root))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".gob") {
					cemWrite(t, root, ".corvint/index/"+entry.Name(), "not a gob snapshot")
				}
			}
			assertSnapshotIsAMiss(t, root, orientationTask, "context", "query")
		}},
		{"hostile", "stale-index", "symlinked snapshot directory is a miss", func(t *testing.T, root string) {
			runIndexForTest(t, root, false)
			elsewhere := filepath.Join(t.TempDir(), "index")
			if err := os.Rename(contextindex.SnapshotDirectory(root), elsewhere); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(elsewhere, contextindex.SnapshotDirectory(root)); err != nil {
				t.Fatal(err)
			}
			// Git sees the symlink as an untracked path, so query honestly
			// labels it mixed-worktree; context is the byte-identical miss.
			freshness := orientationQuery(t, root, orientationTask)["freshness"].(map[string]any)
			if paths, _ := freshness["mixed_paths"].([]any); freshness["state"] != "mixed-worktree" || len(paths) != 1 || paths[0] != ".corvint/index" {
				t.Fatalf("query does not label the symlinked snapshot: %v", freshness)
			}
			assertSnapshotIsAMiss(t, root, orientationTask, "context")
		}},
		{"hostile", "dirty-worktree", "context answers from the committed tree only", func(t *testing.T, root string) {
			appendFile(t, filepath.Join(root, "cache", "demux.go"), "\nfunc Extra() string { return \"\" }\n")
			packet := orientationContext(t, root, "where is `Extra` defined", "cache/demux_test.go")
			if packet["revision"] != gitFixture(t, root, "rev-parse", "HEAD^{tree}") {
				t.Fatalf("revision is not the HEAD tree: %v", packet["revision"])
			}
			committed := gitFixture(t, root, "rev-parse", "HEAD:cache/demux.go")
			evidence := string(mustJSON(t, packet["results"]))
			if strings.Contains(evidence, gitFixture(t, root, "hash-object", "cache/demux.go")) {
				t.Fatalf("a worktree-only definition was cited: %s", evidence)
			}
			if !strings.Contains(evidence, `"blob_hash":"`+committed+`"`) {
				t.Fatalf("evidence is not pinned to the committed blob %s: %s", committed, evidence)
			}
		}},
		{"hostile", "dirty-worktree", "query labels the mixed worktree", func(t *testing.T, root string) {
			appendFile(t, filepath.Join(root, "cache", "demux.go"), "\n// dirty\n")
			freshness := orientationQuery(t, root, "where is Split defined")["freshness"].(map[string]any)
			paths, _ := freshness["mixed_paths"].([]any)
			if freshness["state"] != "mixed-worktree" || len(paths) != 1 || paths[0] != "cache/demux.go" {
				t.Fatalf("freshness does not label the dirty path: %v", freshness)
			}
		}},
		{"hostile", "symlinked-or-relocated-subjects", "tracked symlink subject pins the link blob and marks references incomplete", func(t *testing.T, root string) {
			if err := os.Symlink("cache/demux.go", filepath.Join(root, "link.go")); err != nil {
				t.Fatal(err)
			}
			gitFixture(t, root, "add", "-A")
			gitFixture(t, root, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "link")
			packet := orientationContext(t, root, orientationTask, "link.go")
			subject := packet["subject"].(map[string]any)
			if subject["blob_hash"] != gitFixture(t, root, "rev-parse", "HEAD:link.go") {
				t.Fatalf("subject is not pinned to the link's own blob: %v", subject)
			}
			unexamined := string(mustJSON(t, packet["coverage"].(map[string]any)["unexamined"]))
			if !strings.Contains(unexamined, `{"relation":"reference","state":"subject-symbols-incomplete"`) {
				t.Fatalf("the reference relation is not marked incomplete: %s", unexamined)
			}
		}},
		{"hostile", "malformed-provider-records", "malformed LSP frame degrades to an unavailable provider row", func(t *testing.T, root string) {
			fake := t.TempDir()
			server := "#!/bin/sh\nprintf 'Content-Length: 9\\r\\n\\r\\n{\"id\":1,'\nexit 0\n"
			if err := os.WriteFile(filepath.Join(fake, "gopls"), []byte(server), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("CORVINT_CONTEXT_LSP", "gopls")
			packet := orientationContext(t, root, orientationTask, "cache/demux.go")
			external, _ := packet["external"].(map[string]any)
			providers, _ := external["providers"].([]any)
			if len(providers) != 1 {
				t.Fatalf("one provider row: %v", external)
			}
			row := providers[0].(map[string]any)
			if row["state"] != "unavailable" || row["reason"] != "gopls session failed" {
				t.Fatalf("a malformed record must be a visible unavailable row: %v", row)
			}
			if results, _ := row["results"].([]any); len(results) != 0 {
				t.Fatalf("an unavailable provider contributed results: %v", row)
			}
		}},
		// abstention: no candidate is invented.
		{"abstention", "missing-anchors", "query with no relevant candidate abstains", func(t *testing.T, root string) {
			answer := orientationQuery(t, root, "zzqx frobnicate quuxwidget")
			abstention, results := answer["abstention"].(map[string]any), answer["results"].([]any)
			if abstention["active"] != true || abstention["reason"] != "no-relevant-candidates" || len(results) != 0 {
				t.Fatalf("abstention = %v with %d results, want no-relevant-candidates", abstention, len(results))
			}
			if answer["freshness"].(map[string]any)["state"] != "fresh" {
				t.Fatalf("clean tree is not fresh: %v", answer["freshness"])
			}
		}},
		{"abstention", "dirty-worktree", "query abstains on unindexed worktree changes", func(t *testing.T, root string) {
			appendFile(t, filepath.Join(root, "cache", "demux.go"), "\n// dirty\n")
			answer := orientationQuery(t, root, "zzqx frobnicate quuxwidget")
			abstention, results := answer["abstention"].(map[string]any), answer["results"].([]any)
			if abstention["active"] != true || abstention["reason"] != "unindexed-worktree-changes" || len(results) != 0 {
				t.Fatalf("abstention = %v with %d results, want unindexed-worktree-changes", abstention, len(results))
			}
		}},
		{"abstention", "missing-anchors", "context without specific terms reports no answerability", func(t *testing.T, root string) {
			packet := orientationContext(t, root, "zzqx frobnicate quuxwidget", "cache/demux.go")
			answerability := packet["coverage"].(map[string]any)["answerability"].(map[string]any)
			if answerability["verdict"] != "no-specific-terms" {
				t.Fatalf("answerability = %v, want no-specific-terms", answerability)
			}
		}},
	}
	for _, test := range cases {
		t.Run(test.category+"/"+test.class+"/"+test.name, func(t *testing.T) {
			test.run(t, taskContextRepository(t))
		})
	}
}

func orientationRefusal(t *testing.T, root, subject, reason string) {
	t.Helper()
	code, stdout, stderr := runContextCommand(t, "--root", root, "context", "--task", orientationTask, "--subject", subject)
	if code != 2 || !strings.Contains(string(stdout)+stderr, reason) {
		t.Fatalf("exit %d, want 2 with %q: %s %s", code, reason, stdout, stderr)
	}
}

func orientationContext(t *testing.T, root, task, subject string) map[string]any {
	t.Helper()
	code, stdout, stderr := runContextCommand(t, "--root", root, "context", "--task", task, "--subject", subject)
	if code != 0 {
		t.Fatalf("context exit %d: %s", code, stderr)
	}
	return decodeObject(t, stdout)
}

func orientationQuery(t *testing.T, root, task string) map[string]any {
	t.Helper()
	code, stdout, stderr := runContextCommand(t, "--root", root, "query", "--task", task)
	if code != 0 {
		t.Fatalf("query exit %d: %s", code, stderr)
	}
	return decodeObject(t, stdout)["context"].(map[string]any)
}

// assertSnapshotIsAMiss pins IDX-SNAP-V0-003/005/006: with the snapshot in its
// hostile state, each named command prints exactly the bytes of a cold build
// of the committed tree at HEAD.
func assertSnapshotIsAMiss(t *testing.T, root, task string, commands ...string) {
	t.Helper()
	arguments := map[string][]string{
		"context": {"--root", root, "context", "--task", task, "--subject", "cache/demux.go"},
		"query":   {"--root", root, "query", "--task", task},
	}
	var invocations [][]string
	for _, command := range commands {
		invocations = append(invocations, arguments[command])
	}
	var hostile [][]byte
	for _, arguments := range invocations {
		code, stdout, stderr := runContextCommand(t, arguments...)
		if code != 0 {
			t.Fatalf("%v exit %d: %s", arguments, code, stderr)
		}
		hostile = append(hostile, stdout)
	}
	if err := os.RemoveAll(filepath.Join(root, ".corvint")); err != nil {
		t.Fatal(err)
	}
	for index, arguments := range invocations {
		_, cold, _ := runContextCommand(t, arguments...)
		if !bytes.Equal(hostile[index], cold) {
			t.Fatalf("%v differs from the cold build:\n%s\n%s", arguments, hostile[index], cold)
		}
	}
	if revision := decodeObject(t, hostile[0])["revision"]; revision != gitFixture(t, root, "rev-parse", "HEAD^{tree}") {
		t.Fatalf("answer revision %v is not the HEAD tree", revision)
	}
}
