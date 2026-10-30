package similar

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

type corpusRecord struct {
	ID                 string   `json:"id"`
	Status             string   `json:"status"`
	CreatedAt          string   `json:"createdAt"`
	Title              string   `json:"title"`
	Body               string   `json:"body"`
	AcceptanceCriteria []string `json:"acceptanceCriteria"`
	Labels             []string `json:"labels"`
	TouchPaths         []string `json:"touchPaths"`
}

type evalQuery struct {
	Source  string `json:"source"`
	FiledAt string `json:"filedAt"`
	Text    string `json:"text"`
	// Duplicates are the tickets that already described the item when it
	// was filed; empty means the item was new.
	Duplicates []string `json:"duplicates"`
	// Exclude names the query's own ticket for a historical pair.
	Exclude string `json:"exclude"`
	Note    string `json:"note"`
	// KnownMissRank marks a retained lexical miss: a labelled duplicate
	// must rank at or above it, outside the recall@3 count.
	KnownMissRank int `json:"knownMissRank"`
}

func loadCorpus(t testing.TB) []corpusRecord {
	t.Helper()
	f, err := os.Open("testdata/corpus-2026-10-30.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []corpusRecord
	dec := json.NewDecoder(f)
	for dec.More() {
		var r corpusRecord
		if err := dec.Decode(&r); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func loadQueries(t testing.TB) []evalQuery {
	t.Helper()
	raw, err := os.ReadFile("testdata/queries.json")
	if err != nil {
		t.Fatal(err)
	}
	var qs []evalQuery
	if err := json.Unmarshal(raw, &qs); err != nil {
		t.Fatal(err)
	}
	return qs
}

func recordDoc(r corpusRecord) Doc {
	body := r.Body
	if len(r.AcceptanceCriteria) > 0 {
		body += "\n" + strings.Join(r.AcceptanceCriteria, "\n")
	}
	return Doc{ID: r.ID, Title: r.Title, Status: r.Status, Body: body, Labels: r.Labels, Paths: r.TouchPaths, Recurrences: Recurrences(r.Body)}
}

// query is a historical pair's title and body (its first line and the
// rest), or a friction item's one line as the title.
func (q evalQuery) query() Query {
	title, body, _ := strings.Cut(q.Text, "\n")
	return Query{Title: title, Body: body}
}

// docsBefore is the corpus as it stood when a query was filed.
func docsBefore(corpus []corpusRecord, at, exclude string) []Doc {
	cut, _ := time.Parse(time.RFC3339, at)
	var docs []Doc
	for _, r := range corpus {
		created, err := time.Parse(time.RFC3339, r.CreatedAt)
		if err != nil || !created.Before(cut) || r.ID == exclude {
			continue
		}
		docs = append(docs, recordDoc(r))
	}
	return docs
}

// TestTCLV0009_FrozenDuplicateEval holds recall@3 and recall@5 (the default
// CLI view) at 100% for the labelled duplicates, reports the guard's precision and recall, and bounds the
// full-store latency.
func TestTCLV0009_FrozenDuplicateEval(t *testing.T) {
	corpus := loadCorpus(t)
	queries := loadQueries(t)
	verbose := os.Getenv("TCL_EVAL_VERBOSE") != ""
	var dups, hit3, hit5, misses, refusals, rightRefusals, caught int
	for _, q := range queries {
		idx := NewIndex(docsBefore(corpus, q.FiledAt, q.Exclude))
		got := idx.RankQuery(q.query(), nil, 3)
		if verbose {
			t.Logf("%.70q want=%v", q.Text, q.Duplicates)
			for rank, c := range idx.RankQuery(q.query(), nil, 50) {
				for _, id := range q.Duplicates {
					if c.Doc.ID == id && rank >= 3 {
						t.Logf("   labelled %s at rank %d: %.2f sim=%.2f | %s", id, rank+1, c.Score, c.Similarity, c.Reason())
					}
				}
			}
			for _, c := range got {
				t.Logf("   %s %.2f sim=%.2f dup=%v %.60s | %s", c.Doc.ID, c.Score, c.Similarity, c.Duplicate(), c.Doc.Title, c.Reason())
			}
		}
		labelled := map[string]bool{}
		for _, id := range q.Duplicates {
			labelled[id] = true
		}
		refused, right, found := false, false, false
		for _, c := range got {
			if labelled[c.Doc.ID] {
				found = true
			}
			if c.Duplicate() {
				refused = true
				right = right || labelled[c.Doc.ID]
			}
		}
		if refused {
			refusals++
		}
		if right {
			rightRefusals++
		}
		if len(q.Duplicates) == 0 {
			continue
		}
		if q.KnownMissRank > 0 {
			misses++
			if !labelledWithin(idx.RankQuery(q.query(), nil, q.KnownMissRank), labelled) {
				t.Errorf("known miss %v fell below rank %d for %.70q", q.Duplicates, q.KnownMissRank, q.Text)
			}
			if found {
				t.Errorf("known miss %v now ranks in the top 3; count it in recall@3 for %.70q", q.Duplicates, q.Text)
			}
			continue
		}
		dups++
		if found {
			hit3++
		} else {
			t.Errorf("no labelled duplicate %v in the top 3 for %.70q", q.Duplicates, q.Text)
		}
		if labelledWithin(idx.RankQuery(q.query(), nil, 5), labelled) {
			hit5++
		} else {
			t.Errorf("no labelled duplicate %v in the top 5 for %.70q", q.Duplicates, q.Text)
		}
		if right {
			caught++
		}
	}
	precision := 0.0
	if refusals > 0 {
		precision = float64(rightRefusals) / float64(refusals)
	}
	t.Logf("TCL-V0-009 eval: queries %d, labelled duplicates %d (+%d known misses), recall@3 %d/%d, recall@5 %d/%d; guard refusals %d, naming a labelled duplicate %d (precision %.2f), duplicates refused %d/%d",
		len(queries), dups, misses, hit3, dups, hit5, dups, refusals, rightRefusals, precision, caught, dups)

	docs := make([]Doc, len(corpus))
	for i, r := range corpus {
		docs[i] = recordDoc(r)
	}
	// The fastest of three runs: one wall-clock sample on a shared CI runner
	// measures the other packages' load as much as the index (V1-3237).
	var idx *Index
	var got []Candidate
	elapsed := time.Duration(1<<63 - 1)
	for range 3 {
		start := time.Now()
		idx = NewIndex(docs)
		got = idx.Rank(queries[0].Text, func(d *Doc) bool { return d.Status == "OPEN" }, 5)
		elapsed = min(elapsed, time.Since(start))
	}
	t.Logf("TCL-V0-009 latency: %d tickets, index+rank %s, %d candidates", idx.Len(), elapsed, len(got))
	if elapsed > 300*time.Millisecond {
		t.Errorf("index+rank took %s, want under 300ms", elapsed)
	}
}

func labelledWithin(got []Candidate, labelled map[string]bool) bool {
	for _, c := range got {
		if labelled[c.Doc.ID] {
			return true
		}
	}
	return false
}

func BenchmarkIndexAndRankFullStore(b *testing.B) {
	corpus := loadCorpus(b)
	docs := make([]Doc, len(corpus))
	for i, r := range corpus {
		docs[i] = recordDoc(r)
	}
	query := loadQueries(b)[0].Text
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewIndex(docs).Rank(query, nil, 5)
	}
}
