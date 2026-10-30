package doccorpus

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// retestCorpus reproduces the GitHub #1665 re-test of ec41e4e (V1-2598): the
// question "Where in the source is <Name> implemented?" ranked a paragraph
// holding the frame words "source" and "implemented" above the flow it names,
// missed the flow whose ID slug spells the name, and named a paragraph by one
// word of the name.
func retestCorpus(t *testing.T) *Artifact {
	t.Helper()
	return buildAdoption(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		subject := p.Subjects[0].ID
		add := func(id, kind, name string) {
			p.Subjects = append(p.Subjects, Subject{ID: p.ID + ":" + id, Kind: kind, Name: name, Provider: p.ID, Evidence: ev})
		}
		add("rt-players", "use_case", "Player types and handicap rules")
		add("rt-frame", "document", "The player source is implemented in the scheduler")
		add("flow:flows/reservation-cancellation", "use_case", "Reservation cancellation and financial unwind")
		add("doc:docs/reservation-cancellation.md", "document", "Cancelling a booking")
		add("flow:flows/reservation", "use_case", "Booking hold")
		add("rt-cancellation", "document", "Cancellation")
		add("rt-in-source", "document", "In the source")
		add("rt-named", "use_case", "Reservation Cancellation")
		for _, w := range []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"} {
			add("rt-filler-"+w, "use_case", "reservation "+w+" screen")
		}
		p.Claims = append(p.Claims,
			Claim{ID: p.ID + ":rt-claim-frame", Subject: subject, Text: "The player source is implemented in the scheduler", Provider: p.ID, Evidence: ev},
			Claim{ID: p.ID + ":rt-claim-types", Subject: subject, Text: "Player types decide the tee sheet", Provider: p.ID, Evidence: ev},
		)
	})
}

// DCS-V1-565: in a question, the frame words of DCS-V1-565 and the question
// words are not covered words, so a paragraph holding "source" and
// "implemented" no longer outranks the subject holding the named words; a
// statement still counts every word, and claims follow the same rule.
func TestDCSV1565QuestionCoverageCountsContentWords(t *testing.T) {
	t.Parallel()
	a := retestCorpus(t)
	q := Request{Operation: "search", Query: "Where in the source is Player Types implemented?", Limit: 20}
	order := searchOrders(t, a, q)
	ranks := pageOf(t, a, q).ResultRanks
	players, frame := ranks["adapter:rt-players"], ranks["adapter:rt-frame"]
	if players.Tier != "terms" || players.Terms != 2 || frame.Terms != 1 {
		t.Fatalf("coverage should count only content words: players %+v frame %+v", players, frame)
	}
	if rankOf(order, "adapter:rt-players") > rankOf(order, "adapter:rt-frame") {
		t.Fatalf("the subject holding both content words should order first: %v %v", order, ranks)
	}
	statement := pageOf(t, a, Request{Operation: "search", Query: "player source implemented", Limit: 20}).ResultRanks
	if got := statement["adapter:rt-frame"].Terms; got != 3 {
		t.Fatalf("a statement counts every word: %d", got)
	}
	claims := pageOf(t, a, Request{Operation: "search", Query: "Where in the source is Player Types implemented?", Include: []string{"claims"}, Limit: 20}).ResultRanks
	if f, p := claims["adapter:rt-claim-frame"], claims["adapter:rt-claim-types"]; f.Terms != 1 || p.Terms != 2 || p.Rank > f.Rank {
		t.Fatalf("claim coverage should count only content words: frame %+v types %+v", f, p)
	}
}

// DCS-V1-566: a question names a subject whose ID slug, the ID after its last
// '/' or ':' and without a dot suffix, spells at least two content words of
// the question in order, so the flow flows/reservation-cancellation reaches
// tier named; a one-word slug names nothing.
func TestDCSV1566QuestionNamesAnIDSlug(t *testing.T) {
	t.Parallel()
	a := retestCorpus(t)
	q := Request{Operation: "search", Query: "Where in the source is Reservation Cancellation implemented?", Limit: 20}
	order := searchOrders(t, a, q)
	ranks := pageOf(t, a, q).ResultRanks
	for _, id := range []string{"adapter:flow:flows/reservation-cancellation", "adapter:doc:docs/reservation-cancellation.md", "adapter:rt-named"} {
		if ranks[id].Tier != "named" || rankOf(order, id) > 3 {
			t.Fatalf("%s should rank in tier named in the top three: %v %v", id, order, ranks)
		}
	}
	if tier := ranks["adapter:flow:flows/reservation"].Tier; tier == "named" {
		t.Fatalf("a one-word slug should name nothing: %s", tier)
	}
	if tier := pageOf(t, a, Request{Operation: "search", Query: "reservation cancellation", Limit: 20}).ResultRanks["adapter:flow:flows/reservation-cancellation"].Tier; tier == "named" {
		t.Fatalf("a statement names nothing: %s", tier)
	}
}

