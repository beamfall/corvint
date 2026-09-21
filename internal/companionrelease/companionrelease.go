// Package companionrelease assembles the optional companion distribution
// bundle (nine Go binaries plus five agent-host packages) for a single
// pinned target. It never mutates a
// caller-supplied checkout: every Git read is a plumbing read against HEAD,
// and every build/assembly step runs inside a caller-supplied scratch
// directory. Browser qualification is explicitly out of scope; see
// docs/specs/public-release-v0.md.
package companionrelease

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/gitstatus"
)

// Options names the two clean checkout roots and the working directories
// this package needs. CorvintRoot and TaskmanRoot must already be verified
// clean checkouts of the two source modules (the caller is expected to have
// cloned them fresh; Run re-verifies cleanliness itself and never trusts
// the caller's claim).
type Options struct {
	CorvintRoot  string
	TaskmanRoot  string
	Target       string // must be exactly supportedTarget
	Scratch      string // scratch working directory; Run creates subdirs under it
	OutputParent string // directory the retained bundle is placed under; must not be inside CorvintRoot or TaskmanRoot
	BundleName   string // name of the retained bundle directory under OutputParent
	NPMCache     string // retained compatibility option; unused by core bundles
}

// Report is the human- and machine-readable record of one companion build.
// A zero-value Report (BundlePath == "") means nothing was retained: a
// build failure at any stage keeps no qualified output on disk.
type Report struct {
	Target          string
	Toolchain       Toolchain
	Manifest        BundleManifest
	BundlePath      string // retained directory holding the archive and its checksum
	ArchivePath     string // BundlePath/<BundleName>.tar.gz
	SmokeReportPath string
	ArchiveSHA256   string
	SHA256SUMS      string // the SHA256SUMS member inside the archive
	SmokeSteps      []SmokeStep
	NotRun          []string
}

