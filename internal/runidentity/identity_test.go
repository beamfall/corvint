package runidentity

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

func TestMain(m *testing.M) {
	testsupport.IsolateGit()
	os.Exit(m.Run())
}

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	prefix := []string{"-c", "maintenance.autoDetach=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false", "-C", root}
	out, err := exec.Command("git", append(prefix, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// repo is a committed test repository: a config under e2e/, specs under e2e/tests, support code
// and a lockfile.
func repo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	git(t, root, "init", "-q", "-b", "main")
	write(t, root, map[string]string{
		"e2e/playwright.config.ts":           "export default { testDir: './tests' }\n",
		"e2e/tests/cart.spec.ts":             "test('adds item', () => {})\n",
		"e2e/tests/sub/search.test.js":       "test('search', () => {})\n",
		"e2e/tests/helpers.ts":               "export const x = 1\n",
		"e2e/pages/login.ts":                 "export class Login {}\n",
		"package-lock.json":                  "{}\n",
		".gitignore":                         "test-results/\n",
		"e2e/tests/node_modules/x/a.spec.ts": "ignored\n",
	})
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "init")
	return root
}

func produce(t *testing.T, opts Options) Document {
	t.Helper()
	doc, err := Produce(context.Background(), opts)
	if err != nil {
		t.Fatalf("Produce(%+v): %v", opts, err)
	}
	return doc
}

func lsBlob(t *testing.T, root, path string) string {
	t.Helper()
	return strings.Fields(git(t, root, "ls-files", "-s", "--", path))[1]
}

func TestPWRV0047_TestFileBlobsKeyedAsReportFile(t *testing.T) {
	root := repo(t)
	config := filepath.Join(root, "e2e", "playwright.config.ts")
	doc := produce(t, Options{Config: config, TestDir: filepath.Join(root, "e2e", "tests")})
	want := map[string]string{
		"cart.spec.ts":       lsBlob(t, root, "e2e/tests/cart.spec.ts"),
		"sub/search.test.js": lsBlob(t, root, "e2e/tests/sub/search.test.js"),
	}
	id := doc.CorvintRunIdentity
	if !reflect.DeepEqual(id.TestFileBlobs, want) {
		t.Fatalf("testFileBlobs = %v, want %v", id.TestFileBlobs, want)
	}
	head := git(t, root, "rev-parse", "HEAD")
	if id.TestRevision != head || id.ApplicationRevision != head || id.TestRootPath != "e2e/tests" || !strings.HasPrefix(id.ConfigDigest, "sha256:") {
		t.Fatalf("identity = %+v", id)
	}
	if doc.Profile != Profile || doc.Config != "e2e/playwright.config.ts" || len(doc.Limits) != 0 || doc.Counts.TestFiles != 2 || doc.Counts.DigestFiles != 6 {
		t.Fatalf("document = %+v", doc)
	}
	// testDir defaults to the config's directory, as Playwright's does.
	if d := produce(t, Options{Config: config}); d.CorvintRunIdentity.TestRootPath != "e2e" || d.CorvintRunIdentity.TestFileBlobs["tests/cart.spec.ts"] != want["cart.spec.ts"] {
		t.Fatalf("default testDir = %+v", d.CorvintRunIdentity)
	}
	// A custom testMatch selects other files.
	if d := produce(t, Options{Config: config, TestDir: filepath.Join(root, "e2e", "tests"), TestMatch: `^helpers\.ts$`}); !reflect.DeepEqual(d.CorvintRunIdentity.TestFileBlobs, map[string]string{"helpers.ts": lsBlob(t, root, "e2e/tests/helpers.ts")}) {
		t.Fatalf("custom testMatch = %v", d.CorvintRunIdentity.TestFileBlobs)
	}
}

