package contextindex

import (
	"context"
	"testing"
)

// TCP-V0-960: a query that ranks nothing at all (`no-relevant-candidates`)
// passes no judgement on the files the packet matched lexically, so the
// packet carries no abstention for it.
func TestTaskContextIgnoresQueryEmptyUniverse(t *testing.T) {
	index, err := Build(context.Background(), evalQueryRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	reason, err := QueryRelevanceAbstention(index, "corvint semantic embeddings federation", 10)
	if err != nil || reason != "no-relevant-candidates" {
		t.Fatalf("QueryRelevanceAbstention = %q, %v; want no-relevant-candidates", reason, err)
	}
	packet, err := TaskContext(context.Background(), index, "corvint semantic embeddings federation", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if abstention, present := packet["abstention"]; present {
		t.Fatalf("an empty query universe set a context abstention: %v", abstention)
	}
}