// Run executes the full companion bundle pipeline: toolchain and target
// validation, clean-tree checks on both roots, source export, staging the
// verified export as the build tree, two independent builds per component,
// source archive assembly (built and compared twice), bundle archive assembly
// (built and compared twice), an independent tar.gz decode verification of
// each archive, an installed smoke test, and atomic retention of the bundle
// archive outside both checkout roots — in that order, so a smoke failure
// still leaves the qualified report unwritten and the bundle unretained.
func Run(ctx context.Context, opts Options) (*Report, error) {
	if err := validateTarget(opts.Target); err != nil {
		return nil, err
	}
	if err := validateOutputParent(ctx, opts.OutputParent, opts.CorvintRoot, opts.TaskmanRoot); err != nil {
		return nil, err
	}
	if err := validateBundleName(opts.BundleName); err != nil {
		return nil, err
	}
	if err := validateScratch(ctx, opts.Scratch, opts.CorvintRoot, opts.TaskmanRoot); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(opts.Scratch, 0o700); err != nil {
		return nil, err
	}

	toolchainDir := filepath.Join(opts.Scratch, "toolchain")
	if err := mkdirScratchDir(toolchainDir); err != nil {
		return nil, err
	}
	toolchain, err := resolveToolchain(ctx, toolchainDir)
	if err != nil {
		return nil, err
	}

	if err := requireCleanTree(ctx, toolchain.GitPath, opts.CorvintRoot, opts.Scratch); err != nil {
		return nil, fmt.Errorf("corvint root: %w", err)
	}
	if err := requireCleanTree(ctx, toolchain.GitPath, opts.TaskmanRoot, opts.Scratch); err != nil {
		return nil, fmt.Errorf("taskman root: %w", err)
	}

	corvintExport, err := exportSource(ctx, toolchain.GitPath, opts.CorvintRoot, opts.Scratch)
	if err != nil {
		return nil, fmt.Errorf("export corvint source: %w", err)
	}
	taskmanExport, err := exportSource(ctx, toolchain.GitPath, opts.TaskmanRoot, opts.Scratch)
	if err != nil {
		return nil, fmt.Errorf("export taskman source: %w", err)
	}

	// Every binary is built from the exported, digest-verified tree staged
	// under scratch, never the live checkout: a gitignored file there, or a
	// checkout change after the clean-tree check, cannot reach a shipped binary
	// its source archive would not reproduce.
	corvintBuildRoot, err := stageBuildSource(corvintExport, filepath.Join(opts.Scratch, "corvint-build-src"))
	if err != nil {
		return nil, fmt.Errorf("stage corvint build source: %w", err)
	}
	taskmanBuildRoot, err := stageBuildSource(taskmanExport, filepath.Join(opts.Scratch, "taskman-build-src"))
	if err != nil {
		return nil, fmt.Errorf("stage taskman build source: %w", err)
	}
	corvintBuildNumber, err := sourceBuildNumber(ctx, toolchain.GitPath, opts.CorvintRoot, opts.Scratch)
	if err != nil {
		return nil, err
	}

	type componentSpec struct {
		name       string
		moduleRoot string
		pkgPath    string
		module     string
		export     Export
		buildFlags []string
	}
	specs := []componentSpec{
		{name: "corvint", moduleRoot: corvintBuildRoot, pkgPath: "./cmd/corvint", module: "corvint", export: corvintExport, buildFlags: []string{"-ldflags=-X main.build=" + corvintBuildNumber}},
		{name: "corvint-console", moduleRoot: corvintBuildRoot, pkgPath: "./cmd/corvint-console", module: "corvint", export: corvintExport},
		{name: "corvint-dashboard-snapshot", moduleRoot: corvintBuildRoot, pkgPath: "./cmd/corvint-dashboard-snapshot", module: "corvint", export: corvintExport},
		{name: "corvint-mcp", moduleRoot: corvintBuildRoot, pkgPath: "./cmd/corvint-mcp", module: "corvint", export: corvintExport},
		{name: "corvint-docs-mcp", moduleRoot: corvintBuildRoot, pkgPath: "./cmd/corvint-docs-mcp", module: "corvint", export: corvintExport},
		{name: "corvint-test-validity-mcp", moduleRoot: corvintBuildRoot, pkgPath: "./cmd/corvint-test-validity-mcp", module: "corvint", export: corvintExport},
		{name: "corvint-js-test-provider", moduleRoot: corvintBuildRoot, pkgPath: "./cmd/corvint-js-test-provider", module: "corvint", export: corvintExport},
		{name: "corvint-go-test-provider", moduleRoot: corvintBuildRoot, pkgPath: "./cmd/corvint-go-test-provider", module: "corvint", export: corvintExport},
		{name: "corvint-tasks", moduleRoot: taskmanBuildRoot, pkgPath: "./cmd/corvint-tasks", module: "corvint-tasks", export: taskmanExport},
	}

	corvintSourceFiles, corvintSourcePath, corvintSourceDigest, err := assembleModuleSource("corvint", corvintExport)
	if err != nil {
		return nil, err
	}
	taskmanSourceFiles, taskmanSourcePath, taskmanSourceDigest, err := assembleModuleSource("corvint-tasks", taskmanExport)
	if err != nil {
		return nil, err
	}
	componentFiles := append(corvintSourceFiles, taskmanSourceFiles...)
	var components []ComponentManifest
	for _, spec := range specs {
		built, err := buildComponentTwiceWithFlags(ctx, spec.moduleRoot, spec.pkgPath, spec.name, opts.Target, opts.Scratch, spec.buildFlags)
		if err != nil {
			return nil, fmt.Errorf("build %s: %w", spec.name, err)
		}
		binData, err := os.ReadFile(built.Path)
		if err != nil {
			return nil, err
		}
		sourcePath, sourceDigest := corvintSourcePath, corvintSourceDigest
		if spec.module == "corvint-tasks" {
			sourcePath, sourceDigest = taskmanSourcePath, taskmanSourceDigest
		}
		files, component := assembleBinaryComponent(spec.name, spec.module, spec.export, built, binData, sourcePath, sourceDigest)
		componentFiles = append(componentFiles, files...)
		components = append(components, component)
	}

	var artifacts []ArtifactManifest
	for _, prefix := range []string{"integrations/codex/", "integrations/claude-code/", "integrations/gemini-cli/", "integrations/opencode/", "integrations/pi/"} {
		files, artifact, err := exportedPackageTree(corvintExport, prefix, corvintSourcePath, corvintSourceDigest)
		if err != nil {
			return nil, err
		}
		componentFiles = append(componentFiles, files...)
		artifacts = append(artifacts, artifact)
	}

	manifest := BundleManifest{
		Profile:    json.RawMessage(`"corvint-companion-bundle/2"`),
		Target:     opts.Target,
		GoVersion:  toolchain.GoVersion,
		GitVersion: toolchain.GitVersion,
		NotRun:     NotRunTargets,
		Components: components,
		Artifacts:  artifacts,
	}
	bundleEntries := func() ([]ArchiveEntry, error) { return assembleBundleEntries(componentFiles, manifest) }
	archive, err := buildTarGzTwice(bundleEntries)
	if err != nil {
		return nil, fmt.Errorf("bundle archive: %w", err)
	}
	entries, err := bundleEntries()
	if err != nil {
		return nil, err
	}
	if err := verifyTarGz(archive, entries); err != nil {
		return nil, fmt.Errorf("bundle archive verification: %w", err)
	}

	smoke := func(ctx context.Context, extractDir string) ([]SmokeStep, error) {
		queuePolicy, err := generateQueuePolicy(ctx, taskmanExport, opts.Scratch)
		if err != nil {
			return nil, fmt.Errorf("generate smoke fixtures: %w", err)
		}
		steps, smokeErr := runSmoke(ctx, extractDir, filepath.Join(opts.Scratch, "smoke-run"), queuePolicy, bundleInventory{current: true, pi: true, core: true})
		bindSmokeEvidence(steps, sha256Hex(archive), components, extractDir, bundleInventory{current: true, pi: true, core: true})
		return steps, smokeErr
	}
	// assembleBundleEntries appends SHA256SUMS as the last entry.
	sums := string(entries[len(entries)-1].Data)
	pending := Report{Target: opts.Target, Toolchain: toolchain, Manifest: manifest, SHA256SUMS: sums, NotRun: NotRunTargets}
	return qualifyAndRetain(ctx, opts, pending, entries, archive, smoke)
}

