package corpusindex

import (
	"bytes"
	"context"
	stdjson "encoding/json"
	"fmt"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", root}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}
func fixture(t *testing.T, claims, symbols int) (string, []byte) {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Corpus companion fixture")
	git(t, root, "config", "user.email", "corpus@example.invalid")
	_ = os.Mkdir(filepath.Join(root, "src"), 0700)
	var source strings.Builder
	source.WriteString("package fixture\n")
	for i := 0; i < symbols; i++ {
		fmt.Fprintf(&source, "func F%05d() int { return %d }\n", i, i)
	}
	for path, data := range map[string]string{"go.mod": "module example.invalid/corpus\n\ngo 1.27.1\n", "src/value.go": source.String()} {
		if e := os.WriteFile(filepath.Join(root, path), []byte(data), 0600); e != nil {
			t.Fatal(e)
		}
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "source")
	rev := git(t, root, "rev-parse", "HEAD")
	m, e := doccorpus.Inventory(context.Background(), root, rev, "src", "2026-09-30T00:00:00Z")
	if e != nil {
		t.Fatal(e)
	}
	m.Schema = doccorpus.ManifestSchemaV2
	input := m.Inputs[0]
	line := strings.Split(source.String(), "\n")[1] + "\n"
	anchor := doccorpus.Anchor{Repository: m.Repository.ID, Revision: rev, Path: input.Path, Blob: input.Blob, SHA256: input.SHA256, Start: 2, End: 2, SpanSHA256: doccorpus.Digest([]byte(line)), Symbol: "F00000", Authority: "external-provider", Kind: "declared", Reason: "synthetic declared claim"}
	ev := doccorpus.Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []doccorpus.Anchor{anchor}, Limitations: []string{"synthetic declaration; semantic validity unknown"}}
	p := doccorpus.ProviderRecord{Schema: doccorpus.AdoptionProviderSchema, ID: "adapter", Version: "1", Source: m.Repository, Subjects: []doccorpus.Subject{{ID: "adapter:feature", Kind: "capability", Name: "count", Provider: "adapter", Evidence: ev}}, Claims: []doccorpus.Claim{}, Relations: []doccorpus.Relation{}, Journeys: []doccorpus.Journey{}, Observations: []doccorpus.ObservationLink{}, Capabilities: []doccorpus.CapabilityDeclaration{{Name: "claims", State: "present", Reason: "synthetic declared claims"}, {Name: "subjects", State: "present", Reason: "synthetic feature"}}}
	for i := 0; i < claims; i++ {
		p.Claims = append(p.Claims, doccorpus.Claim{ID: fmt.Sprintf("adapter:c%05d", i), Subject: "adapter:feature", Text: fmt.Sprintf("count declared claim %05d", i), Provider: "adapter", Evidence: ev})
	}
	b, e := doccorpus.Encode(p)
	if e != nil {
		t.Fatal(e)
	}
	_ = os.Mkdir(filepath.Join(root, "evidence"), 0700)
	if e := os.WriteFile(filepath.Join(root, "evidence/provider.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	git(t, root, "add", "evidence/provider.json")
	git(t, root, "commit", "-qm", "provider")
	providerRevision := git(t, root, "rev-parse", "HEAD")
	m.Providers = append(m.Providers, doccorpus.Provider{ID: "adapter", Kind: "records", Version: "1", Revision: providerRevision, Record: "evidence/provider.json"})
	m.Scopes = append(m.Scopes, doccorpus.Scope{Path: "evidence/provider.json", Revision: providerRevision})
	m.Inputs = append(m.Inputs, doccorpus.Input{Path: "evidence/provider.json", Revision: providerRevision, Blob: git(t, root, "rev-parse", providerRevision+":evidence/provider.json"), SHA256: doccorpus.Digest(b), Provider: "adapter", Purpose: "provider"})
	a, e := doccorpus.Build(context.Background(), root, m)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := doccorpus.Encode(a)
	if e != nil {
		t.Fatal(e)
	}
	return root, raw
}
func TestIndexedCorpusReproductionAndProvenance(t *testing.T) {
	t.Run("DCP-V1-040 deterministic source producer and pin boundary", func(t *testing.T) {
		root, corpus := fixture(t, 8, 3)
		first, e := Build(context.Background(), root, corpus)
		if e != nil {
			t.Fatal(e)
		}
		second, e := Build(context.Background(), root, corpus)
		if e != nil || !bytes.Equal(first, second) {
			t.Fatal("nonreproducible index", e)
		}
		pinned := doccorpus.Digest(first)
		r, e := Open(context.Background(), first, pinned)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = Open(context.Background(), first, ""); e == nil {
			t.Fatal("missing operator pin accepted")
		}
		forged := *r.artifact
		copy := *forged.Corpus
		copy.Subjects = append([]doccorpus.Subject{}, copy.Subjects...)
		copy.Subjects[0].Name = "self-consistent forged name"
		copy.SHA256 = ""
		raw, e := doccorpus.Encode(&copy)
		if e != nil {
			t.Fatal(e)
		}
		copy.SHA256 = doccorpus.Digest(raw)
		forged.Corpus = &copy
		forged.Index, e = doccorpus.BuildQueryIndex(context.Background(), &copy)
		if e != nil {
			t.Fatal(e)
		}
		fake, e := Encode(forged)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = Open(context.Background(), fake, pinned); e == nil {
			t.Fatal("self-consistent forged provenance bypassed operator pin")
		}
		t.Run("DCP-V1-040 altered posting alone reaches rederivation guard", func(t *testing.T) {
			copy := *r.artifact
			index := *copy.Index
			index.Paths = make(map[string][]doccorpus.RecordRef, len(copy.Index.Paths))
			for path, refs := range copy.Index.Paths {
				index.Paths[path] = append([]doccorpus.RecordRef{}, refs...)
			}
			copy.Index = &index
			unchanged, err := Encode(copy)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Open(context.Background(), unchanged, doccorpus.Digest(unchanged)); err != nil {
				t.Fatal("unchanged copied postings refused", err)
			}
			if len(index.Paths["F00000"]) == 0 {
				t.Fatal("symbol posting control missing")
			}
			index.Paths["F00000"] = []doccorpus.RecordRef{}
			altered, err := Encode(copy)
			if err != nil || bytes.Equal(altered, unchanged) {
				t.Fatal("posting control did not change canonical bytes", err)
			}
			_, err = Open(context.Background(), altered, doccorpus.Digest(altered))
			if err == nil || err.Error() != "indexed offsets or postings differ from corpus" {
				t.Fatal("posting-only control missed rederivation guard", err)
			}
		})
		tampered := *r.artifact
		idx := *tampered.Index
		idx.IDs = map[string]doccorpus.RecordRef{}
		for id, ref := range tampered.Index.IDs {
			idx.IDs[id] = ref
		}
		idx.IDs["adapter:feature"] = doccorpus.RecordRef{Kind: "subject", Offset: 999999}
		tampered.Index = &idx
		bad, e := Encode(tampered)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = Open(context.Background(), bad, doccorpus.Digest(bad)); e == nil {
			t.Fatal("malformed offset accepted despite self-consistent root hash")
		}
		if e = os.RemoveAll(root); e != nil {
			t.Fatal(e)
		}
		for _, q := range []doccorpus.Request{{Operation: "search", Query: "count"}, {Operation: "get", ID: "adapter:feature"}, {Operation: "claims", Path: "src/value.go"}, {Operation: "validate"}} {
			receipt, e := r.Query(context.Background(), q)
			if e != nil || len(receipt.Results) == 0 || receipt.Envelope.SourceValidation != "index-digest-validated; source-revalidation-unavailable" || receipt.Freshness != "unknown" {
				t.Fatal("query needed live source or upgraded validation", e)
			}
			if q.Operation == "validate" {
				encoded, _ := doccorpus.Encode(receipt)
				if strings.Contains(string(encoded), `"validation":"source-rederived"`) {
					t.Fatal("portable validation lied")
				}
			}
		}
	})
}
func TestIndexedCorpusCapacityQualification(t *testing.T) {
	t.Run("DCP-V1-040 synthetic 25000 claims 5000 symbols", func(t *testing.T) {
		started := time.Now()
		root, corpus := fixture(t, 25000, 5000)
		raw, e := Build(context.Background(), root, corpus)
		if e != nil {
			t.Fatal(e)
		}
		again, e := Build(context.Background(), root, corpus)
		if e != nil || !bytes.Equal(raw, again) {
			t.Fatal("large index reproduction", e)
		}
		again = nil
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		r, e := Open(context.Background(), raw, doccorpus.Digest(raw))
		if e != nil {
			t.Fatal(e)
		}
		q := doccorpus.Request{Operation: "get", ID: "adapter:c24999"}
		receipt, e := r.Query(context.Background(), q)
		if e != nil || len(receipt.Results) != 1 {
			t.Fatal("last indexed claim unavailable", e)
		}
		native, e := doccorpus.ParseArtifact(corpus)
		if e != nil {
			t.Fatal(e)
		}
		native.RuntimeIndex = nil
		left, e := doccorpus.Query(native, q, "unknown", []string{"historical embedded artifact; no live source or Git access", "operator digest pins bytes; producer authentication and current source validation NOT_OBSERVED"})
		if e != nil {
			t.Fatal(e)
		}
		left.Envelope.SourceValidation = receipt.Envelope.SourceValidation
		l, _ := doccorpus.Encode(left)
		right, _ := doccorpus.Encode(receipt)
		if !bytes.Equal(l, right) {
			t.Fatal("shared indexed/native query parity")
		}
		runtime.ReadMemStats(&after)
		allocated := after.TotalAlloc - before.TotalAlloc
		// Race instrumentation changes allocation costs; it still exercises every functional check.
		if !raceEnabled && allocated > 1<<30 {
			t.Fatal("synthetic open/query exceeded 1GiB allocation ceiling")
		}
		symbols := 0
		for _, s := range native.Subjects {
			if s.Kind == "symbol" {
				symbols++
			}
		}
		if symbols < 5000 || len(native.Claims) < 25000 {
			t.Fatal("qualification corpus too small")
		}
		measurementMode, ordinaryBudget := "ordinary", "PASS"
		if raceEnabled {
			measurementMode, ordinaryBudget = "race-instrumented", "NOT_RUN"
		}
		t.Logf("allocation_measurement=%s ordinary_allocation_budget=%s ordinary_allocation_ceiling_bytes=%d", measurementMode, ordinaryBudget, uint64(1<<30))
		t.Logf("synthetic qualification claims=%d symbols=%d corpus_bytes=%d index_bytes=%d open_query_total_alloc=%d retained_heap=%d elapsed=%s external_utility=NOT_OBSERVED", len(native.Claims), symbols, len(corpus), len(raw), after.TotalAlloc-before.TotalAlloc, after.HeapAlloc, time.Since(started))
	})
}
func TestIndexedCorpusSwitchParity(t *testing.T) {
	t.Run("DCP-V1-041 strict recorded semantic parity", func(t *testing.T) {
		root, corpus := fixture(t, 2, 2)
		raw, e := Build(context.Background(), root, corpus)
		if e != nil {
			t.Fatal(e)
		}
		r, e := Open(context.Background(), raw, doccorpus.Digest(raw))
		if e != nil {
			t.Fatal(e)
		}
		q := doccorpus.Request{Operation: "get", ID: "adapter:feature"}
		answer, e := r.Query(context.Background(), q)
		if e != nil {
			t.Fatal(e)
		}
		changed := answer
		changed.Freshness = "fresh"
		missing := answer
		missing.Results = append(append([]any{}, answer.Results...), doccorpus.Subject{ID: "bespoke:missing"})
		recording := Recording{RecordingSchema, []Question{{"equal", "get", q, answer}, {"semantics", "get", q, changed}, {"records", "get", q, missing}}}
		input, e := Encode(recording)
		if e != nil {
			t.Fatal(e)
		}
		report, e := Compare(context.Background(), r, input)
		if e != nil {
			t.Fatal(e)
		}
		if report.Tools["get"].Agreement != 1 || report.Tools["get"].Different != 2 {
			t.Fatal("semantic mismatch missed", report)
		}
		for _, d := range report.Questions {
			if d.ID == "records" && (len(d.Missing) != 1 || d.Missing[0] != "bespoke:missing") {
				t.Fatal("missing answer not named", d)
			}
		}
		again, e := Compare(context.Background(), r, input)
		if e != nil {
			t.Fatal(e)
		}
		a, _ := Encode(report)
		b, _ := Encode(again)
		if !bytes.Equal(a, b) {
			t.Fatal("nondeterministic switch parity")
		}
	})
}

func TestSwitchParityStableTypedIdentities(t *testing.T) {
	t.Run("DCP-V1-041 typed identities and duplicate refusal", func(t *testing.T) {
		left := []byte(`{"results":[{"direction":"downstream","relation":{"id":"edge"}},{"link":{"id":"observation"}},{"subject":"flow","kind":"no-tests"}]}`)
		right := []byte(`{"results":[{"id":"new"},{"direction":"downstream","relation":{"id":"edge"}},{"link":{"id":"observation"}},{"subject":"flow","kind":"no-tests"}]}`)
		l, e := answerRecords(left)
		if e != nil {
			t.Fatal(e)
		}
		r, e := answerRecords(right)
		if e != nil {
			t.Fatal(e)
		}
		for id, b := range l {
			if !bytes.Equal(b, r[id]) {
				t.Fatal("insertion changed typed stable identity", id)
			}
		}
		if _, e := answerRecords([]byte(`{"results":[{"id":"duplicate"},{"id":"duplicate"}]}`)); e == nil {
			t.Fatal("duplicate answers overwritten")
		}
	})
}

func TestSwitchParityRepeatedStaleJourneys(t *testing.T) {
	t.Run("DCP-V1-041 two stale journeys retain multiset self parity", func(t *testing.T) {
		a := &doccorpus.Artifact{Schema: doccorpus.SchemaV2, SHA256: strings.Repeat("a", 64), Subjects: []doccorpus.Subject{{ID: "flow", Kind: "flow"}}, Journeys: []doccorpus.Journey{{ID: "journey1", Subject: "flow", Evidence: doccorpus.Evidence{Freshness: "stale"}}, {ID: "journey2", Subject: "flow", Evidence: doccorpus.Evidence{Freshness: "stale"}}}, Capabilities: []doccorpus.Capability{{Name: "gaps", State: "present"}}}
		index, e := doccorpus.BuildQueryIndex(context.Background(), a)
		if e != nil {
			t.Fatal(e)
		}
		a.RuntimeIndex = index
		r := &Reader{artifact: &Artifact{Corpus: a, Index: index}, digest: strings.Repeat("b", 64)}
		q := doccorpus.Request{Operation: "gaps", ID: "flow"}
		answer, e := r.Query(context.Background(), q)
		if e != nil {
			t.Fatal(e)
		}
		stale := 0
		for _, v := range answer.Results {
			if gap, ok := v.(doccorpus.Gap); ok && gap.Kind == "stale-journey" {
				stale++
			}
		}
		if stale != 2 {
			t.Fatal("fixture did not exercise repeated stale gaps", stale)
		}
		input, e := Encode(Recording{RecordingSchema, []Question{{"two-journeys", "bespoke.gaps", q, answer}}})
		if e != nil {
			t.Fatal(e)
		}
		report, e := Compare(context.Background(), r, input)
		if e != nil || report.Tools["bespoke.gaps"].Agreement != 1 {
			t.Fatal("valid repeated gap receipt could not compare to itself", report, e)
		}
		for _, count := range []int{1, 3} {
			expected := answer
			expected.Results = []any{}
			var repeated any
			for _, v := range answer.Results {
				if g, ok := v.(doccorpus.Gap); ok && g.Kind == "stale-journey" {
					repeated = v
				} else {
					expected.Results = append(expected.Results, v)
				}
			}
			for i := 0; i < count; i++ {
				expected.Results = append(expected.Results, repeated)
			}
			input, e := Encode(Recording{RecordingSchema, []Question{{"changed-multiplicity", "bespoke.gaps", q, expected}}})
			if e != nil {
				t.Fatal(e)
			}
			delta, e := Compare(context.Background(), r, input)
			if e != nil {
				t.Fatal(e)
			}
			d := delta.Questions[0]
			missing, extra := 0, 1
			if count == 3 {
				missing, extra = 1, 0
			}
			if d.SemanticAgreement || len(d.Missing) != missing || len(d.Extra) != extra || len(d.Changed) != 0 || delta.Tools["bespoke.gaps"].Different != 1 {
				t.Fatal("Compare multiplicity delta lost", count, d)
			}
		}
	})
}

func TestSwitchParityWeakMultiplicity(t *testing.T) {
	t.Run("DCP-V1-041 stable weak identities across multiplicity", func(t *testing.T) {
		gap := `{"subject":"flow","kind":"stale-journey","reason":"journey evidence is stale"}`
		other := `{"subject":"flow","kind":"stale-journey","reason":"different evidence"}`
		receipt := func(rows ...string) []byte { return []byte(`{"results":[` + strings.Join(rows, ",") + `]}`) }
		cases := []struct {
			name                    string
			want, got               []byte
			missing, extra, changed int
		}{
			{"one to two", receipt(gap), receipt(gap, gap), 0, 1, 0},
			{"two to one", receipt(gap, gap), receipt(gap), 1, 0, 0},
			{"zero to one", receipt(), receipt(gap), 0, 1, 0},
			{"repeated self", receipt(gap, gap), receipt(gap, gap), 0, 0, 0},
			{"permuted weak contents", receipt(gap, other, gap), receipt(other, gap, gap), 0, 0, 0},
			{"add different weak content", receipt(gap), receipt(other, gap), 0, 1, 0},
			{"change weak content", receipt(gap), receipt(other), 1, 1, 0},
			{"strong content change", receipt(`{"id":"claim","value":1}`), receipt(`{"id":"claim","value":2}`), 0, 0, 1},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				left, err := answerRecords(tc.want)
				if err != nil {
					t.Fatal(err)
				}
				right, err := answerRecords(tc.got)
				if err != nil {
					t.Fatal(err)
				}
				missing, extra, changed := 0, 0, 0
				for id, value := range left {
					if actual, ok := right[id]; !ok {
						missing++
					} else if !bytes.Equal(value, actual) {
						changed++
					}
				}
				for id := range right {
					if _, ok := left[id]; !ok {
						extra++
					}
				}
				if missing != tc.missing || extra != tc.extra || changed != tc.changed {
					t.Errorf("missing/extra/changed=%d/%d/%d; want %d/%d/%d", missing, extra, changed, tc.missing, tc.extra, tc.changed)
				}
			})
		}
		for _, duplicate := range []string{
			`{"results":[{"id":"duplicate"},{"id":"duplicate"}]}`,
			`{"results":[{"direction":"downstream","relation":{"id":"edge"}},{"direction":"downstream","relation":{"id":"edge"}}]}`,
			`{"results":[{"link":{"id":"observation"}},{"link":{"id":"observation"}}]}`,
		} {
			if _, err := answerRecords([]byte(duplicate)); err == nil {
				t.Fatal("duplicate strong identity accepted")
			}
		}
	})
}

