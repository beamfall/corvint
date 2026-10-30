package doccorpus

import (
	"context"
	"reflect"
	"testing"
)

// manyScopesScale is the DCR-V1-451 shape at test size: one single-file scope
// per page, 1,200 native pages and two additional repositories of 600 pinned
// files, so 2,400 scopes over 4,101 tracked paths.
var manyScopesScale = readFixtureScale{docs: 1200, sections: 2, sources: 1500, additional: 2, additionalFiles: 600, manyScopes: true}

func manyScopesRead(tb testing.TB, ctx context.Context, root string, data []byte, opts ReadOptions) Receipt {
	tb.Helper()
	a, err := OpenRead(ctx, root, data, opts)
	if err != nil {
		tb.Fatal(err)
	}
	overlay, err := LiveFreshness(ctx, root, a)
	if err != nil {
		tb.Fatal(err)
	}
	receipt, err := QueryLive(ctx, a, Request{Operation: "search", Query: "ledger", Limit: 20}, overlay)
	if err != nil {
		tb.Fatal(err)
	}
	return receipt
}

// TestDCRV1451ManyScopesReadMatchesRederivation: a manifest of thousands of
// single-file scopes builds, and its verified read with live freshness answers
// exactly as the rederivation, fresh on the unchanged checkout.
func TestDCRV1451ManyScopesReadMatchesRederivation(t *testing.T) {
	if testing.Short() {
		t.Skip("large fixture")
	}
	ctx := context.Background()
	root, _ := largeReadFixture(t, t.TempDir(), manyScopesScale)
	data, err := ReadCorpusFile(root, "corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	a, err := ParseArtifact(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Manifest.Scopes) < 2400 {
		t.Fatalf("fixture declares %d scopes, want at least 2400", len(a.Manifest.Scopes))
	}
	verified := manyScopesRead(t, ctx, root, data, ReadOptions{})
	rederived := manyScopesRead(t, ctx, root, data, ReadOptions{Rederive: true})
	if verified.Freshness != "fresh" || !reflect.DeepEqual(verified, rederived) {
		t.Fatalf("verified read answers differently from the rederived read: %s against %s", verified.Freshness, rederived.Freshness)
	}
}

// BenchmarkDCRV1451ManyScopesVerifiedRead measures a verified read with live
// freshness and a search over the many-scopes fixture: the scope checks of
// validateManifest and liveRepository.
func BenchmarkDCRV1451ManyScopesVerifiedRead(b *testing.B) {
	ctx := context.Background()
	root, _ := largeReadFixture(b, b.TempDir(), manyScopesScale)
	data, err := ReadCorpusFile(root, "corpus.json")
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		manyScopesRead(b, ctx, root, data, ReadOptions{})
	}
}

// BenchmarkDCRV1451ManyScopesRederivedRead measures the rederiving read over
// the many-scopes fixture: two pin passes over every revision, each with the
// compiler's scope check and the worktree pin of every tracked file.
func BenchmarkDCRV1451ManyScopesRederivedRead(b *testing.B) {
	ctx := context.Background()
	root, _ := largeReadFixture(b, b.TempDir(), manyScopesScale)
	data, err := ReadCorpusFile(root, "corpus.json")
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		manyScopesRead(b, ctx, root, data, ReadOptions{Rederive: true})
	}
}