// assembleComponent turns one twice-built binary and its component's export
// into the bundle files that component ships — the binary, a source archive
// built twice and decode-verified, and its notices — plus its manifest record.
func assembleComponent(name, module string, export Export, built BuiltBinary, binData []byte) ([]ArchiveEntry, ComponentManifest, error) {
	files, sourcePath, sourceDigest, err := assembleModuleSource(name, export)
	if err != nil {
		return nil, ComponentManifest{}, err
	}
	binaryFiles, component := assembleBinaryComponent(name, module, export, built, binData, sourcePath, sourceDigest)
	return append(binaryFiles, files...), component, nil
}

func assembleModuleSource(name string, export Export) ([]ArchiveEntry, string, string, error) {
	sourceArchive, err := buildTarGzTwice(func() ([]ArchiveEntry, error) {
		return sourceArchiveEntries(export), nil
	})
	if err != nil {
		return nil, "", "", fmt.Errorf("%s: source archive: %w", name, err)
	}
	if err := verifyTarGz(sourceArchive, sourceArchiveEntries(export)); err != nil {
		return nil, "", "", fmt.Errorf("%s: source archive verification: %w", name, err)
	}
	sourceRelPath := "source/" + name + "-src.tar.gz"
	files := []ArchiveEntry{{Path: sourceRelPath, Mode: 0o644, Data: sourceArchive}}

	notices, err := componentNotices(export, "notices/"+name, name)
	if err != nil {
		return nil, "", "", fmt.Errorf("%s: notices: %w", name, err)
	}
	files = append(files, notices...)
	return files, sourceRelPath, sha256Hex(sourceArchive), nil
}

