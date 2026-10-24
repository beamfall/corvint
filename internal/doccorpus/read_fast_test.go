package doccorpus

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// builtFixture builds the small native fixture and returns its root, artifact and bytes.
func builtFixture(t *testing.T) (string, *Artifact, []byte) {
	t.Helper()
	root, m := fixture(t)
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	return root, a, data
}

// rehashed re-encodes a after edit with a recomputed content digest, as a deliberate editor would.
func rehashed(t *testing.T, a *Artifact, edit func(*Artifact)) []byte {
	t.Helper()
	copy := *a
	edit(&copy)
	copy.SHA256 = ""
	copy.SHA256 = testHash(t, copy)
	data, err := Encode(copy)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// removeObject deletes one loose object, so any rederivation that reads it fails while the
// commit, its trees and the worktree stay intact.
func removeObject(t *testing.T, root, oid string) {
	t.Helper()
	path := filepath.Join(root, ".git", "objects", oid[:2], oid[2:])
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove loose object %s: %v", oid, err)
	}
}

// TestDCRV1410VerifiedReadSkipsRederivation: the verified read decodes the same artifact the
// rederivation produces, reports how it was admitted, and reads no pinned input body: with one
// pinned input blob deleted from the object store, rederivation refuses and the verified read
// still answers with live freshness.
func TestDCRV1410VerifiedReadSkipsRederivation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, a, data := builtFixture(t)
	verified, err := OpenRead(ctx, root, data, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rederived, err := OpenRead(ctx, root, data, ReadOptions{Rederive: true})
	if err != nil {
		t.Fatal(err)
	}
	if verified.validation != ValidationVerified || rederived.validation != ValidationRederived {
		t.Fatalf("validation = %q, %q", verified.validation, rederived.validation)
	}
	for _, got := range []*Artifact{verified, rederived} {
		if !EncodedEqual(got, data) {
			t.Fatal("admitted artifact differs from the bytes read")
		}
	}
	info := func(a *Artifact) any {
		r, err := Query(a, Request{Operation: "info"}, "fresh", nil)
		if err != nil {
			t.Fatal(err)
		}
		return r.Results[0].(map[string]any)["validation"]
	}
	if info(verified) != ValidationVerified || info(rederived) != ValidationRederived || info(a) != ValidationRederived {
		t.Fatalf("info validation = %v, %v, %v", info(verified), info(rederived), info(a))
	}
	removeObject(t, root, a.Manifest.Inputs[0].Blob)
	if _, err := Open(ctx, root, data); err == nil {
		t.Fatal("rederivation read a deleted pinned blob")
	}
	again, err := OpenRead(ctx, root, data, ReadOptions{})
	if err != nil {
		t.Fatalf("verified read rederived: %v", err)
	}
	overlay, err := LiveFreshness(ctx, root, again)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := QueryLive(ctx, again, Request{Operation: "search", Query: "Count"}, overlay)
	if err != nil || receipt.Freshness != "fresh" || len(receipt.Results) == 0 {
		t.Fatalf("verified read receipt: %+v %v", receipt, err)
	}
}