func TestPWRV0047_WorkingTreeBlobsAndUntrackedSpecs(t *testing.T) {
	root := repo(t)
	opts := Options{Config: filepath.Join(root, "e2e", "playwright.config.ts"), TestDir: filepath.Join(root, "e2e", "tests")}
	clean := produce(t, opts)
	write(t, root, map[string]string{"e2e/tests/cart.spec.ts": "test('adds two', () => {})\n", "e2e/tests/new.spec.ts": "test('new', () => {})\n"})
	doc := produce(t, opts)
	blobs := doc.CorvintRunIdentity.TestFileBlobs
	if blobs["cart.spec.ts"] != git(t, root, "hash-object", "e2e/tests/cart.spec.ts") || blobs["cart.spec.ts"] == clean.CorvintRunIdentity.TestFileBlobs["cart.spec.ts"] {
		t.Fatalf("modified spec blob = %q", blobs["cart.spec.ts"])
	}
	if blobs["new.spec.ts"] != git(t, root, "hash-object", "e2e/tests/new.spec.ts") || doc.Counts.ModifiedTestFiles != 1 {
		t.Fatalf("untracked spec = %+v", doc)
	}
	if doc.CorvintRunIdentity.ConfigDigest != clean.CorvintRunIdentity.ConfigDigest {
		t.Fatal("a spec change moved configDigest")
	}
	// A deleted spec is not declared.
	if err := os.Remove(filepath.Join(root, "e2e", "tests", "sub", "search.test.js")); err != nil {
		t.Fatal(err)
	}
	if _, ok := produce(t, opts).CorvintRunIdentity.TestFileBlobs["sub/search.test.js"]; ok {
		t.Fatal("deleted spec still declared")
	}
}

func TestPWRV0048_ConfigDigestCoversTrackedSupportCode(t *testing.T) {
	root := repo(t)
	opts := Options{Config: filepath.Join(root, "e2e", "playwright.config.ts"), TestDir: filepath.Join(root, "e2e", "tests")}
	clean := produce(t, opts).CorvintRunIdentity.ConfigDigest
	if again := produce(t, opts).CorvintRunIdentity.ConfigDigest; again != clean {
		t.Fatalf("digest not stable: %s then %s", clean, again)
	}
	for _, name := range []string{"e2e/pages/login.ts", "package-lock.json", "e2e/playwright.config.ts", "e2e/tests/helpers.ts"} {
		before, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		write(t, root, map[string]string{name: string(before) + "// changed\n"})
		doc := produce(t, opts)
		if doc.CorvintRunIdentity.ConfigDigest == clean || doc.Counts.ModifiedDigested != 1 {
			t.Fatalf("%s: uncommitted change kept digest %s (%+v)", name, clean, doc.Counts)
		}
		write(t, root, map[string]string{name: string(before)})
	}
	// An untracked non-test file is outside the digest and named as a limit; an ignored one is not.
	write(t, root, map[string]string{"e2e/pages/extra.ts": "export {}\n", "test-results/out.json": "{}\n"})
	doc := produce(t, opts)
	if doc.CorvintRunIdentity.ConfigDigest != clean || !reflect.DeepEqual(doc.Limits, []string{LimitUntrackedOutsideDigest}) || doc.Counts.UntrackedOutside != 1 {
		t.Fatalf("untracked = %+v", doc)
	}
	// A committed support change moves the digest too.
	write(t, root, map[string]string{"e2e/pages/login.ts": "export class Login { go() {} }\n"})
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "support")
	if produce(t, opts).CorvintRunIdentity.ConfigDigest == clean {
		t.Fatal("committed support change kept the digest")
	}
}

func TestPWRV0049_ApplicationRevisionFromNamedCheckout(t *testing.T) {
	root := repo(t)
	app := t.TempDir()
	git(t, app, "init", "-q", "-b", "main")
	write(t, app, map[string]string{"server.go": "package main\n"})
	git(t, app, "add", "-A")
	git(t, app, "commit", "-q", "-m", "app")
	opts := Options{Config: filepath.Join(root, "e2e", "playwright.config.ts"), AppRoot: app}
	doc := produce(t, opts)
	appHead := git(t, app, "rev-parse", "HEAD")
	if doc.CorvintRunIdentity.ApplicationRevision != appHead || doc.CorvintRunIdentity.TestRevision != git(t, root, "rev-parse", "HEAD") {
		t.Fatalf("identity = %+v", doc.CorvintRunIdentity)
	}
	write(t, app, map[string]string{"server.go": "package main\n\nfunc main() {}\n"})
	dirty := produce(t, opts)
	rev := dirty.CorvintRunIdentity.ApplicationRevision
	if !strings.HasPrefix(rev, appHead+worktreeSuffix) || len(rev) != len(appHead)+len(worktreeSuffix)+64 || !reflect.DeepEqual(dirty.Limits, []string{LimitApplicationWorktreeDirty}) {
		t.Fatalf("dirty application = %+v", dirty)
	}
	write(t, app, map[string]string{"server.go": "package main\n\nfunc main() { panic(1) }\n"})
	if other := produce(t, opts).CorvintRunIdentity.ApplicationRevision; other == rev {
		t.Fatal("a different uncommitted application change kept the revision")
	}
}