func assembleBinaryComponent(name, module string, export Export, built BuiltBinary, binData []byte, sourcePath, sourceDigest string) ([]ArchiveEntry, ComponentManifest) {
	binRelPath := "bin/" + name
	files := []ArchiveEntry{{Path: binRelPath, Mode: 0o755, Data: binData}}

	component := ComponentManifest{
		Name:                name,
		Module:              module,
		BinaryPath:          binRelPath,
		BinarySHA256:        built.SHA256,
		BinarySizeBytes:     built.Size,
		Commit:              export.HeadCommit,
		Tree:                export.HeadTree,
		SourceArchivePath:   sourcePath,
		SourceArchiveSHA256: sourceDigest,
	}
	return files, component
}

// assembleBundleEntries renders the bundle's complete member set from the
// component files and manifest: MANIFEST.json, README.md, and a SHA256SUMS
// covering every other member. It reads no clock or disk, so two calls on the
// same inputs yield the same entries and buildTarGzTwice can compare them.
func assembleBundleEntries(componentFiles []ArchiveEntry, manifest BundleManifest) ([]ArchiveEntry, error) {
	if err := validateManifestReferences(componentFiles, manifest); err != nil {
		return nil, err
	}
	manifestJSON, err := renderManifestJSON(manifest)
	if err != nil {
		return nil, err
	}
	entries := append([]ArchiveEntry(nil), componentFiles...)
	entries = append(entries,
		ArchiveEntry{Path: "MANIFEST.json", Mode: 0o644, Data: manifestJSON},
		ArchiveEntry{Path: "README.md", Mode: 0o644, Data: renderBundleReadme(manifest)},
	)
	sums := renderSHA256SUMS(entries)
	return append(entries, ArchiveEntry{Path: "SHA256SUMS", Mode: 0o644, Data: sums}), nil
}

// qualifyAndRetain is Run's final stage. It stages the verified bundle
// members for the installed smoke test, then retains the bundle archive and
// its checksum atomically. Any smoke error or failed step returns before
// retainBundle, so no output is retained and no report is produced.
func qualifyAndRetain(ctx context.Context, opts Options, pending Report, entries []ArchiveEntry, archive []byte,
	smoke func(ctx context.Context, extractDir string) ([]SmokeStep, error)) (*Report, error) {
	// verifyTarGz has already proven the archive decodes to exactly these
	// entries, so the staged tree holds the extracted bundle bytes.
	extractDir := filepath.Join(opts.Scratch, "smoke-extract")
	if err := writeTree(extractDir, entries); err != nil {
		return nil, fmt.Errorf("stage bundle for smoke test: %w", err)
	}
	smokeSteps, err := smoke(ctx, extractDir)
	if err != nil {
		return nil, fmt.Errorf("installed smoke test failed: %w (steps=%+v)", err, smokeSteps)
	}
	for _, s := range smokeSteps {
		if !s.OK {
			return nil, fmt.Errorf("installed smoke test step %q failed: %s", s.Name, s.Detail)
		}
	}

	archiveName := opts.BundleName + ".tar.gz"
	archiveSHA256 := sha256Hex(archive)
	smokeJSON, err := json.MarshalIndent(struct {
		BundleSHA256 string      `json:"bundleSha256"`
		Steps        []SmokeStep `json:"steps"`
	}{archiveSHA256, smokeSteps}, "", "  ")
	if err != nil {
		return nil, err
	}
	smokeJSON = append(smokeJSON, '\n')
	smokeName := opts.BundleName + ".smoke.json"
	retained := []ArchiveEntry{
		{Path: archiveName, Mode: 0o644, Data: archive},
		{Path: archiveName + ".sha256", Mode: 0o644, Data: []byte(archiveSHA256 + "  " + archiveName + "\n")},
		{Path: smokeName, Mode: 0o644, Data: smokeJSON},
	}
	bundlePath, err := retainBundle(opts.Scratch, opts.OutputParent, opts.BundleName, retained)
	if err != nil {
		return nil, fmt.Errorf("retain bundle: %w", err)
	}

	report := pending
	report.BundlePath = bundlePath
	report.ArchivePath = filepath.Join(bundlePath, archiveName)
	report.ArchiveSHA256 = archiveSHA256
	report.SmokeReportPath = filepath.Join(bundlePath, smokeName)
	report.SmokeSteps = smokeSteps
	return &report, nil
}

