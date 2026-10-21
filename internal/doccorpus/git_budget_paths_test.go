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

// unboundedPerPathReads turns the batches off and lifts the operation bound,
// so a test can produce the per-path reads' output to compare byte for byte.
func unboundedPerPathReads(t *testing.T) {
	t.Helper()
	pathPrefetchOff = true
	t.Cleanup(func() { pathPrefetchOff = false })
	newGitBudget = func() *gitrun.Budget { return gitrun.NewBudget(1<<20, gitrun.DefaultTotalBudget) }
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

func bulkSymbolSource(i int) (string, string) {
	return fmt.Sprintf("src/m%d/f%d.ts", i/100, i), fmt.Sprintf("export function sym%d(): number {\n  return %d;\n}\n", i, i)
}

// TestDCPV1216SymbolAnchorsGitOpsIndependentOfAnchorCount pins DCP-V1-216 for
// symbol anchors (V1-1991): 5,000 cited anchors whose files all moved resolve
// within the phase bound, and their checks equal the per-path reads'.
func TestDCPV1216SymbolAnchorsGitOpsIndependentOfAnchorCount(t *testing.T) {
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
	run := func(t *testing.T) []byte {
		t.Helper()
		receipt := Receipt{Citations: citations}
		checker := &symbolChecker{overlay: &symbolOverlay{root: root, repository: id, live: live, files: &sharedSymbolFiles{files: map[string]sharedSymbolFile{}}}, checks: map[string]SymbolAnchorCheck{}}
		defer checker.close()
		if err := attachSymbolAnchors(ctx, &receipt, checker); err != nil {
			t.Fatal(err)
		}
		if len(receipt.Limitations) != 0 || len(receipt.SymbolAnchors) != n {
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
	boundCorpusGitOps(t, phaseGitOps)
	batched := run(t)
	unboundedPerPathReads(t)
	if !bytes.Equal(batched, run(t)) {
		t.Fatal("batched symbol checks differ from the per-path reads")
	}
}

// TestDCPV1216CEMProjectionGitOpsIndependentOfAnchorCount pins DCP-V1-216 for
// the CEM claim projection: 5,000 anchors at the base project within the phase
// bound with the per-path reads' rows and omission count.
func TestDCPV1216CEMProjectionGitOpsIndependentOfAnchorCount(t *testing.T) {
	const n = 5000
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
	run := func(t *testing.T) []byte {
		t.Helper()
		rows, omitted, err := projectClaims(context.Background(), root, a, "", base, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != MaxResults || omitted != n-MaxResults {
			t.Fatalf("rows=%d omitted=%d", len(rows), omitted)
		}
		raw, err := json.Marshal(rows)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	boundCorpusGitOps(t, phaseGitOps)
	batched := run(t)
	unboundedPerPathReads(t)
	if !bytes.Equal(batched, run(t)) {
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
	n := MaxRecords
	root := t.TempDir()
	files := map[string]string{}
	for i := 0; i < n; i++ {
		data := fmt.Sprintf("// helper %d\n", i)
		if i%16 == 0 {
			data = fmt.Sprintf("describe('suite %d', () => {\n  it('case %d', () => {\n    expect(%d).toBe(%d);\n  });\n});\n", i, i, i, i)
		}
		files[fmt.Sprintf("e2e/flat/f%d.spec.js", i)] = data
	}
	rev := writeBulkFixture(t, root, files)
	run := func(t *testing.T) []byte {
		t.Helper()
		inventory, raw, err := legacyInventory(t, root, "jasmine", rev, "e2e", nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(inventory.Legacy) != n/16 {
			t.Fatalf("cases=%d unknown=%d", len(inventory.Legacy), len(inventory.Unknown))
		}
		return raw
	}
	boundCorpusGitOps(t, phaseGitOps)
	batched := run(t)
	unboundedPerPathReads(t)
	if !bytes.Equal(batched, run(t)) {
		t.Fatal("batched legacy inventory differs from the per-path reads")
	}
}

// TestDCPV1216PlaywrightDiscoveryGitOpsIndependentOfPathCount pins DCP-V1-216
// for Playwright discovery: 2,000 more listed test files in one directory,
// each run by both projects,
// (DCP-V1-217) discover within the phase bound with the per-path reads' bytes.
func TestDCPV1216PlaywrightDiscoveryGitOpsIndependentOfPathCount(t *testing.T) {
	const n = 2000
	repo := newPlaywrightBehaviorRepo(t)
	var listing map[string]any
	if err := json.Unmarshal(repo.listing, &listing); err != nil {
		t.Fatal(err)
	}
	suites := listing["suites"].([]any)
	for i := 0; i < n; i++ {
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
	run := func(t *testing.T) []byte {
		t.Helper()
		out, err := BuildPlaywrightDiscovery(context.Background(), PlaywrightDiscoveryInput{Root: repo.root, Migration: repo.migration, ConfigPath: playwrightBehaviorConfig, Listing: repo.listing, Receipt: repo.receipt})
		if err != nil {
			t.Fatal(err)
		}
		var discovery BehaviorDiscovery
		if err := decode(out, &discovery); err != nil || len(discovery.Executions) < 2*n {
			t.Fatalf("executions=%d err=%v", len(discovery.Executions), err)
		}
		return out
	}
	boundCorpusGitOps(t, phaseGitOps)
	batched := run(t)
	unboundedPerPathReads(t)
	if !bytes.Equal(batched, run(t)) {
		t.Fatal("batched Playwright discovery differs from the per-path reads")
	}
}

// symbolAnchorReceipt checks cited symbol anchors once under the current
// budget and returns the receipt's checks and limitations as JSON.
func symbolAnchorReceipt(t *testing.T, root, id string, live *contextindex.Index, citations []Anchor) []byte {
	t.Helper()
	receipt := Receipt{Citations: citations}
	checker := &symbolChecker{overlay: &symbolOverlay{root: root, repository: id, live: live, files: &sharedSymbolFiles{files: map[string]sharedSymbolFile{}}}, checks: map[string]SymbolAnchorCheck{}}
	defer checker.close()
	if err := attachSymbolAnchors(context.Background(), &receipt, checker); err != nil {
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
	batched := symbolAnchorReceipt(t, root, id, live, citations)
	if !strings.Contains(string(batched), string(cemcode.GitBudgetExceeded)) || strings.Contains(string(batched), `"moved"`) {
		t.Fatalf("batched fallback did not withhold on the budget: %.400s", batched)
	}
	pathPrefetchOff = true
	t.Cleanup(func() { pathPrefetchOff = false })
	if perPath := symbolAnchorReceipt(t, root, id, live, citations); !bytes.Equal(batched, perPath) {
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
	pathPrefetchOff = true
	t.Cleanup(func() { pathPrefetchOff = false })
	_, _, perPath := projectClaims(context.Background(), root, a, "", base, nil)
	if perPath == nil || perPath.Error() != batched.Error() {
		t.Fatalf("batched %v, per-path %v", batched, perPath)
	}
}

// TestDCPV1243ResolvedInputsKeepPerPathTreeBound pins DCP-V1-243 (V1-2151):
// the query freshness and batch input walks refuse a path whose parent tree
// alone passes the 4 MiB per-path lookup bound with the per-path lookup's
// own refusal, byte for byte, although the walk's 32 MiB bound admits it.
func TestDCPV1243ResolvedInputsKeepPerPathTreeBound(t *testing.T) {
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
