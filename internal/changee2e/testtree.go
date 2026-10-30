package changee2e

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Test-tree reasons (CCE-V0-017): either one makes the proof INCONCLUSIVE before any run.
const (
	ReasonTestTreeUnresolved = "test-tree-unresolved"
	ReasonTestTreeOverlaps   = "test-tree-overlaps-application"
)

// Test-tree bounds. A tree or a scan past them is unresolved, never silently partial.
const (
	maxTestTreeFiles    = 512
	maxApplicationScans = 50000
)

// testTreeManifests are the root package manifest and lockfiles that resolve the runner's
// dependencies; each one present at the test revision is part of the test tree.
var testTreeManifests = []string{"package.json", "package-lock.json", "npm-shrinkwrap.json", "yarn.lock", "pnpm-lock.yaml"}

// moduleExtensions are the script extensions an import specifier may omit, in resolution order.
var moduleExtensions = []string{".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs"}

// importSpecifier matches the module specifier of a static import or export, a dynamic import and
// a require call. Text in comments matches too, which can only add an import, never hide one.
var importSpecifier = regexp.MustCompile(`(?:\bfrom\s*|\bimport\s*\(?\s*|\brequire\s*\(\s*)['"]([^'"\n]+)['"]`)

// testTree is the head test tree one proof runs on both sides (CCE-V0-017): the compiled scenario,
// its transitive relative imports, the generated runner configuration, and the package manifest
// and lockfiles. Paths are repository-relative with forward slashes.
type testTree struct {
	files  map[string][]byte
	digest string
}

// buildTestTree reads the test tree of sc from headDir, an extraction of the test revision. It
// returns ReasonTestTreeUnresolved when a relative import resolves to no file inside the tree,
// escapes it, or the tree exceeds its bound, and ReasonTestTreeOverlaps when an application file
// imports a test-tree file.
func buildTestTree(headDir string, sc Scenario) (testTree, string) {
	files := map[string][]byte{sc.Placement: sc.Text, proveConfigName: []byte(proveConfig)}
	queue := []string{sc.Placement}
	for len(queue) > 0 {
		file := queue[0]
		queue = queue[1:]
		for _, spec := range relativeImports(files[file]) {
			target, ok := resolveModule(headDir, path.Dir(file), spec)
			if !ok {
				return testTree{}, ReasonTestTreeUnresolved
			}
			if _, seen := files[target]; seen {
				continue
			}
			data, err := os.ReadFile(filepath.Join(headDir, filepath.FromSlash(target)))
			if err != nil {
				return testTree{}, ReasonTestTreeUnresolved
			}
			files[target] = data
			if len(files) > maxTestTreeFiles {
				return testTree{}, ReasonTestTreeUnresolved
			}
			queue = append(queue, target)
		}
	}
	for _, name := range testTreeManifests {
		if data, err := os.ReadFile(filepath.Join(headDir, name)); err == nil {
			files[name] = data
		}
	}
	overlaps, err := overlapsApplication(headDir, files)
	if err != nil {
		return testTree{}, ReasonTestTreeUnresolved
	}
	if overlaps {
		return testTree{}, ReasonTestTreeOverlaps
	}
	return testTree{files: files, digest: treeDigest(files)}, ""
}

// treeDigest is SHA-256 over the sorted `path NUL sha256(content) LF` lines of files.
func treeDigest(files map[string][]byte) string {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var b strings.Builder
	for _, p := range paths {
		b.WriteString(p + "\x00" + sha256Hex(files[p]) + "\n")
	}
	return "sha256:" + sha256Hex([]byte(b.String()))
}

// relativeImports returns the distinct relative module specifiers in text, in order.
func relativeImports(text []byte) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range importSpecifier.FindAllSubmatch(text, -1) {
		spec := string(m[1])
		if (strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../")) && !seen[spec] {
			seen[spec] = true
			out = append(out, spec)
		}
	}
	return out
}

// resolveModule resolves a relative specifier from dir to a regular file inside root: the exact
// path, then each omitted extension, then a TypeScript source for a `.js` specifier, then an index
// file. A target outside root never resolves.
func resolveModule(root, dir, spec string) (string, bool) {
	joined := path.Clean(path.Join(dir, spec))
	if joined == "." || joined == ".." || strings.HasPrefix(joined, "../") || path.IsAbs(joined) {
		return "", false
	}
	candidates := []string{joined}
	for _, ext := range moduleExtensions {
		candidates = append(candidates, joined+ext)
	}
	if ext := path.Ext(joined); ext == ".js" || ext == ".jsx" || ext == ".mjs" || ext == ".cjs" {
		stem := strings.TrimSuffix(joined, ext)
		candidates = append(candidates, stem+".ts", stem+".tsx", stem+".mts", stem+".cts")
	}
	for _, ext := range moduleExtensions {
		candidates = append(candidates, joined+"/index"+ext)
	}
	for _, c := range candidates {
		if info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(c))); err == nil && info.Mode().IsRegular() {
			return c, true
		}
	}
	return "", false
}

// testLikePath reports whether a repository path is test code rather than application code: a
// `.spec.` or `.test.` file, or a file under a conventional test directory.
func testLikePath(p string) bool {
	base := path.Base(p)
	if strings.Contains(base, ".spec.") || strings.Contains(base, ".test.") {
		return true
	}
	for _, dir := range strings.Split(path.Dir(p), "/") {
		switch dir {
		case "e2e", "test", "tests", "__tests__", "spec", "cypress", "playwright":
			return true
		}
	}
	return false
}

var errApplicationScanOverLimit = errors.New("application scan over limit")

// overlapsApplication reports whether any application script under root imports a test-tree
// script. The manifests and the generated configuration are shared by design and never overlap.
func overlapsApplication(root string, files map[string][]byte) (bool, error) {
	shared := map[string]bool{proveConfigName: true}
	for _, name := range testTreeManifests {
		shared[name] = true
	}
	scanned, overlap := 0, false
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "node_modules" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if _, inTree := files[rel]; inTree || testLikePath(rel) || !d.Type().IsRegular() || !moduleFile(rel) {
			return nil
		}
		if scanned++; scanned > maxApplicationScans {
			return errApplicationScanOverLimit
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, spec := range relativeImports(data) {
			if target, ok := resolveModule(root, path.Dir(rel), spec); ok && !shared[target] {
				if _, hit := files[target]; hit {
					overlap = true
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	return overlap, err
}

func moduleFile(p string) bool {
	ext := path.Ext(p)
	for _, e := range moduleExtensions {
		if ext == e {
			return true
		}
	}
	return false
}
