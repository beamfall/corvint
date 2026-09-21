package doccorpus

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCorpusEvaluation(t *testing.T) {
	t.Run("DCP-V1-020 evaluation", func(t *testing.T) {
		root, m := fixture(t)
		started := time.Now()
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		labels, err := os.ReadFile("testdata/evaluation.json")
		if err != nil {
			t.Fatal(err)
		}
		var suite struct {
			Relationships []string `json:"relationships"`
			Cases         []struct {
				Query    string   `json:"query"`
				Subjects []string `json:"subjects"`
				Abstain  bool     `json:"abstain"`
			} `json:"cases"`
		}
		if err := json.Unmarshal(labels, &suite); err != nil {
			t.Fatal(err)
		}
		hits, returned, truth, abstentions, bytes := 0, 0, 0, 0, 0
		rows := []any{}
		for _, test := range suite.Cases {
			start := time.Now()
			r, err := Query(a, Request{Operation: "search", Query: test.Query, Limit: 256}, "fresh", nil)
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]bool{}
			for _, id := range test.Subjects {
				want[id] = true
			}
			matched := 0
			for _, raw := range r.Results {
				subject := raw.(Subject)
				if want[subject.ID] {
					matched++
				}
			}
			hitAbstention := (len(r.Results) == 0) == test.Abstain
			if hitAbstention {
				abstentions++
			}
			encoded, _ := Encode(r)
			bytes += len(encoded)
			hits += matched
			returned += len(r.Results)
			truth += len(want)
			rows = append(rows, map[string]any{"query": test.Query, "true_positive": matched, "returned": len(r.Results), "relevant": len(want), "abstention_correct": hitAbstention, "latency_ns": time.Since(start).Nanoseconds(), "receipt_bytes": len(encoded)})
		}
		ratio := func(n, d int) any {
			if d == 0 {
				return nil
			}
			return float64(n) / float64(d)
		}
		truthRelations := map[string]bool{}
		for _, id := range suite.Relationships {
			truthRelations[id] = true
		}
		falseRelations := 0
		for _, relation := range a.Relations {
			if !truthRelations[relation.ID] {
				falseRelations++
			}
		}
		report := map[string]any{"schema": "corvint-corpus-evaluation/1", "kind": "synthetic-development", "labels_sha256": Digest(labels), "source_revision": m.Repository.Revision, "artifact_sha256": a.SHA256, "builder": a.Builder, "cases": rows, "precision": ratio(hits, returned), "recall": ratio(hits, truth), "abstention_accuracy": ratio(abstentions, len(suite.Cases)), "false_positive_relationships": falseRelations, "receipt_bytes": bytes, "latency_ns": time.Since(started).Nanoseconds(), "limitations": []string{"small author-labelled synthetic fixture; no external utility or savings claim"}}
		data, _ := Encode(report)
		t.Log(string(data))
		if hits != truth || hits != returned || abstentions != len(suite.Cases) || falseRelations != 0 || len(a.Relations) != len(truthRelations) {
			t.Fatalf("labelled evaluation failed: %s", data)
		}
	})
}
func TestCorpusSelfDocumentation(t *testing.T) {
	t.Run("DCP-V1-019 self", func(t *testing.T) {
		root, err := filepath.Abs("../..")
		if err != nil {
			t.Fatal(err)
		}
		rev := git(t, root, "rev-parse", "HEAD")
		m, err := Inventory(context.Background(), root, rev, "internal/doccompiler/draft.go", "2026-09-19T00:00:00Z")
		if err != nil {
			t.Fatal(err)
		}
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := Encode(a)
		r, err := ReadQuery(context.Background(), root, raw, Request{Operation: "search", Query: "DraftSources"})
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Results) == 0 {
			t.Fatal("Corvint could not query its own committed documentation source")
		}
		t.Logf("self-corpus source=%s artifact=%s records=%d receipt_results=%d", rev, a.SHA256, len(a.Subjects), len(r.Results))
	})
}
