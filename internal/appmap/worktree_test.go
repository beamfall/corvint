package appmap

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// objectCount returns the repository's loose and packed object counts as git reports them.
func objectCount(t *testing.T, root string) string {
	t.Helper()
	return gitTest(t, root, "count-objects", "-v")
}

// AMAP-V0-976: `--worktree` builds from the working tree: an untracked manifest and router, a
// tracked edit and a tracked deletion enter the snapshot, ignored files do not, the map carries
// working_tree, two builds are identical, and the repository's status and objects are unchanged.
func TestAMAPV0976WorktreeBuild(t *testing.T) {
	root := t.TempDir()
	gitTest(t, root, "init", "-q")
	writeFile(t, root, "README.md", "fixture\n")
	writeFile(t, root, "src/old.tsx", "export const gone = 1;\n")
	writeFile(t, root, ".gitignore", "build/\n")
	gitTest(t, root, "add", "-A")
	gitTest(t, root, "-c", "user.name=Fixture", "-c", "user.email=", "commit", "-qm", "fixture")
	head := gitTest(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "appmap.json", `{"schema": "application-map-manifest/0", "app": "web",
  "routers": [{"path": "src/routes.tsx", "dialect": "react-router/0"}],
  "tests": {"root": "e2e", "specs": ["e2e/specs"]}}
`)
	writeFile(t, root, "src/routes.tsx", `import { createBrowserRouter } from 'react-router-dom';
export const router = createBrowserRouter([
  { path: '/', element: <Home /> },
  { path: '/drafts', element: <Drafts /> },
]);
`)
	writeFile(t, root, "e2e/specs/drafts.spec.ts", "test('drafts', async ({ page }) => { await page.goto('/drafts'); });\n")
	writeFile(t, root, "README.md", "edited\n")
	writeFile(t, root, "build/specs/ignored.spec.ts", "test('x', async ({ page }) => { await page.goto('/'); });\n")
	if err := os.Remove(filepath.Join(root, "src/old.tsx")); err != nil {
		t.Fatal(err)
	}
	status, objects := gitTest(t, root, "status", "--porcelain=v1", "--untracked-files=all"), objectCount(t, root)

	// AMAP-V0-977: the pinned build keeps refusing, and names --worktree.
	if _, err := Build(context.Background(), root, "appmap.json", "HEAD"); codeOf(err) != "appmap-invalid-manifest" || !strings.Contains(err.Error(), "--worktree") {
		t.Fatalf("a pinned build of an uncommitted manifest: %v", err)
	}
	first, err := BuildWorktree(context.Background(), root, "appmap.json")
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildWorktree(context.Background(), root, "appmap.json")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := Encode(first)
	b, _ := Encode(second)
	if string(a) != string(b) {
		t.Fatalf("two worktree builds differ:\n%s\n%s", a, b)
	}
	if first.WorkingTree == nil || first.WorkingTree.BaseRevision != head || first.WorkingTree.Evidence != "unpinned" || first.Revision == head {
		t.Fatalf("working_tree = %+v, revision %s, head %s", first.WorkingTree, first.Revision, head)
	}
	if !strings.Contains(string(a), `"working_tree":{"base_revision":"`+head+`","evidence":"unpinned"}`) {
		t.Errorf("encoded map lacks working_tree: %s", a)
	}
	templates := []string{}
	for _, s := range first.Screens {
		templates = append(templates, s.Template)
	}
	slices.Sort(templates)
	if !slices.Equal(templates, []string{"/", "/drafts"}) || reachedTemplates(first)["/drafts"] != "goto" {
		t.Errorf("screens %v reached %v", templates, reachedTemplates(first))
	}
	files := []string{}
	for _, f := range first.Files {
		files = append(files, f.Path)
	}
	if !slices.Equal(files, []string{"e2e/specs/drafts.spec.ts"}) {
		t.Errorf("test files %v: the ignored spec must stay out", files)
	}
	// Readers decode the member (AMAP-V0-960) and the digest covers it.
	path := filepath.Join(t.TempDir(), "map.json")
	if err := os.WriteFile(path, a, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadMap(path)
	if err != nil || loaded.WorkingTree == nil || *loaded.WorkingTree != *first.WorkingTree {
		t.Fatalf("LoadMap: %v %+v", err, loaded)
	}
	if got := gitTest(t, root, "status", "--porcelain=v1", "--untracked-files=all"); got != status {
		t.Errorf("status changed:\n%s\nwant\n%s", got, status)
	}
	if got := objectCount(t, root); got != objects {
		t.Errorf("objects changed:\n%s\nwant\n%s", got, objects)
	}
	if got := gitTest(t, root, "rev-parse", "HEAD"); got != head {
		t.Errorf("HEAD moved to %s", got)
	}

	// A force-added file the ignore rules match is part of the working tree.
	writeFile(t, root, ".git/info/exclude", "e2e/specs/forced.spec.ts\n")
	writeFile(t, root, "e2e/specs/forced.spec.ts", "test('f', async ({ page }) => { await page.goto('/'); });\n")
	gitTest(t, root, "add", "-f", "e2e/specs/forced.spec.ts")
	forced, err := BuildWorktree(context.Background(), root, "appmap.json")
	if err != nil {
		t.Fatal(err)
	}
	if got := testPaths(forced); !slices.Equal(got, []string{"e2e/specs/drafts.spec.ts", "e2e/specs/forced.spec.ts"}) {
		t.Errorf("test files %v: the force-added spec belongs, the ignored one does not", got)
	}

	// An unborn branch builds from the untracked files alone, with an empty base revision.
	unborn := t.TempDir()
	gitTest(t, unborn, "init", "-q")
	for _, name := range []string{"appmap.json", "src/routes.tsx", "e2e/specs/drafts.spec.ts"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, unborn, name, string(data))
	}
	m, err := BuildWorktree(context.Background(), unborn, "appmap.json")
	if err != nil || m.WorkingTree.BaseRevision != "" || len(m.Screens) != 2 {
		t.Fatalf("unborn worktree build: %v %+v", err, m)
	}
}