// TestDCRV1410VerifiedReadRefusals: every check the verified read trusts instead of rederiving
// refuses on its own; nothing unverified is served. A deliberately rehashed record edit passes
// the verified checks and is refused by the rederivation the caller can request.
func TestDCRV1410VerifiedReadRefusals(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, a, data := builtFixture(t)
	flipped := bytes.Clone(data)
	at := bytes.Index(flipped, []byte(`"Count`))
	flipped[at+1] = 'K'
	spaced := append([]byte{' '}, data...)
	cases := []struct {
		name, want string
		data       []byte
	}{
		{"tampered byte", "artifact digest mismatch", flipped},
		{"noncanonical", "invalid closed JSON input", append(bytes.Clone(data), '{')},
		{"leading whitespace", "noncanonical artifact", spaced},
		{"builder", "derivation version differs", rehashed(t, a, func(c *Artifact) {
			c.Builder.Revision = "0000000000000000000000000000000000000000"
			c.Builder.Engine = strings.Replace(c.Builder.Engine, "/derivation-"+DerivationVersion+"/", "/derivation-0/", 1)
		})},
		{"manifest digest", "manifest digest differs", rehashed(t, a, func(c *Artifact) { c.ManifestSHA256 = strings.Repeat("0", 64) })},
		{"profile digest", "profile digest differs", rehashed(t, a, func(c *Artifact) { c.ProfileSHA256 = strings.Repeat("0", 64) })},
		{"schema pair", "schemas differ", rehashed(t, a, func(c *Artifact) { c.Schema = SchemaV2 })},
		{"tree", "recorded tree differs", rehashed(t, a, func(c *Artifact) { c.Tree = strings.Repeat("1", 40) })},
		{"missing revision", "", rehashed(t, a, func(c *Artifact) {
			c.Manifest.Inputs = append([]Input{}, c.Manifest.Inputs...)
			c.Manifest.Inputs[0].Revision = strings.Repeat("2", 40)
			c.ManifestSHA256 = testHash(t, c.Manifest)
		})},
	}
	for _, tc := range cases {
		if _, err := OpenRead(ctx, root, tc.data, ReadOptions{}); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: verified read admitted or refused otherwise: %v", tc.name, err)
		}
	}
	// The header work runs Git in the checkouts the manifest names, so an invalid manifest never
	// reaches it.
	escaping := rehashed(t, a, func(c *Artifact) {
		c.Manifest.AdditionalRepositories = []AdditionalRepository{{ID: c.Manifest.Repository.ID, Revision: c.Manifest.Repository.Revision, Root: "../../elsewhere"}}
		c.ManifestSHA256 = testHash(t, c.Manifest)
	})
	headers := 0
	if _, err := OpenRead(ctx, root, escaping, ReadOptions{Header: func(*Artifact) { headers++ }}); err == nil || headers != 0 {
		t.Fatalf("invalid manifest: err=%v header runs=%d", err, headers)
	}
	edited := rehashed(t, a, func(c *Artifact) {
		c.Claims = append([]Claim{}, c.Claims...)
		c.Claims[0].Text = "invented behavior"
	})
	if _, err := OpenRead(ctx, root, edited, ReadOptions{}); err != nil {
		t.Fatalf("a rehashed record edit is only detectable by rederivation: %v", err)
	}
	if _, err := OpenRead(ctx, root, edited, ReadOptions{Rederive: true}); err == nil {
		t.Fatal("rederivation admitted a rehashed record edit")
	}
}

// TestDCRV1414ParallelDecodeMatchesClosedDecode: the parallel decoder admits exactly the canonical
// artifact the closed decoder admits, with the same records, and refuses every other shape.
func TestDCRV1414ParallelDecodeMatchesClosedDecode(t *testing.T) {
	t.Parallel()
	_, a, data := builtFixture(t)
	parallel, err := decodeArtifactParallel(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parallel, a) {
		t.Fatal("parallel decode differs from the built artifact")
	}
	digest, err := canonicalArtifactDigest(a, data)
	if err != nil || digest != a.SHA256 {
		t.Fatalf("digest from bytes = %s %v, want %s", digest, err, a.SHA256)
	}
	insert := func(after string, text string) []byte {
		at := bytes.Index(data, []byte(after))
		if at < 0 {
			t.Fatalf("no %q in fixture", after)
		}
		at += len(after)
		return append(append(bytes.Clone(data[:at]), text...), data[at:]...)
	}
	replace := func(old, new string) []byte {
		if !bytes.Contains(data, []byte(old)) {
			t.Fatalf("no %q in fixture", old)
		}
		return bytes.Replace(data, []byte(old), []byte(new), 1)
	}
	cases := map[string][]byte{
		"space after claims bracket":  insert(`"claims":[`, " "),
		"space between elements":      replace(`},{"id"`, `}, {"id"`),
		"space before closing":        replace(`}],"relations"`, `} ],"relations"`),
		"space inside element":        replace(`"subject":`, `"subject": `),
		"unknown top-level member":    insert(`{`, `"extra":1,`),
		"unknown element member":      insert(`"claims":[{`, `"extra":1,`),
		"duplicate member":            insert(`{`, `"tree":"x",`),
		"null records":                replace(`"journeys":[]`, `"journeys":null`),
		"empty omitempty member":      insert(`{`, `"details":{},`),
		"renamed member":              replace(`"schema":`, `"schemz":`),
		"trailing value":              append(bytes.Clone(data[:len(data)-1]), []byte("{}\n")...),
		"missing trailing newline":    data[:len(data)-1],
		"element encodes differently": replace(`"start_line":1`, `"start_line":1.0`),
	}
	for name, bad := range cases {
		_, closed := ParseArtifact(bad)
		decoded, err := decodeArtifactParallel(bad, nil)
		if err == nil {
			err = hashArtifactTail(bad).verify(decoded)
		}
		if err == nil || closed == nil {
			t.Errorf("%s: parallel=%v closed=%v; both must refuse", name, err, closed)
		}
	}
}