func TestSwitchParityCompleteReceiptMutations(t *testing.T) {
	t.Run("DCP-V1-041 complete Compare semantics and record deltas", func(t *testing.T) {
		root, corpus := fixture(t, 2, 2)
		raw, err := Build(context.Background(), root, corpus)
		if err != nil {
			t.Fatal(err)
		}
		r, err := Open(context.Background(), raw, doccorpus.Digest(raw))
		if err != nil {
			t.Fatal(err)
		}
		q := doccorpus.Request{Operation: "get", ID: "adapter:feature"}
		answer, err := r.Query(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			name                    string
			edit                    func(*doccorpus.Receipt)
			missing, extra, changed int
		}{
			{"trust", func(a *doccorpus.Receipt) { a.Envelope.ContentStatus = "verified" }, 0, 0, 0},
			{"freshness", func(a *doccorpus.Receipt) { a.Freshness = "fresh" }, 0, 0, 0},
			{"capability", func(a *doccorpus.Receipt) { a.State = "unavailable" }, 0, 0, 0},
			{"omissions", func(a *doccorpus.Receipt) { a.Omitted++ }, 0, 0, 0},
			{"retirement", func(a *doccorpus.Receipt) { a.Envelope.Retirement.State = "retired-by-policy" }, 0, 0, 0},
			{"citations", func(a *doccorpus.Receipt) { a.Citations = []doccorpus.Anchor{} }, 0, 0, 0},
			{"limitations", func(a *doccorpus.Receipt) { a.Limitations = append(a.Limitations, "recorded limitation") }, 0, 0, 0},
			{"selection", func(a *doccorpus.Receipt) {
				a.Selection = &doccorpus.TestRecommendation{State: "full-relevant-suite-required", Required: "full relevant suite", MissingEvidence: []string{"not observed"}}
			}, 0, 0, 0},
			{"details", func(a *doccorpus.Receipt) {
				a.Details = map[string]doccorpus.RecordDetails{"adapter:feature": {ClaimKind: "behavior"}}
			}, 0, 0, 0},
			{"missing", func(a *doccorpus.Receipt) { a.Results = append(a.Results, doccorpus.Subject{ID: "bespoke:missing"}) }, 1, 0, 0},
			{"extra", func(a *doccorpus.Receipt) { a.Results = []any{} }, 0, 3, 0},
			{"changed", func(a *doccorpus.Receipt) { s := a.Results[0].(map[string]any); s["name"] = "Recorded different name" }, 0, 0, 1},
		} {
			t.Run(tc.name, func(t *testing.T) {
				// Decode to detach all slices/maps/pointers before a single edit.
				encoded, _ := doccorpus.Encode(answer)
				var expected doccorpus.Receipt
				if err := stdjson.Unmarshal(encoded, &expected); err != nil {
					t.Fatal(err)
				}
				tc.edit(&expected)
				recording, _ := Encode(Recording{RecordingSchema, []Question{{"mutated", "bespoke.get", q, expected}, {"control", "other.get", q, answer}}})
				report, err := Compare(context.Background(), r, recording)
				if err != nil {
					t.Fatal(err)
				}
				if report.Tools["bespoke.get"].Different != 1 || report.Tools["other.get"].Agreement != 1 {
					t.Fatal("per-tool semantic mismatch lost", report)
				}
				for _, d := range report.Questions {
					if d.ID == "mutated" && (d.SemanticAgreement || len(d.Missing) != tc.missing || len(d.Extra) != tc.extra || len(d.Changed) != tc.changed) {
						t.Fatal("record diagnostics disagree", d)
					}
				}
				again, err := Compare(context.Background(), r, recording)
				left, _ := Encode(report)
				right, _ := Encode(again)
				if err != nil || !bytes.Equal(left, right) {
					t.Fatal("non-deterministic report", err)
				}
			})
		}
	})
}

