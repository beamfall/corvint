package doccorpus

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// tieCorpus is the DCX-V1-477 tie corpus: four claims and a longer one that
// all hold "booking" once, a rare-term claim, four "a tee" claims and three
// subjects that hold "marshal" in their names.
func tieCorpus(t *testing.T) *Artifact {
	t.Helper()
	return buildAdoption(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		subject := p.Subjects[0].ID
		for _, n := range []int{2004, 2052, 209, 211} {
			p.Claims = append(p.Claims, Claim{ID: fmt.Sprintf("%s:claim-%d", p.ID, n), Subject: subject, Text: "tee time booking", Provider: p.ID, Evidence: ev})
		}
		p.Claims = append(p.Claims,
			Claim{ID: p.ID + ":claim-1", Subject: subject, Text: "tee time booking for a guest", Provider: p.ID, Evidence: ev},
			Claim{ID: p.ID + ":claim-9000", Subject: subject, Text: "a waitlist", Provider: p.ID, Evidence: ev},
		)
		for i := range 4 {
			p.Claims = append(p.Claims, Claim{ID: fmt.Sprintf("%s:claim-%d", p.ID, 3000+i), Subject: subject, Text: "a tee", Provider: p.ID, Evidence: ev})
		}
		p.Subjects = append(p.Subjects,
			Subject{ID: p.ID + ":s10", Kind: "use_case", Name: "marshal sheet view", Provider: p.ID, Evidence: ev},
			Subject{ID: p.ID + ":s9", Kind: "use_case", Name: "marshal sheet", Provider: p.ID, Evidence: ev},
			Subject{ID: p.ID + ":s100", Kind: "use_case", Name: "marshal sheet", Provider: p.ID, Evidence: ev},
		)
	})
}