// TestDCRV1412SymbolIndexBuiltOnlyWhenNeeded: live freshness indexes no source; the first cited
// symbol anchor whose pinned blob changed builds the live index once, and an index built from
// another checkout state than the overlay observed is refused.
func TestDCRV1412SymbolIndexBuiltOnlyWhenNeeded(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, a, _ := builtFixture(t)
	overlay, err := LiveFreshness(ctx, root, a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := QueryLive(ctx, a, Request{Operation: "search", Query: "showCount"}, overlay); err != nil {
		t.Fatal(err)
	}
	if overlay.symbols.index != nil {
		t.Fatal("an unchanged cited anchor built the live index")
	}
	original, err := os.ReadFile(filepath.Join(root, "src/view.ts"))
	if err != nil {
		t.Fatal(err)
	}
	commitFiles(t, root, map[string]string{"src/view.ts": insertLines(string(original), 20, "const padding = 1;\n")})
	moved, err := LiveFreshness(ctx, root, a)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := QueryLive(ctx, a, Request{Operation: "search", Query: "showCount"}, moved)
	if err != nil {
		t.Fatal(err)
	}
	if check := symbolCheck(receipt, "src/view.ts", "showCount"); check == nil || check.State != SymbolAnchorMoved || moved.symbols.index == nil {
		t.Fatalf("moved anchor: %+v index built=%v", receipt.SymbolAnchors, moved.symbols.index != nil)
	}
	stale, err := LiveFreshness(ctx, root, a)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src/report.py"), []byte("def changed():\n    return 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := stale.symbols.liveIndex(ctx); err == nil || !strings.Contains(err.Error(), "changed after its freshness observation") {
		t.Fatalf("index from a later checkout state admitted: %v", err)
	}
	withheld, err := QueryLive(ctx, a, Request{Operation: "search", Query: "showCount"}, stale)
	if err != nil {
		t.Fatal(err)
	}
	if check := symbolCheck(withheld, "src/view.ts", "showCount"); check == nil || check.State != SymbolAnchorUnresolved || !strings.Contains(strings.Join(withheld.Limitations, "\n"), "symbol-anchor re-resolution withheld") {
		t.Fatalf("mixed-state re-resolution not withheld: %+v %v", withheld.SymbolAnchors, withheld.Limitations)
	}
}

// TestDCRV1418LargeFixtureVerifiedReadIsFast: on a generated corpus with thousands of inputs
// across additional repositories, a verified read with live freshness is several times faster
// than the rederivation every read used to run, and answers identically. Wall times are logged;
// the ratio, not an absolute bound, is asserted so host load cannot flake it.
func TestDCRV1418LargeFixtureVerifiedReadIsFast(t *testing.T) {
	if testing.Short() {
		t.Skip("large fixture")
	}
	ctx := context.Background()
	root, _ := largeReadFixture(t, t.TempDir(), readFixtureScale{docs: 1200, sections: 4, sources: 1500, additional: 2, additionalFiles: 600})
	data, err := ReadCorpusFile(root, "corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	read := func(opts ReadOptions) (Receipt, time.Duration) {
		start := time.Now()
		a, err := OpenRead(ctx, root, data, opts)
		if err != nil {
			t.Fatal(err)
		}
		overlay, err := LiveFreshness(ctx, root, a)
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := QueryLive(ctx, a, Request{Operation: "search", Query: "ledger", Limit: 20}, overlay)
		if err != nil {
			t.Fatal(err)
		}
		return receipt, time.Since(start)
	}
	rederived, slow := read(ReadOptions{Rederive: true})
	verified, cold := read(ReadOptions{})
	_, warm := read(ReadOptions{})
	t.Logf("artifact %d bytes: rederived read %v, verified read %v then %v", len(data), slow, cold, warm)
	if verified.Freshness != "fresh" || !reflect.DeepEqual(verified, rederived) {
		t.Fatal("verified read answers differently from the rederived read")
	}
	if best := min(cold, warm); best*3 > slow {
		t.Fatalf("verified read %v is not at least 3x faster than rederivation %v", best, slow)
	}
}

// BenchmarkDCRV1418VerifiedRead measures one verified read with live freshness and a search.
func BenchmarkDCRV1418VerifiedRead(b *testing.B) {
	ctx := context.Background()
	root, _ := largeReadFixture(b, b.TempDir(), readFixtureScale{docs: 1200, sections: 4, sources: 1500, additional: 2, additionalFiles: 600})
	data, err := ReadCorpusFile(root, "corpus.json")
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		a, err := OpenRead(ctx, root, data, ReadOptions{})
		if err != nil {
			b.Fatal(err)
		}
		overlay, err := LiveFreshness(ctx, root, a)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := QueryLive(ctx, a, Request{Operation: "search", Query: "ledger", Limit: 20}, overlay); err != nil {
			b.Fatal(err)
		}
	}
}

