package doccorpus

import (
	"context"
	"fmt"
	"runtime"
	"testing"
)

// suppliedModuleCorpus is n supplied module subjects whose anchor path is not
// their name, so each one has DCS-V1-400 path terms, indexed as the runtime
// reader holds it.
func suppliedModuleCorpus(tb testing.TB, n int) *Artifact {
	tb.Helper()
	a := &Artifact{}
	for i := range n {
		ev := Evidence{Anchors: []Anchor{{Path: fmt.Sprintf("src/billing/ledger_%04d/invoice_export.ts", i)}}}
		a.Subjects = append(a.Subjects, Subject{ID: fmt.Sprintf("adapter:module-%04d", i), Kind: "module", Name: fmt.Sprintf("Billing ledger module %04d", i), Provider: "adapter", Evidence: ev})
	}
	index, err := BuildQueryIndex(context.Background(), a)
	if err != nil {
		tb.Fatal(err)
	}
	a.RuntimeIndex = index
	return a
}

// DCS-V1-403 (review P3): a query that matches no subject retains no
// per-subject name terms. The name-field function returned for scoring keeps
// only query terms, and only for subjects it is asked about, so the heap held
// after a miss over thousands of supplied modules stays flat.
func TestDCSV1403MissRetainsNoSubjectNameTerms(t *testing.T) {
	a := suppliedModuleCorpus(t, 4000)
	terms := []string{"zzunmatched"}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	postings, names, err := subjectPostings(context.Background(), a, a.RuntimeIndex, terms)
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(postings)
	runtime.KeepAlive(names)
	if len(postings["zzunmatched"]) != 0 {
		t.Fatalf("miss matched subjects: %v", postings)
	}
	grown := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	t.Logf("retained after miss: %d heap bytes", grown)
	if grown > 256<<10 {
		t.Fatalf("miss over 4000 supplied modules retained %d heap bytes", grown)
	}
	// A hit returns only query terms in the subject's name field.
	_, names, err = subjectPostings(context.Background(), a, a.RuntimeIndex, []string{"invoice", "zzunmatched"})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(7); len(got) != 1 || !got["invoice"] {
		t.Fatalf("name terms should be scoped to the query: %v", got)
	}
}

// BenchmarkDCSV1403SearchMissSuppliedModules measures a docs_search miss over
// 4000 supplied module subjects with path terms (DCS-V1-403).
func BenchmarkDCSV1403SearchMissSuppliedModules(b *testing.B) {
	a := suppliedModuleCorpus(b, 4000)
	request := Request{Operation: "search", Query: "zzunmatched"}
	b.ReportAllocs()
	for b.Loop() {
		var r Receipt
		if err := search(context.Background(), a, request, &r); err != nil {
			b.Fatal(err)
		}
	}
}
