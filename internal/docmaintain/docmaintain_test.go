package docmaintain

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixtureRepo(t *testing.T, ownerDigest string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":      "module example.test/docmaintain\n\ngo 1.27.0\n",
		"owner.md":    ownerDigest,
		"widget/w.go": "package widget\n\n// Split does one thing.\nfunc Split(key string) string { return key }\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit(t, root, "fixture")
	return root
}

func commit(t *testing.T, root, message string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(err) {
		run(t, root, "init", "-q")
	}
	run(t, root, "add", "-A")
	run(t, root, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", message)
}

func run(t *testing.T, root string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
}

const digestV1 = "# Widget\n\n## Agent digest\n- Claim: splits keys.\n"
const digestV2 = "# Widget\n\n## Agent digest\n- Claim: splits keys in half.\n"

func pagePath(root string) string { return filepath.Join(root, "docs", "page.md") }

func writePage(t *testing.T, root, content string) {
	t.Helper()
	path := pagePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readPage(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(pagePath(root))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func alwaysPolicy() Policy {
	return Policy{Enabled: true, Apply: true, MaxWrites: 10, MaxWallClock: time.Minute}
}

// TestSessionAddsChangesAndRemoves drives three real revisions of the fixture
// page across a temp git repo: the generated block appears, then refreshes on
// a source change, and the human paragraph outside the markers survives all
// three, matching the IPR-05 acceptance criterion.
func TestSessionAddsChangesAndRemoves(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	writePage(t, root, "# Page\n\nHuman intro paragraph, never touched.\n\n"+insertionPoint+"\n")
	selector := []Selector{{Source: "owner.md", Package: "widget"}}

	// Revision 1: add.
	result, err := Run(context.Background(), root, "docs/page.md", selector, alwaysPolicy())
	if err != nil {
		t.Fatalf("revision 1: %v", err)
	}
	if !result.Receipt.Applied || result.Receipt.Blocks[0].Skipped != "" {
		t.Fatalf("revision 1 did not apply: %+v", result.Receipt)
	}
	if err := os.WriteFile(pagePath(root), result.ProposedPage, 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "page revision 1")
	after1 := readPage(t, root)
	if !strings.Contains(after1, "Human intro paragraph, never touched.") {
		t.Fatal("human paragraph lost after add")
	}
	if !strings.Contains(after1, "splits keys.") {
		t.Fatal("generated content missing after add")
	}

	// Revision 2: change the owner source; the block must refresh.
	if err := os.WriteFile(filepath.Join(root, "owner.md"), []byte(digestV2), 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "owner change")
	result2, err := Run(context.Background(), root, "docs/page.md", selector, alwaysPolicy())
	if err != nil {
		t.Fatalf("revision 2: %v", err)
	}
	if !result2.Receipt.Applied || !result2.Receipt.Blocks[0].Eligible {
		t.Fatalf("revision 2 did not refresh: %+v", result2.Receipt)
	}
	if err := os.WriteFile(pagePath(root), result2.ProposedPage, 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "page revision 2")
	after2 := readPage(t, root)
	if !strings.Contains(after2, "splits keys in half.") {
		t.Fatal("generated content did not refresh")
	}
	if strings.Contains(after2, "splits keys.\n") && !strings.Contains(after2, "splits keys in half.") {
		t.Fatal("stale content retained")
	}
	if !strings.Contains(after2, "Human intro paragraph, never touched.") {
		t.Fatal("human paragraph lost after change")
	}

	// Revision 3: re-running with no source change is a no-op (byte-identical block).
	result3, err := Run(context.Background(), root, "docs/page.md", selector, alwaysPolicy())
	if err != nil {
		t.Fatalf("revision 3: %v", err)
	}
	if result3.Receipt.Applied {
		t.Fatalf("revision 3 rewrote an unchanged block: %+v", result3.Receipt)
	}
	if result3.Receipt.Blocks[0].Skipped != "unchanged" {
		t.Fatalf("expected unchanged skip, got %+v", result3.Receipt.Blocks[0])
	}
}

func TestUnrelatedBlockStaysByteIdentical(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	if err := os.MkdirAll(filepath.Join(root, "widget2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "widget2", "w2.go"), []byte("package widget2\n\nfunc Other() string { return \"x\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "owner2.md"), []byte("# Other\n\n## Agent digest\n- Claim: other.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "second selector")

	selectors := []Selector{{Source: "owner.md", Package: "widget"}, {Source: "owner2.md", Package: "widget2"}}
	writePage(t, root, "# Page\n\n"+insertionPoint+"\n")
	result, err := Run(context.Background(), root, "docs/page.md", selectors, alwaysPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pagePath(root), result.ProposedPage, 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "page with two blocks")
	_, sha1Before, _, _, _ := findBlock(result.ProposedPage, selectors[0])

	// Change only the second source; the first selector's block must be byte-identical.
	if err := os.WriteFile(filepath.Join(root, "owner2.md"), []byte("# Other\n\n## Agent digest\n- Claim: other, changed.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "second source change")
	result2, err := Run(context.Background(), root, "docs/page.md", selectors, alwaysPolicy())
	if err != nil {
		t.Fatal(err)
	}
	body1, sha1After, _, _, found1 := findBlock(result2.ProposedPage, selectors[0])
	if !found1 || sha1After != sha1Before {
		t.Fatalf("unrelated block churned: before=%s after=%s", sha1Before, sha1After)
	}
	_ = body1
	if result2.Receipt.Blocks[0].Applied {
		t.Fatal("unrelated block was rewritten")
	}
	if !result2.Receipt.Blocks[1].Applied {
		t.Fatal("changed block was not refreshed")
	}
}

// TestCleanRegenerationEqualsIncremental checks that, at the same repository
// state, regenerating a selector's block from a page that never had one
// (clean) produces the identical block an incremental refresh of an already
// -present, stale block does — the session's output does not depend on
// whether the block previously existed.
func TestCleanRegenerationEqualsIncremental(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	selector := []Selector{{Source: "owner.md", Package: "widget"}}

	// Seed one page with an already-generated (now stale) block, as if an
	// earlier session had run against an older revision.
	staleBlock := renderBlock(selector[0], "0000000000000000000000000000000000000000", "0000000000000000000000000000000000000000", "stale-digest-does-not-match", []byte("stale body"))
	writePage(t, root, "# Page\n\nHuman intro.\n\n"+insertionPoint+"\n"+string(staleBlock))

	// A second, block-free page at the identical repository state.
	writePage2 := filepath.Join(root, "docs", "clean.md")
	if err := os.WriteFile(writePage2, []byte("# Page\n\n"+insertionPoint+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	incremental, err := Run(context.Background(), root, "docs/page.md", selector, alwaysPolicy())
	if err != nil {
		t.Fatal(err)
	}
	clean, err := Run(context.Background(), root, "docs/clean.md", selector, alwaysPolicy())
	if err != nil {
		t.Fatal(err)
	}
	incrementalBody, _, _, _, found1 := findBlock(incremental.ProposedPage, selector[0])
	cleanBody, _, _, _, found2 := findBlock(clean.ProposedPage, selector[0])
	if !found1 || !found2 {
		t.Fatal("expected both pages to have a generated block")
	}
	if string(incrementalBody) != string(cleanBody) {
		t.Fatalf("clean regeneration diverged from incremental refresh:\nclean:\n%s\nincremental:\n%s", cleanBody, incrementalBody)
	}
}

// TestApplyDetectsConcurrentEditAndRefusesWithNoWrite exercises the actual
// conflict boundary: Preview reads the page, something else edits it before
// Apply runs, and Apply must refuse without writing anything.
func TestApplyDetectsConcurrentEditAndRefusesWithNoWrite(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	writePage(t, root, "# Page\n\nHuman intro.\n\n"+insertionPoint+"\n")
	selector := []Selector{{Source: "owner.md", Package: "widget"}}
	policy := alwaysPolicy()

	preview, err := Preview(context.Background(), root, "docs/page.md", selector, policy)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Receipt.Blocks[0].Eligible {
		t.Fatal("expected an eligible block to preview")
	}

	// A concurrent edit lands after Preview read the page.
	concurrentEdit := "# Page\n\nHuman intro, edited concurrently.\n\n" + insertionPoint + "\n"
	writePage(t, root, concurrentEdit)

	applied, err := Apply(root, preview, policy)
	if err == nil {
		t.Fatal("expected a conflict refusal")
	}
	refusal, ok := err.(*Refusal)
	if !ok || refusal.Code != "maintenance-conflict" {
		t.Fatalf("unexpected error: %v", err)
	}
	if !applied.Receipt.Conflict || applied.Receipt.Applied {
		t.Fatalf("unexpected receipt: %+v", applied.Receipt)
	}
	onDisk := readPage(t, root)
	if onDisk != concurrentEdit {
		t.Fatal("Apply wrote despite a detected conflict")
	}
}

func TestUnauthorizedApplyRefuses(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	writePage(t, root, "# Page\n\n"+insertionPoint+"\n")
	selector := []Selector{{Source: "owner.md", Package: "widget"}}
	policy := Policy{Enabled: true, Apply: false, MaxWrites: 10, MaxWallClock: time.Minute}

	result, err := Run(context.Background(), root, "docs/page.md", selector, policy)
	if err != nil {
		t.Fatal(err)
	}
	if result.Receipt.Applied {
		t.Fatal("preview-only session applied a write")
	}
	if !result.Receipt.PreviewOnly {
		t.Fatal("expected PreviewOnly")
	}
	onDisk := readPage(t, root)
	if strings.Contains(onDisk, "corvint:docmaintain begin") {
		t.Fatal("preview-only session wrote to disk")
	}
}

func TestDisabledPolicyRefuses(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	writePage(t, root, "# Page\n\n"+insertionPoint+"\n")
	selector := []Selector{{Source: "owner.md", Package: "widget"}}
	_, err := Run(context.Background(), root, "docs/page.md", selector, Policy{Apply: true})
	if err == nil {
		t.Fatal("expected refusal for a disabled session")
	}
	refusal, ok := err.(*Refusal)
	if !ok || refusal.Code != "maintenance-session-disabled" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestSymlinkPageIsRefused ensures a page path that is a symlink out of the
// repository is never read on Preview nor replaced on Apply.
func TestSymlinkPageIsRefused(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	outside := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(outside, []byte("outside content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, pagePath(root)); err != nil {
		t.Fatal(err)
	}
	selector := []Selector{{Source: "owner.md", Package: "widget"}}
	policy := alwaysPolicy()

	if _, err := Preview(context.Background(), root, "docs/page.md", selector, policy); err == nil {
		t.Fatal("expected preview to refuse a symlinked page")
	} else if refusal, ok := err.(*Refusal); !ok || refusal.Code != "invalid-page-path" {
		t.Fatalf("unexpected preview error: %v", err)
	}

	fakePreview := &Result{Receipt: Receipt{Page: "docs/page.md", PageExistedAtStart: true}}
	if _, err := Apply(root, fakePreview, policy); err == nil {
		t.Fatal("expected apply to refuse a symlinked page")
	} else if refusal, ok := err.(*Refusal); !ok || refusal.Code != "invalid-page-path" {
		t.Fatalf("unexpected apply error: %v", err)
	}

	target, err := os.Readlink(pagePath(root))
	if err != nil || target != outside {
		t.Fatal("symlink was touched")
	}
	if content, err := os.ReadFile(outside); err != nil || string(content) != "outside content" {
		t.Fatal("outside file was touched")
	}
}

// TestApplyPreservesPageMode ensures the atomic replace keeps an existing
// page's permission bits instead of the temp file's owner-only mode.
func TestApplyPreservesPageMode(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	writePage(t, root, "# Page\n\n"+insertionPoint+"\n")
	if err := os.Chmod(pagePath(root), 0o644); err != nil {
		t.Fatal(err)
	}
	selector := []Selector{{Source: "owner.md", Package: "widget"}}
	result, err := Run(context.Background(), root, "docs/page.md", selector, alwaysPolicy())
	if err != nil || !result.Receipt.Applied {
		t.Fatalf("apply result=%+v err=%v", result, err)
	}
	info, err := os.Stat(pagePath(root))
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("page mode after apply = %v, %v", info.Mode().Perm(), err)
	}
}

// TestNonRepositoryRootIsRefused ensures a root with no Git work tree yields
// no draft rather than an unevidenced block.
func TestNonRepositoryRootIsRefused(t *testing.T) {
	root := t.TempDir()
	writePage(t, root, "# Page\n\n"+insertionPoint+"\n")
	selector := []Selector{{Source: "owner.md", Package: "widget"}}
	_, err := Preview(context.Background(), root, "docs/page.md", selector, alwaysPolicy())
	if refusal, ok := err.(*Refusal); !ok || refusal.Code != "repository-unavailable" {
		t.Fatalf("expected a repository-unavailable refusal, got %v", err)
	}
}

// TestGitMetadataPageIsRefused ensures no page path can name Git metadata,
// in any letter case, so Apply never writes inside .git.
func TestGitMetadataPageIsRefused(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	selector := []Selector{{Source: "owner.md", Package: "widget"}}
	for _, page := range []string{".git/page.md", "docs/.GIT/page.md"} {
		if _, err := Preview(context.Background(), root, page, selector, alwaysPolicy()); err == nil {
			t.Fatalf("expected preview to refuse %s", page)
		} else if refusal, ok := err.(*Refusal); !ok || refusal.Code != "invalid-page-path" {
			t.Fatalf("unexpected preview error for %s: %v", page, err)
		}
		fakePreview := &Result{Receipt: Receipt{Page: page, Blocks: []BlockOutcome{{Applied: true}}}}
		if _, err := Apply(root, fakePreview, alwaysPolicy()); err == nil {
			t.Fatalf("expected apply to refuse %s", page)
		} else if refusal, ok := err.(*Refusal); !ok || refusal.Code != "invalid-page-path" {
			t.Fatalf("unexpected apply error for %s: %v", page, err)
		}
	}
}

// TestNonMarkdownPageIsRefused ensures the session maintains only a Markdown
// page (SDD-V0-007), so Apply never splices a generated block into a module
// file or Go source.
func TestNonMarkdownPageIsRefused(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	selector := []Selector{{Source: "owner.md", Package: "widget"}}
	for _, page := range []string{"go.mod", "widget/w.go"} {
		original, err := os.ReadFile(filepath.Join(root, page))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Run(context.Background(), root, page, selector, alwaysPolicy()); err == nil {
			t.Errorf("expected run to refuse %s", page)
		} else if refusal, ok := err.(*Refusal); !ok || refusal.Code != "invalid-page-path" {
			t.Errorf("unexpected run error for %s: %v", page, err)
		}
		fakePreview := &Result{Receipt: Receipt{Page: page, Blocks: []BlockOutcome{{Applied: true}}}}
		if _, err := Apply(root, fakePreview, alwaysPolicy()); err == nil {
			t.Errorf("expected apply to refuse %s", page)
		} else if refusal, ok := err.(*Refusal); !ok || refusal.Code != "invalid-page-path" {
			t.Errorf("unexpected apply error for %s: %v", page, err)
		}
		if after, _ := os.ReadFile(filepath.Join(root, page)); string(after) != string(original) {
			t.Errorf("%s was rewritten", page)
		}
	}
}

// TestMarkerInDraftContentIsRefused ensures a draft whose rendered body
// contains the literal end marker cannot truncate the block span.
func TestMarkerInDraftContentIsRefused(t *testing.T) {
	root := fixtureRepo(t, "# Widget\n\n## Agent digest\n"+endMarker+"\n")
	writePage(t, root, "# Page\n\nHuman intro, never touched.\n\n"+insertionPoint+"\n")
	selector := []Selector{{Source: "owner.md", Package: "widget"}}

	_, err := Run(context.Background(), root, "docs/page.md", selector, alwaysPolicy())
	if err == nil {
		t.Fatal("expected refusal for marker text in draft content")
	}
	refusal, ok := err.(*Refusal)
	if !ok || refusal.Code != "marker-in-content" {
		t.Fatalf("unexpected error: %v", err)
	}
	onDisk := readPage(t, root)
	if strings.Contains(onDisk, "corvint:docmaintain begin") {
		t.Fatal("refused session wrote a block")
	}
}

func TestSessionBoundsStopCleanly(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	if err := os.MkdirAll(filepath.Join(root, "widget2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "widget2", "w2.go"), []byte("package widget2\n\nfunc Other() string { return \"x\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "owner2.md"), []byte("# Other\n\n## Agent digest\n- Claim: other.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "second selector")
	writePage(t, root, "# Page\n\n"+insertionPoint+"\n")

	selectors := []Selector{{Source: "owner.md", Package: "widget"}, {Source: "owner2.md", Package: "widget2"}}
	policy := Policy{Enabled: true, Apply: true, MaxWrites: 1, MaxWallClock: time.Minute}
	result, err := Run(context.Background(), root, "docs/page.md", selectors, policy)
	if err != nil {
		t.Fatal(err)
	}
	if result.Receipt.StoppedReason != "max-writes" {
		t.Fatalf("expected max-writes stop, got %+v", result.Receipt)
	}
	if !result.Receipt.Blocks[0].Applied || result.Receipt.Blocks[1].Applied {
		t.Fatalf("expected only the first selector applied: %+v", result.Receipt.Blocks)
	}

	policy2 := Policy{Enabled: true, Apply: true, MaxWrites: 10, MaxWallClock: time.Nanosecond}
	result2, err := Run(context.Background(), root, "docs/page.md", selectors, policy2)
	if err != nil {
		t.Fatal(err)
	}
	if result2.Receipt.StoppedReason != "max-wall-clock" {
		t.Fatalf("expected max-wall-clock stop, got %+v", result2.Receipt)
	}
}

// TestRemovedDeclarationLeavesTheBlock closes the removal half of the IPR-05
// acceptance criterion ("add/change/remove a supported capability across real
// revisions"): TestSessionAddsChangesAndRemoves covers add and owner-prose
// change, but nothing exercised a capability leaving the selected package. A
// removed exported declaration must make the block eligible again (its cited
// source identities changed, SDD-V0-008), must stop being cited after the
// refresh, and must not disturb the surviving declaration or the human prose.
func TestRemovedDeclarationLeavesTheBlock(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	joinPath := filepath.Join(root, "widget", "j.go")
	if err := os.WriteFile(joinPath, []byte("package widget\n\n// Join does one other thing.\nfunc Join(key string) string { return key }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "add a second capability")

	selector := []Selector{{Source: "owner.md", Package: "widget"}}
	writePage(t, root, "# Page\n\nHuman intro paragraph, never touched.\n\n"+insertionPoint+"\n")

	withJoin, err := Run(context.Background(), root, "docs/page.md", selector, alwaysPolicy())
	if err != nil {
		t.Fatalf("seed session: %v", err)
	}
	if !withJoin.Receipt.Applied {
		t.Fatalf("seed session did not apply: %+v", withJoin.Receipt)
	}
	seeded := readPage(t, root)
	if !strings.Contains(seeded, "func Join(key string) string") {
		t.Fatal("seeded block does not cite the capability that is about to be removed")
	}
	commit(t, root, "page cites both capabilities")

	// Remove the capability in a real revision.
	if err := os.Remove(joinPath); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "remove the second capability")

	afterRemoval, err := Run(context.Background(), root, "docs/page.md", selector, alwaysPolicy())
	if err != nil {
		t.Fatalf("removal session: %v", err)
	}
	if !afterRemoval.Receipt.Blocks[0].Eligible {
		t.Fatalf("removal did not make the block eligible: %+v", afterRemoval.Receipt.Blocks[0])
	}
	if !afterRemoval.Receipt.Applied {
		t.Fatalf("removal session did not refresh the block: %+v", afterRemoval.Receipt)
	}
	if afterRemoval.Receipt.Blocks[0].OldSHA256 == afterRemoval.Receipt.Blocks[0].NewSHA256 {
		t.Fatal("source digest did not change when a cited declaration was removed")
	}

	refreshed := readPage(t, root)
	if strings.Contains(refreshed, "func Join(key string) string") {
		t.Fatal("removed capability is still cited after the refresh")
	}
	if !strings.Contains(refreshed, "func Split(key string) string") {
		t.Fatal("surviving capability was dropped by the refresh")
	}
	if !strings.Contains(refreshed, "Human intro paragraph, never touched.") {
		t.Fatal("human paragraph lost after removal")
	}

	// The refreshed page is now current: a further session is a no-op.
	settled, err := Run(context.Background(), root, "docs/page.md", selector, alwaysPolicy())
	if err != nil {
		t.Fatalf("settled session: %v", err)
	}
	if settled.Receipt.Applied || settled.Receipt.Blocks[0].Skipped != "unchanged" {
		t.Fatalf("post-removal session was not a no-op: %+v", settled.Receipt)
	}
}

// TestSelectorCommentBreakoutRefused guards renderBlock's begin marker: %q
// leaves a literal "-->" in selector.Package untouched, which would close
// the marker's own HTML comment early and turn the rest of the page into
// live content. Preview must refuse the selector rather than emit that page.
func TestSelectorCommentBreakoutRefused(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	writePage(t, root, "# Page\n\n"+insertionPoint+"\n")
	for _, pkg := range []string{"widget-->x", `widget"x`} {
		selector := []Selector{{Source: "owner.md", Package: pkg}}
		_, err := Preview(context.Background(), root, "docs/page.md", selector, alwaysPolicy())
		refusal, ok := err.(*Refusal)
		if !ok || refusal.Code != "invalid-selector" {
			t.Fatalf("package %q: expected an invalid-selector refusal, got %v", pkg, err)
		}
	}
}

// TestSelectorThatQuotingChangesIsRefused guards findBlock: the marker records
// the %q-escaped selector while findBlock compares the raw one, so a real
// package directory whose name %q escapes would never be re-found and its
// block would be inserted again on every session.
func TestSelectorThatQuotingChangesIsRefused(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	for _, pkg := range []string{`wid\get`, "wid\u200bget"} {
		directory := filepath.Join(root, pkg)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "w.go"), []byte("package widget\n\n// Split does one thing.\nfunc Split(key string) string { return key }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit(t, root, "escaped package names")
	writePage(t, root, "# Page\n\n"+insertionPoint+"\n")
	for _, pkg := range []string{`wid\get`, "wid\u200bget"} {
		selector := []Selector{{Source: "owner.md", Package: pkg}}
		_, err := Preview(context.Background(), root, "docs/page.md", selector, alwaysPolicy())
		if refusal, ok := err.(*Refusal); !ok || refusal.Code != "invalid-selector" {
			t.Errorf("package %q: expected an invalid-selector refusal, got %v", pkg, err)
		}
	}
}

// TestApplyRefusesWithoutApplyAuthority calls Apply directly: Run skips Apply
// when policy.Apply is false, so only a direct call reaches Apply's own
// authorization check.
func TestApplyRefusesWithoutApplyAuthority(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	page := "# Page\n\n" + insertionPoint + "\n"
	writePage(t, root, page)
	selector := []Selector{{Source: "owner.md", Package: "widget"}}
	preview, err := Preview(context.Background(), root, "docs/page.md", selector, alwaysPolicy())
	if err != nil || !preview.Receipt.Blocks[0].Applied {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	_, err = Apply(root, preview, Policy{Enabled: true, Apply: false, MaxWrites: 10, MaxWallClock: time.Minute})
	if refusal, ok := err.(*Refusal); !ok || refusal.Code != "apply-not-authorized" {
		t.Fatalf("expected apply-not-authorized, got %v", err)
	}
	if readPage(t, root) != page {
		t.Fatal("unauthorized Apply wrote the page")
	}
}

// TestParentTraversalPageIsRefused ensures a page path that leaves the
// repository through ".." is refused by Preview and Apply, and nothing is
// written outside the root.
func TestParentTraversalPageIsRefused(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	selector := []Selector{{Source: "owner.md", Package: "widget"}}
	page := "../escape.md"
	if _, err := Preview(context.Background(), root, page, selector, alwaysPolicy()); err == nil {
		t.Fatal("expected preview to refuse a parent-traversal page")
	} else if refusal, ok := err.(*Refusal); !ok || refusal.Code != "invalid-page-path" {
		t.Fatalf("unexpected preview error: %v", err)
	}
	fakePreview := &Result{ProposedPage: []byte("escaped\n"), Receipt: Receipt{Page: page, Blocks: []BlockOutcome{{Applied: true}}}}
	if _, err := Apply(root, fakePreview, alwaysPolicy()); err == nil {
		t.Fatal("expected apply to refuse a parent-traversal page")
	} else if refusal, ok := err.(*Refusal); !ok || refusal.Code != "invalid-page-path" {
		t.Fatalf("unexpected apply error: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(filepath.Dir(root), "escape.md")); !os.IsNotExist(err) {
		t.Fatalf("a file was written outside the repository: %v", err)
	}
}

// TestSameSourceDifferentPackageKeepsSeparateBlocks ensures a block is
// identified by both its source and its package: two selectors sharing one
// owner source must produce two blocks, not one block overwritten twice.
func TestSameSourceDifferentPackageKeepsSeparateBlocks(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	if err := os.MkdirAll(filepath.Join(root, "widget2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "widget2", "w2.go"), []byte("package widget2\n\nfunc Other() string { return \"x\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "second package")
	writePage(t, root, "# Page\n\n"+insertionPoint+"\n")
	selectors := []Selector{{Source: "owner.md", Package: "widget"}, {Source: "owner.md", Package: "widget2"}}
	result, err := Run(context.Background(), root, "docs/page.md", selectors, alwaysPolicy())
	if err != nil {
		t.Fatal(err)
	}
	onDisk := readPage(t, root)
	if strings.Count(onDisk, beginMarkerPrefix) != 2 ||
		!strings.Contains(onDisk, `source="owner.md" package="widget" `) ||
		!strings.Contains(onDisk, `source="owner.md" package="widget2" `) {
		t.Fatalf("expected one block per package, receipt=%+v page:\n%s", result.Receipt, onDisk)
	}
}

// TestMarkerInExistingBlockBodyIsRefused ensures a page whose managed block
// body already carries marker text is refused before any splice, and the page
// is left unchanged.
func TestMarkerInExistingBlockBodyIsRefused(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	page := "# Page\n\n" + insertionPoint + "\n" +
		`<!-- corvint:docmaintain begin source="owner.md" package="widget" commit="x|y" source_digest="00" -->` + "\n" +
		"stale body quoting " + beginMarkerPrefix + "text\n" + endMarker + "\n"
	writePage(t, root, page)
	selector := []Selector{{Source: "owner.md", Package: "widget"}}
	_, err := Run(context.Background(), root, "docs/page.md", selector, alwaysPolicy())
	if refusal, ok := err.(*Refusal); !ok || refusal.Code != "marker-in-content" {
		t.Fatalf("expected marker-in-content, got %v", err)
	}
	if readPage(t, root) != page {
		t.Fatal("refused session changed the page")
	}
}

// TestApplyRefusesWhenPageExistenceChanged ensures the conflict boundary
// covers page existence, not only its digest: a page absent at Preview and
// created empty before Apply has the same digest but is still a conflict.
func TestApplyRefusesWhenPageExistenceChanged(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	selector := []Selector{{Source: "owner.md", Package: "widget"}}
	preview, err := Preview(context.Background(), root, "docs/page.md", selector, alwaysPolicy())
	if err != nil || preview.Receipt.PageExistedAtStart || !preview.Receipt.Blocks[0].Applied {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	writePage(t, root, "")
	_, err = Apply(root, preview, alwaysPolicy())
	if refusal, ok := err.(*Refusal); !ok || refusal.Code != "maintenance-conflict" {
		t.Fatalf("expected maintenance-conflict, got %v", err)
	}
	if readPage(t, root) != "" {
		t.Fatal("Apply wrote a page created after the preview")
	}
}

// TestZeroMaxWritesDefaultsToSelectorCount ensures an unset MaxWrites bound
// admits one write per selector instead of stopping every block.
func TestZeroMaxWritesDefaultsToSelectorCount(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	writePage(t, root, "# Page\n\n"+insertionPoint+"\n")
	selector := []Selector{{Source: "owner.md", Package: "widget"}}
	policy := Policy{Enabled: true, MaxWallClock: time.Minute}
	preview, err := Preview(context.Background(), root, "docs/page.md", selector, policy)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Receipt.StoppedReason != "" || !preview.Receipt.Blocks[0].Applied {
		t.Fatalf("MaxWrites 0 must default to the selector count: %+v", preview.Receipt)
	}
}