// DCS-V1-567: tier named needs two content words of a multi-word name, and a
// one-word name only a capitalized word that no other capitalized word
// adjoins, so neither "Cancellation" nor "In the source" is named by the
// question that names Reservation Cancellation.
func TestDCSV1567NamedTierNeedsTheWholeName(t *testing.T) {
	t.Parallel()
	a := retestCorpus(t)
	q := Request{Operation: "search", Query: "Where in the source is Reservation Cancellation implemented?", Limit: 20}
	ranks := pageOf(t, a, q).ResultRanks
	for _, id := range []string{"adapter:rt-cancellation", "adapter:rt-in-source"} {
		if ranks[id].Tier == "named" {
			t.Fatalf("%s should not be named: %+v", id, ranks)
		}
	}
	for query, want := range map[string]bool{"How does Waitlist work?": true, "Where is Reservation Cancellation implemented?": false, "Does Waitlist work?": true} {
		if got := capitalizedWords(query)["waitlist"] || capitalizedWords(query)["cancellation"]; got != want {
			t.Fatalf("capitalizedWords(%q) = %v", query, capitalizedWords(query))
		}
	}
}

// rewriteOwnedProvider edits the records provider of an ownedSourceCorpus,
// commits it and repins the manifest's provider, scope and input.
func rewriteOwnedProvider(t *testing.T, root string, m Manifest, edit func(*ProviderRecord)) Manifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "evidence/provider.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record ProviderRecord
	if err := decodeBounded(raw, &record, MaxProviderBytes); err != nil {
		t.Fatal(err)
	}
	edit(&record)
	if raw, err = Encode(record); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "evidence/provider.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "evidence/provider.json")
	git(t, root, "commit", "-qm", "provider records")
	rev := git(t, root, "rev-parse", "HEAD")
	for i := range m.Providers {
		if m.Providers[i].ID == "adapter" {
			m.Providers[i].Revision = rev
		}
	}
	for i := range m.Scopes {
		if m.Scopes[i].Path == "evidence/provider.json" {
			m.Scopes[i].Revision = rev
		}
	}
	for i := range m.Inputs {
		if m.Inputs[i].Path == "evidence/provider.json" {
			m.Inputs[i].Revision, m.Inputs[i].Blob, m.Inputs[i].SHA256 = rev, git(t, root, "rev-parse", rev+":evidence/provider.json"), Digest(raw)
		}
	}
	return m
}

// derivedBoundGap returns the one derived declaration bound gap.
func derivedBoundGap(t *testing.T, a *Artifact) Gap {
	t.Helper()
	var gaps []Gap
	for _, g := range a.Gaps {
		if strings.Contains(g.Reason, "derived declaration bound") {
			gaps = append(gaps, g)
		}
	}
	if len(gaps) != 1 || gaps[0].Kind != "not-collected" || gaps[0].Subject != "native:derived-declarations" {
		t.Fatalf("one derived declaration bound gap on native:derived-declarations expected: %+v", gaps)
	}
	return gaps[0]
}

// DCS-V1-568: derivation stops at the corpus record bound in the first
// compile, so a records provider that leaves room for a few declarations gets
// exactly that many, with no retry, and one gap names the bound, its value,
// the count derived and dropped and where dropping began.
func TestDCSV1568DerivationStopsAtTheRecordBound(t *testing.T) {
	// Not parallel: BuildsForTest counts every compile of the package.
	const functions = 30
	var src strings.Builder
	for i := range functions {
		fmt.Fprintf(&src, "def g%02d():\n    return %d\n", i, i)
	}
	files := map[string]string{"src/many.py": src.String(), "src/readme.md": "# Many\n\nSome functions.\n"}
	root, m := ownedSourceCorpus(t, files, []string{"src/many.py"}, false)
	m = rewriteOwnedProvider(t, root, m, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		for i := range MaxRecords - 12 {
			p.Subjects = append(p.Subjects, Subject{fmt.Sprintf("adapter:r%04d", i), "capability", "r", "adapter", ev, nil})
		}
	})
	before := BuildsForTest()
	a := buildOwned(t, root, m)
	if got := BuildsForTest() - before; got != 1 {
		t.Fatalf("the record bound needs no retry: %d compiles", got)
	}
	derived := len(subjectsWithPrefix(a, "native:symbol:src/many.py:"))
	if derived == 0 || derived >= functions || len(a.Subjects) != MaxRecords {
		t.Fatalf("derivation should fill the record bound: %d derived, %d subjects", derived, len(a.Subjects))
	}
	gap := derivedBoundGap(t, a)
	first := fmt.Sprintf("src/many.py:%d", 2*derived+1)
	for _, want := range []string{"record bound 4096", fmt.Sprintf("derived %d of %d", derived, functions), fmt.Sprintf("%d dropped", functions-derived), first} {
		if !strings.Contains(gap.Reason, want) {
			t.Fatalf("the gap should name %q: %s", want, gap.Reason)
		}
	}
}

