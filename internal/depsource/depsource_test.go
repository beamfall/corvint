package depsource

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	fixtureModule  = "example.com/Dep"
	fixtureVersion = "v1.2.3"
)

// expectedHash1 recomputes the "h1:" hash from the documented formula alone, so
// a test never proves the implementation against itself.
func expectedHash1(t *testing.T, files map[string]string, prefix string) string {
	t.Helper()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, prefix+"/"+name)
	}
	sort.Strings(names)
	summary := sha256.New()
	for _, name := range names {
		content := files[strings.TrimPrefix(name, prefix+"/")]
		digest := sha256.Sum256([]byte(content))
		fmt.Fprintf(summary, "%x  %s\n", digest[:], name)
	}
	return "h1:" + base64.StdEncoding.EncodeToString(summary.Sum(nil))
}

func writeFiles(t *testing.T, directory string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		full := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// newCache writes a fixture module cache: the extracted module directory plus
// the cache's own ziphash record.
func newCache(t *testing.T, files map[string]string, hash string) string {
	t.Helper()
	cache := t.TempDir()
	writeFiles(t, filepath.Join(cache, "example.com", "!dep@"+fixtureVersion), files)
	ziphash := filepath.Join(cache, "cache", "download", "example.com", "!dep", "@v")
	if err := os.MkdirAll(ziphash, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ziphash, fixtureVersion+".ziphash"), []byte(hash), 0o644); err != nil {
		t.Fatal(err)
	}
	return cache
}