// TestDCRV1410UnattestedBuilderRederives: a dirty or unrecorded builder label is shared by builds
// of different source, so a running builder with such a label never treats an equal recorded
// label as rebuild equivalence. It rederives instead: a record edit with a recomputed digest,
// which stands for an artifact written by another build that shares the label and extracts
// differently, is refused, and an untouched artifact is admitted as source-rederived. Only an
// attested label (a recorded revision without uncommitted changes) admits the verified read.
func TestDCRV1410UnattestedBuilderRederives(t *testing.T) {
	for label, attested := range map[string]bool{
		"unrecorded": false,
		"0123456789abcdef0123456789abcdef01234567+dirty": false,
		"0123456789abcdef0123456789abcdef01234567":       true,
		strings.Repeat("ab", 32):                         true,
		"":                                               false,
		"garbage":                                        false,
		"0123456789ABCDEF0123456789ABCDEF01234567": false,
		"0123456789abcdef0123456789abcdef0123456":  false,
	} {
		if attestedBuilder(Builder{Version: Schema, Revision: label}) != attested {
			t.Errorf("attestedBuilder(%q) != %v", label, attested)
		}
	}
	if attestedBuilder(currentBuilder()) {
		t.Skip("this test binary records an attested build")
	}
	unattestedBuilderTrusted.Store(false)
	t.Cleanup(func() { unattestedBuilderTrusted.Store(true) })
	ctx := context.Background()
	root, a, data := builtFixture(t)
	read, err := OpenRead(ctx, root, data, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if read.validation != ValidationRederived {
		t.Fatalf("unattested builder admitted the artifact as %s", read.validation)
	}
	edited := rehashed(t, a, func(c *Artifact) {
		c.Claims = append([]Claim{}, c.Claims...)
		c.Claims[0].Text = "behavior another build with the same label extracted"
	})
	if _, err := OpenRead(ctx, root, edited, ReadOptions{}); err == nil {
		t.Fatal("an unattested builder admitted another build's records without rederiving them")
	}
	headers := 0
	if _, err := OpenRead(ctx, root, data, ReadOptions{Header: func(*Artifact) { headers++ }}); err != nil || headers != 0 {
		t.Fatalf("rederived read ran the verified read's header work: %v %d", err, headers)
	}
}
