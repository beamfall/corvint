package companionrelease

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/procgroup"
)

const (
	installedStageOutputLimit = 1 << 20
	installedTotalOutputLimit = 8 << 20
	maxNPMCacheFiles          = 200_000
	maxNPMCacheBytes          = int64(4 << 30)
)

type InstalledOptions struct {
	BundleDirectory, Scratch, OutputPath, NPMCache, BrowserCache string
	SourceRoot                                                   string
	NodePath, PythonPath, ExpectedCommit, ExpectedTree           string
	ExpectedPythonSHA256                                         string
}

type InstalledReport struct {
	Profile      string              `json:"profile"`
	Status       string              `json:"status"`
	BundleSHA256 string              `json:"bundleSha256"`
	SourceCommit string              `json:"sourceCommit"`
	SourceTree   string              `json:"sourceTree"`
	NodeSHA256   string              `json:"nodeSha256"`
	PythonSHA256 string              `json:"pythonSha256"`
	EditorHost   json.RawMessage     `json:"editorHost"`
	Roadmap      json.RawMessage     `json:"roadmap"`
	Docs         []InstalledEvidence `json:"docs"`
	Stages       []InstalledStage    `json:"stages"`
}

type InstalledEvidence struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Raw    string `json:"raw"`
}

type InstalledStage struct {
	Name         string `json:"name"`
	OutputSHA256 string `json:"outputSha256"`
}

// RunInstalledQualification executes the installed browser/editor, automatic
// docs, and planning demonstrations only from a verified retained bundle and
// its retained Corvint source. It writes one result after every child and the
// owned scratch tree have been cleaned up.
func RunInstalledQualification(ctx context.Context, opts InstalledOptions) (report InstalledReport, err error) {
	if err := admitInstalledOptions(opts); err != nil {
		return report, err
	}
	verified, err := VerifyRetainedBundle(opts.BundleDirectory)
	if err != nil {
		return report, err
	}
	inventory, err := inventoryForManifest(verified.Manifest)
	if err != nil || inventory.core {
		return report, fmt.Errorf("editor qualification refuses core companion profile /2")
	}
	commit, tree, sourceMember, err := verified.CorvintSourceIdentity()
	if err != nil {
		return report, err
	}
	if commit != opts.ExpectedCommit || tree != opts.ExpectedTree {
		return report, fmt.Errorf("bundle source identity %s/%s differs from frozen %s/%s", commit, tree, opts.ExpectedCommit, opts.ExpectedTree)
	}
	report = InstalledReport{Profile: "corvint-public-release-installed/0", Status: "PASS", BundleSHA256: verified.ArchiveSHA256, SourceCommit: commit, SourceTree: tree}
	report.NodeSHA256, err = hashRegular(opts.NodePath)
	if err != nil {
		return report, err
	}
	report.PythonSHA256, err = hashRegular(opts.PythonPath)
	if err != nil {
		return report, err
	}
	if report.NodeSHA256 != verified.Manifest.VSIXTools.NodeSHA256 {
		return report, fmt.Errorf("Node digest differs from bundle toolchain")
	}
	if report.PythonSHA256 != opts.ExpectedPythonSHA256 {
		return report, fmt.Errorf("Python digest differs from frozen identity")
	}
	err = runInstalledInScratch(ctx, opts, verified, sourceMember, &report)
	if err != nil {
		return report, err
	}
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return report, err
	}
	body = append(body, '\n')
	if err := validateSerializedEvidence(body); err != nil {
		return report, err
	}
	if err := writeExclusiveAtomic(opts.OutputPath, body); err != nil {
		return report, err
	}
	return report, nil
}

