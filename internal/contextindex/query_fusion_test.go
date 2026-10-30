package contextindex

import (
	"context"
	"reflect"
	"testing"
)

// TestTaskLexicalChannelRanksTheNamedIdentifierFirst pins the lexical
// identifier channel (GPK-V0-092): the source that spells the task's
// identifier leads, names it in Identifiers, and carries the same lexical row
// the task-context fill renders; a source matched on prose alone names none.
func TestTaskLexicalChannelRanksTheNamedIdentifierFirst(t *testing.T) {
	t.Parallel()
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, map[string]string{
		"go.mod":          "module example.test/channel\n\ngo 1.27.0\n",
		"store/ledger.go": "package store\n\n// openLedgerFile opens the ledger.\nfunc openLedgerFile() {}\n",
		"store/cache.go":  "package store\n\nfunc warm() {}\n",
		"docs/ledger.md":  "# Ledger\n\nThe ledger file is opened once.\n",
		"native/ledger.c": "/* ledger file */\nint open_ledger(void) { return 0; }\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	channel, err := TaskLexicalChannel(context.Background(), index, "Fix openLedgerFile so the ledger file opens once", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(channel.Identifiers, []string{"openLedgerFile"}) || channel.Terms == 0 {
		t.Fatalf("channel searched %v over %d terms", channel.Identifiers, channel.Terms)
	}
	if len(channel.Candidates) == 0 || channel.Candidates[0].Path != "store/ledger.go" {
		t.Fatalf("candidates = %v, want store/ledger.go first", channel.Candidates)
	}
	lead := channel.Candidates[0]
	if !reflect.DeepEqual(lead.Identifiers, []string{"openLedgerFile"}) || lead.BM25 <= 0 {
		t.Fatalf("lead = %+v, want the identifier named with a positive score", lead)
	}
	if lead.Row["kind"] != "lexical" || lead.Row["id"] != "store/ledger.go" {
		t.Fatalf("lead row = %v", lead.Row)
	}
	evidence := mapsFromAny(lead.Row["evidence"])
	if len(evidence) != 1 || evidence[0]["path"] != "store/ledger.go" || evidence[0]["blob_hash"] == "" || evidence[0]["authority"] != "vocabulary" {
		t.Fatalf("lead evidence = %v", evidence)
	}
	for _, candidate := range channel.Candidates[1:] {
		if len(candidate.Identifiers) != 0 {
			t.Fatalf("%s names %v, want prose-only candidates after the lead", candidate.Path, candidate.Identifiers)
		}
		if candidate.Path == "docs/ledger.md" && candidate.Row["kind"] != "documentation" {
			t.Fatalf("documentation candidate rendered as %v", candidate.Row["kind"])
		}
	}
	if _, err := TaskLexicalChannel(context.Background(), index, "x", 0); err == nil {
		t.Fatal("limit 0 accepted")
	}
	if _, err := TaskLexicalChannel(context.Background(), index, "   ", 5); err == nil {
		t.Fatal("empty task accepted")
	}
}
