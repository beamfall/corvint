package doccorpus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
)

// unboundedPerPathReads returns ctx with the batches off and the operation
// bound lifted, so a test can produce the per-path reads' output to compare
// byte for byte.
func unboundedPerPathReads(ctx context.Context) context.Context {
	return perPathReads(boundCorpusGitOps(ctx, 1<<20))
}

// perPathReads returns ctx with every path batch turned into a no-op.
func perPathReads(ctx context.Context) context.Context {
	hooks := corpusGitHooksOf(ctx)
	hooks.prefetchOff = true
	return context.WithValue(ctx, corpusGitHooksKey{}, hooks)
}

// treeBlobs maps every path at rev to its blob in one ls-tree.
func treeBlobs(t *testing.T, root, rev string) map[string]string {
	t.Helper()
	blobs := map[string]string{}
	for _, line := range strings.Split(git(t, root, "ls-tree", "-r", rev), "\n") {
		meta, path, _ := strings.Cut(line, "\t")
		blobs[path] = strings.Fields(meta)[2]
	}
	return blobs
}

// strideSample returns every k-th element of all, from the first.
func strideSample[T any](all []T, k int) []T {
	out := make([]T, 0, (len(all)+k-1)/k)
	for i := 0; i < len(all); i += k {
		out = append(out, all[i])
	}
	return out
}

func bulkSymbolSource(i int) (string, string) {
	return fmt.Sprintf("src/m%d/f%d.ts", i/100, i), fmt.Sprintf("export function sym%d(): number {\n  return %d;\n}\n", i, i)
}

