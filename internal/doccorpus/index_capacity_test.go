package doccorpus

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// indexCapacityWords is a fixed vocabulary for synthetic claim text.
var indexCapacityWords = strings.Fields("request response handler widget order invoice account payment customer session token cache queue worker schedule report export import validate render template signal metric trace span event stream batch record shard index query result filter route policy grant audit ledger journal")

// padIndexCapacity adds synthetic subjects, claims and relations, each with
// its own path and symbol anchor, in the shape of a large adoption corpus.
func padIndexCapacity(a *Artifact, subjects, claims, relations int) {
	word := func(i int) string { return indexCapacityWords[i%len(indexCapacityWords)] }
	first := len(a.Subjects)
	for i := 0; i < subjects; i++ {
		path := fmt.Sprintf("internal/pkg%03d/file%02d.go", i%800, i%37)
		symbol := fmt.Sprintf("%s%sFunc%d", strings.ToUpper(word(i)[:1])+word(i)[1:], strings.ToUpper(word(i / 7)[:1])+word(i / 7)[1:], i)
		a.Subjects = append(a.Subjects, Subject{ID: "pad:" + path + "#" + symbol, Kind: "symbol", Name: symbol + " " + word(i/3) + " " + word(i/11), Provider: "pad", Evidence: Evidence{Anchors: []Anchor{{Path: path, Symbol: symbol}}}})
	}
	for i := 0; i < claims; i++ {
		s := a.Subjects[first+(i*7)%subjects]
		text := []string{}
		for j := 0; j < 24; j++ {
			text = append(text, word(i*13+j*5))
		}
		a.Claims = append(a.Claims, Claim{ID: fmt.Sprintf("pad:claim:%06d", i), Subject: s.ID, Text: strings.Join(text, " ") + fmt.Sprintf(" Item%dValue", i), Provider: "pad", Evidence: s.Evidence})
	}
	for i := 0; i < relations; i++ {
		from, to := a.Subjects[first+(i*3)%subjects], a.Subjects[first+(i*5+1)%subjects]
		a.Relations = append(a.Relations, Relation{ID: fmt.Sprintf("pad:relation:%06d", i), From: from.ID, To: to.ID, Type: "calls", Provider: "pad", Evidence: from.Evidence})
	}
}

// storedIndexSize is what an admitted index holds: one posting per stored
// reference and the bytes of its distinct keys.
func storedIndexSize(x *QueryIndex) (postings, keyBytes int) {
	keys := map[string]bool{}
	key := func(k string) {
		if !keys[k] {
			keys[k] = true
			keyBytes += len(k)
		}
	}
	for k := range x.IDs {
		postings++
		key(k)
	}
	for k, v := range x.Paths {
		postings += len(v)
		key(k)
	}
	for _, m := range []map[string][]int{x.Terms, x.Claims, x.Edges, x.ClaimTerms} {
		for k, v := range m {
			postings += len(v)
			key(k)
		}
	}
	return postings, keyBytes
}

func withIndexBounds(t *testing.T, postings, keyBytes int) {
	t.Helper()
	oldPostings, oldKeyBytes := indexPostingBound, indexKeyByteBound
	indexPostingBound, indexKeyByteBound = postings, keyBytes
	t.Cleanup(func() { indexPostingBound, indexKeyByteBound = oldPostings, oldKeyBytes })
}

func capacityMessage(t *testing.T, err error) string {
	t.Helper()
	var coded *Error
	if !errors.As(err, &coded) || coded.Code != CapacityCode {
		t.Fatalf("want %s, got %v", CapacityCode, err)
	}
	return coded.Message
}