func TestIndexedCorpusByteAdmissionBounds(t *testing.T) {
	t.Run("DCP-V1-040 wrapper and embedded input byte bounds", func(t *testing.T) {
		oversized := make([]byte, MaxBytes+1)
		if _, err := Open(context.Background(), oversized, doccorpus.Digest(oversized)); err == nil || err.Error() != "operator index digest missing or mismatched" {
			t.Fatal("correctly pinned oversized wrapper did not reach byte guard", err)
		}
		if _, err := doccorpus.ParseArtifact(make([]byte, doccorpus.MaxCorpusBytes+1)); err == nil || !strings.Contains(err.Error(), "input bound exceeded") {
			t.Fatal("oversized embedded corpus did not reach byte guard", err)
		}
		path := filepath.Join(t.TempDir(), "oversized.json")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = f.Truncate(MaxBytes + 1); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadFile(path, MaxBytes); err == nil || err.Error() != "unsafe or oversized input" {
			t.Fatal("oversized file did not reach byte guard", err)
		}
	})
}

func TestIndexedVerifiedCorpusIsolation(t *testing.T) {
	t.Run("DCP-V1-040 RCP-V0-007 source-validated token and unchanged ordinary cold oracle", func(t *testing.T) {
		ctx := context.Background()
		root, corpus := fixture(t, 8, 3)
		verified, err := doccorpus.OpenVerified(ctx, root, corpus)
		if err != nil {
			t.Fatal(err)
		}
		cold, err := Build(ctx, root, corpus)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := BuildVerified(ctx, verified)
		if err != nil || !bytes.Equal(actual, cold) {
			t.Fatal("verified index differs from cold source oracle", err)
		}
		if _, err := BuildVerified(ctx, doccorpus.VerifiedCorpus{}); err == nil {
			t.Fatal("zero token admitted")
		}
		snapshot, _ := verified.Snapshot()
		snapshot.Subjects[0].Name = "caller supplied fake corpus"
		copy, _ := verified.Bytes()
		copy[0] = '['
		after, err := BuildVerified(ctx, verified)
		if err != nil || !bytes.Equal(after, actual) {
			t.Fatal("caller mutated indexed token", err)
		}
		reader, err := Open(ctx, actual, doccorpus.Digest(actual))
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := reader.Query(ctx, doccorpus.Request{Operation: "inventory", Limit: 1})
		if err != nil || receipt.Envelope == nil || receipt.Envelope.SourceValidation != "index-digest-validated; source-revalidation-unavailable" {
			t.Fatal("consumer trust upgraded", err)
		}
	})
}