func TestPWRV0050_RefusalsAndReadOnly(t *testing.T) {
	root := repo(t)
	config := filepath.Join(root, "e2e", "playwright.config.ts")
	elsewhere := t.TempDir()
	cases := []struct {
		name   string
		opts   Options
		reason string
	}{
		{"missing config", Options{Config: filepath.Join(root, "nope.ts")}, ReasonConfigUnreadable},
		{"not a repository", Options{Config: writeLoose(t, elsewhere)}, ReasonNotRepository},
		{"test dir outside", Options{Config: config, TestDir: elsewhere}, ReasonTestDirOutside},
		{"bad regexp", Options{Config: config, TestMatch: "("}, ReasonTestMatchInvalid},
		{"no test files", Options{Config: config, TestMatch: `\.nothing$`}, ReasonNoTestFiles},
		{"app not a checkout", Options{Config: config, AppRoot: elsewhere}, ReasonApplicationUnresolved},
	}
	for _, c := range cases {
		_, err := Produce(context.Background(), c.opts)
		var coded *Error
		if !errors.As(err, &coded) || coded.Reason != c.reason {
			t.Fatalf("%s: err = %v, want %s", c.name, err, c.reason)
		}
	}
	empty := t.TempDir()
	git(t, empty, "init", "-q")
	write(t, empty, map[string]string{"playwright.config.ts": "x\n", "a.spec.ts": "x\n"})
	var coded *Error
	if _, err := Produce(context.Background(), Options{Config: filepath.Join(empty, "playwright.config.ts")}); !errors.As(err, &coded) || coded.Reason != ReasonRevisionUnresolved {
		t.Fatalf("unborn HEAD: %v", err)
	}
	// Reads write nothing: the index and the status are byte-identical afterwards.
	write(t, root, map[string]string{"e2e/pages/login.ts": "changed\n"})
	index := filepath.Join(root, ".git", "index")
	status := git(t, root, "status", "--porcelain") // may refresh the index; snapshot after it
	before, _ := os.ReadFile(index)
	stat, _ := os.Stat(index)
	produce(t, Options{Config: config})
	after, _ := os.ReadFile(index)
	stat2, _ := os.Stat(index)
	if string(before) != string(after) || !stat.ModTime().Equal(stat2.ModTime()) || git(t, root, "status", "--porcelain") != status {
		t.Fatal("Produce changed the repository")
	}
}

func writeLoose(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "playwright.config.ts")
	if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPWRV0048_ConfigBoundIntoDigest(t *testing.T) {
	root := repo(t)
	write(t, root, map[string]string{"e2e/smoke.config.ts": "export default { testDir: './tests' }\n"})
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "second config")
	testDir := filepath.Join(root, "e2e", "tests")
	a := produce(t, Options{Config: filepath.Join(root, "e2e", "playwright.config.ts"), TestDir: testDir})
	b := produce(t, Options{Config: filepath.Join(root, "e2e", "smoke.config.ts"), TestDir: testDir})
	if a.CorvintRunIdentity.ConfigDigest == b.CorvintRunIdentity.ConfigDigest {
		t.Fatal("two configs over one tree share a configDigest")
	}
}