// A 100,000-record adoption corpus, past the former 1,000,000-posting index
// bound, answers every indexed read.
func TestDCPV1193HundredThousandRecordCorpusAnswersIndexedReads(t *testing.T) {
	t.Parallel()
	if MaxIndexPostings != 8000000 || MaxIndexKeyBytes != 64<<20 || indexPostingBound != MaxIndexPostings || indexKeyByteBound != MaxIndexKeyBytes {
		t.Fatal("documented index bounds changed", MaxIndexPostings, MaxIndexKeyBytes)
	}
	root, m := adoptionFixture(t, nil)
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	padIndexCapacity(a, 60000-len(a.Subjects), 25000-len(a.Claims), 15000-len(a.Relations))
	if records := len(a.Subjects) + len(a.Claims) + len(a.Relations) + len(a.Journeys) + len(a.Observations); records < 100000 {
		t.Fatal("fixture below 100,000 records", records)
	}
	index, err := BuildQueryIndex(context.Background(), a)
	if err != nil {
		t.Fatal("100,000-record corpus refused by the index:", err)
	}
	postings, keyBytes := storedIndexSize(index)
	if postings <= 1000000 {
		t.Fatal("fixture does not exceed the former posting bound", postings)
	}
	t.Logf("index: %d postings, %d unique key bytes", postings, keyBytes)
	indexed := *a
	indexed.RuntimeIndex = index
	cases := []Request{{Operation: "concept", Query: "Count"}, {Operation: "claims", Path: "src/value.go"}, {Operation: "flow", ID: "adapter:flow"}, {Operation: "dependencies", ID: "adapter:flow"}, {Operation: "recommend-tests", Path: "src/value.go"}, {Operation: "navigation", ID: "adapter:flow"}, {Operation: "vocabulary", Query: "purchase"}, {Operation: "intent", ID: "adapter:intent"}, {Operation: "gaps"}, {Operation: "claims", Path: "internal/pkg001/file01.go"}}
	for _, q := range cases {
		if !IndexedOperation(q.Operation) {
			t.Fatal("not an indexed read", q.Operation)
		}
		if _, err := Query(&indexed, q, "fresh", nil); err != nil {
			t.Fatal(q.Operation, err)
		}
	}
	// The unindexed artifact derives the same index on demand.
	if _, err := Query(a, Request{Operation: "claims", Path: "internal/pkg001/file01.go"}, "fresh", nil); err != nil {
		t.Fatal(err)
	}
}

// The refusal names the bound and the observed total, which equals what the
// admitted index stores: each posting is counted once, and counting continues
// past the bound.
func TestDCPV1194IndexRefusalNamesBoundAndObservedCount(t *testing.T) {
	root, m := adoptionFixture(t, nil)
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	padIndexCapacity(a, 200, 300, 100)
	index, err := BuildQueryIndex(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	postings, keyBytes := storedIndexSize(index)
	withIndexBounds(t, postings, keyBytes)
	if _, err := BuildQueryIndex(context.Background(), a); err != nil {
		t.Fatal("exact bounds refused", err)
	}
	withIndexBounds(t, postings/2, keyBytes)
	if _, err := BuildQueryIndex(context.Background(), a); capacityMessage(t, err) != fmt.Sprintf("indexed postings %d exceeds the limit of %d", postings, postings/2) {
		t.Fatal("posting refusal does not name the bound and observed count:", err)
	}
	withIndexBounds(t, postings, keyBytes-1)
	if _, err := BuildQueryIndex(context.Background(), a); capacityMessage(t, err) != fmt.Sprintf("indexed unique key bytes %d exceeds the limit of %d", keyBytes, keyBytes-1) {
		t.Fatal("key-byte refusal does not name the bound and observed count:", err)
	}
}

// docs corpus build refuses a corpus whose index exceeds its bounds; Build,
// which Open rederives through, is unchanged and within the bounds the
// admitted artifact is the same.
func TestDCPV1196BuildAdmittedRefusesOverBoundIndex(t *testing.T) {
	root, m := adoptionFixture(t, nil)
	built, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := BuildAdmitted(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	left, _ := Encode(built)
	right, _ := Encode(admitted)
	if string(left) != string(right) {
		t.Fatal("admission changed the artifact bytes")
	}
	withIndexBounds(t, 1, MaxIndexKeyBytes)
	if _, err := BuildAdmitted(context.Background(), root, m); !strings.HasPrefix(capacityMessage(t, err), "indexed postings ") {
		t.Fatal("over-bound index admitted at build", err)
	}
	if _, err := Open(context.Background(), root, left); err != nil {
		t.Fatal("open must still rederive an artifact written before", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	withIndexBounds(t, MaxIndexPostings, MaxIndexKeyBytes)
	if err := AdmitIndex(ctx, built); !errors.Is(err, context.Canceled) {
		t.Fatal("an interrupted admission must fail closed", err)
	}
}

// DCP-V1-244: admission refuses with the same typed code every condition the
// query-time index derivation refuses, not only the capacity bound (V1-2002).
// A claim reusing a subject's identity is refused by the derivation as
// corpus-refused; before the fix admission returned nil for it.
func TestDCPV1244AdmitIndexRefusesEveryDerivationRefusal(t *testing.T) {
	root, m := adoptionFixture(t, nil)
	built, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(built.Subjects) == 0 || len(built.Claims) == 0 {
		t.Fatal("fixture has no subject or claim to collide")
	}
	built.Claims[0].ID = built.Subjects[0].ID
	_, queryErr := BuildQueryIndex(context.Background(), built)
	var queried, admitted *Error
	if !errors.As(queryErr, &queried) || queried.Code == CapacityCode {
		t.Fatal("fixture does not reach a non-capacity derivation refusal:", queryErr)
	}
	if err := AdmitIndex(context.Background(), built); !errors.As(err, &admitted) || admitted.Code != queried.Code || admitted.Message != queried.Message {
		t.Fatalf("admission returned %v, query time refuses %v", err, queryErr)
	}
}