func runInstalledInScratch(ctx context.Context, opts InstalledOptions, verified *VerifiedRetainedBundle, sourceMember string, report *InstalledReport) (err error) {
	inventory, err := inventoryForManifest(verified.Manifest)
	if err != nil {
		return err
	}
	if err := os.Mkdir(opts.Scratch, 0o700); err != nil {
		return err
	}
	defer func() { err = errorsJoin(err, os.RemoveAll(opts.Scratch)) }()
	bundleRoot, sourceRoot, cacheRoot, browserRoot := filepath.Join(opts.Scratch, "bundle"), filepath.Join(opts.Scratch, "corvint-source"), filepath.Join(opts.Scratch, "npm-cache"), filepath.Join(opts.Scratch, "home", "Library", "Caches", "ms-playwright")
	if err := verified.Extract(bundleRoot); err != nil {
		return err
	}
	if err := verified.ExtractSourceArchive(sourceMember, sourceRoot); err != nil {
		return err
	}
	if err := copyBoundedTree(ctx, opts.NPMCache, cacheRoot, false); err != nil {
		return fmt.Errorf("copy npm cache: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(browserRoot), 0o700); err != nil {
		return err
	}
	if err := copyBoundedTree(ctx, opts.BrowserCache, browserRoot, true); err != nil {
		return fmt.Errorf("copy Playwright browser cache: %w", err)
	}
	env := []string{"HOME=" + filepath.Join(opts.Scratch, "home"), "PATH=" + filepath.Dir(opts.NodePath) + ":/usr/bin:/bin:/usr/sbin:/sbin", "NPM_CONFIG_CACHE=" + cacheRoot, "NPM_CONFIG_OFFLINE=true", "NPM_CONFIG_AUDIT=false", "NPM_CONFIG_FUND=false", "PLAYWRIGHT_BROWSERS_PATH=" + browserRoot, "PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z"}
	npmCLI := filepath.Join(filepath.Dir(filepath.Dir(opts.NodePath)), "lib/node_modules/npm/bin/npm-cli.js")
	verifyRuntime := func() error {
		checks := []struct{ path, expected, name string }{{opts.NodePath, verified.Manifest.VSIXTools.NodeSHA256, "Node"}, {opts.PythonPath, opts.ExpectedPythonSHA256, "Python"}, {npmCLI, verified.Manifest.VSIXTools.NPMSHA256, "npm CLI"}}
		for _, check := range checks {
			got, hashErr := hashRegular(check.path)
			if hashErr != nil {
				return fmt.Errorf("hash %s: %w", check.name, hashErr)
			}
			if got != check.expected {
				return fmt.Errorf("%s differs from frozen toolchain", check.name)
			}
		}
		return nil
	}
	if err := verifyRuntime(); err != nil {
		return err
	}
	var total int
	var lastStdout []byte
	runExit := func(name, dir string, timeout time.Duration, expected int, argv ...string) error {
		if err := verifyRuntime(); err != nil {
			return err
		}
		obs := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: dir, Env: env, Timeout: timeout, ShutdownTimeout: 5 * time.Second, OutputLimit: installedStageOutputLimit})
		runtimeErr := verifyRuntime()
		if obs.Err != nil || !obs.ExitObserved || obs.ExitStatus != expected {
			return errorsJoin(fmt.Errorf("stage %s failed: %v exit=%d want=%d stderr=%s", name, obs.Err, obs.ExitStatus, expected, trimForError(obs.Stderr)), runtimeErr)
		}
		if runtimeErr != nil {
			return runtimeErr
		}
		combined := append(append([]byte(nil), obs.Stdout...), obs.Stderr...)
		lastStdout = append(lastStdout[:0], obs.Stdout...)
		total += len(combined)
		if total > installedTotalOutputLimit {
			return fmt.Errorf("qualification output exceeds %d bytes", installedTotalOutputLimit)
		}
		report.Stages = append(report.Stages, InstalledStage{Name: name, OutputSHA256: sha256Hex(combined)})
		return nil
	}
	run := func(name, dir string, timeout time.Duration, argv ...string) error {
		return runExit(name, dir, timeout, 0, argv...)
	}
	if err := run("retained-source-git-init", sourceRoot, 30*time.Second, "/usr/bin/git", "init", "-q"); err != nil {
		return err
	}
	if err := run("retained-source-git-add", sourceRoot, 30*time.Second, "/usr/bin/git", "add", "-A"); err != nil {
		return err
	}
	if err := run("retained-source-write-tree", sourceRoot, 30*time.Second, "/usr/bin/git", "write-tree"); err != nil {
		return err
	}
	if err := run("retained-source-tree", sourceRoot, 30*time.Second, "/usr/bin/git", "diff", "--cached", "--quiet", opts.ExpectedTree, "--"); err != nil {
		return fmt.Errorf("retained source files do not reconstruct frozen tree: %w", err)
	}
	if err := run("retained-source-commit", sourceRoot, 30*time.Second, "/usr/bin/git", "-c", "user.name=Corvint Qualification", "-c", "user.email=corvint@example.invalid", "commit", "-qm", "retained source qualification"); err != nil {
		return err
	}
	if err := run("extension-dependencies", filepath.Join(sourceRoot, "extensions/vscode"), 3*time.Minute, opts.NodePath, npmCLI, "ci", "--offline", "--ignore-scripts", "--no-audit", "--no-fund"); err != nil {
		return err
	}
	harness := filepath.Join(sourceRoot, "extensions/vscode/test/installed/run.mjs")
	env = append(env, "CORVINT_RELEASE_ARCHIVE="+verified.ArchivePath, "CORVINT_RELEASE_EXTRACTED_ROOT="+bundleRoot, "CORVINT_VSIX="+filepath.Join(bundleRoot, inventory.vsix()), "CORVINT_JS_PROVIDER="+filepath.Join(bundleRoot, inventory.binary("corvint-js-test-provider")), "CORVINT_GO_PROVIDER="+filepath.Join(bundleRoot, inventory.binary("corvint-go-test-provider")), "CORVINT_TEST_VALIDITY_MCP="+filepath.Join(bundleRoot, inventory.binary("corvint-test-validity-mcp")))
	if err := run("installed-vscode", sourceRoot, 8*time.Minute, opts.NodePath, harness); err != nil {
		return err
	}
	report.EditorHost, err = selectProfile(lastStdout, "corvint-interactive-alpha-installed/0")
	if err != nil {
		return err
	}
	if err := run("installed-vscode-interruption", sourceRoot, 4*time.Minute, opts.NodePath, harness, "--interrupt-witness"); err != nil {
		return err
	}
	docsOut := filepath.Join(opts.Scratch, "docs-proof")
	if err := os.Mkdir(docsOut, 0o700); err != nil {
		return err
	}
	docsDriver := filepath.Join(sourceRoot, "internal/docmaintain/testdata/watch_mcp_proof.py")
	if err := run("automatic-docs-mcp", sourceRoot, 3*time.Minute, opts.PythonPath, docsDriver, "--corvint", filepath.Join(bundleRoot, inventory.binary("corvint")), "--docs-mcp", filepath.Join(bundleRoot, inventory.binary("corvint-docs-mcp")), "--output-root", docsOut); err != nil {
		return err
	}
	docsFixtures, err := os.ReadDir(docsOut)
	if err != nil || len(docsFixtures) != 1 || !docsFixtures[0].IsDir() || docsFixtures[0].Type()&os.ModeSymlink != 0 {
		return fmt.Errorf("automatic docs proof did not retain one real fixture directory")
	}
	docsFixture := filepath.Join(docsOut, docsFixtures[0].Name())
	for _, name := range []string{"watch-receipt.json", "mcp-discover.json", "mcp-list.json", "mcp-draft.json", "mcp-consume.json"} {
		body, readErr := readBoundedRegular(filepath.Join(docsFixture, name), installedStageOutputLimit)
		if readErr != nil {
			return fmt.Errorf("retain docs evidence %s: %w", name, readErr)
		}
		if _, parseErr := wire.Parse(body); parseErr != nil {
			return fmt.Errorf("docs evidence %s: %w", name, parseErr)
		}
		if bytes.Contains(body, []byte(opts.Scratch)) {
			return fmt.Errorf("docs evidence %s exposes scratch path", name)
		}
		report.Docs = append(report.Docs, InstalledEvidence{Name: name, SHA256: sha256Hex(body), Raw: string(body)})
	}
	planning := filepath.Join(opts.Scratch, "planning")
	seed := filepath.Join(sourceRoot, "script/seed-planning-store.sh")
	if err := run("planning-seed", sourceRoot, 2*time.Minute, "/bin/sh", seed, filepath.Join(bundleRoot, inventory.binary("atm")), planning); err != nil {
		return err
	}
	if err := run("planning-git-init", planning, 30*time.Second, "/usr/bin/git", "init", "-q"); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(planning, "README.md"), []byte("# Disposable Corvint release planning fixture\n"), 0o600); err != nil {
		return err
	}
	if err := run("planning-git-commit", planning, 30*time.Second, "/usr/bin/git", "-c", "user.name=Corvint Qualification", "-c", "user.email=corvint@example.invalid", "add", "README.md"); err != nil {
		return err
	}
	if err := run("planning-git-freeze", planning, 30*time.Second, "/usr/bin/git", "-c", "user.name=Corvint Qualification", "-c", "user.email=corvint@example.invalid", "commit", "-qm", "qualification fixture"); err != nil {
		return err
	}
	before, err := hashDirectory(filepath.Join(planning, ".git", "taskman"))
	if err != nil {
		return err
	}
	if err := run("planning-seed-replay", sourceRoot, 2*time.Minute, "/bin/sh", seed, filepath.Join(bundleRoot, inventory.binary("atm")), planning); err != nil {
		return err
	}
	after, err := hashDirectory(filepath.Join(planning, ".git", "taskman"))
	if err != nil {
		return err
	}
	if before != after {
		return fmt.Errorf("idempotent planning replay changed taskman store")
	}
	fixture := filepath.Join(sourceRoot, "conformance/interactive-alpha/fixture")
	if err := run("browser-dependencies", fixture, 3*time.Minute, opts.NodePath, npmCLI, "ci", "--offline", "--ignore-scripts", "--no-audit", "--no-fund"); err != nil {
		return err
	}
	if err := run("roadmap-browser", sourceRoot, 2*time.Minute, opts.NodePath, filepath.Join(sourceRoot, "conformance/interactive-alpha/roadmap-proof.mjs"), filepath.Join(bundleRoot, inventory.binary("corvint-console")), filepath.Join(bundleRoot, inventory.binary("atm")), filepath.Join(bundleRoot, inventory.binary("corvint-dashboard-snapshot")), planning, sourceRoot, fixture); err != nil {
		return err
	}
	report.Roadmap, err = selectProfile(lastStdout, "corvint-installed-roadmap-proof/0")
	if err != nil {
		return err
	}
	if err := verifyRuntime(); err != nil {
		return err
	}
	return rehashRetained(verified)
}