// worktreeFixture commits a README and leaves an uncommitted manifest, router and two specs in the
// working tree, the second at e2e/specs/extra.spec.ts.
func worktreeFixture(t *testing.T, manifest string) string {
	t.Helper()
	root := t.TempDir()
	gitTest(t, root, "init", "-q")
	writeFile(t, root, "README.md", "fixture\n")
	gitTest(t, root, "add", "-A")
	gitTest(t, root, "-c", "user.name=Fixture", "-c", "user.email=", "commit", "-qm", "fixture")
	writeFile(t, root, "appmap.json", manifest)
	writeFile(t, root, "src/routes.tsx", "import { createBrowserRouter } from 'react-router-dom';\n"+
		"export const router = createBrowserRouter([\n  { path: '/', element: <Home /> },\n  { path: '/drafts', element: <Drafts /> },\n]);\n")
	writeFile(t, root, "e2e/specs/drafts.spec.ts", "test('drafts', async ({ page }) => { await page.goto('/drafts'); });\n")
	writeFile(t, root, "e2e/specs/extra.spec.ts", "test('extra', async ({ page }) => { await page.goto('/'); });\n")
	return root
}

const worktreeManifest = "{\"schema\": \"application-map-manifest/0\", \"app\": \"web\",\n" +
	"  \"routers\": [{\"path\": \"src/routes.tsx\", \"dialect\": \"react-router/0\"}],\n" +
	"  \"tests\": {\"root\": \"e2e\", \"specs\": [\"e2e/specs\"]}}\n"

func blobID(text string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(text), text))))
}

func testPaths(m *Map) []string {
	out := []string{}
	for _, f := range m.Files {
		out = append(out, f.Path)
	}
	return out
}

