package gitauth

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "HOME="+t.TempDir(), "XDG_CONFIG_HOME="+t.TempDir(),
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const bodyV1 = "package p\n\nfunc Alpha() {\n\ta := 1\n\tb := 2\n\tc := 3\n\td := 4\n\te := 5\n\tf := 6\n\tg := 7\n\th := 8\n}\n"
const bodyV2 = "package p\n\nfunc Alpha() {\n\ta := 1\n\tb := 2\n\tc := 3\n\td := 4\n\te := 5\n\tf := 6\n\tg := 7\n\th := 9\n}\n"

// makeRepo builds a two-commit repository whose tree carries a hostile
// .gitattributes diff-driver mapping.
func makeRepo(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	gitCmd(t, root, "config", "user.name", "t")
	gitCmd(t, root, "config", "user.email", "t@example.invalid")
	writeFile(t, root, ".gitattributes", "*.go diff=evil\n")
	writeFile(t, root, "f.go", bodyV1)
	writeFile(t, root, "docs/rule.txt", "the rule\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "f.go", bodyV2)
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "change")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	return root, base, target
}

func open(t *testing.T, root string) *Repository {
	t.Helper()
	repository, err := Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	return repository
}

func TestPrimaryOpenAndResolve(t *testing.T) {
	root, base, _ := makeRepo(t)
	repository := open(t, root)
	resolved, err := repository.Resolve(context.Background(), base)
	if err != nil || resolved != base {
		t.Fatalf("resolved %q err %v, want %q", resolved, err, base)
	}
	if _, err := repository.Resolve(context.Background(), "--upload-pack=/bin/false"); cemcode.CodeOf(err) != cemcode.InvalidArguments {
		t.Fatalf("dash-prefixed revision: %v", err)
	}
}

// TestResolveIgnoresRepositoryGrafts checks that an `info/grafts` entry
// cannot rewrite a commit's parent: graft files are not replace objects, so
// GIT_NO_REPLACE_OBJECTS alone would bind a forged ancestor.
func TestResolveIgnoresRepositoryGrafts(t *testing.T) {
	root, base, target := makeRepo(t)
	writeFile(t, root, "f.go", bodyV1)
	gitCmd(t, root, "commit", "-qam", "revert")
	head := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, ".git/info/grafts", head+" "+base+"\n")
	resolved, err := open(t, root).Resolve(context.Background(), "HEAD^")
	if err != nil || resolved != target {
		t.Fatalf("resolved %q err %v, want true parent %q", resolved, err, target)
	}
}

// TestRevisionOperandsAreNeverOptions checks that an option-shaped revision
// reaching `diff` or `ls-tree` is refused as a revision: without
// `--end-of-options`, `diff` honors `--output=` and writes a file.
func TestRevisionOperandsAreNeverOptions(t *testing.T) {
	root, _, target := makeRepo(t)
	repository := open(t, root)
	injected := filepath.Join(t.TempDir(), "injected")
	if _, err := repository.CanonicalDiff(context.Background(), "--output="+injected, target); cemcode.CodeOf(err) != cemcode.GitDiffFailed {
		t.Fatalf("diff option-shaped base: %v", err)
	}
	if _, _, err := repository.LookupTreeEntry(context.Background(), "--output="+injected, "f.go"); cemcode.CodeOf(err) != cemcode.InvalidArguments {
		t.Fatalf("ls-tree option-shaped revision: %v", err)
	}
	if _, err := os.Lstat(injected); !os.IsNotExist(err) {
		t.Fatalf("option-shaped revision wrote a file: %v", err)
	}
}

// TestValidRevisionTextLengthBoundIsInclusive checks that the 256-byte
// revision expression length bound is inclusive: exactly 256 bytes is
// accepted, and 257 is refused.
func TestValidRevisionTextLengthBoundIsInclusive(t *testing.T) {
	if !validRevisionText(strings.Repeat("a", 256)) {
		t.Error("a 256-byte revision expression was refused")
	}
	if validRevisionText(strings.Repeat("a", 257)) {
		t.Error("a 257-byte revision expression was accepted")
	}
}

// TestTreePathsExcludesTheDirectoryEntryItself checks that `ls-tree -t`'s own
// entry for the requested directory (a bare "docs", with no trailing
// separator) is filtered out and only paths below it are returned.
func TestTreePathsExcludesTheDirectoryEntryItself(t *testing.T) {
	root, base, _ := makeRepo(t)
	repository := open(t, root)
	paths, err := repository.TreePaths(context.Background(), base, "docs")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "docs/rule.txt" {
		t.Fatalf("TreePaths(_, _, %q) = %v, want [docs/rule.txt]", "docs", paths)
	}
}