func pageOf(t *testing.T, a *Artifact, q Request) Receipt {
	t.Helper()
	r, err := Query(a, q, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// DCS-V1-479 (GitHub #1665 row 3): every visible docs_search result carries
// its rank before paging, its tie group, its tier and its normalized score in
// result_ranks, so a caller sees a tie and can cut at a group boundary. A page
// carries only its own results' ranks; other operations carry none.
func TestDCSV1479SearchServesRankGroups(t *testing.T) {
	t.Parallel()
	a := tieCorpus(t)
	r := pageOf(t, a, Request{Operation: "search", Query: "marshal", Limit: 10})
	want := map[string]int{"adapter:s9": 1, "adapter:s100": 2, "adapter:s10": 3}
	if len(r.ResultRanks) != len(r.Results) {
		t.Fatalf("every visible result should carry a rank: %v for %d results", r.ResultRanks, len(r.Results))
	}
	for id, rank := range want {
		got := r.ResultRanks[id]
		// DCS-V1-544 amends DCS-V1-479: docs_search now serves terms, the
		// one query word each result holds.
		if got.Rank != rank || got.Group != 1 || got.Tier != "terms" || got.Score <= 0 || got.Score > 1 || got.Terms != 1 {
			t.Fatalf("%s: %+v, want rank %d in tie group 1 with a terms score in (0,1] and one held word", id, got, rank)
		}
	}
	if r.ResultRanks["adapter:s9"].Score != r.ResultRanks["adapter:s10"].Score {
		t.Fatalf("a tie group should share its score: %v", r.ResultRanks)
	}
	tiers := pageOf(t, a, Request{Operation: "search", Query: "marshal sheet", Limit: 10}).ResultRanks
	if tiers["adapter:s9"].Tier != "exact" || tiers["adapter:s100"].Group != tiers["adapter:s9"].Group || tiers["adapter:s10"].Tier != "phrase" || tiers["adapter:s10"].Group <= tiers["adapter:s9"].Group {
		t.Fatalf("exact names should form the first group and the phrase match a later one: %v", tiers)
	}
	page := pageOf(t, a, Request{Operation: "search", Query: "marshal", Limit: 1, Offset: 1})
	if len(page.ResultRanks) != 1 || page.ResultRanks["adapter:s100"].Rank != 2 {
		t.Fatalf("a page should carry only its own results, ranked before paging: %v", page.ResultRanks)
	}
	merged := pageOf(t, a, Request{Operation: "search", Query: "booking", Include: []string{"claims"}, Limit: 20})
	if len(merged.ResultRanks) != len(merged.Results) || merged.ResultRanks["adapter:claim-209"].Group != merged.ResultRanks["adapter:claim-1"].Group {
		t.Fatalf("merged claim hits should carry ranks and share their tie group: %v", merged.ResultRanks)
	}
	data, err := Encode(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"result_ranks":{`) || !strings.Contains(string(data), `"rank":1,"group":1,"tier":"terms","score":`) {
		t.Fatalf("wire member missing: %s", data)
	}
	var back Receipt
	if err := json.Unmarshal(data, &back); err != nil || back.ResultRanks["adapter:s9"] != r.ResultRanks["adapter:s9"] {
		t.Fatalf("result_ranks should decode: %v %v", back.ResultRanks, err)
	}
	for _, q := range []Request{{Operation: "get", ID: "adapter:s9"}, {Operation: "concept", Query: "marshal"}, {Operation: "claims"}} {
		q.Limit = 10
		got := pageOf(t, a, q)
		if got.ResultRanks != nil {
			t.Fatalf("%s should carry no ranks: %v", q.Operation, got.ResultRanks)
		}
		data, err := Encode(got)
		if err != nil || strings.Contains(string(data), "result_ranks") {
			t.Fatalf("%s receipt bytes should not name result_ranks: %v", q.Operation, err)
		}
	}
}

// DCS-V1-480 (GitHub #1665 row 3): docs_claims with a query serves each
// visible claim's rank, its tie group (claims that hold as many query terms
// of the same summed idf), its tier, its normalized idf score and the count
// of distinct query terms it holds. The tie the issue showed as an ID-ordered
// run is one visible group.
func TestDCSV1480ClaimsServeRankGroups(t *testing.T) {
	t.Parallel()
	a := tieCorpus(t)
	r := pageOf(t, a, Request{Operation: "claims", Query: "booking", Limit: 10})
	order := []string{"adapter:claim-209", "adapter:claim-211", "adapter:claim-2004", "adapter:claim-2052", "adapter:claim-1"}
	for n, id := range order {
		got := r.ResultRanks[id]
		if got.Rank != n+1 || got.Group != 1 || got.Tier != "terms" || got.Terms != 1 || got.Score != 1 {
			t.Fatalf("%s: %+v, want rank %d of tie group 1 holding the one query term", id, got, n+1)
		}
	}
	split := pageOf(t, a, Request{Operation: "claims", Query: "tee waitlist", Limit: 20}).ResultRanks
	rare, common := split["adapter:claim-9000"], split["adapter:claim-3000"]
	if rare.Group != 1 || common.Group <= rare.Group || rare.Score <= common.Score || rare.Terms != 1 || common.Terms != 1 {
		t.Fatalf("the rarer term should lead its own group with the higher score: %+v %+v", rare, common)
	}
	if got := pageOf(t, a, Request{Operation: "claims", Query: "a waitlist", Limit: 20}).ResultRanks["adapter:claim-9000"]; got.Tier != "exact" || got.Rank != 1 {
		t.Fatalf("an exact claim text match should rank first in the exact tier: %+v", got)
	}
	page := pageOf(t, a, Request{Operation: "claims", Query: "booking", Limit: 2, Offset: 2})
	if !slices.Equal(slices.Sorted(func(yield func(string) bool) {
		for id := range page.ResultRanks {
			if !yield(id) {
				return
			}
		}
	}), []string{"adapter:claim-2004", "adapter:claim-2052"}) || page.ResultRanks["adapter:claim-2004"].Rank != 3 {
		t.Fatalf("a claims page should carry only its own ranks: %v", page.ResultRanks)
	}
}

// ownedSourceCorpus commits files under src, inventories them natively, then
// hands the inputs at owned to a records provider "adapter" whose one subject
// and claim are anchored at the first owned path. With coclaim the native
// provider also keeps them under the disjoint-union merge rule. Each of texts
// adds one more adapter claim, adapter:owner:claim-N, with the same anchor.
func ownedSourceCorpus(t *testing.T, files map[string]string, owned []string, coclaim bool, texts ...string) (string, Manifest) {
	t.Helper()
	return ownedSourceCorpusWith(t, files, owned, coclaim, nil, texts...)
}

// ownedSourceCorpusWith is ownedSourceCorpus with edit, when not nil, applied
// to the provider record before it is encoded.
func ownedSourceCorpusWith(t *testing.T, files map[string]string, owned []string, coclaim bool, edit func(*ProviderRecord), texts ...string) (string, Manifest) {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Corpus test")
	git(t, root, "config", "user.email", "corpus@example.invalid")
	for p, data := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "source")
	rev := git(t, root, "rev-parse", "HEAD")
	m, err := Inventory(context.Background(), root, rev, "src", "2026-10-24T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	ev := declaredEvidence(t, root, m, owned[0], 1)
	record := ProviderRecord{Schema: ProviderSchema, ID: "adapter", Version: "1", Source: m.Repository, Subjects: []Subject{{"adapter:owner", "capability", "Owned source", "adapter", ev, nil}}, Claims: []Claim{{"adapter:owner:claim", "adapter:owner", "The adapter produces records for this source", "adapter", ev}}, Relations: []Relation{}, Journeys: []Journey{}, Observations: []ObservationLink{}, Capabilities: []CapabilityDeclaration{{"subjects", "present", "fixture subjects"}, {"claims", "present", "fixture claims"}}}
	for n, text := range texts {
		record.Claims = append(record.Claims, Claim{fmt.Sprintf("adapter:owner:claim-%d", n+1), "adapter:owner", text, "adapter", ev})
	}
	if edit != nil {
		edit(&record)
	}
	data, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "evidence"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "evidence/provider.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "evidence/provider.json")
	git(t, root, "commit", "-qm", "provider")
	providerRevision := git(t, root, "rev-parse", "HEAD")
	m.Providers = append(m.Providers, Provider{ID: "adapter", Kind: "records", Version: "1", Revision: providerRevision, Record: "evidence/provider.json"})
	m.Scopes = append(m.Scopes, Scope{"evidence/provider.json", providerRevision})
	m.Inputs = append(m.Inputs, Input{Path: "evidence/provider.json", Revision: providerRevision, Blob: git(t, root, "rev-parse", providerRevision+":evidence/provider.json"), SHA256: Digest(data), Provider: "adapter", Purpose: "provider"})
	for i, in := range m.Inputs {
		if slices.Contains(owned, in.Path) && in.Purpose == "source" {
			if coclaim {
				adapter := in
				adapter.Provider = "adapter"
				m.Inputs = append(m.Inputs, adapter)
			} else {
				m.Inputs[i].Provider = "adapter"
			}
		}
	}
	if coclaim {
		m.MergeRule = "disjoint-union"
	}
	return root, m
}

func buildOwned(t *testing.T, root string, m Manifest) *Artifact {
	t.Helper()
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func subjectsWithPrefix(a *Artifact, prefix string) []Subject {
	var out []Subject
	for _, s := range a.Subjects {
		if strings.HasPrefix(s.ID, prefix) {
			out = append(out, s)
		}
	}
	return out
}

var ownedSources = map[string]string{
	"src/pos.rb":         "class PosSync\n  def sync_with_pos\n    true\n  end\nend\n",
	"src/report.py":      "def report_count():\n    return 1\n",
	"src/report_test.py": "def test_report_count():\n    assert True\n",
	"src/readme.md":      "# Pos\n\nSyncs with the point of sale.\n",
}

// DCS-V1-482 (GitHub #1665 row 5): an identifier declared in a source input
// that only a records provider claims is searchable. The native provider
// derives its declaration subject and claim, so docs_search for
// sync_with_pos or report_count ranks the declaration first, with or
// without a symbol kind filter, and a test path's declaration is a test
// subject.
func TestDCSV1482DeclarationsOfRecordsOwnedSource(t *testing.T) {
	t.Parallel()
	root, m := ownedSourceCorpus(t, ownedSources, []string{"src/pos.rb", "src/report.py", "src/report_test.py"}, false)
	a := buildOwned(t, root, m)
	for query, want := range map[string]string{"sync_with_pos": "native:symbol:src/pos.rb:2:sync_with_pos", "report_count": "native:symbol:src/report.py:1:report_count"} {
		if order := searchOrders(t, a, Request{Operation: "search", Query: query}); len(order) == 0 || order[0] != want {
			t.Fatalf("%s: the derived declaration should rank first: %v", query, order)
		}
		if order := searchOrders(t, a, Request{Operation: "search", Query: query, Kinds: []string{"symbol"}}); len(order) == 0 || order[0] != want {
			t.Fatalf("%s --kind symbol: %v", query, order)
		}
	}
	if order := claimOrder(t, a, "sync_with_pos"); len(order) == 0 || order[0] != "native:symbol:src/pos.rb:2:sync_with_pos:declaration" {
		t.Fatalf("the declaration claim should rank first: %v", order)
	}
	var kinds []string
	for _, s := range subjectsWithPrefix(a, "native:symbol:src/report_test.py:") {
		kinds = append(kinds, s.Kind)
	}
	if !slices.Contains(kinds, "test") {
		t.Fatalf("a test path declaration should be a test subject: %v", kinds)
	}
	for _, s := range subjectsWithPrefix(a, "native:symbol:src/pos.rb:") {
		if s.Provider != "native" || len(s.Evidence.Anchors) != 1 || s.Evidence.Anchors[0].Symbol != s.Name || s.Evidence.Derivation != "source-derived" {
			t.Fatalf("derived declaration evidence: %+v", s)
		}
	}
	if got := subjectsWithPrefix(a, "native:file:src/pos.rb"); len(got) != 0 {
		t.Fatalf("no native file subject is derived for a records-owned input: %v", got)
	}
	encoded, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenRead(context.Background(), root, encoded, ReadOptions{}); err != nil {
		t.Fatalf("the built corpus should pass the verified read: %v", err)
	}
}

// DCS-V1-483: derivation covers only inputs no native provider claims, once
// per path. A native co-claim keeps one declaration per symbol, and a
// manifest without a native provider derives none.
func TestDCSV1483DerivationScope(t *testing.T) {
	t.Parallel()
	root, m := ownedSourceCorpus(t, ownedSources, []string{"src/pos.rb"}, true)
	a := buildOwned(t, root, m)
	if got := subjectsWithPrefix(a, "native:symbol:src/pos.rb:2:sync_with_pos"); len(got) != 1 {
		t.Fatalf("a native co-claim should keep exactly one declaration: %v", got)
	}
	if got := subjectsWithPrefix(a, "native:file:src/pos.rb"); len(got) != 1 {
		t.Fatalf("the co-claimed input keeps its native file subject: %v", got)
	}
	root, m = ownedSourceCorpus(t, ownedSources, []string{"src/pos.rb", "src/report.py", "src/report_test.py", "src/readme.md"}, false)
	m.Providers = slices.DeleteFunc(m.Providers, func(p Provider) bool { return p.Kind == "native" })
	none := buildOwned(t, root, m)
	if got := subjectsWithPrefix(none, "native:"); len(got) != 0 {
		t.Fatalf("no native provider should derive no declaration: %v", got)
	}
}

// DCS-V1-484: derivation never refuses a manifest the compiler built before it.
// Declarations of a large records-owned source would exceed the /1 output byte
// bound, so the refused compile is retried with the prefix that fits
// (DCS-V1-569): the build succeeds with the provider's records, fewer derived
// declarations and one gap naming the bound, and an incremental build reports
// each shard once.
func TestDCSV1484DerivationNeverRefusesABuildableManifest(t *testing.T) {
	t.Parallel()
	var big strings.Builder
	for i := range MaxRecords + 8 {
		fmt.Fprintf(&big, "def f%05d():\n    return %d\n", i, i)
	}
	files := map[string]string{"src/big.py": big.String(), "src/readme.md": "# Big\n\nMany functions.\n"}
	root, m := ownedSourceCorpus(t, files, []string{"src/big.py"}, false)
	before := BuildsForTest()
	a := buildOwned(t, root, m)
	if derived := subjectsWithPrefix(a, "native:symbol:src/big.py:"); len(derived) >= MaxRecords+8 {
		t.Fatalf("the retry must drop declarations: %d derived subjects", len(derived))
	}
	if got := BuildsForTest() - before; got < 2 {
		t.Fatalf("the refused compile must be retried: %d compiles", got)
	}
	var gaps []Gap
	for _, g := range a.Gaps {
		if strings.Contains(g.Reason, "derived declaration bound") {
			gaps = append(gaps, g)
		}
	}
	if len(gaps) != 1 || gaps[0].Kind != "not-collected" || gaps[0].Subject != "native:derived-declarations" {
		t.Fatalf("one bound gap expected: %+v", gaps)
	}
	if !slices.ContainsFunc(a.Subjects, func(s Subject) bool { return s.ID == "adapter:owner" }) {
		t.Fatal("the provider's records must be kept")
	}
	if _, err := Encode(a); err != nil {
		t.Fatal(err)
	}
	ids := []ShardIdentity{{Provider: "adapter", Path: "evidence/provider.json"}}
	b, cache, stats, err := BuildIncremental(context.Background(), root, m, ids, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if b.SHA256 != a.SHA256 || len(stats.Compiled)+len(stats.Reused)+len(stats.Fallback) != 1 || len(cache.Shards) > 1 {
		t.Fatalf("the retry must restore incremental state: sha %v, stats %+v, %d cached shards", b.SHA256 == a.SHA256, stats, len(cache.Shards))
	}
}

// DCS-V1-484: a refusal of the first derived declaration (an oversized
// declaration line) is retried like any later one, and the retry restores the
// incremental shard cache, so an eligible adoption shard is compiled once cold
// and reused warm, with exactly one cached shard either way.
func TestDCSV1484RefusedFirstDeclarationRestoresShardCache(t *testing.T) {
	t.Parallel()
	files := map[string]string{"src/long.py": "# header\n\ndef long_line(x=\"" + strings.Repeat("a", maxSpanBytes) + "\"):\n    return x\n", "src/readme.md": "# Long\n\nOne long declaration.\n"}
	root, m := ownedSourceCorpus(t, files, []string{"src/long.py"}, false)
	raw, err := os.ReadFile(filepath.Join(root, "evidence/provider.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record ProviderRecord
	if err := decodeBounded(raw, &record, MaxProviderBytes); err != nil {
		t.Fatal(err)
	}
	record.Schema = AdoptionProviderSchema
	if raw, err = Encode(record); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "evidence/provider.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "evidence/provider.json")
	git(t, root, "commit", "-qm", "adoption provider")
	rev := git(t, root, "rev-parse", "HEAD")
	m.Schema = ManifestSchemaV2
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
	cold := buildOwned(t, root, m)
	if derived := subjectsWithPrefix(cold, "native:symbol:src/long.py:"); len(derived) != 0 {
		t.Fatalf("the retry must derive nothing: %d derived subjects", len(derived))
	}
	if !slices.ContainsFunc(cold.Gaps, func(g Gap) bool {
		return g.Subject == "native:derived-declarations" && strings.Contains(g.Reason, "derived declaration bound") && strings.Contains(g.Reason, "vacuous or oversized anchor")
	}) {
		t.Fatalf("the refused first declaration should leave the bound gap: %+v", cold.Gaps)
	}
	ids := []ShardIdentity{{Provider: "adapter", Path: "evidence/provider.json"}}
	first, cache, stats, err := BuildIncremental(context.Background(), root, m, ids, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if first.SHA256 != cold.SHA256 || !slices.Equal(stats.Compiled, ids) || len(stats.Reused)+len(stats.Fallback) != 0 || len(cache.Shards) != 1 || cache.Shards[0].Identity != ids[0] {
		t.Fatalf("cold incremental build after the retry: sha %v, stats %+v, %d cached shards", first.SHA256 == cold.SHA256, stats, len(cache.Shards))
	}
	prior, err := EncodeIncrementalCache(*cache)
	if err != nil {
		t.Fatal(err)
	}
	warm, cache, stats, err := BuildIncremental(context.Background(), root, m, ids, prior, Digest(prior))
	if err != nil {
		t.Fatal(err)
	}
	if warm.SHA256 != cold.SHA256 || !slices.Equal(stats.Reused, ids) || len(stats.Compiled)+len(stats.Fallback) != 0 || len(cache.Shards) != 1 {
		t.Fatalf("warm incremental build after the retry: sha %v, stats %+v, %d cached shards", warm.SHA256 == cold.SHA256, stats, len(cache.Shards))
	}
}

// DCS-V1-484: only a corpus refusal of a compile that attempted derivation is
// retried; Git, capacity and input-availability failures are returned as they are.
func TestDCSV1484RetryAnswersOnlyCorpusRefusals(t *testing.T) {
	t.Parallel()
	refused := fail("output bound exceeded")
	cases := []struct {
		name    string
		derived int
		err     error
		want    bool
	}{
		{"success", 1, nil, false},
		{"no derivation attempted", 0, refused, false},
		{"corpus refusal", 1, refused, true},
		{"wrapped corpus refusal", 1, fmt.Errorf("compile: %w", refused), true},
		{"capacity", 1, &Error{Code: CapacityCode, Message: "git budget"}, false},
		{"input unavailable", 1, &Error{Code: "corpus-input-unavailable", Message: "additional repository checkout unavailable: other"}, false},
		{"other error", 1, context.Canceled, false},
	}
	for _, c := range cases {
		if got := retryWithoutDerived(c.derived, c.err); got != c.want {
			t.Errorf("%s: retry %v, want %v", c.name, got, c.want)
		}
	}
}

// DCS-V1-485: derived declarations change the records the same inputs build,
// so the builder engine names derivation 2 or later (3 since DCS-V1-549). The verified read refuses an
// artifact recorded by derivation 1, which lacks them, and names the rebuild;
// the rebuilt corpus serves them.
func TestDCSV1485DerivationTwoRefusesDerivationOneArtifacts(t *testing.T) {
	t.Parallel()
	if v, err := strconv.Atoi(DerivationVersion); err != nil || v < 2 || derivationOf(currentBuilder()) != DerivationVersion {
		t.Fatalf("derivation %s, engine %q: derived declarations need derivation 2 or later", DerivationVersion, currentBuilder().Engine)
	}
	root, m := ownedSourceCorpus(t, ownedSources, []string{"src/pos.rb"}, false)
	a := buildOwned(t, root, m)
	old := rehashed(t, a, func(c *Artifact) {
		c.Builder.Revision = strings.Repeat("a", 40)
		c.Builder.Engine = strings.Replace(c.Builder.Engine, "/derivation-"+DerivationVersion+"/", "/derivation-1/", 1)
	})
	_, err := OpenRead(context.Background(), root, old, ReadOptions{})
	if err == nil || !strings.Contains(err.Error(), "derivation version differs (artifact 1, running builder "+DerivationVersion+"); rebuild the corpus") {
		t.Fatalf("a derivation 1 artifact should be refused naming the rebuild: %v", err)
	}
	if got := subjectsWithPrefix(a, "native:symbol:src/pos.rb:"); len(got) == 0 {
		t.Fatal("the rebuilt corpus should serve the derived declarations")
	}
}