// DCS-V1-569: a compile refused by the output byte bound is retried deriving
// a prefix of the declarations that fits under the bound, not none, and the
// gap names the byte bound, its value, the size reached and the dropped count.
func TestDCSV1569DerivationStopsAtTheOutputBound(t *testing.T) {
	// Not parallel: BuildsForTest counts every compile of the package.
	var big strings.Builder
	for i := range MaxRecords + 8 {
		fmt.Fprintf(&big, "def f%05d():\n    return %d\n", i, i)
	}
	files := map[string]string{"src/big.py": big.String(), "src/readme.md": "# Big\n\nMany functions.\n"}
	root, m := ownedSourceCorpus(t, files, []string{"src/big.py"}, false)
	before, pinsBefore := BuildsForTest(), PinsForTest()
	a := buildOwned(t, root, m)
	if got := BuildsForTest() - before; got != 2 {
		t.Fatalf("the output bound is known after one compile, so one retry: %d compiles", got)
	}
	// DCS-V1-572: the refused compile pins to open, and the retry reads what it pinned and
	// pins only to close, where re-pinning the retry's opening pinned three times.
	if got := PinsForTest() - pinsBefore; got != 2 {
		t.Fatalf("the retry should not pin its inputs again before it compiles: %d pins", got)
	}
	derived := len(subjectsWithPrefix(a, "native:symbol:src/big.py:"))
	if derived == 0 || derived >= MaxRecords {
		t.Fatalf("the retry should derive the prefix that fits: %d derived", derived)
	}
	gap := derivedBoundGap(t, a)
	for _, want := range []string{fmt.Sprintf("output byte bound %d", MaxBytes), fmt.Sprintf("derived %d of %d", derived, MaxRecords+8), fmt.Sprintf("%d dropped", MaxRecords+8-derived), fmt.Sprintf("src/big.py:%d", 2*derived+1)} {
		if !strings.Contains(gap.Reason, want) {
			t.Fatalf("the gap should name %q: %s", want, gap.Reason)
		}
	}
	if _, err := Encode(a); err != nil {
		t.Fatal(err)
	}
}

// DCS-V1-569: the byte estimate of a test declaration is a lower bound, so a
// test-heavy corpus over the output byte bound still keeps the prefix that
// fits in one retry instead of a second refusal that derives none.
func TestDCSV1569TestDeclarationsStopAtTheOutputBound(t *testing.T) {
	// Not parallel: BuildsForTest counts every compile of the package.
	var big strings.Builder
	for i := range MaxRecords + 8 {
		fmt.Fprintf(&big, "def test_f%05d():\n    assert %d\n", i, i)
	}
	files := map[string]string{"src/test_big.py": big.String(), "src/readme.md": "# Big\n\nMany tests.\n"}
	root, m := ownedSourceCorpus(t, files, []string{"src/test_big.py"}, false)
	before := BuildsForTest()
	a := buildOwned(t, root, m)
	if got := BuildsForTest() - before; got != 2 {
		t.Fatalf("one retry should fit the output bound: %d compiles", got)
	}
	derived := len(subjectsWithPrefix(a, "native:symbol:src/test_big.py:"))
	if derived == 0 || derived >= MaxRecords {
		t.Fatalf("the retry should derive the prefix that fits: %d derived", derived)
	}
	if gap := derivedBoundGap(t, a); !strings.Contains(gap.Reason, fmt.Sprintf("derived %d of %d", derived, MaxRecords+8)) {
		t.Fatalf("the gap should name the counts: %s", gap.Reason)
	}
}
