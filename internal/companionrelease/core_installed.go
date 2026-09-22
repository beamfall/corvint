package companionrelease

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/procgroup"
)

const coreInstalledProfile = "corvint-public-release-core-installed/0"

var coreDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)
var coreGitID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

type CoreInstalledOptions struct {
	InstalledOptions
	ExpectedNodeSHA256, ExpectedNPMSHA256      string
	GoAuthorityPath, ExpectedGoAuthoritySHA256 string
}
type CoreInstalledStage struct {
	Name         string `json:"name"`
	StdoutSHA256 string `json:"stdoutSha256"`
	StderrSHA256 string `json:"stderrSha256"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
}
type CoreInstalledReport struct {
	Profile      string               `json:"profile"`
	Status       string               `json:"status"`
	BundleSHA256 string               `json:"bundleSha256"`
	SourceCommit string               `json:"sourceCommit"`
	SourceTree   string               `json:"sourceTree"`
	NodeSHA256   string               `json:"nodeSha256"`
	NPMSHA256    string               `json:"npmSha256"`
	PythonSHA256 string               `json:"pythonSha256"`
	Identity     InstalledEvidence    `json:"identity"`
	Providers    []InstalledEvidence  `json:"providers"`
	Console      []InstalledEvidence  `json:"console"`
	Roadmap      []InstalledEvidence  `json:"roadmap"`
	Docs         []InstalledEvidence  `json:"docs"`
	Stages       []CoreInstalledStage `json:"stages"`
}
type coreNamedDigest struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}
type coreRuntime struct {
	Name    string `json:"name"`
	SHA256  string `json:"sha256"`
	Version string `json:"version"`
}
type coreFixture struct {
	Name          string `json:"name"`
	TreeSHA256    string `json:"treeSha256"`
	PackageSHA256 string `json:"packageSha256"`
	LockSHA256    string `json:"lockSha256"`
	ConfigSHA256  string `json:"configSha256"`
	TestSHA256    string `json:"testSha256"`
}
type coreIdentity struct {
	Profile            string            `json:"profile"`
	ArchiveSHA256      string            `json:"archiveSha256"`
	ChecksumsSHA256    string            `json:"checksumsSha256"`
	SmokeSHA256        string            `json:"smokeSha256"`
	SourceCommit       string            `json:"sourceCommit"`
	SourceTree         string            `json:"sourceTree"`
	TasksCommit        string            `json:"tasksCommit"`
	Binaries           []coreNamedDigest `json:"binaries"`
	Runtimes           []coreRuntime     `json:"runtimes"`
	Fixtures           []coreFixture     `json:"fixtures"`
	NPMCacheSHA256     string            `json:"npmCacheSha256"`
	BrowserCacheSHA256 string            `json:"browserCacheSha256"`
	GoAuthoritySHA256  string            `json:"goAuthoritySha256"`
	Platform           struct {
		OS   string `json:"os"`
		Arch string `json:"arch"`
	} `json:"platform"`
	UnchangedAfter bool `json:"unchangedAfter"`
}
type corePin struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type coreAuthority struct {
	Profile              string   `json:"profile"`
	VerifierExecutable   string   `json:"verifierExecutable"`
	VerifierSHA256       string   `json:"verifierExecutableRawSha256"`
	GitExecutable        string   `json:"gitExecutable"`
	GitSHA256            string   `json:"gitExecutableRawSha256"`
	GoExecutable         string   `json:"goExecutable"`
	GoWorkPath           string   `json:"goWorkPath"`
	GOARCH               string   `json:"goarch"`
	GOOS                 string   `json:"goos"`
	GOROOT               string   `json:"goroot"`
	ModuleCacheDirectory string   `json:"moduleCacheDirectory"`
	ModuleMode           string   `json:"moduleMode"`
	OutputLimitBytes     int64    `json:"outputLimitBytes"`
	Packages             []string `json:"packages"`
	RepositoryRoot       string   `json:"repositoryRoot"`
	TemporaryParent      string   `json:"temporaryParent"`
	TimeoutMilliseconds  int64    `json:"timeoutMilliseconds"`
}

var coreStageNames = []string{"offline-dependencies", "providers", "provider-interruption", "docs", "planning", "console"}
var coreDocsNames = []string{"watch-receipt.json", "mcp-discover.json", "mcp-list.json", "mcp-draft.json", "mcp-consume.json", "conflict-receipt.json"}
var coreRoadmapNames = []string{"tasks-seed-first", "tasks-seed-replay", "tasks-roadmap", "tasks-ticket-details", "tasks-no-dispatch"}

func coreProviderNames() []string {
	var names []string
	for _, p := range []string{"js-unit", "js-e2e", "go"} {
		for _, n := range []string{"initial-provider", "initial-mcp", "pending-fail-mcp", "fail-provider", "fail-mcp", "pending-fix-mcp", "fix-provider", "fix-mcp"} {
			names = append(names, p+"-"+n)
		}
	}
	return append(names, "test-validity-discover", "test-validity-list", "provider-transitions", "provider-interruption")
}

// RunCoreInstalledQualification runs only helpers reconstructed from the verified
// retained source. Failed scratch is evidence, never a successful report.
func RunCoreInstalledQualification(ctx context.Context, opts CoreInstalledOptions) (report CoreInstalledReport, err error) {
	if err = admitCoreInstalled(opts); err != nil {
		return
	}
	verified, e := VerifyRetainedBundle(opts.BundleDirectory)
	if e != nil {
		return report, e
	}
	inventory, e := inventoryForManifest(verified.Manifest)
	if e != nil || !inventory.core {
		return report, fmt.Errorf("core qualification requires companion profile /2")
	}
	commit, tree, sourceMember, e := verified.CorvintSourceIdentity()
	if e != nil {
		return report, e
	}
	if commit != opts.ExpectedCommit || tree != opts.ExpectedTree {
		return report, fmt.Errorf("core bundle differs from frozen source")
	}
	npm := filepath.Join(filepath.Dir(filepath.Dir(opts.NodePath)), "lib/node_modules/npm/bin/npm-cli.js")
	authorityBytes, e := readBoundedRegular(opts.GoAuthorityPath, 1<<20)
	if e != nil {
		return report, e
	}
	if sha256Hex(authorityBytes) != opts.ExpectedGoAuthoritySHA256 {
		return report, fmt.Errorf("Go authority digest mismatch")
	}
	var authority coreAuthority
	if e = decodeCoreClosed(authorityBytes, &authority); e != nil {
		return report, fmt.Errorf("Go attachment: %w", e)
	}
	canonicalAuthority, e := json.Marshal(authority)
	if e != nil || !bytes.Equal(canonicalAuthority, authorityBytes) {
		return report, fmt.Errorf("Go attachment must retain canonical native bytes")
	}
	bundleRoot := filepath.Join(opts.Scratch, "bundle")
	sourceRoot := filepath.Join(opts.Scratch, "source")
	providerRoot := filepath.Join(opts.Scratch, "providers")
	if authority.GitExecutable != "/usr/bin/git" || authority.Profile != "corvint-go-live-authority-attachment/0" || authority.RepositoryRoot != filepath.Join(providerRoot, "go") || authority.VerifierExecutable != filepath.Join(bundleRoot, "bin/corvint-go-test-provider") || authority.ModuleMode != "MODULE_READONLY" || authority.GoWorkPath != "" || authority.GOOS != runtime.GOOS || authority.GOARCH != runtime.GOARCH || authority.TemporaryParent != filepath.Join(opts.Scratch, "go-temporary") || authority.ModuleCacheDirectory != filepath.Join(opts.Scratch, "go-modules") || len(authority.Packages) != 1 || authority.Packages[0] != "example.test/corvint-interactive-alpha/go" {
		return report, fmt.Errorf("Go attachment does not bind selected installed fixture and trusted-local scope")
	}
	var pins []corePin
	for _, p := range []corePin{{opts.NodePath, opts.ExpectedNodeSHA256}, {npm, opts.ExpectedNPMSHA256}, {opts.PythonPath, opts.ExpectedPythonSHA256}, {opts.GoAuthorityPath, opts.ExpectedGoAuthoritySHA256}, {authority.GitExecutable, authority.GitSHA256}} {
		if e = verifyCorePin(p); e != nil {
			return report, e
		}
		pins = append(pins, p)
	}
	goSHA, e := hashRegular(authority.GoExecutable)
	if e != nil {
		return report, e
	}
	pins = append(pins, corePin{authority.GoExecutable, goSHA})
	var identity coreIdentity
	identity.Profile = "corvint-core-installed-identity/0"
	identity.ArchiveSHA256 = verified.ArchiveSHA256
	identity.ChecksumsSHA256 = verified.ChecksumSHA256
	identity.SmokeSHA256 = verified.SmokeSHA256
	identity.SourceCommit = commit
	identity.SourceTree = tree
	identity.GoAuthoritySHA256 = opts.ExpectedGoAuthoritySHA256
	identity.Platform.OS = runtime.GOOS
	identity.Platform.Arch = runtime.GOARCH
	identity.NPMCacheSHA256, e = coreTreeHash(opts.NPMCache)
	if e != nil {
		return report, e
	}
	identity.BrowserCacheSHA256, e = coreTreeHash(opts.BrowserCache)
	if e != nil {
		return report, e
	}
	lock, e := os.OpenFile(opts.OutputPath+".lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return report, fmt.Errorf("core qualification lock: %w", e)
	}
	lock.Close()
	defer os.Remove(opts.OutputPath + ".lock")
	if e = os.Mkdir(opts.Scratch, 0700); e != nil {
		return report, e
	}
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w; failed owned scratch retained at %s", err, opts.Scratch)
		}
	}()
	if e = verified.Extract(bundleRoot); e != nil {
		return report, e
	}
	if e = verified.ExtractSourceArchive(sourceMember, sourceRoot); e != nil {
		return report, e
	}
	for _, component := range verified.Manifest.Components {
		identity.Binaries = append(identity.Binaries, coreNamedDigest{component.BinaryPath, component.BinarySHA256})
		pins = append(pins, corePin{filepath.Join(bundleRoot, component.BinaryPath), component.BinarySHA256})
		if component.Module == "github.com/Beamfall/corvint-tasks" || component.Name == "corvint-tasks" {
			identity.TasksCommit = component.Commit
		}
	}
	if identity.TasksCommit == "" {
		return report, fmt.Errorf("core bundle lacks Tasks identity")
	}
	if got, e := hashRegular(authority.VerifierExecutable); e != nil || got != authority.VerifierSHA256 {
		return report, fmt.Errorf("Go verifier differs from frozen attachment")
	}
	cacheRoot := filepath.Join(opts.Scratch, "npm-cache")
	browserRoot := filepath.Join(opts.Scratch, "browser-cache")
	if e = copyBoundedTree(ctx, opts.NPMCache, cacheRoot, false); e != nil {
		return report, e
	}
	if e = copyBoundedTree(ctx, opts.BrowserCache, browserRoot, true); e != nil {
		return report, e
	}
	home := filepath.Join(opts.Scratch, "home")
	if e = os.Mkdir(home, 0700); e != nil {
		return report, e
	}
	pinPath := filepath.Join(opts.Scratch, "runtime-pins.json")
	writePins := func() error {
		raw, e := json.Marshal(pins)
		if e != nil {
			return e
		}
		return os.WriteFile(pinPath, raw, 0600)
	}
	if e = writePins(); e != nil {
		return report, e
	}
	env := []string{"HOME=" + home, "PATH=" + filepath.Dir(opts.NodePath) + ":/usr/bin:/bin:/usr/sbin:/sbin", "GOTOOLCHAIN=local", "GOENV=off", "GOPROXY=off", "NPM_CONFIG_CACHE=" + cacheRoot, "NPM_CONFIG_OFFLINE=true", "NPM_CONFIG_AUDIT=false", "NPM_CONFIG_FUND=false", "PLAYWRIGHT_BROWSERS_PATH=" + browserRoot, "PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1", "CORVINT_CORE_PINS=" + pinPath, "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z"}
	report = CoreInstalledReport{Profile: coreInstalledProfile, Status: "PASS", BundleSHA256: verified.ArchiveSHA256, SourceCommit: commit, SourceTree: tree, NodeSHA256: opts.ExpectedNodeSHA256, NPMSHA256: opts.ExpectedNPMSHA256, PythonSHA256: opts.ExpectedPythonSHA256}
	streams := map[string]*CoreInstalledStage{}
	for _, name := range coreStageNames {
		streams[name] = &CoreInstalledStage{Name: name}
	}
	evidenceRoot := filepath.Join(opts.Scratch, "evidence")
	if e = os.Mkdir(evidenceRoot, 0700); e != nil {
		return report, e
	}
	run := func(stage, dir string, timeout time.Duration, argv ...string) ([]byte, error) {
		for _, pin := range pins {
			if e := verifyCorePin(pin); e != nil {
				return nil, e
			}
		}
		observation := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: dir, Env: env, Timeout: timeout, ShutdownTimeout: 5 * time.Second, OutputLimit: installedStageOutputLimit})
		row := streams[stage]
		if !utf8.Valid(observation.Stdout) || !utf8.Valid(observation.Stderr) {
			return nil, fmt.Errorf("stage %s emitted non-UTF8 evidence", stage)
		}
		row.Stdout += string(observation.Stdout)
		row.Stderr += string(observation.Stderr)
		if len(row.Stdout)+len(row.Stderr) > installedStageOutputLimit {
			return nil, fmt.Errorf("stage %s output exceeded 1MiB", stage)
		}
		if e := os.WriteFile(filepath.Join(evidenceRoot, "stage-"+stage+".stdout"), []byte(row.Stdout), 0600); e != nil {
			return nil, e
		}
		if e := os.WriteFile(filepath.Join(evidenceRoot, "stage-"+stage+".stderr"), []byte(row.Stderr), 0600); e != nil {
			return nil, e
		}
		for _, pin := range pins {
			if e := verifyCorePin(pin); e != nil {
				return nil, e
			}
		}
		if observation.Err != nil || !observation.ExitObserved || observation.ExitStatus != 0 || !observation.OwnedProcessGroupCleanup {
			return nil, fmt.Errorf("stage %s failed exit=%d: %v %s", stage, observation.ExitStatus, observation.Err, trimForError(observation.Stderr))
		}
		return observation.Stdout, nil
	}
	runOK := func(stage, dir string, timeout time.Duration, argv ...string) error {
		_, e := run(stage, dir, timeout, argv...)
		return e
	}
	sourceStatus, e := run("offline-dependencies", opts.SourceRoot, time.Minute, authority.GitExecutable, "status", "--porcelain")
	if e != nil || len(bytes.TrimSpace(sourceStatus)) != 0 {
		return report, fmt.Errorf("checker source root is not clean: %w", e)
	}
	head, e := run("offline-dependencies", opts.SourceRoot, time.Minute, authority.GitExecutable, "rev-parse", "HEAD")
	if e != nil || strings.TrimSpace(string(head)) != commit {
		return report, fmt.Errorf("checker source root revision mismatch")
	}
	for _, argv := range [][]string{{authority.GitExecutable, "init", "-q"}, {authority.GitExecutable, "add", "-A"}} {
		if e = runOK("offline-dependencies", sourceRoot, time.Minute, argv...); e != nil {
			return report, e
		}
	}
	rebuilt, e := run("offline-dependencies", sourceRoot, time.Minute, authority.GitExecutable, "write-tree")
	if e != nil || strings.TrimSpace(string(rebuilt)) != tree {
		return report, fmt.Errorf("retained source tree differs from frozen tree")
	}
	if e = runOK("offline-dependencies", sourceRoot, time.Minute, authority.GitExecutable, "-c", "user.name=Core qualification", "-c", "user.email=core@example.invalid", "commit", "-qm", "Retained core source"); e != nil {
		return report, e
	}
	fixtureSource := filepath.Join(sourceRoot, "conformance/interactive-alpha/fixture")
	if e = runOK("offline-dependencies", fixtureSource, 3*time.Minute, opts.NodePath, npm, "ci", "--offline", "--ignore-scripts", "--no-audit", "--no-fund"); e != nil {
		return report, e
	}
	browserBytes, e := run("offline-dependencies", fixtureSource, time.Minute, opts.NodePath, "--input-type=module", "-e", `import { pathToFileURL } from "node:url"; const { coreBrowserExecutable } = await import(pathToFileURL(process.argv[1])); process.stdout.write(coreBrowserExecutable(process.argv[2]));`, filepath.Join(sourceRoot, "conformance/interactive-alpha/core-process.mjs"), fixtureSource)
	if e != nil {
		return report, e
	}
	browser := strings.TrimSpace(string(browserBytes))
	if !pathWithin(browser, browserRoot) {
		return report, fmt.Errorf("browser escapes private cache")
	}
	browserSHA, e := hashRegular(browser)
	if e != nil {
		return report, e
	}
	pins = append(pins, corePin{browser, browserSHA})
	if e = writePins(); e != nil {
		return report, e
	}
	for _, tool := range []struct {
		name, path string
		args       []string
	}{{"node", opts.NodePath, []string{"--version"}}, {"npm", opts.NodePath, []string{npm, "--version"}}, {"python", opts.PythonPath, []string{"--version"}}, {"git", authority.GitExecutable, []string{"--version"}}, {"go", authority.GoExecutable, []string{"version"}}, {"browser", browser, []string{"--version"}}} {
		output, e := run("offline-dependencies", sourceRoot, time.Minute, append([]string{tool.path}, tool.args...)...)
		if e != nil {
			return report, e
		}
		path := tool.path
		if tool.name == "npm" {
			path = npm
		}
		digest, e := hashRegular(path)
		if e != nil {
			return report, e
		}
		identity.Runtimes = append(identity.Runtimes, coreRuntime{tool.name, digest, strings.TrimSpace(string(output))})
	}
	providerOut := filepath.Join(evidenceRoot, "providers")
	driver := filepath.Join(sourceRoot, "conformance/interactive-alpha/core-provider-proof.mjs")
	for _, phase := range []struct{ stage, mode string }{{"providers", "sessions"}, {"provider-interruption", "interrupt"}} {
		if e = runOK(phase.stage, sourceRoot, 8*time.Minute, opts.NodePath, driver, filepath.Join(bundleRoot, "bin"), sourceRoot, providerRoot, opts.GoAuthorityPath, providerOut, phase.mode); e != nil {
			return report, e
		}
	}
	if report.Providers, e = readCoreEvidence(providerOut, coreProviderNames(), false); e != nil {
		return report, e
	}
	for _, kind := range []string{"unit", "e2e", "go"} {
		var f coreFixture
		if e = readCoreJSON(filepath.Join(providerOut, "fixture-"+kind+".json"), &f); e != nil {
			return report, e
		}
		identity.Fixtures = append(identity.Fixtures, f)
	}
	docsOut := filepath.Join(opts.Scratch, "docs-proof")
	if e = os.Mkdir(docsOut, 0700); e != nil {
		return report, e
	}
	if e = runOK("docs", sourceRoot, 3*time.Minute, opts.PythonPath, filepath.Join(sourceRoot, "internal/docmaintain/testdata/watch_mcp_proof.py"), "--core", "--corvint", filepath.Join(bundleRoot, "bin/corvint"), "--docs-mcp", filepath.Join(bundleRoot, "bin/corvint-docs-mcp"), "--output-root", docsOut); e != nil {
		return report, e
	}
	docsDirs, e := os.ReadDir(docsOut)
	if e != nil || len(docsDirs) != 1 || !docsDirs[0].IsDir() {
		return report, fmt.Errorf("docs proof fixture inventory differs")
	}
	docsFixture := filepath.Join(docsOut, docsDirs[0].Name())
	if report.Docs, e = readCoreEvidence(docsFixture, coreDocsNames, true); e != nil {
		return report, e
	}
	var docsIdentity coreFixture
	if e = readCoreJSON(filepath.Join(docsFixture, "fixture-identity.json"), &docsIdentity); e != nil {
		return report, e
	}
	identity.Fixtures = append(identity.Fixtures, docsIdentity)
	planning := filepath.Join(opts.Scratch, "planning")
	planningOut := filepath.Join(evidenceRoot, "planning")
	if e = runOK("planning", sourceRoot, 3*time.Minute, opts.NodePath, filepath.Join(sourceRoot, "conformance/interactive-alpha/core-planning-proof.mjs"), filepath.Join(bundleRoot, "bin/corvint-tasks"), sourceRoot, planning, planningOut); e != nil {
		return report, e
	}
	if report.Roadmap, e = readCoreEvidence(planningOut, coreRoadmapNames, false); e != nil {
		return report, e
	}
	var planningIdentity coreFixture
	if e = readCoreJSON(filepath.Join(planningOut, "fixture-planning.json"), &planningIdentity); e != nil {
		return report, e
	}
	identity.Fixtures = append(identity.Fixtures, planningIdentity)
	consoleOut, e := run("console", sourceRoot, 3*time.Minute, opts.NodePath, filepath.Join(sourceRoot, "conformance/interactive-alpha/roadmap-proof.mjs"), filepath.Join(bundleRoot, "bin/corvint-console"), filepath.Join(bundleRoot, "bin/corvint-tasks"), filepath.Join(bundleRoot, "bin/corvint-dashboard-snapshot"), planning, sourceRoot, fixtureSource, "--core")
	if e != nil {
		return report, e
	}
	consoleRaw, e := selectCoreProfile(consoleOut, "corvint-core-console-proof/0")
	if e != nil {
		return report, e
	}
	report.Console = []InstalledEvidence{{Name: "browser", SHA256: sha256Hex(consoleRaw), Raw: string(consoleRaw)}}
	for _, pin := range pins {
		if e = verifyCorePin(pin); e != nil {
			return report, e
		}
	}
	if e = rehashRetained(verified); e != nil {
		return report, e
	}
	for _, check := range []struct{ path, want string }{{opts.NPMCache, identity.NPMCacheSHA256}, {opts.BrowserCache, identity.BrowserCacheSHA256}} {
		got, e := coreTreeHash(check.path)
		if e != nil || got != check.want {
			return report, fmt.Errorf("offline source cache drift")
		}
	}
	for _, check := range []struct {
		argv []string
		want string
	}{{[]string{"status", "--porcelain"}, ""}, {[]string{"rev-parse", "HEAD"}, opts.ExpectedCommit}, {[]string{"rev-parse", "HEAD^{tree}"}, opts.ExpectedTree}} {
		got, e := run("console", opts.SourceRoot, time.Minute, append([]string{authority.GitExecutable}, check.argv...)...)
		if e != nil || strings.TrimSpace(string(got)) != check.want {
			return report, fmt.Errorf("frozen source drift at completion")
		}
	}
	identity.UnchangedAfter = true
	raw, e := json.Marshal(identity)
	if e != nil {
		return report, e
	}
	report.Identity = InstalledEvidence{Name: "identity", SHA256: sha256Hex(raw), Raw: string(raw)}
	for _, name := range coreStageNames {
		row := streams[name]
		row.StdoutSHA256 = sha256Hex([]byte(row.Stdout))
		row.StderrSHA256 = sha256Hex([]byte(row.Stderr))
		report.Stages = append(report.Stages, *row)
	}
	body, e := json.MarshalIndent(report, "", "  ")
	if e != nil {
		return report, e
	}
	body = append(body, '\n')
	if len(body) > installedTotalOutputLimit {
		return report, fmt.Errorf("assembled core report exceeds size bound")
	}
	if e = writeExclusiveAtomic(filepath.Join(evidenceRoot, "candidate-report.unvalidated.json"), body); e != nil {
		return report, fmt.Errorf("retain unvalidated core report: %w", e)
	}
	if e = ValidateCoreInstalledReport(body); e != nil {
		return report, e
	}
	if e = writeExclusiveAtomic(opts.OutputPath, body); e != nil {
		return report, e
	}
	if e = os.RemoveAll(opts.Scratch); e != nil {
		_ = os.Remove(opts.OutputPath)
		return report, fmt.Errorf("core scratch cleanup: %w", e)
	}
	return report, nil
}

func admitCoreInstalled(opts CoreInstalledOptions) error {
	if e := admitInstalledOptionsForProfile(opts.InstalledOptions, true); e != nil {
		return e
	}
	for _, digest := range []string{opts.ExpectedNodeSHA256, opts.ExpectedNPMSHA256, opts.ExpectedPythonSHA256, opts.ExpectedGoAuthoritySHA256} {
		if !coreDigest.MatchString(digest) {
			return fmt.Errorf("core requires all exact canonical runtime and authority digests")
		}
	}
	if !coreGitID.MatchString(opts.ExpectedCommit) || !coreGitID.MatchString(opts.ExpectedTree) {
		return fmt.Errorf("core requires canonical Git identities")
	}
	if !filepath.IsAbs(opts.GoAuthorityPath) || filepath.Clean(opts.GoAuthorityPath) != opts.GoAuthorityPath {
		return fmt.Errorf("Go authority path must be canonical")
	}
	for _, path := range []string{opts.Scratch, opts.OutputPath, opts.OutputPath + ".lock"} {
		if pathsOverlap(path, opts.GoAuthorityPath) {
			return fmt.Errorf("core output overlaps Go authority")
		}
	}
	if _, e := os.Lstat(opts.OutputPath + ".lock"); !os.IsNotExist(e) {
		return fmt.Errorf("core qualification lock already exists or is unavailable")
	}
	return nil
}
func verifyCorePin(pin corePin) error {
	resolved, e := filepath.EvalSymlinks(pin.Path)
	if e != nil || resolved != pin.Path || !coreDigest.MatchString(pin.SHA256) {
		return fmt.Errorf("invalid core runtime pin %s", pin.Path)
	}
	info, e := os.Lstat(pin.Path)
	if e != nil || !info.Mode().IsRegular() || info.Size() > 512<<20 {
		return fmt.Errorf("core runtime exceeds regular-file bound")
	}
	got, e := hashRegular(pin.Path)
	if e != nil || got != pin.SHA256 {
		return fmt.Errorf("core runtime identity drift: %s", pin.Path)
	}
	return nil
}
func coreTreeHash(root string) (string, error) {
	info, e := os.Lstat(root)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("inventory root must be a real directory: %s", root)
	}
	var rows []string
	var count int
	var total int64
	e = filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if path == root {
			return nil
		}
		count++
		if count > maxNPMCacheFiles {
			return fmt.Errorf("core inventory count exceeds bound")
		}
		if d.IsDir() {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(root, path)
		if d.Type()&os.ModeSymlink != 0 {
			link, e := os.Readlink(path)
			if e != nil || filepath.IsAbs(link) || !pathWithin(filepath.Clean(filepath.Join(filepath.Dir(path), link)), root) {
				return fmt.Errorf("core inventory symlink escapes root")
			}
			rows = append(rows, filepath.ToSlash(rel)+"\x00link\x00"+link)
			total += int64(len(link))
		} else {
			if !info.Mode().IsRegular() || info.Size() > 512<<20 {
				return fmt.Errorf("core inventory member exceeds regular-file bound")
			}
			total += info.Size()
			sum, e := hashRegular(path)
			if e != nil {
				return e
			}
			rows = append(rows, filepath.ToSlash(rel)+"\x00"+sum)
		}
		if total > maxNPMCacheBytes {
			return fmt.Errorf("core inventory bytes exceed bound")
		}
		return nil
	})
	if e != nil {
		return "", e
	}
	sort.Strings(rows)
	return sha256Hex([]byte(strings.Join(rows, "\n"))), nil
}
func readCoreJSON(path string, target any) error {
	body, e := readBoundedRegular(path, installedStageOutputLimit)
	if e != nil {
		return e
	}
	return decodeCoreClosed(body, target)
}
func readCoreEvidence(root string, names []string, hasExtension bool) ([]InstalledEvidence, error) {
	var result []InstalledEvidence
	for _, name := range names {
		file := name
		if !hasExtension {
			file += ".json"
		}
		raw, e := readBoundedRegular(filepath.Join(root, file), installedStageOutputLimit)
		if e != nil {
			return nil, e
		}
		result = append(result, InstalledEvidence{Name: name, SHA256: sha256Hex(raw), Raw: string(raw)})
	}
	return result, nil
}

func selectCoreProfile(stdout []byte, profile string) ([]byte, error) {
	var selected []byte
	for _, line := range bytes.SplitAfter(stdout, []byte("\n")) {
		var value struct {
			Profile string `json:"profile"`
		}
		if json.Unmarshal(line, &value) == nil && value.Profile == profile {
			if selected != nil {
				return nil, fmt.Errorf("duplicate core profile %s", profile)
			}
			selected = append([]byte(nil), line...)
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("core profile %s missing", profile)
	}
	return selected, nil
}
