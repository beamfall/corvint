package doccorpus

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// asOfFixture builds the provider fixture with the observation input declaring
// asOf, and writes probe (when non-empty) outside every declared scope.
func asOfFixture(t *testing.T, asOf InputAsOf, probe string) (string, *Artifact) {
	t.Helper()
	root, m := providerFixture(t, nil)
	for i := range m.Inputs {
		if m.Inputs[i].Path == "evidence/run.json" {
			m.Inputs[i].AsOf = &asOf
		}
	}
	if probe != "" {
		writeProbe(t, root, asOf.Probe, probe)
	}
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	return root, a
}

func writeProbe(t *testing.T, root, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func hasPrefixed(limits []string, prefix string) bool {
	return slices.ContainsFunc(limits, func(x string) bool { return strings.HasPrefix(x, prefix) })
}

func TestDCPV1574InputAsOfValidation(t *testing.T) {
	t.Parallel()
	root, m := providerFixture(t, nil)
	baseline, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	// An undeclared as-of point keeps the manifest bytes (omitzero).
	if data, err := Encode(baseline.Manifest); err != nil || strings.Contains(string(data), "as_of") {
		t.Fatalf("undeclared as_of changed manifest bytes: %v", err)
	}
	// An explicit but empty as_of decodes as declared, so validation refuses it.
	var decoded Input
	if err := decode([]byte(`{"path":"p","revision":"r","blob":"b","sha256":"s","provider":"x","purpose":"evidence","as_of":{}}`), &decoded); err != nil || decoded.AsOf == nil || validInputAsOf(decoded) == "" {
		t.Fatalf("empty as_of was not kept as a refused declaration: err=%v", err)
	}
	index := slices.IndexFunc(m.Inputs, func(in Input) bool { return in.Path == "evidence/run.json" })
	source := slices.IndexFunc(m.Inputs, func(in Input) bool { return in.Purpose == "source" })
	for name, c := range map[string]struct {
		input int
		asOf  InputAsOf
		ok    bool
	}{
		"sequence":           {index, InputAsOf{Source: "upstream ledger", Kind: "sequence", Value: "42", Probe: "upstream/seq"}, true},
		"timestamp":          {index, InputAsOf{Source: "upstream", Kind: "timestamp", Value: "2026-10-25T00:00:00Z"}, true},
		"sha256":             {index, InputAsOf{Source: "upstream", Kind: "sha256", Value: strings.Repeat("a", 64)}, true},
		"source purpose":     {source, InputAsOf{Source: "upstream", Kind: "sequence", Value: "1"}, false},
		"empty source":       {index, InputAsOf{Kind: "sequence", Value: "1"}, false},
		"control source":     {index, InputAsOf{Source: "up\nstream", Kind: "sequence", Value: "1"}, false},
		"unknown kind":       {index, InputAsOf{Source: "upstream", Kind: "version", Value: "1"}, false},
		"leading zero":       {index, InputAsOf{Source: "upstream", Kind: "sequence", Value: "042"}, false},
		"bad timestamp":      {index, InputAsOf{Source: "upstream", Kind: "timestamp", Value: "yesterday"}, false},
		"upper hex":          {index, InputAsOf{Source: "upstream", Kind: "sha256", Value: strings.Repeat("A", 64)}, false},
		"parent probe":       {index, InputAsOf{Source: "upstream", Kind: "sequence", Value: "1", Probe: "../seq"}, false},
		"git control probe":  {index, InputAsOf{Source: "upstream", Kind: "sequence", Value: "1", Probe: ".git/HEAD"}, false},
		"case git probe":     {index, InputAsOf{Source: "upstream", Kind: "sequence", Value: "1", Probe: ".GIT/HEAD"}, false},
		"nested git probe":   {index, InputAsOf{Source: "upstream", Kind: "sequence", Value: "1", Probe: "sub/.Git/x"}, false},
		"empty declaration":  {index, InputAsOf{}, false},
		"absolute probe":     {index, InputAsOf{Source: "upstream", Kind: "sequence", Value: "1", Probe: "/etc/passwd"}, false},
		"value without kind": {index, InputAsOf{Source: "upstream", Value: "1"}, false},
	} {
		edited := m
		edited.Inputs = slices.Clone(m.Inputs)
		asOf := c.asOf
		edited.Inputs[c.input].AsOf = &asOf
		a, err := Build(context.Background(), root, edited)
		if c.ok != (err == nil) {
			t.Fatalf("%s: err=%v, want ok=%v", name, err, c.ok)
		}
		if c.ok && (a.Manifest.Inputs[c.input].AsOf == nil || *a.Manifest.Inputs[c.input].AsOf != c.asOf) {
			t.Fatalf("%s: as_of not recorded in build provenance", name)
		}
	}
}

func TestDCPV1575DCPV1576InputAsOfLiveStates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("sequence fresh then lagging", func(t *testing.T) {
		root, a := asOfFixture(t, InputAsOf{Source: "upstream ledger", Kind: "sequence", Value: "42", Probe: "upstream/seq"}, "42\n")
		overlay, err := LiveFreshness(ctx, root, a)
		if err != nil || overlay.State != "fresh" {
			t.Fatalf("equal sequence: %+v %v", overlay, err)
		}
		writeProbe(t, root, "upstream/seq", "45\n")
		overlay, err = LiveFreshness(ctx, root, a)
		if err != nil || overlay.State != "stale" || len(overlay.inputs) != 1 || overlay.inputs[0].Lag == nil || *overlay.inputs[0].Lag != 3 || overlay.inputs[0].LagUnit != "entries" {
			t.Fatalf("advanced sequence: %+v %v", overlay, err)
		}
		if !hasPrefixed(overlay.Limitations, "compiled input evidence/run.json (upstream ledger) as of sequence 42; source has advanced by 3 entries") {
			t.Fatalf("missing per-input limitation: %v", overlay.Limitations)
		}
		r, err := QueryLive(ctx, a, Request{Operation: "info"}, overlay)
		if err != nil {
			t.Fatal(err)
		}
		if r.Freshness != "stale" || r.Envelope.Freshness != "stale" || !hasPrefixed(r.Envelope.Limitations, "compiled input evidence/run.json") {
			t.Fatalf("answer reported a lagging input as current: %s %s %v", r.Freshness, r.Envelope.Freshness, r.Envelope.Limitations)
		}
		info := r.Results[0].(map[string]any)
		listed, ok := info["inputs_as_of"].([]InputFreshness)
		if !ok || len(listed) != 1 || listed[0].Path != "evidence/run.json" || listed[0].Live != "45" || *listed[0].Lag != 3 {
			t.Fatalf("info does not list per-input lag: %#v", info["inputs_as_of"])
		}
	})
	t.Run("behind sequence is unknown", func(t *testing.T) {
		root, a := asOfFixture(t, InputAsOf{Source: "upstream", Kind: "sequence", Value: "42", Probe: "upstream/seq"}, "41")
		overlay, err := LiveFreshness(ctx, root, a)
		if err != nil || overlay.State != "unknown" || !hasPrefixed(overlay.Limitations, "compiled input evidence/run.json (upstream) as of sequence 42; source position NOT_OBSERVED") {
			t.Fatalf("behind sequence: %+v %v", overlay, err)
		}
	})
	t.Run("no probe or missing probe is unknown, never fresh", func(t *testing.T) {
		for _, probe := range []string{"", "upstream/absent"} {
			root, a := asOfFixture(t, InputAsOf{Source: "upstream", Kind: "sequence", Value: "1", Probe: probe}, "")
			overlay, err := LiveFreshness(ctx, root, a)
			if err != nil || overlay.State != "unknown" || !hasPrefixed(overlay.Limitations, "compiled input evidence/run.json (upstream) as of sequence 1; source position NOT_OBSERVED") {
				t.Fatalf("probe %q: %+v %v", probe, overlay, err)
			}
			r, err := QueryLive(ctx, a, Request{Operation: "search", Query: "count"}, overlay)
			if err != nil || r.Freshness == "fresh" || r.Envelope.Freshness == "fresh" {
				t.Fatalf("probe %q answer reported current: %+v %v", probe, r.Freshness, err)
			}
			if state, _, err := Freshness(ctx, root, a); err != nil || state == "fresh" {
				t.Fatalf("probe %q: maintenance state %q %v", probe, state, err)
			}
		}
	})
	t.Run("sha256 probe", func(t *testing.T) {
		root, a := asOfFixture(t, InputAsOf{Source: "upstream", Kind: "sha256", Value: Digest([]byte("v1")), Probe: "upstream/snapshot"}, "v1")
		if overlay, err := LiveFreshness(ctx, root, a); err != nil || overlay.State != "fresh" {
			t.Fatalf("equal digest: %+v %v", overlay, err)
		}
		writeProbe(t, root, "upstream/snapshot", "v2")
		if overlay, err := LiveFreshness(ctx, root, a); err != nil || overlay.State != "stale" || overlay.inputs[0].Lag != nil {
			t.Fatalf("changed digest: %+v %v", overlay, err)
		}
	})
	t.Run("timestamp probe", func(t *testing.T) {
		root, a := asOfFixture(t, InputAsOf{Source: "upstream", Kind: "timestamp", Value: "2026-01-01T00:00:00Z", Probe: "upstream/export"}, "x")
		old := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
		if err := os.Chtimes(filepath.Join(root, "upstream/export"), old, old); err != nil {
			t.Fatal(err)
		}
		if overlay, err := LiveFreshness(ctx, root, a); err != nil || overlay.State != "fresh" {
			t.Fatalf("unmodified probe: %+v %v", overlay, err)
		}
		later := time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC)
		if err := os.Chtimes(filepath.Join(root, "upstream/export"), later, later); err != nil {
			t.Fatal(err)
		}
		overlay, err := LiveFreshness(ctx, root, a)
		if err != nil || overlay.State != "stale" || *overlay.inputs[0].Lag != 60 || overlay.inputs[0].LagUnit != "seconds" {
			t.Fatalf("modified probe: %+v %v", overlay, err)
		}
	})
	t.Run("non-live reader is unknown", func(t *testing.T) {
		_, a := asOfFixture(t, InputAsOf{Source: "upstream", Kind: "sequence", Value: "1", Probe: "upstream/seq"}, "1")
		r, err := Query(a, Request{Operation: "info"}, "fresh", nil)
		if err != nil || r.Freshness != "unknown" || r.Envelope.Freshness != "unknown" || !hasPrefixed(r.Limitations, "compiled input evidence/run.json (upstream) as of sequence 1; source position NOT_OBSERVED: no live reader probed the source") {
			t.Fatalf("static reader: %+v %v", r, err)
		}
		if listed, ok := r.Results[0].(map[string]any)["inputs_as_of"].([]InputFreshness); !ok || listed[0].State != "unknown" {
			t.Fatalf("static info listing: %#v", r.Results[0])
		}
	})
	t.Run("undeclared inputs keep fresh", func(t *testing.T) {
		root, m := providerFixture(t, nil)
		a, err := Build(ctx, root, m)
		if err != nil {
			t.Fatal(err)
		}
		overlay, err := LiveFreshness(ctx, root, a)
		if err != nil || overlay.State != "fresh" {
			t.Fatalf("undeclared: %+v %v", overlay, err)
		}
		r, err := QueryLive(ctx, a, Request{Operation: "info"}, overlay)
		if _, listed := r.Results[0].(map[string]any)["inputs_as_of"]; err != nil || listed || r.Freshness != "fresh" {
			t.Fatalf("undeclared info: %+v %v", r.Results[0], err)
		}
	})
}