// TestCanonicalBytesIndependentOfHostileConfig proves repository and ambient
// diff configuration, tree diff drivers, and worktree attribute edits do not
// change canonical bytes: the exact failure class of the rejected build.
func TestCanonicalBytesIndependentOfHostileConfig(t *testing.T) {
	root, base, target := makeRepo(t)
	control, err := open(t, root).CanonicalDiff(context.Background(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(control) == 0 || !bytes.Contains(control, []byte("f.go")) {
		t.Fatalf("control diff unusable: %q", control)
	}
	// Hostile repository configuration.
	gitCmd(t, root, "config", "diff.evil.xfuncname", "^.*:=.*$")
	gitCmd(t, root, "config", "diff.algorithm", "histogram")
	gitCmd(t, root, "config", "diff.noprefix", "true")
	gitCmd(t, root, "config", "diff.mnemonicPrefix", "true")
	gitCmd(t, root, "config", "diff.context", "8")
	gitCmd(t, root, "config", "diff.suppressBlankEmpty", "true")
	// Hostile uncommitted worktree attributes.
	writeFile(t, root, ".gitattributes", "*.go diff=evil -diff\n")
	hostile, err := open(t, root).CanonicalDiff(context.Background(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(control, hostile) {
		t.Fatalf("canonical bytes changed under hostile configuration:\n-- control --\n%s\n-- hostile --\n%s", control, hostile)
	}
}

func TestCanonicalDiffExcludesSidecarOnly(t *testing.T) {
	root, _, target := makeRepo(t)
	writeFile(t, root, ".corvint/change.cem.json", "{}\n")
	writeFile(t, root, "docs/rule.txt", "the amended rule\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "sidecar+doc")
	next := gitCmd(t, root, "rev-parse", "HEAD")
	diff, err := open(t, root).CanonicalDiff(context.Background(), target, next)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(diff, []byte("change.cem.json")) {
		t.Fatalf("sidecar leaked into the canonical patch:\n%s", diff)
	}
	if !bytes.Contains(diff, []byte("docs/rule.txt")) {
		t.Fatalf("expected doc change missing:\n%s", diff)
	}
}

// TestLinkedWorktreeByteIdentical proves a primary checkout and a real linked
// worktree derive byte-identical canonical patches.
func TestLinkedWorktreeByteIdentical(t *testing.T) {
	root, base, target := makeRepo(t)
	linked := filepath.Join(t.TempDir(), "linked")
	gitCmd(t, root, "worktree", "add", "-q", "--detach", linked, "main")
	primary, err := open(t, root).CanonicalDiff(context.Background(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	viaLinked, err := open(t, linked).CanonicalDiff(context.Background(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(primary, viaLinked) {
		t.Fatal("primary and linked worktree derived different canonical bytes")
	}
}

// TestRelativePathsLinkedWorktree (V1-0215): a worktree created with --relative-paths records a
// back-pointer relative to its per-worktree Git directory; it opens and derives the primary's
// canonical patch, while a relative back-pointer naming another worktree is still refused.
func TestRelativePathsLinkedWorktree(t *testing.T) {
	root, base, target := makeRepo(t)
	parent := filepath.Dir(root)
	add := exec.Command("git", "worktree", "add", "-q", "--relative-paths", "--detach", filepath.Join(parent, "relwt"), "main")
	add.Dir = root
	if out, err := add.CombinedOutput(); err != nil {
		t.Skipf("git lacks worktree add --relative-paths: %v\n%s", err, out)
	}
	linked := filepath.Join(parent, "relwt")
	admin := filepath.Join(root, ".git", "worktrees", "relwt")
	pointer, err := os.ReadFile(filepath.Join(admin, "gitdir"))
	if err != nil || filepath.IsAbs(strings.TrimSpace(string(pointer))) {
		t.Fatalf("fixture back-pointer is not relative: %q %v", pointer, err)
	}
	primary, err := open(t, root).CanonicalDiff(context.Background(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	viaLinked, err := open(t, linked).CanonicalDiff(context.Background(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(primary, viaLinked) {
		t.Fatal("primary and relative-paths worktree derived different canonical bytes")
	}
	gitCmd(t, root, "worktree", "add", "-q", "--relative-paths", "--detach", filepath.Join(parent, "otherwt"), "main")
	other, err := filepath.Rel(admin, filepath.Join(parent, "otherwt", ".git"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, admin, "gitdir", other+"\n")
	if _, err := Open(linked, gitrun.NewDefaultBudget()); cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
		t.Fatalf("mismatched relative back-pointer: got %v, want repository-object-unavailable", err)
	}
}

// TestForgedGitfileRejected proves a forged gitfile cannot redirect reads into
// a sibling repository and that the sibling's canary never surfaces.
func TestForgedGitfileRejected(t *testing.T) {
	siblingRoot, _, _ := makeRepo(t)
	writeFile(t, siblingRoot, "canary.txt", "CANARY-9d3f\n")
	gitCmd(t, siblingRoot, "add", ".")
	gitCmd(t, siblingRoot, "commit", "-qm", "canary")
	forged := t.TempDir()
	writeFile(t, forged, ".git", "gitdir: "+filepath.Join(siblingRoot, ".git")+"\n")
	_, err := Open(forged, gitrun.NewDefaultBudget())
	if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
		t.Fatalf("got %v, want repository-object-unavailable", err)
	}
	if strings.Contains(err.Error(), "CANARY") {
		t.Fatal("canary disclosed through the rejection")
	}
}

func TestAlternatesDenied(t *testing.T) {
	root, _, _ := makeRepo(t)
	alternates := filepath.Join(root, ".git", "objects", "info", "alternates")
	if err := os.WriteFile(alternates, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Open(root, gitrun.NewDefaultBudget())
	if cemcode.CodeOf(err) != cemcode.UnsupportedObjectAlternates {
		t.Fatalf("empty alternates file: got %v", err)
	}
	if err := os.Remove(alternates); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", t.TempDir())
	_, err = Open(root, gitrun.NewDefaultBudget())
	if cemcode.CodeOf(err) != cemcode.UnsupportedObjectAlternates {
		t.Fatalf("ambient alternates: got %v", err)
	}
}

// TestReciprocalFailureBeforeAlternateDenial fixes the frozen precedence:
// forged worktree metadata rejects before alternate denial.
func TestReciprocalFailureBeforeAlternateDenial(t *testing.T) {
	siblingRoot, _, _ := makeRepo(t)
	forged := t.TempDir()
	writeFile(t, forged, ".git", "gitdir: "+filepath.Join(siblingRoot, ".git")+"\n")
	t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", t.TempDir())
	_, err := Open(forged, gitrun.NewDefaultBudget())
	if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
		t.Fatalf("got %v, want repository-object-unavailable before alternate denial", err)
	}
}

func TestSymlinkedObjectsTopologyRejected(t *testing.T) {
	for _, victim := range []string{"info", "pack"} {
		root, _, _ := makeRepo(t)
		target := filepath.Join(root, ".git", "objects", victim)
		moved := target + "-moved"
		if err := os.Rename(target, moved); err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(moved, 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Fatal(err)
			}
		}
		if err := os.Symlink(moved, target); err != nil {
			t.Fatal(err)
		}
		_, err := Open(root, gitrun.NewDefaultBudget())
		if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
			t.Fatalf("symlinked objects/%s: got %v", victim, err)
		}
	}
}

func TestInfoAttributesFailClosed(t *testing.T) {
	root, base, target := makeRepo(t)
	attributes := filepath.Join(root, ".git", "info", "attributes")
	if err := os.MkdirAll(filepath.Dir(attributes), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attributes, []byte("*.go diff=evil\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Open(root, gitrun.NewDefaultBudget())
	if cemcode.CodeOf(err) != cemcode.UnsupportedRepositoryAttributes {
		t.Fatalf("effective info/attributes: got %v", err)
	}
	if err := os.WriteFile(attributes, []byte("# comment only\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repository := open(t, root)
	if _, err := repository.CanonicalDiff(context.Background(), base, target); err != nil {
		t.Fatalf("comment-only info/attributes blocked derivation: %v", err)
	}
}

func TestBoundedMetadataRejections(t *testing.T) {
	// Oversized gitfile.
	forged := t.TempDir()
	writeFile(t, forged, ".git", "gitdir: "+strings.Repeat("x", maxGitfileBytes+1)+"\n")
	if _, err := Open(forged, gitrun.NewDefaultBudget()); cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
		t.Fatalf("oversized gitfile: got %v", err)
	}
	// Symlinked .git marker.
	realRoot, _, _ := makeRepo(t)
	symlinked := t.TempDir()
	if err := os.Symlink(filepath.Join(realRoot, ".git"), filepath.Join(symlinked, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(symlinked, gitrun.NewDefaultBudget()); cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
		t.Fatalf("symlinked .git marker: got %v", err)
	}
	// Missing marker.
	if _, err := Open(t.TempDir(), gitrun.NewDefaultBudget()); cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
		t.Fatal("missing marker accepted")
	}
}

func TestSha256Repository(t *testing.T) {
	root := t.TempDir()
	command := exec.Command("git", "init", "-q", "-b", "main", "--object-format=sha256")
	command.Dir = root
	if out, err := command.CombinedOutput(); err != nil {
		t.Skipf("git lacks sha256 support: %v %s", err, out)
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "config", "user.name", "t")
	gitCmd(t, root, "config", "user.email", "t@example.invalid")
	writeFile(t, root, "a.txt", "one\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "a.txt", "two\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "next")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	repository := open(t, root)
	if err := repository.LoadObjectFormat(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repository.ObjectFormat != "sha256" || repository.EmptyTreeOID() != emptyTreeSha256 {
		t.Fatalf("object format %q", repository.ObjectFormat)
	}
	if len(base) != 64 || len(target) != 64 {
		t.Fatalf("unexpected OID lengths %d/%d", len(base), len(target))
	}
	diff, err := repository.CanonicalDiff(context.Background(), base, target)
	if err != nil || !bytes.Contains(diff, []byte("a.txt")) {
		t.Fatalf("sha256 canonical diff: %v\n%s", err, diff)
	}
}

// TestShallowCloneLocallyCompleteDerives proves shallow state does not change
// canonical bytes when every required object resolves locally.
func TestShallowCloneLocallyCompleteDerives(t *testing.T) {
	root, base, target := makeRepo(t)
	full, err := open(t, root).CanonicalDiff(context.Background(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	shallowDir := filepath.Join(t.TempDir(), "shallow")
	gitCmd(t, filepath.Dir(shallowDir), "clone", "-q", "--depth", "2", "file://"+root, shallowDir)
	shallowDir, err = filepath.EvalSymlinks(shallowDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(filepath.Join(shallowDir, ".git", "shallow")); statErr != nil {
		t.Skipf("clone did not produce a shallow repository: %v", statErr)
	}
	viaShallow, err := open(t, shallowDir).CanonicalDiff(context.Background(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(full, viaShallow) {
		t.Fatal("shallow state changed canonical bytes")
	}
}

func TestLookupTreeEntryAndBlob(t *testing.T) {
	root, base, _ := makeRepo(t)
	repository := open(t, root)
	entry, exists, err := repository.LookupTreeEntry(context.Background(), base, "docs/rule.txt")
	if err != nil || !exists || entry.Mode != "100644" || entry.Type != "blob" {
		t.Fatalf("entry %+v exists=%v err=%v", entry, exists, err)
	}
	data, err := repository.BlobBytes(context.Background(), entry.OID)
	if err != nil || string(data) != "the rule\n" {
		t.Fatalf("blob %q err %v", data, err)
	}
	_, exists, err = repository.LookupTreeEntry(context.Background(), base, "missing.txt")
	if err != nil || exists {
		t.Fatalf("missing path: exists=%v err=%v", exists, err)
	}
}

func TestDeterministicAcrossFreshRuns(t *testing.T) {
	root, base, target := makeRepo(t)
	first, err := open(t, root).CanonicalDiff(context.Background(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	second, err := open(t, root).CanonicalDiff(context.Background(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("fresh derivations disagree")
	}
}

// TestLookupTreeEntryLiteralPath fixes literal-path resolution: a path whose
// bytes are Git pathspec magic ("data[1].txt" globs to "data1.txt") must
// resolve to its own blob, never to a glob match.
func TestLookupTreeEntryLiteralPath(t *testing.T) {
	root, _, _ := makeRepo(t)
	writeFile(t, root, "data1.txt", "glob decoy\n")
	writeFile(t, root, "data[1].txt", "literal target\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "pathspec fixtures")
	head := gitCmd(t, root, "rev-parse", "HEAD")
	repository := open(t, root)
	entry, exists, err := repository.LookupTreeEntry(context.Background(), head, "data[1].txt")
	if err != nil || !exists {
		t.Fatalf("literal path did not resolve: exists=%v err=%v", exists, err)
	}
	data, err := repository.BlobBytes(context.Background(), entry.OID)
	if err != nil || string(data) != "literal target\n" {
		t.Fatalf("literal path resolved to the wrong blob: %q %v", data, err)
	}
	_, exists, err = repository.LookupTreeEntry(context.Background(), head, "data[9].txt")
	if err != nil || exists {
		t.Fatalf("absent literal path: exists=%v err=%v", exists, err)
	}
}

// TestBlobBudgetChargesDistinctOids fixes the cumulative budget semantics:
// repeated reads of one blob charge its bytes exactly once.
func TestBlobBudgetChargesDistinctOids(t *testing.T) {
	root, base, _ := makeRepo(t)
	repository := open(t, root)
	entry, _, err := repository.LookupTreeEntry(context.Background(), base, "docs/rule.txt")
	if err != nil {
		t.Fatal(err)
	}
	first, err := repository.BlobBytes(context.Background(), entry.OID)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := repository.BlobBytes(context.Background(), entry.OID); err != nil {
			t.Fatal(err)
		}
	}
	if repository.blobBytes != int64(len(first)) {
		t.Fatalf("budget charged %d bytes for one %d-byte blob", repository.blobBytes, len(first))
	}
}

// TestPartialClonePromisorLocality proves both halves of the partial-clone
// contract with a live-origin fetch canary: the promisor remote stays
// reachable, so a lazy fetch of the promised blob WOULD succeed — the typed
// local refusal therefore proves no fetch was attempted. Removing the
// GIT_NO_LAZY_FETCH environment pin makes the blob resolve and fails this
// test.
func TestPartialClonePromisorLocality(t *testing.T) {
	root, base, _ := makeRepo(t)
	gitCmd(t, root, "config", "uploadpack.allowfilter", "true")
	partial := filepath.Join(t.TempDir(), "partial")
	gitCmd(t, filepath.Dir(partial), "clone", "-q", "--no-local", "--filter=blob:none", "file://"+root, partial)
	partial, err := filepath.EvalSymlinks(partial)
	if err != nil {
		t.Fatal(err)
	}
	promisors, _ := filepath.Glob(filepath.Join(partial, ".git", "objects", "pack", "*.promisor"))
	if len(promisors) == 0 {
		t.Skipf("clone did not produce a promisor pack")
	}
	repository := open(t, partial)
	head := gitCmd(t, partial, "rev-parse", "HEAD")
	local, exists, err := repository.LookupTreeEntry(context.Background(), head, "docs/rule.txt")
	if err != nil || !exists {
		t.Fatalf("checkout blob entry: exists=%v err=%v", exists, err)
	}
	data, err := repository.BlobBytes(context.Background(), local.OID)
	if err != nil || string(data) != "the rule\n" {
		t.Fatalf("locally complete blob read failed: %q %v", data, err)
	}
	promised, exists, err := repository.LookupTreeEntry(context.Background(), base, "f.go")
	if err != nil || !exists {
		t.Fatalf("base tree entry: exists=%v err=%v", exists, err)
	}
	_, err = repository.BlobBytes(context.Background(), promised.OID)
	if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
		t.Fatalf("promised-missing blob: got %v, want repository-object-unavailable without fetch", err)
	}
}

// TestBlobBudgetExactLimitAllowsChargedReread: with the distinct-blob budget
// exactly saturated, rereading an already-charged OID adds no distinct bytes
// and must still succeed, while any new OID is refused.
func TestBlobBudgetExactLimitAllowsChargedReread(t *testing.T) {
	root, base, target := makeRepo(t)
	repository := open(t, root)
	charged, _, err := repository.LookupTreeEntry(context.Background(), base, "docs/rule.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.BlobBytes(context.Background(), charged.OID); err != nil {
		t.Fatal(err)
	}
	repository.blobBytes = MaxTotalBlobBytes
	if _, err := repository.BlobBytes(context.Background(), charged.OID); err != nil {
		t.Fatalf("charged OID refused at the exact limit: %v", err)
	}
	fresh, _, err := repository.LookupTreeEntry(context.Background(), target, "f.go")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.BlobBytes(context.Background(), fresh.OID); cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
		t.Fatalf("new OID admitted past the saturated budget: %v", err)
	}
}