// validateOutputParent refuses an output location nested inside either
// checkout root, so retaining a bundle can never be mistaken for a change
// to either source tree. It uses the same gitstatus directory-identity walk
// as validateScratch, so neither a symlink nor a case or Unicode alias can
// reach a root. A not-yet-created suffix cannot be a root, so the walk starts
// at the nearest existing ancestor of the symlink-resolved path.
func validateOutputParent(ctx context.Context, outputParent, corvintRoot, taskmanRoot string) error {
	existing, _, err := resolveExistingAncestor(outputParent)
	if err != nil {
		return fmt.Errorf("resolve output parent %s: %w", outputParent, err)
	}
	if err := gitstatus.ScratchOutside(ctx, existing, corvintRoot, taskmanRoot); err != nil {
		return fmt.Errorf("output parent %s is not provably outside checkout roots %s and %s: %w", outputParent, corvintRoot, taskmanRoot, err)
	}
	return nil
}

// validateScratch refuses a scratch directory that is, or lies under, either
// checkout root, and either checkout root that lies under an existing scratch
// directory, before Run writes or clears anything there. It reuses gitstatus's
// directory-identity walk, which a case or Unicode alias cannot pass. A
// not-yet-created suffix cannot be a root, so the walk starts at the nearest
// existing ancestor of the symlink-resolved path, and a not-yet-created
// scratch cannot contain a root.
func validateScratch(ctx context.Context, scratch, corvintRoot, taskmanRoot string) error {
	existing, missing, err := resolveExistingAncestor(scratch)
	if err != nil {
		return fmt.Errorf("resolve scratch %s: %w", scratch, err)
	}
	if err := gitstatus.ScratchOutside(ctx, existing, corvintRoot, taskmanRoot); err != nil {
		return fmt.Errorf("scratch %s is not provably outside checkout roots %s and %s: %w", scratch, corvintRoot, taskmanRoot, err)
	}
	if missing != "" {
		return nil
	}
	for _, root := range []string{corvintRoot, taskmanRoot} {
		if err := rootOutsideScratch(ctx, root, scratch, existing); err != nil {
			return err
		}
	}
	return nil
}

// mkdirScratchDir creates dir directly under a validated scratch, or reuses an
// existing real directory there. An existing entry that is not a real
// directory (a symlink or a file) is refused: validateScratch judged only the
// scratch path, so a link planted at dir would redirect writes outside it.
func mkdirScratchDir(dir string) error {
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return os.MkdirAll(dir, 0o700)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("refusing scratch entry %s: not a real directory", dir)
	}
	return nil
}

// rootOutsideScratch refuses a checkout root that is, or lies under, the
// existing symlink-resolved scratch directory, walking the root's resolved
// ancestry by directory identity.
func rootOutsideScratch(ctx context.Context, root, scratch, resolvedScratch string) error {
	resolvedRoot, err := resolveThroughExistingAncestor(root)
	if err != nil {
		return fmt.Errorf("resolve checkout root %s: %w", root, err)
	}
	if err := gitstatus.ScratchOutside(ctx, resolvedRoot, resolvedScratch); err != nil {
		return fmt.Errorf("checkout root %s is not provably outside scratch %s: %w", root, scratch, err)
	}
	return nil
}