// AMAP-V0-976/977: --worktree refuses a --root below the worktree top, which a working-tree read
// would have to widen, and the pinned refusal names --worktree only when it could build; a sparse
// checkout is refused. The
// repository settings that decide what `git add` stores (core.autocrlf, core.excludesFile,
// info/attributes) apply to the snapshot, and the temporary repository is removed on success and
// on failure.
func TestAMAPV0976WorktreeRootAndConfig(t *testing.T) {
	root := worktreeFixture(t, worktreeManifest)
	if _, err := BuildWorktree(context.Background(), root, "appmap.json"); err != nil {
		t.Fatal(err)
	}
	// A Git directory whose path holds a space still resolves its common directory.
	spaced := filepath.Join(t.TempDir(), "my repo")
	if err := os.Rename(worktreeFixture(t, worktreeManifest), spaced); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildWorktree(context.Background(), spaced, "appmap.json"); err != nil {
		t.Errorf("a worktree under a path with a space: %v", err)
	}
	if _, err := BuildWorktree(context.Background(), filepath.Join(root, "src"), "appmap.json"); codeOf(err) != "appmap-root-unavailable" || !strings.Contains(err.Error(), "not the top of a Git worktree") {
		t.Errorf("a --root below the worktree top: %v", err)
	}
	if _, err := Build(context.Background(), filepath.Join(root, "src"), "appmap.json", "HEAD"); err == nil || strings.Contains(err.Error(), "--worktree") {
		t.Errorf("a pinned build from a subdirectory names --worktree, which refuses it: %v", err)
	}

	sparse := worktreeFixture(t, worktreeManifest)
	gitTest(t, sparse, "config", "core.sparseCheckout", "true")
	if _, err := BuildWorktree(context.Background(), sparse, "appmap.json"); codeOf(err) != "appmap-root-unavailable" || !strings.Contains(err.Error(), "sparse checkout") {
		t.Errorf("a sparse checkout: %v", err)
	}
	gitTest(t, sparse, "config", "core.sparseCheckout", "2")
	if _, err := BuildWorktree(context.Background(), sparse, "appmap.json"); codeOf(err) != "appmap-root-unavailable" || !strings.Contains(err.Error(), "sparse checkout") {
		t.Errorf("a sparse checkout set to a nonzero integer: %v", err)
	}

	crlf := strings.ReplaceAll(worktreeManifest, "\n", "\r\n")
	autocrlf := worktreeFixture(t, crlf)
	gitTest(t, autocrlf, "config", "core.autocrlf", "true")
	m, err := BuildWorktree(context.Background(), autocrlf, "appmap.json")
	if err != nil || m.Manifest.Blob != blobID(worktreeManifest) {
		t.Errorf("core.autocrlf: %v, manifest blob %s, want the normalized %s", err, m.Manifest.Blob, blobID(worktreeManifest))
	}
	attributes := worktreeFixture(t, crlf)
	writeFile(t, attributes, ".git/info/attributes", "*.json text\n")
	m, err = BuildWorktree(context.Background(), attributes, "appmap.json")
	if err != nil || m.Manifest.Blob != blobID(worktreeManifest) {
		t.Errorf("info/attributes: %v, manifest blob %s, want the normalized %s", err, m.Manifest.Blob, blobID(worktreeManifest))
	}

	excluded := worktreeFixture(t, worktreeManifest)
	ignore := filepath.Join(t.TempDir(), "ignore")
	if err := os.WriteFile(ignore, []byte("extra.spec.ts\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTest(t, excluded, "config", "core.excludesFile", ignore)
	m, err = BuildWorktree(context.Background(), excluded, "appmap.json")
	if err != nil || !slices.Equal(testPaths(m), []string{"e2e/specs/drafts.spec.ts"}) {
		t.Errorf("core.excludesFile: %v, test files %v", err, testPaths(m))
	}

	// No snapshot directory survives a build, whether it succeeds or fails after the snapshot.
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	if _, err := BuildWorktree(context.Background(), root, "appmap.json"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "appmap.json", "{not json\n")
	if _, err := BuildWorktree(context.Background(), root, "appmap.json"); codeOf(err) != "appmap-invalid-manifest" {
		t.Fatalf("a malformed working-tree manifest: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(tmp, "corvint-appmap-worktree-*")); len(left) != 0 {
		t.Errorf("snapshot directories left behind: %v", left)
	}
}
