package appmap

import (
	"context"
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