// validateBundleName refuses a bundle name that is not exactly one path
// element: retainBundle joins it under the output parent, so a separator or a
// dot element would place the retained directory outside the location
// validateOutputParent checked.
func validateBundleName(name string) error {
	if name != filepath.Base(name) {
		return fmt.Errorf("bundle name %q must be a single path element", name)
	}
	if name == "." {
		return fmt.Errorf("bundle name %q must be a single path element", name)
	}
	if name == ".." {
		return fmt.Errorf("bundle name %q must be a single path element", name)
	}
	return nil
}

// resolveThroughExistingAncestor returns path's absolute form with every
// symlink resolved. A not-yet-created suffix is resolved through its nearest
// existing ancestor and appended unchanged; it holds no symlink because it
// does not exist. A component that exists but cannot be resolved (a dangling
// symlink, a permission error) is refused rather than compared lexically.
func resolveThroughExistingAncestor(path string) (string, error) {
	resolved, missing, err := resolveExistingAncestor(path)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, missing), nil
}

// resolveExistingAncestor returns the symlink-resolved nearest existing
// ancestor of path's absolute form and the not-yet-created suffix below it.
func resolveExistingAncestor(path string) (string, string, error) {
	current, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	missing := ""
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return resolved, missing, nil
		}
		if _, statErr := os.Lstat(current); !os.IsNotExist(statErr) {
			return "", "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", "", err
		}
		missing = filepath.Join(filepath.Base(current), missing)
		current = parent
	}
}

// sourceArchiveEntries converts an Export's hash-verified files into
// ArchiveEntry values for the deterministic tar.gz assembler.
func sourceArchiveEntries(export Export) []ArchiveEntry {
	entries := make([]ArchiveEntry, 0, len(export.Files))
	for _, f := range export.Files {
		mode := int64(0o644)
		if f.Mode == "100755" {
			mode = 0o755
		}
		entries = append(entries, ArchiveEntry{Path: f.Path, Mode: mode, Data: f.Data})
	}
	return entries
}

// componentNotices assembles one component's required per-component
// notices plus the "no third-party dependencies" statement. Every notice
// is sourced from export's hash-verified commit tree, never the live
// checkout: a bundle's notices are pinned to the same Git content its
// source archive ships.
func componentNotices(export Export, prefix, moduleName string) ([]ArchiveEntry, error) {
	required := []string{"LICENSE", "PROVENANCE.md"}
	var entries []ArchiveEntry
	for _, name := range required {
		found, err := noticeEntries(export, prefix, []string{name})
		if err != nil {
			return nil, err
		}
		entries = append(entries, found...)
	}
	for _, optional := range []string{"LICENSE-APACHE-2.0", "LICENSING.md"} {
		if _, ok := findExportFile(export, optional); ok {
			found, err := noticeEntries(export, prefix, []string{optional})
			if err != nil {
				return nil, err
			}
			entries = append(entries, found...)
		}
	}
	entries = append(entries, thirdPartyNoticeEntry(prefix, moduleName))
	return entries, nil
}

