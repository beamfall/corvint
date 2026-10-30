//go:build unix

package main

import (
	"encoding/json"
	"testing"
)

func TestCorpusWorkEvidenceIsNonAuthoritative(t *testing.T) {
	t.Run("DCP-V1-015 work", func(t *testing.T) {
		root := workProductionFixture(t)
		cemWrite(t, root, ".gitignore", "corpus-input.json\ncorpus.json\n")
		materializationGit(t, root, "add", ".gitignore")
		materializationGit(t, root, "commit", "-qm", "local corpus output policy")
		revision := materializationGit(t, root, "rev-parse", "HEAD")
		writeCorpusFixture(t, root, revision, "go.mod")
		code, out, stderr := corpusCLI(t, root, "work", "observe", "--corpus=corpus.json")
		if code != 0 {
			t.Fatalf("work observe: %d %s %s", code, out, stderr)
		}
		var result map[string]json.RawMessage
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatal(err)
		}
		var doc struct {
			TaskAuthority bool   `json:"task_authority"`
			Authority     string `json:"authority"`
		}
		if err := json.Unmarshal(result["documentation"], &doc); err != nil {
			t.Fatal(err)
		}
		if doc.TaskAuthority || doc.Authority != "generated-documentation" || string(result["state"]) != `"OK"` {
			t.Fatalf("work state or evidence authority changed: %s", out)
		}
	})
}