func TestPWRV0050_CheckoutValidation(t *testing.T) {
	refused := func(t *testing.T, opts Options, reason string) {
		t.Helper()
		var coded *Error
		if _, err := Produce(context.Background(), opts); !errors.As(err, &coded) || coded.Reason != reason {
			t.Fatalf("err = %v, want %s", err, reason)
		}
	}
	for _, flag := range []string{"--assume-unchanged", "--skip-worktree"} {
		t.Run(flag, func(t *testing.T) {
			root := repo(t)
			git(t, root, "update-index", flag, "e2e/pages/login.ts")
			refused(t, Options{Config: filepath.Join(root, "e2e", "playwright.config.ts")}, ReasonIndexFlags)
		})
	}
	for _, key := range []string{"filter.lfs.clean", "filter.x.process"} {
		t.Run(key, func(t *testing.T) {
			root := repo(t)
			git(t, root, "config", key, "false")
			refused(t, Options{Config: filepath.Join(root, "e2e", "playwright.config.ts")}, ReasonFilterConfigured)
		})
	}
	t.Run("application checkout", func(t *testing.T) {
		root := repo(t)
		config := filepath.Join(root, "e2e", "playwright.config.ts")
		app := t.TempDir()
		git(t, app, "init", "-q", "-b", "main")
		lib := filepath.Join(app, "lib")
		if err := os.MkdirAll(lib, 0o755); err != nil {
			t.Fatal(err)
		}
		git(t, lib, "init", "-q", "-b", "main")
		write(t, lib, map[string]string{"a.go": "package lib\n"})
		git(t, lib, "add", "-A")
		git(t, lib, "commit", "-q", "-m", "lib")
		git(t, app, "update-index", "--add", "--cacheinfo", "160000,"+git(t, lib, "rev-parse", "HEAD")+",lib")
		write(t, app, map[string]string{"server.go": "package main\n"})
		git(t, app, "add", "server.go")
		git(t, app, "commit", "-q", "-m", "app")
		produce(t, Options{Config: config, AppRoot: app})
		write(t, lib, map[string]string{"a.go": "package lib // edited\n"})
		refused(t, Options{Config: config, AppRoot: app}, ReasonSubmoduleModified)
		write(t, lib, map[string]string{"a.go": "package lib\n"})
		git(t, app, "update-index", "--assume-unchanged", "server.go")
		refused(t, Options{Config: config, AppRoot: app}, ReasonIndexFlags)
	})
}

// setupRepo mirrors GitHub #1866: top-level testDir ./tests and a setup project at ./setup whose
// testMatch is /.*\.setup\.ts/.
func setupRepo(t *testing.T) (string, Options) {
	t.Helper()
	root := repo(t)
	write(t, root, map[string]string{
		"e2e/setup/global.setup.ts": "setup('seed', () => {})\n",
		"e2e/setup/auth.setup.ts":   "setup('login', () => {})\n",
		"e2e/setup/helpers.ts":      "export const seed = 1\n",
	})
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "setup")
	return root, Options{Config: filepath.Join(root, "e2e", "playwright.config.ts"), TestDir: filepath.Join(root, "e2e", "tests"),
		ProjectTestDirs: []ProjectTestDir{{Dir: filepath.Join(root, "e2e", "setup"), TestMatch: `\.setup\.ts$`}}}
}

func TestPWRV0052_ProjectTestDirKeyedAsReportFile(t *testing.T) {
	root, opts := setupRepo(t)
	without := produce(t, Options{Config: opts.Config, TestDir: opts.TestDir})
	doc := produce(t, opts)
	blobs := doc.CorvintRunIdentity.TestFileBlobs
	want := map[string]string{
		"cart.spec.ts":             lsBlob(t, root, "e2e/tests/cart.spec.ts"),
		"sub/search.test.js":       lsBlob(t, root, "e2e/tests/sub/search.test.js"),
		"../setup/global.setup.ts": lsBlob(t, root, "e2e/setup/global.setup.ts"),
		"../setup/auth.setup.ts":   lsBlob(t, root, "e2e/setup/auth.setup.ts"),
	}
	if !reflect.DeepEqual(blobs, want) || len(without.CorvintRunIdentity.TestFileBlobs) != 2 {
		t.Fatalf("testFileBlobs = %v, want %v (without project dir: %v)", blobs, want, without.CorvintRunIdentity.TestFileBlobs)
	}
	wantDirs := []*ProjectTestDirReport{{Dir: "e2e/setup", TestMatch: `\.setup\.ts$`, Status: "RESOLVED", TestFiles: 2}}
	if !reflect.DeepEqual(doc.ProjectTestDirs, wantDirs) || len(doc.Limits) != 0 || doc.Counts.TestFiles != 4 {
		t.Fatalf("document = %+v dirs=%+v", doc, doc.ProjectTestDirs)
	}
	// Setup test files leave the digest (they pool through their blobs); their helpers stay in it.
	if doc.Counts.DigestFiles != without.Counts.DigestFiles-2 {
		t.Fatalf("digest files = %d, without = %d", doc.Counts.DigestFiles, without.Counts.DigestFiles)
	}
	write(t, root, map[string]string{"e2e/setup/global.setup.ts": "setup('seed two', () => {})\n", "e2e/setup/new.setup.ts": "setup('new', () => {})\n"})
	edited := produce(t, opts)
	eb := edited.CorvintRunIdentity.TestFileBlobs
	if eb["../setup/global.setup.ts"] != git(t, root, "hash-object", "e2e/setup/global.setup.ts") || eb["../setup/new.setup.ts"] != git(t, root, "hash-object", "e2e/setup/new.setup.ts") ||
		edited.CorvintRunIdentity.ConfigDigest != doc.CorvintRunIdentity.ConfigDigest || edited.Counts.ModifiedTestFiles != 1 {
		t.Fatalf("edited setup = %+v", edited)
	}
	// REGEXP defaults to the top-level testMatch; a project dir inside testDir keeps its plain keys.
	inherit := produce(t, Options{Config: opts.Config, TestDir: opts.TestDir, ProjectTestDirs: []ProjectTestDir{{Dir: filepath.Join(root, "e2e", "tests", "sub")}}})
	if !reflect.DeepEqual(inherit.CorvintRunIdentity.TestFileBlobs, without.CorvintRunIdentity.TestFileBlobs) || inherit.ProjectTestDirs[0].TestFiles != 1 || inherit.ProjectTestDirs[0].TestMatch != DefaultTestMatch {
		t.Fatalf("inherited = %+v %+v", inherit.CorvintRunIdentity.TestFileBlobs, inherit.ProjectTestDirs[0])
	}
}