func renderBundleReadme(m BundleManifest) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "Corvint companion distribution bundle\n")
	fmt.Fprintf(&b, "Target: %s\n", m.Target)
	fmt.Fprintf(&b, "Go: %s   Git: %s\n\n", m.GoVersion, m.GitVersion)
	fmt.Fprintf(&b, "Contents:\n")
	for _, c := range m.Components {
		fmt.Fprintf(&b, "  %s  (commit %s)\n", c.BinaryPath, c.Commit)
	}
	for _, a := range m.Artifacts {
		fmt.Fprintf(&b, "  %s  (%s, %s)\n", a.Path, a.Kind, a.Support)
	}
	fmt.Fprintf(&b, "\nCorvint components use source/corvint-src.tar.gz; corvint-tasks uses source/corvint-tasks-src.tar.gz.\n")
	fmt.Fprintf(&b, "Both are reconstructed from the recorded commit tree byte-for-byte (see MANIFEST.json). Rebuild with:\n")
	fmt.Fprintf(&b, "  tar xzf source/<module>-src.tar.gz -C <dir> && cd <dir> && \\\n")
	fmt.Fprintf(&b, "  GOFLAGS= GOPROXY=off GOSUMDB=off GOWORK=off CGO_ENABLED=0 \\\n")
	fmt.Fprintf(&b, "  GOTOOLCHAIN=local go build -trimpath -buildvcs=false -o <name> <pkg>\n\n")
	fmt.Fprintf(&b, "Verify checksums with: shasum -a 256 -c SHA256SUMS\n\n")
	fmt.Fprintf(&b, "Install: copy bin/corvint and bin/corvint-tasks to ~/.local/bin, /opt/homebrew/bin, /usr/local/bin, or another reviewed absolute directory.\n")
	fmt.Fprintf(&b, "`corvint-tasks --version` and `corvint --version` name the installed build; the commit\n")
	fmt.Fprintf(&b, "above and MANIFEST.json bind it to source module github.com/Beamfall/corvint-tasks and\n")
	fmt.Fprintf(&b, "github.com/Beamfall/corvint respectively.\n\n")
	fmt.Fprintf(&b, "Work queue adoption (WQO-V0-046..050): in a repository, run\n")
	fmt.Fprintf(&b, "  corvint work init --repository NAME --corvint-executable /absolute/path/to/corvint\n")
	fmt.Fprintf(&b, "which writes the queue policy .corvint/work-queue-policy.json, the worklist\n")
	fmt.Fprintf(&b, ".corvint/worklist.json and the adapter .corvint/work-queue-adapter; commit them, then run\n")
	fmt.Fprintf(&b, "`corvint work observe` and `corvint work propose-wave`. Each observation carries one adapter\n")
	fmt.Fprintf(&b, "receipt per snapshot/details/verify run. Neither command dispatches, leases, merges or executes.\n")
	fmt.Fprintf(&b, "Missing capability states: no committed adoption, or a dirty worktree -> ERROR/SOURCE_UNQUALIFIED;\n")
	fmt.Fprintf(&b, "missing or changed executable binding -> ERROR/SOURCE_UNQUALIFIED and requires reviewed work rebind;\n")
	fmt.Fprintf(&b, "adapter failure -> ERROR/ADAPTER_FAILED; queue-source drift -> STALE; executable identity, containment, mutation enforcement and network stay reported unknowns.\n\n")
	fmt.Fprintf(&b, "This bundle qualifies only: %s\n", m.Target)
	fmt.Fprintf(&b, "Not run (no attempt made) for this bundle: %s\n", strings.Join(m.NotRun, ", "))
	fmt.Fprintf(&b, "Host plugins are exact source packages at FALLBACK support; no host or dependency is bundled.\n")
	fmt.Fprintf(&b, "Browser/UI qualification is a separate, explicitly out-of-scope step;\n")
	fmt.Fprintf(&b, "this bundle's smoke test covers CLI and loopback-HTTP checks only.\n")
	return []byte(b.String())
}

// writeTree writes a bundle's entries to disk under dir with their recorded
// modes, for the smoke test to run against installed, on-disk artifacts. It
// first clears dir: a reused scratch directory could otherwise still hold a
// file from an earlier attempt's entry set that the current bundle no
// longer includes, and the smoke test would then run against extra files
// that were never actually part of what got assembled.
func writeTree(dir string, entries []ArchiveEntry) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	for _, e := range entries {
		full := filepath.Join(dir, e.Path)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(full, e.Data, os.FileMode(e.Mode)); err != nil {
			return err
		}
	}
	return nil
}