// TestDCPV1216SymbolAnchorsGitOpsIndependentOfAnchorCount pins DCP-V1-216 for
// symbol anchors (V1-1991): 5,000 cited anchors whose files all moved resolve
// within the phase bound, and their checks equal the per-path reads'.
func TestDCPV1216SymbolAnchorsGitOpsIndependentOfAnchorCount(t *testing.T) {
	t.Parallel()
	const n = 5000
	ctx := context.Background()
	root := t.TempDir()
	files := map[string]string{}
	for i := 0; i < n; i++ {
		path, data := bulkSymbolSource(i)
		files[path] = data
	}
	rev := writeBulkFixture(t, root, files)
	id, err := contextindex.CorpusRepositoryID(ctx, root, rev)
	if err != nil {
		t.Fatal(err)
	}
	blobs := treeBlobs(t, root, rev)
	citations := make([]Anchor, 0, n)
	moved := map[string]string{}
	for i := 0; i < n; i++ {
		path, data := bulkSymbolSource(i)
		citations = append(citations, Anchor{Repository: id, Revision: rev, Path: path, Blob: blobs[path], SHA256: Digest([]byte(data)), Start: 1, End: 3, SpanSHA256: spanDigest([]byte(data), 1, 3), Symbol: fmt.Sprintf("sym%d", i)})
		moved[path] = "const padding = 1;\n" + data
	}
	live, err := contextindex.BuildRevisionContext(ctx, root, commitFiles(t, root, moved))
	if err != nil {
		t.Fatal(err)
	}
	run := func(t *testing.T, ctx context.Context, citations []Anchor) []byte {
		t.Helper()
		receipt := Receipt{Citations: citations}
		checker := &symbolChecker{overlay: indexedSymbolOverlay(root, id, live), checks: map[string]SymbolAnchorCheck{}}
		defer checker.close()
		if err := attachSymbolAnchors(ctx, &receipt, checker); err != nil {
			t.Fatal(err)
		}
		if len(receipt.Limitations) != 0 || len(receipt.SymbolAnchors) != len(citations) {
			t.Fatalf("checks=%d limitations=%v", len(receipt.SymbolAnchors), receipt.Limitations)
		}
		for _, check := range receipt.SymbolAnchors {
			if check.State != SymbolAnchorMoved || check.Live == nil || check.Live.Start != 2 {
				t.Fatalf("check %+v", check)
			}
		}
		raw, err := json.Marshal(receipt.SymbolAnchors)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	// The bound holds for all n anchors. The byte comparison runs on every
	// tenth anchor, one or more in each of the 50 directories: the batches
	// have no chunk size, so the sample takes the same code path, and the
	// per-path reference spawns Git per path, which was 80% of this test's
	// time (V1-2342).
	run(t, boundCorpusGitOps(ctx, phaseGitOps), citations)
	sample := strideSample(citations, 10)
	if !bytes.Equal(run(t, boundCorpusGitOps(ctx, phaseGitOps), sample), run(t, unboundedPerPathReads(ctx), sample)) {
		t.Fatal("batched symbol checks differ from the per-path reads")
	}
}

// TestDCPV1216CEMProjectionGitOpsIndependentOfAnchorCount pins DCP-V1-216 for
// the CEM claim projection: 5,000 anchors at the base project within the phase
// bound with the per-path reads' rows and omission count.
func TestDCPV1216CEMProjectionGitOpsIndependentOfAnchorCount(t *testing.T) {
	t.Parallel()
	const n = 5000
	ctx := context.Background()
	root := t.TempDir()
	files := map[string]string{}
	for i := 0; i < n; i++ {
		files[fmt.Sprintf("docs/d%d/p%d.md", i/100, i)] = fmt.Sprintf("# Page %d\n\nClaim %d holds.\n", i, i)
	}
	base := writeBulkFixture(t, root, files)
	blobs := treeBlobs(t, root, base)
	a := &Artifact{Schema: SchemaV2}
	for i := 0; i < n; i++ {
		path := fmt.Sprintf("docs/d%d/p%d.md", i/100, i)
		raw := []byte(files[path])
		a.Claims = append(a.Claims, Claim{ID: fmt.Sprintf("claim-%d", i), Evidence: Evidence{Derivation: "extracted", Trust: "documented", Anchors: []Anchor{{Path: path, Blob: blobs[path], Revision: base, SHA256: Digest(raw), Start: 3, End: 3, SpanSHA256: spanDigest(raw, 3, 3)}}}})
	}
	run := func(t *testing.T, ctx context.Context, a *Artifact) []byte {
		t.Helper()
		rows, omitted, err := projectClaims(ctx, root, a, "", base, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != MaxResults || omitted != len(a.Claims)-MaxResults {
			t.Fatalf("rows=%d omitted=%d", len(rows), omitted)
		}
		raw, err := json.Marshal(rows)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	// The bound holds for all n anchors; the byte comparison runs on every
	// tenth claim, still more than MaxResults so the omission count is
	// compared too (V1-2342, as for the symbol anchors above).
	run(t, boundCorpusGitOps(ctx, phaseGitOps), a)
	sample := &Artifact{Schema: SchemaV2, Claims: strideSample(a.Claims, 10)}
	if !bytes.Equal(run(t, boundCorpusGitOps(ctx, phaseGitOps), sample), run(t, unboundedPerPathReads(ctx), sample)) {
		t.Fatal("batched CEM projection differs from the per-path reads")
	}
}

// TestDCPV1216LegacyDiscoveryGitOpsIndependentOfPathCount pins DCP-V1-216 for
// legacy discovery at its record bound: 4,096 test files, one in 16 with a
// case so the inventory stays inside its output bound, inventory within the
// phase bound with the per-path reads' bytes. The files share one directory
// because the revision walk still costs one operation per directory
// (DCP-V1-217).
func TestDCPV1216LegacyDiscoveryGitOpsIndependentOfPathCount(t *testing.T) {
	t.Parallel()
	n := MaxRecords
	ctx := context.Background()
	// fixture writes every k-th of the n files, keeping each file's index, so
	// the sample holds the same contents in the same single directory.
	fixture := func(k int) (root, rev string, cases int) {
		root = t.TempDir()
		files := map[string]string{}
		for i := 0; i < n; i += k {
			data := fmt.Sprintf("// helper %d\n", i)
			if i%16 == 0 {
				data = fmt.Sprintf("describe('suite %d', () => {\n  it('case %d', () => {\n    expect(%d).toBe(%d);\n  });\n});\n", i, i, i, i)
				cases++
			}
			files[fmt.Sprintf("e2e/flat/f%d.spec.js", i)] = data
		}
		return root, writeBulkFixture(t, root, files), cases
	}
	run := func(t *testing.T, ctx context.Context, root, rev string, cases int) []byte {
		t.Helper()
		inventory, raw, err := legacyInventoryContext(ctx, t, root, "jasmine", rev, "e2e", nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(inventory.Legacy) != cases {
			t.Fatalf("cases=%d unknown=%d, want %d cases", len(inventory.Legacy), len(inventory.Unknown), cases)
		}
		return raw
	}
	// The bound holds for all n files. The byte comparison runs on every tenth
	// file: the batches have no chunk size, so the sample takes the same code
	// path, and the per-path reference spawns Git per file, which was most of
	// this test's time (V1-2342).
	root, rev, cases := fixture(1)
	if cases != n/16 {
		t.Fatalf("fixture cases=%d, want %d", cases, n/16)
	}
	run(t, boundCorpusGitOps(ctx, phaseGitOps), root, rev, cases)
	root, rev, cases = fixture(10)
	if !bytes.Equal(run(t, boundCorpusGitOps(ctx, phaseGitOps), root, rev, cases), run(t, unboundedPerPathReads(ctx), root, rev, cases)) {
		t.Fatal("batched legacy inventory differs from the per-path reads")
	}
}

// TestDCPV1216PlaywrightDiscoveryGitOpsIndependentOfPathCount pins DCP-V1-216
// for Playwright discovery: 2,000 more listed test files in one directory,
// each run by both projects,
// (DCP-V1-217) discover within the phase bound with the per-path reads' bytes.
func TestDCPV1216PlaywrightDiscoveryGitOpsIndependentOfPathCount(t *testing.T) {
	t.Parallel()
	const n = 2000
	ctx := context.Background()
	// fixture adds every k-th of the n generated files, keeping each file's
	// index, and returns the repository and the number added.
	fixture := func(k int) (playwrightBehaviorRepo, int) {
		repo := newPlaywrightBehaviorRepo(t)
		var listing map[string]any
		if err := json.Unmarshal(repo.listing, &listing); err != nil {
			t.Fatal(err)
		}
		suites := listing["suites"].([]any)
		added := 0
		for i := 0; i < n; i += k {
			added++
			file := fmt.Sprintf("gen/f%d.spec.ts", i)
			writePlaywrightBehaviorFile(t, repo.root, "e2e/"+file, []byte(fmt.Sprintf("import { test } from '@playwright/test';\n\ntest('extra %d', async () => {});\n", i)))
			specs := []any{}
			for _, project := range []string{"alpha", "beta"} {
				specs = append(specs, map[string]any{"title": fmt.Sprintf("extra %d", i), "ok": true, "tags": []any{}, "id": fmt.Sprintf("gen-%d-%s", i, project), "file": file, "line": 3, "column": 1,
					"tests": []any{map[string]any{"timeout": 30000, "annotations": []any{}, "expectedStatus": "passed", "projectId": project, "projectName": project, "results": []any{}, "status": "skipped"}}})
			}
			suites = append(suites, map[string]any{"title": file, "file": file, "column": 0, "line": 0, "suites": []any{}, "specs": specs})
		}
		listing["suites"] = suites
		raw, err := json.Marshal(listing)
		if err != nil {
			t.Fatal(err)
		}
		repo.listing = raw
		git(t, repo.root, "add", ".")
		git(t, repo.root, "commit", "-qm", "generated tests")
		repo.revision = git(t, repo.root, "rev-parse", "HEAD")
		repo.revisions.App.Revision, repo.revisions.E2E.Revision, repo.revisions.Docs.Revision = repo.revision, repo.revision, repo.revision
		repo.migration = behaviorAdapterRaw(t, BehaviorMigration{Revisions: repo.revisions, Schema: 2, ContractID: "items-contract", SourceRevision: repo.revision, DocumentationRevision: repo.revision})
		return repo, added
	}
	run := func(t *testing.T, ctx context.Context, repo playwrightBehaviorRepo, added int) []byte {
		t.Helper()
		out, err := BuildPlaywrightDiscovery(ctx, PlaywrightDiscoveryInput{Root: repo.root, Migration: repo.migration, ConfigPath: playwrightBehaviorConfig, Listing: repo.listing, Receipt: repo.receipt})
		if err != nil {
			t.Fatal(err)
		}
		var discovery BehaviorDiscovery
		if err := decode(out, &discovery); err != nil || len(discovery.Executions) < 2*added {
			t.Fatalf("executions=%d err=%v", len(discovery.Executions), err)
		}
		return out
	}
	// The bound holds for all n files; the byte comparison runs on every
	// tenth file, as for the legacy inventory above (V1-2342).
	repo, added := fixture(1)
	run(t, boundCorpusGitOps(ctx, phaseGitOps), repo, added)
	repo, added = fixture(10)
	if !bytes.Equal(run(t, boundCorpusGitOps(ctx, phaseGitOps), repo, added), run(t, unboundedPerPathReads(ctx), repo, added)) {
		t.Fatal("batched Playwright discovery differs from the per-path reads")
	}
}

// symbolAnchorReceipt checks cited symbol anchors once under the current
// budget and returns the receipt's checks and limitations as JSON.
func symbolAnchorReceipt(ctx context.Context, t *testing.T, root, id string, live *contextindex.Index, citations []Anchor) []byte {
	t.Helper()
	receipt := Receipt{Citations: citations}
	checker := &symbolChecker{overlay: indexedSymbolOverlay(root, id, live), checks: map[string]SymbolAnchorCheck{}}
	defer checker.close()
	if err := attachSymbolAnchors(ctx, &receipt, checker); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(struct {
		Checks      []SymbolAnchorCheck
		Limitations []string
	}{receipt.SymbolAnchors, receipt.Limitations})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestDCPV1216SymbolFallbackBudgetWithholdsStates pins DCP-V1-216's
// fallback: one missing pinned blob fails the header batch, the per-blob
// reads then exhaust the default operation budget, and that exhaustion
// withholds every symbol state with its budget diagnostic, as the per-path
// reads do, instead of reading as an unavailable blob.
func TestDCPV1216SymbolFallbackBudgetWithholdsStates(t *testing.T) {
	t.Parallel()
	const n = 600
	ctx := context.Background()
	root := t.TempDir()
	files := map[string]string{}
	for i := 0; i < n; i++ {
		path, data := bulkSymbolSource(i)
		files[path] = data
	}
	rev := writeBulkFixture(t, root, files)
	id, err := contextindex.CorpusRepositoryID(ctx, root, rev)
	if err != nil {
		t.Fatal(err)
	}
	blobs := treeBlobs(t, root, rev)
	moved := map[string]string{}
	citations := []Anchor{}
	for i := 0; i < n; i++ {
		path, data := bulkSymbolSource(i)
		citations = append(citations, Anchor{Repository: id, Revision: rev, Path: path, Blob: blobs[path], SHA256: Digest([]byte(data)), Start: 1, End: 3, SpanSHA256: spanDigest([]byte(data), 1, 3), Symbol: fmt.Sprintf("sym%d", i)})
		moved[path] = "const padding = 1;\n" + data
	}
	missing := citations[0]
	missing.Blob, missing.Symbol = strings.Repeat("e", len(missing.Blob)), "symMissing"
	citations = append([]Anchor{missing}, citations...)
	live, err := contextindex.BuildRevisionContext(ctx, root, commitFiles(t, root, moved))
	if err != nil {
		t.Fatal(err)
	}
	batched := symbolAnchorReceipt(ctx, t, root, id, live, citations)
	if !strings.Contains(string(batched), string(cemcode.GitBudgetExceeded)) || strings.Contains(string(batched), `"moved"`) {
		t.Fatalf("batched fallback did not withhold on the budget: %.400s", batched)
	}
	if perPath := symbolAnchorReceipt(perPathReads(ctx), t, root, id, live, citations); !bytes.Equal(batched, perPath) {
		t.Fatalf("batched fallback %.400s\nper-path %.400s", batched, perPath)
	}
}

// gitStdin runs one git plumbing command with stdin in root.
func gitStdin(t *testing.T, root, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Corpus test", "GIT_AUTHOR_EMAIL=corpus@example.invalid", "GIT_COMMITTER_NAME=Corpus test", "GIT_COMMITTER_EMAIL=corpus@example.invalid", "GIT_AUTHOR_DATE=2026-10-19T00:00:00Z", "GIT_COMMITTER_DATE=2026-10-19T00:00:00Z")
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %.300s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestDCPV1216ProjectionKeepsPerPathTreeBound pins DCP-V1-216's admission
// equivalence: an anchor whose parent directory tree alone passes the 4 MiB
// per-path lookup bound is refused by the batched projection exactly as by
// the per-path lookup, although the batched walk's 32 MiB bound admits it.
func TestDCPV1216ProjectionKeepsPerPathTreeBound(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	git(t, root, "init", "-q")
	doc := "# Page\n\nClaim holds.\n"
	docBlob := gitStdin(t, root, doc, "hash-object", "-w", "--stdin")
	empty := gitStdin(t, root, "", "hash-object", "-w", "--stdin")
	var big strings.Builder
	// Each entry is "100644 <240-digit name>\x00<binary OID>" in the tree body.
	for i := 0; i*(7+240+1+len(empty)/2) < gitauth.MaxTreeBytes+(256<<10); i++ {
		fmt.Fprintf(&big, "100644 blob %s\t%0240d\n", empty, i)
	}
	fmt.Fprintf(&big, "100644 blob %s\tpage.md\n", docBlob)
	bigTree := gitStdin(t, root, big.String(), "mktree")
	docsTree := gitStdin(t, root, "040000 tree "+bigTree+"\tbig\n", "mktree")
	rootTree := gitStdin(t, root, "040000 tree "+docsTree+"\tdocs\n", "mktree")
	base := gitStdin(t, root, "", "commit-tree", rootTree, "-m", "big tree")
	raw := []byte(doc)
	a := &Artifact{Schema: SchemaV2, Claims: []Claim{{ID: "claim-0", Evidence: Evidence{Derivation: "extracted", Trust: "documented", Anchors: []Anchor{{Path: "docs/big/page.md", Blob: docBlob, Revision: base, SHA256: Digest(raw), Start: 3, End: 3, SpanSHA256: spanDigest(raw, 3, 3)}}}}}}
	_, _, batched := projectClaims(context.Background(), root, a, "", base, nil)
	if cemcode.CodeOf(batched) != cemcode.GitOutputExceeded {
		t.Fatalf("batched projection = %v, want the per-path tree bound refusal", batched)
	}
	_, _, perPath := projectClaims(perPathReads(context.Background()), root, a, "", base, nil)
	if perPath == nil || perPath.Error() != batched.Error() {
		t.Fatalf("batched %v, per-path %v", batched, perPath)
	}
}

// TestDCPV1243ResolvedInputsKeepPerPathTreeBound pins DCP-V1-243 (V1-2151):
// the query freshness and batch input walks refuse a path whose parent tree
// alone passes the 4 MiB per-path lookup bound with the per-path lookup's
// own refusal, byte for byte, although the walk's 32 MiB bound admits it.
func TestDCPV1243ResolvedInputsKeepPerPathTreeBound(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	git(t, root, "init", "-q")
	docBlob := gitStdin(t, root, "# Page\n", "hash-object", "-w", "--stdin")
	empty := gitStdin(t, root, "", "hash-object", "-w", "--stdin")
	var big strings.Builder
	for i := 0; i*(7+240+1+len(empty)/2) < gitauth.MaxTreeBytes+(256<<10); i++ {
		fmt.Fprintf(&big, "100644 blob %s\t%0240d\n", empty, i)
	}
	fmt.Fprintf(&big, "100644 blob %s\tpage.md\n", docBlob)
	bigTree := gitStdin(t, root, big.String(), "mktree")
	smallTree := gitStdin(t, root, "100644 blob "+docBlob+"\tpage.md\n", "mktree")
	docsTree := gitStdin(t, root, "040000 tree "+bigTree+"\tbig\n040000 tree "+smallTree+"\tsmall\n", "mktree")
	rootTree := gitStdin(t, root, "040000 tree "+docsTree+"\tdocs\n", "mktree")
	base := gitStdin(t, root, "", "commit-tree", rootTree, "-m", "big tree")
	auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, _, perPath := auth.LookupTreeEntry(ctx, base, "docs/big/page.md")
	if cemcode.CodeOf(perPath) != cemcode.GitOutputExceeded {
		t.Fatalf("per-path lookup = %v, want the tree bound refusal", perPath)
	}
	if _, err := auth.ResolvePaths(ctx, base, []string{"docs/big/page.md"}); err != nil {
		t.Fatalf("fixture must pass the walk's aggregate bound: %v", err)
	}
	_, batched := resolvePathsPerPathBound(ctx, auth, base, []string{"docs/small/page.md", "docs/big/page.md"})
	if batched == nil || batched.Error() != perPath.Error() || cemcode.CodeOf(batched) != cemcode.CodeOf(perPath) {
		t.Fatalf("batched %v, per-path %v", batched, perPath)
	}
	index := &contextindex.Index{CommitRevision: base}
	_, staged := stageInputs(ctx, auth, map[string]*contextindex.Index{base: index}, map[string][]string{base: {"docs/big/page.md"}}, MaxBytes)
	if staged == nil || staged.Error() != perPath.Error() {
		t.Fatalf("staged inputs %v, per-path %v", staged, perPath)
	}
	entries, err := resolvePathsPerPathBound(ctx, auth, base, []string{"docs/small/page.md"})
	if err != nil || entries["docs/small/page.md"].OID != docBlob {
		t.Fatalf("small tree path: %v %v", entries, err)
	}
}

// TestDCPV1216GitHooksTravelWithTheContext pins V1-2286: the operation bound
// and the batch switch reach a phase through its context alone, so the
// equivalence tests above compare real per-path reads while running in
// parallel. 40 anchors project within the bound in batches; the same bound
// with the batches off fails with the budget error; a context without hooks
// keeps the default budget and the batches.
func TestDCPV1216GitHooksTravelWithTheContext(t *testing.T) {
	t.Parallel()
	const n = 40
	root := t.TempDir()
	files := map[string]string{}
	for i := 0; i < n; i++ {
		files[fmt.Sprintf("docs/d%d/p%d.md", i/10, i)] = fmt.Sprintf("# Page %d\n\nClaim %d holds.\n", i, i)
	}
	base := writeBulkFixture(t, root, files)
	blobs := treeBlobs(t, root, base)
	a := &Artifact{Schema: SchemaV2}
	for i := 0; i < n; i++ {
		path := fmt.Sprintf("docs/d%d/p%d.md", i/10, i)
		raw := []byte(files[path])
		a.Claims = append(a.Claims, Claim{ID: fmt.Sprintf("claim-%d", i), Evidence: Evidence{Derivation: "extracted", Trust: "documented", Anchors: []Anchor{{Path: path, Blob: blobs[path], Revision: base, SHA256: Digest(raw), Start: 3, End: 3, SpanSHA256: spanDigest(raw, 3, 3)}}}})
	}
	ctx := context.Background()
	bounded := boundCorpusGitOps(ctx, phaseGitOps)
	if _, _, err := projectClaims(bounded, root, a, "", base, nil); err != nil {
		t.Fatalf("batched projection within %d operations: %v", phaseGitOps, err)
	}
	if _, _, err := projectClaims(perPathReads(bounded), root, a, "", base, nil); cemcode.CodeOf(err) != cemcode.GitBudgetExceeded {
		t.Fatalf("per-path projection of %d anchors within %d operations = %v, want the budget error", n, phaseGitOps, err)
	}
	if hooks := corpusGitHooksOf(ctx); hooks.budget != nil || hooks.prefetchOff {
		t.Fatalf("derived contexts changed their parent: %+v", hooks)
	}
	if _, _, err := projectClaims(perPathReads(ctx), root, a, "", base, nil); err != nil {
		t.Fatalf("per-path projection under the default budget: %v", err)
	}
}

// indexedSymbolOverlay is an overlay over a prebuilt live index, as one whose
// re-resolution has already built it (DCR-V1-412).
func indexedSymbolOverlay(root, id string, live *contextindex.Index) *symbolOverlay {
	observed := contextindex.Observation{ObjectFormat: live.ObjectFormat, CommitRevision: live.CommitRevision, Revision: live.Revision, StatusSHA256: live.StatusSHA256, DirtyPaths: live.DirtyPaths}
	return &symbolOverlay{root: root, repository: id, live: &contextindex.TreeObservation{Observation: observed}, observation: ObservationKey(observed), index: live, files: &sharedSymbolFiles{files: map[string]sharedSymbolFile{}}}
}