func TestPWRV0053_ProjectTestDirUnresolvedOrOutside(t *testing.T) {
	root, opts := setupRepo(t)
	base := Options{Config: opts.Config, TestDir: opts.TestDir}
	with := func(dirs ...ProjectTestDir) Options {
		o := base
		o.ProjectTestDirs = dirs
		return o
	}
	// A directory that does not exist is never guessed at: it is named under a limit.
	doc := produce(t, with(ProjectTestDir{Dir: filepath.Join(root, "e2e", "missing")}, opts.ProjectTestDirs[0]))
	if len(doc.CorvintRunIdentity.TestFileBlobs) != 4 || !reflect.DeepEqual(doc.Limits, []string{LimitProjectTestDirUnresolved}) ||
		doc.ProjectTestDirs[0].Status != "UNRESOLVED" || doc.ProjectTestDirs[0].Reason != LimitProjectTestDirUnresolved || doc.ProjectTestDirs[0].Dir != "e2e/missing" {
		t.Fatalf("missing dir = %+v %+v", doc.Limits, doc.ProjectTestDirs[0])
	}
	if doc := produce(t, with(ProjectTestDir{Dir: filepath.Join(root, "e2e", "setup", "helpers.ts")})); doc.ProjectTestDirs[0].Reason != LimitProjectTestDirUnresolved {
		t.Fatalf("file as dir = %+v", doc.ProjectTestDirs[0])
	}
	if doc := produce(t, with(ProjectTestDir{Dir: filepath.Join(root, "e2e", "setup"), TestMatch: `\.nothing$`})); !reflect.DeepEqual(doc.Limits, []string{LimitProjectTestDirNoTests}) || doc.ProjectTestDirs[0].Status != "UNRESOLVED" {
		t.Fatalf("no tests = %+v %+v", doc.Limits, doc.ProjectTestDirs[0])
	}
	var coded *Error
	if _, err := Produce(context.Background(), with(ProjectTestDir{Dir: filepath.Join(root, "e2e", "setup"), TestMatch: "("})); !errors.As(err, &coded) || coded.Reason != ReasonTestMatchInvalid {
		t.Errorf("invalid project testMatch: err = %v", err)
	}
	outside := t.TempDir()
	write(t, outside, map[string]string{"x.setup.ts": "secret\n"})
	dirs := []ProjectTestDir{{Dir: outside}, {Dir: filepath.Join(root, "..")}}
	if err := os.Symlink(outside, filepath.Join(root, "e2e", "linked")); err == nil {
		dirs = append(dirs, ProjectTestDir{Dir: filepath.Join(root, "e2e", "linked")})
	}
	if err := os.Symlink(filepath.Join(outside, "missing"), filepath.Join(root, "e2e", "dangling")); err == nil {
		dirs = append(dirs, ProjectTestDir{Dir: filepath.Join(root, "e2e", "dangling")}, ProjectTestDir{Dir: filepath.Join(root, "e2e", "dangling", "deeper")})
	}
	for _, dir := range dirs {
		if _, err := Produce(context.Background(), with(dir)); !errors.As(err, &coded) || coded.Reason != ReasonProjectTestDirOutside {
			t.Errorf("%s: err = %v", dir.Dir, err)
		}
	}
}