func TestDCPV1577ProbeChangesObservation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, a := asOfFixture(t, InputAsOf{Source: "upstream", Kind: "sequence", Value: "1", Probe: "upstream/seq"}, "1")
	settled := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(root, "upstream/seq"), settled, settled); err != nil {
		t.Fatal(err)
	}
	before, err := CorpusObservation(ctx, root, a)
	if err != nil {
		t.Fatal(err)
	}
	overlay, err := LiveFreshness(ctx, root, a)
	if err != nil || overlay.Observation() != before {
		t.Fatalf("overlay key differs from the observation: %v", err)
	}
	writeProbe(t, root, "upstream/seq", "22")
	if err := os.Chtimes(filepath.Join(root, "upstream/seq"), settled.Add(time.Minute), settled.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	after, err := CorpusObservation(ctx, root, a)
	if err != nil || after == before {
		t.Fatalf("probe change left the reuse key unchanged: %v", err)
	}
	// A same-size rewrite that restores the modification time still changes
	// the key, because a sequence probe contributes its content.
	writeProbe(t, root, "upstream/seq", "33")
	if err := os.Chtimes(filepath.Join(root, "upstream/seq"), settled.Add(time.Minute), settled.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if same, err := CorpusObservation(ctx, root, a); err != nil || same == after {
		t.Fatalf("same-size, same-mtime probe rewrite left the reuse key unchanged: %v", err)
	}
	// A probe modified inside the racy window never yields a reusable key.
	writeProbe(t, root, "upstream/seq", "3")
	first, _ := CorpusObservation(ctx, root, a)
	second, _ := CorpusObservation(ctx, root, a)
	if first == second {
		t.Fatal("racy probe produced a reusable key")
	}
}