func validateSerializedEvidence(body []byte) error {
	var decoded struct {
		Docs []InstalledEvidence `json:"docs"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return err
	}
	for _, evidence := range decoded.Docs {
		if sha256Hex([]byte(evidence.Raw)) != evidence.SHA256 {
			return fmt.Errorf("serialized docs evidence %s differs from retained digest", evidence.Name)
		}
	}
	return nil
}

func selectProfile(output []byte, profile string) (json.RawMessage, error) {
	var selected []byte
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if _, err := wire.Parse(line); err != nil {
			continue
		}
		var header struct {
			Profile string `json:"profile"`
			Status  string `json:"status"`
		}
		if err := json.Unmarshal(line, &header); err == nil && header.Profile == profile && header.Status == "PASS" {
			if selected != nil {
				return nil, fmt.Errorf("stage emitted duplicate %s records", profile)
			}
			selected = append([]byte(nil), line...)
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("stage omitted required %s PASS record", profile)
	}
	return json.RawMessage(selected), nil
}

func admitInstalledOptions(opts InstalledOptions) error {
	return admitInstalledOptionsForProfile(opts, false)
}

func admitInstalledOptionsForProfile(opts InstalledOptions, core bool) error {
	for name, value := range map[string]string{"source": opts.SourceRoot, "bundle": opts.BundleDirectory, "scratch": opts.Scratch, "output": opts.OutputPath, "npm cache": opts.NPMCache, "browser cache": opts.BrowserCache, "node": opts.NodePath, "python": opts.PythonPath} {
		if !filepath.IsAbs(value) || filepath.Clean(value) != value {
			return fmt.Errorf("%s path must be absolute and normalized", name)
		}
	}
	if (len(opts.ExpectedCommit) != 40 && (!core || len(opts.ExpectedCommit) != 64)) || (len(opts.ExpectedTree) != 40 && (!core || len(opts.ExpectedTree) != 64)) {
		return fmt.Errorf("expected commit and tree must be full SHA-1 identities")
	}
	if len(opts.ExpectedPythonSHA256) != 64 {
		return fmt.Errorf("expected Python SHA-256 must be 64 hex characters")
	}
	if _, err := os.Lstat(opts.OutputPath); err == nil {
		return fmt.Errorf("refusing to overwrite result %s", opts.OutputPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(opts.OutputPath))
	if err != nil {
		return fmt.Errorf("result parent: %w", err)
	}
	if info, err := os.Lstat(parent); err != nil || !info.IsDir() {
		return fmt.Errorf("result parent is not a real directory")
	}
	if parent != filepath.Dir(opts.OutputPath) {
		return fmt.Errorf("result parent path must be canonical")
	}
	if _, err := os.Lstat(opts.Scratch); err == nil {
		return fmt.Errorf("scratch already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	inputs := make(map[string]string)
	for name, path := range map[string]string{"source": opts.SourceRoot, "bundle": opts.BundleDirectory, "npm cache": opts.NPMCache, "browser cache": opts.BrowserCache} {
		resolved, resolveErr := filepath.EvalSymlinks(path)
		if resolveErr != nil {
			return fmt.Errorf("resolve %s: %w", name, resolveErr)
		}
		if resolved != path {
			return fmt.Errorf("%s path must be canonical", name)
		}
		inputs[name] = resolved
	}
	scratch, err := resolveMissingPath(opts.Scratch)
	if err != nil {
		return fmt.Errorf("scratch: %w", err)
	}
	if scratch != opts.Scratch {
		return fmt.Errorf("scratch path must be canonical")
	}
	output := filepath.Join(parent, filepath.Base(opts.OutputPath))
	for name, input := range inputs {
		if pathsOverlap(scratch, input) {
			return fmt.Errorf("scratch overlaps %s", name)
		}
		if pathsOverlap(output, input) {
			return fmt.Errorf("result overlaps %s", name)
		}
	}
	if pathsOverlap(scratch, output) {
		return fmt.Errorf("scratch overlaps result")
	}
	for _, path := range []string{opts.NodePath, opts.PythonPath} {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || resolved != path {
			return fmt.Errorf("executable path must be canonical: %s", path)
		}
		if _, err := hashRegular(path); err != nil {
			return err
		}
	}
	return nil
}

func resolveMissingPath(path string) (string, error) {
	clean := filepath.Clean(path)
	ancestor := filepath.Dir(clean)
	parts := []string{filepath.Base(clean)}
	for {
		resolved, err := filepath.EvalSymlinks(ancestor)
		if err == nil {
			return filepath.Join(append([]string{resolved}, parts...)...), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", err
		}
		parts = append([]string{filepath.Base(ancestor)}, parts...)
		ancestor = parent
	}
}

func pathsOverlap(a, b string) bool { return pathWithin(a, b) || pathWithin(b, a) }
func pathWithin(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func writeExclusiveAtomic(path string, body []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	if _, err = f.Write(body); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Link(tmp, path); err != nil {
		return fmt.Errorf("publish result without overwrite: %w", err)
	}
	if err = os.Remove(tmp); err != nil {
		_ = os.Remove(path)
		return err
	}
	ok = true
	return nil
}

func copyBoundedTree(ctx context.Context, source, target string, allowInternalSymlinks bool) error {
	resolved, err := filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	if info, statErr := os.Lstat(resolved); statErr != nil || !info.IsDir() {
		return fmt.Errorf("npm cache is not a real directory")
	}
	if err = os.Mkdir(target, 0o700); err != nil {
		return err
	}
	count := 0
	var total int64
	return filepath.WalkDir(resolved, func(path string, d os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if path == resolved {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if !allowInternalSymlinks {
				return fmt.Errorf("cache contains symlink %s", path)
			}
			link, err := os.Readlink(path)
			if err != nil || filepath.IsAbs(link) {
				return fmt.Errorf("cache contains invalid symlink %s", path)
			}
			resolvedTarget := filepath.Clean(filepath.Join(filepath.Dir(path), link))
			inside, err := filepath.Rel(resolved, resolvedTarget)
			if err != nil || inside == ".." || filepath.IsAbs(inside) || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
				return fmt.Errorf("cache symlink escapes source: %s", path)
			}
			rel, err := filepath.Rel(resolved, path)
			if err != nil {
				return err
			}
			count++
			total += int64(len(link))
			if count > maxNPMCacheFiles || total > maxNPMCacheBytes {
				return fmt.Errorf("cache exceeds bounds")
			}
			return os.Symlink(link, filepath.Join(target, rel))
		}
		rel, err := filepath.Rel(resolved, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(target, rel)
		if d.IsDir() {
			return os.Mkdir(dst, 0o700)
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("npm cache member is not regular: %s", path)
		}
		count++
		total += info.Size()
		if count > maxNPMCacheFiles || total > maxNPMCacheBytes {
			return fmt.Errorf("npm cache exceeds bounds")
		}
		in, err := openCopyRegular(path, info)
		if err != nil {
			return err
		}
		mode := os.FileMode(0o600)
		if info.Mode().Perm()&0o111 != 0 {
			mode = 0o700
		}
		out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			_ = in.Close()
			return err
		}
		copyErr := copyExactContext(ctx, out, in, info.Size())
		inErr := in.Close()
		closeErr := out.Close()
		return errorsJoin(errorsJoin(copyErr, inErr), closeErr)
	})
}

func copyExactContext(ctx context.Context, dst io.Writer, src io.Reader, size int64) error {
	remaining := size
	buf := make([]byte, 64<<10)
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		want := int64(len(buf))
		if remaining < want {
			want = remaining
		}
		n, err := src.Read(buf[:want])
		if n > 0 {
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			remaining -= int64(n)
		}
		if err != nil {
			if err == io.EOF && remaining == 0 {
				break
			}
			return fmt.Errorf("cache member changed during copy: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("cache member made no copy progress")
		}
	}
	var extra [1]byte
	if n, err := src.Read(extra[:]); n != 0 || (err != nil && err != io.EOF) {
		return fmt.Errorf("cache member grew during copy")
	}
	return nil
}

func hashRegular(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("not a regular file: %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func hashDirectory(root string) (string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in state")
		}
		if !d.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		rel, _ := filepath.Rel(root, path)
		sum, err := hashRegular(path)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%s\n", filepath.ToSlash(rel), sum)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func rehashRetained(v *VerifiedRetainedBundle) error {
	archive, err := hashRegular(v.ArchivePath)
	if err != nil {
		return err
	}
	if archive != v.ArchiveSHA256 {
		return fmt.Errorf("retained archive drifted during qualification")
	}
	checksum, err := hashRegular(v.ChecksumPath)
	if err != nil || checksum != v.ChecksumSHA256 {
		return fmt.Errorf("retained checksum sidecar drifted during qualification")
	}
	smoke, err := hashRegular(v.SmokePath)
	if err != nil || smoke != v.SmokeSHA256 {
		return fmt.Errorf("retained smoke sidecar drifted during qualification")
	}
	again, err := VerifyRetainedBundle(v.Directory)
	if err != nil {
		return err
	}
	if again.ArchiveSHA256 != v.ArchiveSHA256 || again.ChecksumSHA256 != v.ChecksumSHA256 || again.SmokeSHA256 != v.SmokeSHA256 {
		return fmt.Errorf("retained sidecars drifted during qualification")
	}
	return nil
}