// newRepository commits the given files and then dirties the worktree copy of
// go.mod, so a test can prove the committed blob is the evidence source.
func newRepository(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	home := t.TempDir()
	run := func(arguments ...string) {
		t.Helper()
		command := exec.Command("git", arguments...)
		command.Dir = root
		command.Env = []string{
			"PATH=" + os.Getenv("PATH"), "HOME=" + home, "LANG=C", "LC_ALL=C",
			"GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
			"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
		}
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	run("init", "-q", "-b", "main")
	writeFiles(t, root, files)
	run("add", "-A")
	run("commit", "-q", "-m", "fixture")
	return root
}

func goModFor(requires, replaces string) string {
	return "module example.com/host\n\ngo 1.27.0\n\nrequire (\n" + requires + ")\n" + replaces
}

func fixtureRepository(t *testing.T, hash string) string {
	t.Helper()
	return newRepository(t, map[string]string{
		"go.mod": goModFor("\t"+fixtureModule+" "+fixtureVersion+"\n", ""),
		"go.sum": fixtureModule + " " + fixtureVersion + " " + hash + "\n" +
			fixtureModule + " " + fixtureVersion + "/go.mod h1:bogusgomodhash=\n",
	})
}

var fixtureFiles = map[string]string{
	"go.mod":       "module example.com/Dep\n",
	"dep.go":       "package dep\n\nfunc Answer() int { return 42 }\n",
	"sub/inner.go": "package sub\n",
}

func resolveFixture(t *testing.T, options Options) Result {
	t.Helper()
	result, err := Resolve(context.Background(), options)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return result
}

// DSE-V0-003: module path escaping follows the x/mod rule.
func TestEscapeModulePath(t *testing.T) {
	for input, want := range map[string]string{
		"example.com/Dep":         "example.com/!dep",
		"github.com/BurntSushi/x": "github.com/!burnt!sushi/x",
		"v1.0.0-RC1":              "v1.0.0-!r!c1",
	} {
		got, err := escapeModulePath(input)
		if err != nil || got != want {
			t.Fatalf("escapeModulePath(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := escapeModulePath("bad!path"); err == nil {
		t.Fatal("expected an unescapable '!' to be refused")
	}
}

// DSE-V0-004: an exact h1 match over the extracted directory yields verified evidence.
func TestVerifiedModuleListing(t *testing.T) {
	hash := expectedHash1(t, fixtureFiles, fixtureModule+"@"+fixtureVersion)
	result := resolveFixture(t, Options{
		Root:        fixtureRepository(t, hash),
		Module:      fixtureModule,
		ModuleCache: newCache(t, fixtureFiles, hash),
	})
	if !result.Verified || result.Reason != "" {
		t.Fatalf("expected verified evidence, got %+v", result)
	}
	if result.GoSumHash != hash || result.ComputedHash != hash {
		t.Fatalf("hash mismatch: %+v", result)
	}
	if result.Version != fixtureVersion || result.Resolution != "require" {
		t.Fatalf("unexpected resolution: %+v", result)
	}
}

// DSE-V0-002: the committed go.mod blob is the resolution source, never the worktree.
func TestResolveReadsCommittedGoMod(t *testing.T) {
	hash := expectedHash1(t, fixtureFiles, fixtureModule+"@"+fixtureVersion)
	root := fixtureRepository(t, hash)
	dirty := goModFor("\t"+fixtureModule+" v9.9.9\n", "")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(dirty), 0o644); err != nil {
		t.Fatal(err)
	}
	result := resolveFixture(t, Options{Root: root, Module: fixtureModule, ModuleCache: newCache(t, fixtureFiles, hash)})
	if result.Version != fixtureVersion || !result.Verified {
		t.Fatalf("dirty worktree leaked into evidence: %+v", result)
	}
}

// DSE-V0-002: Git output exceeding the evidence-read cap is discarded while
// the command runs and refused with the existing bounded-read diagnostic.
func TestGitOutputReadIsBounded(t *testing.T) {
	root := newRepository(t, map[string]string{"large.txt": strings.Repeat("x", maxGitOutputBytes+1)})
	revision, err := committedRevision(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	output, err := git(context.Background(), root, "show", revision+":large.txt")
	if err == nil || err.Error() != "git show emitted more than the bounded read limit" {
		t.Fatalf("want bounded-read refusal, got output=%d err=%v", len(output), err)
	}
	if output != nil {
		t.Fatalf("overflow returned %d partial bytes", len(output))
	}
}

// DSE-V0-005: a recomputed hash that differs from go.sum abstains with no content.
func TestHashMismatchAbstains(t *testing.T) {
	hash := expectedHash1(t, fixtureFiles, fixtureModule+"@"+fixtureVersion)
	tampered := map[string]string{}
	for name, content := range fixtureFiles {
		tampered[name] = content
	}
	tampered["dep.go"] = "package dep\n\nfunc Answer() int { return 43 }\n"
	result := resolveFixture(t, Options{
		Root:        fixtureRepository(t, hash),
		Module:      fixtureModule,
		ModuleCache: newCache(t, tampered, hash),
	})
	if result.Verified || result.Reason != "hash-mismatch" {
		t.Fatalf("expected hash-mismatch abstention, got %+v", result)
	}
	if len(result.Files) != 0 || result.File != nil {
		t.Fatal("an abstention must carry no file evidence")
	}
}

// DSE-V0-006: a module absent from the local cache abstains, never downloads.
func TestMissingCacheEntryAbstains(t *testing.T) {
	hash := expectedHash1(t, fixtureFiles, fixtureModule+"@"+fixtureVersion)
	result := resolveFixture(t, Options{
		Root:        fixtureRepository(t, hash),
		Module:      fixtureModule,
		ModuleCache: t.TempDir(),
	})
	if result.Verified || result.Reason != "missing-cache-entry" {
		t.Fatalf("expected missing-cache-entry abstention, got %+v", result)
	}
}

// DSE-V0-007: an unrequired module, a mismatched version, an ambiguous version
// and a missing go.sum line each abstain.
func TestResolutionAbstentions(t *testing.T) {
	hash := expectedHash1(t, fixtureFiles, fixtureModule+"@"+fixtureVersion)
	cache := newCache(t, fixtureFiles, hash)
	root := fixtureRepository(t, hash)

	cases := []struct {
		name    string
		options Options
		reason  string
	}{
		{"unrequired", Options{Root: root, Module: "example.com/Absent", ModuleCache: cache}, "module-not-required"},
		{"mismatch", Options{Root: root, Module: fixtureModule, Version: "v0.0.1", ModuleCache: cache}, "version-mismatch"},
	}
	for _, testCase := range cases {
		result := resolveFixture(t, testCase.options)
		if result.Verified || result.Reason != testCase.reason {
			t.Fatalf("%s: want %s, got %+v", testCase.name, testCase.reason, result)
		}
	}

	ambiguous := newRepository(t, map[string]string{
		"go.mod": goModFor("\t"+fixtureModule+" "+fixtureVersion+"\n\t"+fixtureModule+" v2.0.0\n", ""),
		"go.sum": "",
	})
	if result := resolveFixture(t, Options{Root: ambiguous, Module: fixtureModule, ModuleCache: cache}); result.Reason != "ambiguous-version" {
		t.Fatalf("want ambiguous-version, got %+v", result)
	}

	noSum := newRepository(t, map[string]string{
		"go.mod": goModFor("\t"+fixtureModule+" "+fixtureVersion+"\n", ""),
		"go.sum": "example.com/other v1.0.0 h1:x=\n",
	})
	if result := resolveFixture(t, Options{Root: noSum, Module: fixtureModule, ModuleCache: cache}); result.Reason != "missing-go-sum-entry" {
		t.Fatalf("want missing-go-sum-entry, got %+v", result)
	}
}

// DSE-V0-007: every committed-evidence and cache-location abstention the
// verb can reach is named by the spec, so each is asserted by its reason.
func TestCacheStageAbstentions(t *testing.T) {
	require := "\t" + fixtureModule + " " + fixtureVersion + "\n"
	sumLine := fixtureModule + " " + fixtureVersion + " h1:x=\n"
	cases := []struct {
		name, reason string
		files        map[string]string
		module       string
	}{
		{"no-go-sum", "missing-go-sum", map[string]string{"go.mod": goModFor(require, "")}, fixtureModule},
		{"oversized-go-sum", "go-sum-exceeds-read-limit", map[string]string{
			"go.mod": goModFor(require, ""), "go.sum": sumLine + strings.Repeat("#", maxSumBytes)}, fixtureModule},
		{"replace-without-version", "replace-without-version", map[string]string{
			"go.mod": goModFor(require, "\nreplace "+fixtureModule+" => example.com/fork\n"), "go.sum": sumLine}, fixtureModule},
		{"unescapable-path", "unescapable-module-path", map[string]string{
			"go.mod": goModFor("\tevil.example/../etc "+fixtureVersion+"\n", ""),
			"go.sum": "evil.example/../etc " + fixtureVersion + " h1:x=\n"}, "evil.example/../etc"},
		{"unescapable-version", "unescapable-module-version", map[string]string{
			"go.mod": goModFor("\t"+fixtureModule+" v1.2.3!x\n", ""),
			"go.sum": fixtureModule + " v1.2.3!x h1:x=\n"}, fixtureModule},
	}
	for _, testCase := range cases {
		root := newRepository(t, testCase.files)
		result := resolveFixture(t, Options{Root: root, Module: testCase.module, ModuleCache: t.TempDir()})
		if result.Verified || result.Reason != testCase.reason || len(result.Files) != 0 {
			t.Fatalf("%s: want %s, got %+v", testCase.name, testCase.reason, result)
		}
	}

	root := newRepository(t, map[string]string{"go.mod": goModFor(require, ""), "go.sum": sumLine})
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	onlyGit := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(onlyGit, "git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOMODCACHE", "")
	t.Setenv("PATH", onlyGit)
	if result := resolveFixture(t, Options{Root: root, Module: fixtureModule}); result.Reason != "module-cache-unavailable" {
		t.Fatalf("want module-cache-unavailable, got %+v", result)
	}
}

// DSE-V0-008: the listing is deterministic, bounded, and marks truncation.
func TestListingBoundedAndOrdered(t *testing.T) {
	hash := expectedHash1(t, fixtureFiles, fixtureModule+"@"+fixtureVersion)
	options := Options{
		Root:        fixtureRepository(t, hash),
		Module:      fixtureModule,
		ModuleCache: newCache(t, fixtureFiles, hash),
		Limit:       2,
	}
	result := resolveFixture(t, options)
	if result.FileCount != 3 || !result.Truncated || len(result.Files) != 2 {
		t.Fatalf("expected a bounded listing, got %+v", result)
	}
	if result.Files[0].Path != "dep.go" || result.Files[1].Path != "go.mod" {
		t.Fatalf("listing is not path-ordered: %+v", result.Files)
	}
	if result.Files[0].Size != int64(len(fixtureFiles["dep.go"])) {
		t.Fatalf("listing size is wrong: %+v", result.Files[0])
	}
}

// DSE-V0-009: --file emits the file's sha256 and bounded content, or abstains.
func TestFileExcerpt(t *testing.T) {
	large := strings.Repeat("x\n", 40<<10) // 80 KiB, above the 64 KiB cap.
	files := map[string]string{"go.mod": "module example.com/Dep\n", "big.txt": large}
	hash := expectedHash1(t, files, fixtureModule+"@"+fixtureVersion)
	options := Options{
		Root:        fixtureRepository(t, hash),
		Module:      fixtureModule,
		ModuleCache: newCache(t, files, hash),
		File:        "big.txt",
	}
	result := resolveFixture(t, options)
	if !result.Verified || result.File == nil {
		t.Fatalf("expected a verified excerpt, got %+v", result)
	}
	digest := sha256.Sum256([]byte(large))
	if result.File.SHA256 != fmt.Sprintf("%x", digest[:]) {
		t.Fatalf("excerpt digest is not the whole file's: %+v", result.File)
	}
	if !result.File.Truncated || !result.Truncated {
		t.Fatal("an over-cap excerpt must be marked truncated")
	}
	if got := len(strings.Join(result.File.Lines, "\n")) + 1; got > maxExcerptBytes {
		t.Fatalf("excerpt is %d bytes, above the cap", got)
	}

	options.File = "absent.go"
	if missing := resolveFixture(t, options); missing.Reason != "file-not-in-module" {
		t.Fatalf("want file-not-in-module, got %+v", missing)
	}
}

// DSE-V0-004 and DSE-V0-009: hashing a requested large file streams its whole
// digest and size while retaining only the bounded excerpt from the same handle.
func TestLargeFileCaptureIsBounded(t *testing.T) {
	directory := t.TempDir()
	large := strings.Repeat("0123456789abcdef", (maxExcerptBytes/16)+1024)
	source := map[string]string{"large.txt": large}
	writeFiles(t, directory, source)
	prefix := fixtureModule + "@" + fixtureVersion

	directoryHash, _, captured, err := hashDirectory(context.Background(), directory, prefix, "large.txt")
	if err != nil {
		t.Fatalf("hashDirectory: %v", err)
	}
	if want := expectedHash1(t, source, prefix); directoryHash != want {
		t.Fatalf("directory hash mismatch: got %s want %s", directoryHash, want)
	}
	if captured == nil {
		t.Fatal("requested file was not captured")
	}
	if len(captured.content) != maxExcerptBytes {
		t.Fatalf("capture length = %d, want %d", len(captured.content), maxExcerptBytes)
	}
	digest := sha256.Sum256([]byte(large))
	if captured.sha256 != fmt.Sprintf("%x", digest[:]) {
		t.Fatalf("whole-file digest mismatch: %+v", captured)
	}
	if captured.size != int64(len(large)) {
		t.Fatalf("whole-file size mismatch: %+v", captured)
	}
	result := cacheFile(Result{}, "large.txt", captured)
	if result.File == nil || !result.File.Truncated || result.File.SHA256 != captured.sha256 {
		t.Fatalf("bounded capture rendered incorrectly: %+v", result.File)
	}
}

// DSE-V0-004: the excerpt hashDirectory captures is the bytes it hashed, so a
// write landing after it returns cannot reach the caller as evidence.
func TestFileExcerptCapturedDuringHash(t *testing.T) {
	directory := t.TempDir()
	source := map[string]string{"go.mod": "module example.com/Dep\n", "dep.go": "package dep\n"}
	writeFiles(t, directory, source)
	prefix := fixtureModule + "@" + fixtureVersion

	digest, _, captured, err := hashDirectory(context.Background(), directory, prefix, "dep.go")
	if err != nil {
		t.Fatalf("hashDirectory: %v", err)
	}
	if want := expectedHash1(t, source, prefix); digest != want {
		t.Fatalf("hash mismatch: got %s want %s", digest, want)
	}
	if string(captured.content) != source["dep.go"] {
		t.Fatalf("captured content is wrong: %q", captured.content)
	}

	// A write after hashDirectory has returned must never reach the caller:
	// there is no second read of the path left to observe it.
	if err := os.WriteFile(filepath.Join(directory, "dep.go"), []byte("package dep\n\nfunc Tampered() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if string(captured.content) != source["dep.go"] {
		t.Fatalf("captured content changed after a post-hash write: %q", captured.content)
	}
	if result := cacheFile(Result{}, "dep.go", captured); result.File == nil || result.File.Lines[0] != "package dep" {
		t.Fatalf("cacheFile did not render the captured bytes: %+v", result.File)
	}

	if _, _, missing, err := hashDirectory(context.Background(), directory, prefix, "absent.go"); err != nil || missing != nil {
		t.Fatalf("a wantRel matching no file must capture nothing, got %v, %v", missing, err)
	}
	if abstained := cacheFile(Result{}, "absent.go", nil); abstained.Verified || abstained.Reason != "file-not-in-module" {
		t.Fatalf("nil captured content must abstain, got %+v", abstained)
	}
}

// DSE-V0-010: a local replace inside the tree is pinned by blob hash; one
// outside the tree abstains.
func TestLocalReplace(t *testing.T) {
	root := newRepository(t, map[string]string{
		"go.mod": goModFor("\t"+fixtureModule+" "+fixtureVersion+"\n",
			"\nreplace "+fixtureModule+" => ./vendorlocal\n"),
		"go.sum":                   "",
		"vendorlocal/go.mod":       "module example.com/Dep\n",
		"vendorlocal/dep.go":       "package dep\n",
		"vendorlocal/sub/inner.go": "package sub\n",
	})
	result := resolveFixture(t, Options{Root: root, Module: fixtureModule})
	if !result.Verified || result.Resolution != "local-replace" {
		t.Fatalf("expected a verified local replace, got %+v", result)
	}
	if result.FileCount != 3 || result.Files[0].Path != "dep.go" || result.Files[0].Blob == "" {
		t.Fatalf("local replace is not pinned by blob: %+v", result.Files)
	}

	withFile := resolveFixture(t, Options{Root: root, Module: fixtureModule, File: "dep.go"})
	if withFile.File == nil || withFile.File.Blob == "" || withFile.File.Lines[0] != "package dep" {
		t.Fatalf("expected pinned blob content, got %+v", withFile.File)
	}

	outside := newRepository(t, map[string]string{
		"go.mod": goModFor("\t"+fixtureModule+" "+fixtureVersion+"\n",
			"\nreplace "+fixtureModule+" => ../elsewhere\n"),
		"go.sum": "",
	})
	if escaped := resolveFixture(t, Options{Root: outside, Module: fixtureModule}); escaped.Verified ||
		escaped.Reason != "replace-outside-repository" {
		t.Fatalf("want replace-outside-repository, got %+v", escaped)
	}
}

// DSE-V0-004: the cache's own ziphash record is a second comparison, so a cache
// entry whose recorded ziphash disagrees with go.sum is refused even when the
// recomputed directory hash matches.
func TestZipHashMismatchIsRefused(t *testing.T) {
	hash := expectedHash1(t, fixtureFiles, fixtureModule+"@"+fixtureVersion)
	result := resolveFixture(t, Options{
		Root:        fixtureRepository(t, hash),
		Module:      fixtureModule,
		ModuleCache: newCache(t, fixtureFiles, "h1:"+strings.Repeat("A", 43)+"="),
	})
	if result.Verified || result.Reason != "ziphash-mismatch" {
		t.Fatalf("want ziphash-mismatch, got %+v", result)
	}
	if result.ComputedHash != hash {
		t.Fatalf("the directory hash still had to be recomputed: %+v", result)
	}
}

// DSE-V0-004: a corrupt ziphash larger than its small read cap is detected
// without retaining the full file and produces the established mismatch abstention.
func TestOversizedZipHashIsRefused(t *testing.T) {
	hash := expectedHash1(t, fixtureFiles, fixtureModule+"@"+fixtureVersion)
	cache := newCache(t, fixtureFiles, hash)
	ziphashPath := filepath.Join(cache, "cache", "download", "example.com", "!dep", "@v", fixtureVersion+".ziphash")
	if err := os.WriteFile(ziphashPath, []byte(strings.Repeat("x", maxZipHashBytes+1)), 0o644); err != nil {
		t.Fatal(err)
	}
	value, found, overflow := readZipHash(cache, "example.com/!dep", fixtureVersion)
	if !found || !overflow || len(value) > maxZipHashBytes {
		t.Fatalf("oversized ziphash was not bounded: found=%v overflow=%v bytes=%d", found, overflow, len(value))
	}
	result := resolveFixture(t, Options{
		Root:        fixtureRepository(t, hash),
		Module:      fixtureModule,
		ModuleCache: cache,
	})
	if result.Verified || result.Reason != "ziphash-mismatch" {
		t.Fatalf("want ziphash-mismatch, got %+v", result)
	}
}

// DSE-V0-010: a local replace whose target is inside the repository but absent
// from the committed tree has no immutable pin, so the verb abstains.
func TestLocalReplaceTargetNotCommitted(t *testing.T) {
	root := newRepository(t, map[string]string{
		"go.mod": goModFor("\t"+fixtureModule+" "+fixtureVersion+"\n",
			"\nreplace "+fixtureModule+" => ./uncommitted\n"),
		"go.sum": "",
	})
	if err := os.MkdirAll(filepath.Join(root, "uncommitted"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "uncommitted", "dep.go"), []byte("package dep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := resolveFixture(t, Options{Root: root, Module: fixtureModule})
	if result.Verified || result.Reason != "replace-path-not-committed" {
		t.Fatalf("want replace-path-not-committed, got %+v", result)
	}
}

// DSE-V0-010: a replace target that is a committed file or symlink is not a
// committed module directory, so it abstains instead of verifying one blob.
func TestLocalReplaceTargetNotADirectory(t *testing.T) {
	for _, target := range []string{"README.md", "linkdir"} {
		root := newRepository(t, map[string]string{
			"go.mod": goModFor("\t"+fixtureModule+" "+fixtureVersion+"\n",
				"\nreplace "+fixtureModule+" => ./"+target+"\n"),
			"go.sum": "", "README.md": "hello\n", "real/dep.go": "package dep\n",
		})
		if err := os.Symlink("real", filepath.Join(root, "linkdir")); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-qm", "link")
		command.Dir = root
		if err := exec.Command("git", "-C", root, "add", "linkdir").Run(); err != nil {
			t.Fatal(err)
		}
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("commit link: %v: %s", err, output)
		}
		result := resolveFixture(t, Options{Root: root, Module: fixtureModule})
		if result.Verified || result.Reason != "replace-path-not-committed" || len(result.Files) != 0 {
			t.Fatalf("%s: want replace-path-not-committed, got %+v", target, result)
		}
	}
}

// DSE-V0-002: a required version the committed go.mod excludes must abstain
// rather than being resolved and reported as pinned evidence.
func TestExcludedVersionAbstains(t *testing.T) {
	hash := expectedHash1(t, fixtureFiles, fixtureModule+"@"+fixtureVersion)
	root := newRepository(t, map[string]string{
		"go.mod": goModFor("\t"+fixtureModule+" "+fixtureVersion+"\n", "") +
			"\nexclude " + fixtureModule + " " + fixtureVersion + "\n",
		"go.sum": fixtureModule + " " + fixtureVersion + " " + hash + "\n",
	})
	result := resolveFixture(t, Options{Root: root, Module: fixtureModule, ModuleCache: newCache(t, fixtureFiles, hash)})
	if result.Verified || result.Reason != "excluded-version" {
		t.Fatalf("want excluded-version, got %+v", result)
	}
}

// DSE-V0-004: an escaped module path is joined onto the module cache root, so a
// replace target carrying a parent-directory element must be refused before any
// directory is read.
func TestCacheEscapingModulePathIsRefused(t *testing.T) {
	for _, escaping := range []string{"evil.example/../../etc", "evil.example//sub", ".hidden/dep"} {
		if _, err := escapeModulePath(escaping); err == nil {
			t.Fatalf("escapeModulePath(%q) was accepted", escaping)
		}
	}
	if escaped, err := escapeModulePath("example.com/Dep"); err != nil || escaped != "example.com/!dep" {
		t.Fatalf("escapeModulePath rejected a legitimate path: %q %v", escaped, err)
	}
}

// DSE-V0-007: a committed go.mod the restricted grammar cannot tokenize must
// abstain by name. Dropping the line instead would answer module-not-required,
// asserting an absence the reader never established.
func TestUnparsableGoModAbstains(t *testing.T) {
	for name, suffix := range map[string]string{
		"malformed-literal":  "\nrequire \"example.com/open\n",
		"unsupported-syntax": "\nrequire " + fixtureModule + " " + fixtureVersion + " extra\n",
		"invalid-utf8":       "\n// invalid " + string([]byte{0xff}) + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := newRepository(t, map[string]string{
				"go.mod": goModFor("\t"+fixtureModule+" "+fixtureVersion+"\n", "") + suffix,
				"go.sum": "",
			})
			result := resolveFixture(t, Options{Root: root, Module: fixtureModule})
			if result.Verified || result.Reason != "unparsable-go-mod" {
				t.Fatalf("want unparsable-go-mod, got %+v", result)
			}
		})
	}
}

// slowReader never reaches EOF and cancels its context on the first read.
type slowReader struct{ cancel context.CancelFunc }

func (reader slowReader) Read(content []byte) (int, error) {
	reader.cancel()
	content[0] = 'x'
	return 1, nil
}

func (slowReader) Close() error { return nil }

// DSE-V0-004: the streaming hash observes cancellation mid-file, and a
// cancellation during verification is a failure, never an abstention.
func TestHashObservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := hash1(ctx, []string{"m@v/endless"}, func(string) (io.ReadCloser, error) {
			return slowReader{cancel: cancel}, nil
		})
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("hash1 ignored cancellation of an endless reader")
	}

	hash := expectedHash1(t, fixtureFiles, fixtureModule+"@"+fixtureVersion)
	resolveCtx, resolveCancel := context.WithCancel(context.Background())
	defer resolveCancel()
	afterCacheInventory = resolveCancel
	defer func() { afterCacheInventory = func() {} }()
	result, err := Resolve(resolveCtx, Options{
		Root:        fixtureRepository(t, hash),
		Module:      fixtureModule,
		ModuleCache: newCache(t, fixtureFiles, hash),
	})
	if !errors.Is(err, context.Canceled) || result.Reason != "" {
		t.Fatalf("want a context.Canceled failure, got %+v, %v", result, err)
	}
}
