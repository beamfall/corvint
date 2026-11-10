// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/conformance/release-artifact-v0/archivebuild"
	"github.com/Beamfall/corvint/conformance/release-artifact-v0/archivewire"
)

// BuildOptions is one `build` invocation: one product binary, one version, one output
// directory, every catalog target or only the host's (ARTIFACT-BIN-V0-002).
type BuildOptions struct {
	Root     string
	Catalog  Catalog
	Binary   string
	Version  string
	Output   string
	HostOnly bool
}

// archiveName is the ARTIFACT-GO-V0-002 naming applied to any binary: tar.gz except Windows.
func archiveName(binary string, target Target) string {
	root := binary + "_" + target.GOOS + "_" + target.GOARCH
	if target.GOOS == "windows" {
		return root + ".zip"
	}
	return root + ".tar.gz"
}

func gitOutput(ctx context.Context, root string, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, arguments...)...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// cleanHead resolves HEAD and refuses any tracked change or untracked path, so the stamped
// source commit is the bytes that were compiled.
func cleanHead(ctx context.Context, root string) (commit, build string, err error) {
	commit, err = gitOutput(ctx, root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return "", "", err
	}
	if !commitPattern.MatchString(commit) {
		return "", "", fmt.Errorf("HEAD resolved to %q, not a 40-hex commit", commit)
	}
	status, err := gitOutput(ctx, root, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return "", "", err
	}
	if status != "" {
		return "", "", fmt.Errorf("dirty-worktree: commit or remove these paths before a release build:\n%s", status)
	}
	build, err = gitOutput(ctx, root, "rev-list", "--count", "--first-parent", "HEAD")
	if err != nil {
		return "", "", err
	}
	if !buildPattern.MatchString(build) {
		return "", "", fmt.Errorf("first-parent count %q is not a positive integer", build)
	}
	return commit, build, nil
}

// exportHead clones the repository's objects into destination and checks out exactly commit,
// so the build reads the committed bytes alone: an ignored or untracked file in the working tree
// (which the clean check does not see for ignored paths) cannot reach the compiler, and the
// embedded vcs.revision still binds the commit. The clone shares objects and writes nothing
// into the repository.
func exportHead(ctx context.Context, root, commit, destination string) error {
	if _, err := gitOutput(ctx, root, "clone", "--quiet", "--shared", "--no-checkout", root, destination); err != nil {
		return err
	}
	if _, err := gitOutput(ctx, destination, "checkout", "--quiet", "--detach", commit); err != nil {
		return err
	}
	return nil
}

// buildEnvironment is a closed allowlist over the inherited environment: the module and build
// caches are reused for speed, but no inherited GOFLAGS, GOPROXY, GOTOOLCHAIN or GOWORK can
// change the profile.
func buildEnvironment(target Target) []string {
	environment := map[string]string{
		"CGO_ENABLED": "0",
		"GOENV":       "off",
		"GOFLAGS":     "-mod=readonly",
		"GOPROXY":     "off",
		"GOSUMDB":     "off",
		"GOTOOLCHAIN": "local",
		"GOWORK":      "off",
		"GOOS":        target.GOOS,
		"GOARCH":      target.GOARCH,
	}
	for _, key := range []string{"HOME", "PATH", "TMPDIR", "TEMP", "TMP", "GOCACHE", "GOMODCACHE", "GOPATH", "GOROOT", "SystemRoot", "USERPROFILE"} {
		if value, set := os.LookupEnv(key); set {
			environment[key] = value
		}
	}
	pairs := make([]string, 0, len(environment))
	for key, value := range environment {
		pairs = append(pairs, key+"="+value)
	}
	sort.Strings(pairs)
	return pairs
}

// stampFlags is the -ldflags argument that sets the main package's stamp variables: none for a
// `stamp: none` binary, the build number and source commit for a core-set binary, and the
// release version too for an independent binary (ARTIFACT-BIN-V0-002).
func stampFlags(binary Binary, version, build, commit string) string {
	values := map[string]string{"build": build, "sourceCommit": commit, "version": version}
	var flags []string
	for _, name := range binary.stampVariables() {
		flags = append(flags, "-X main."+name+"="+values[name])
	}
	return "-ldflags=" + strings.Join(flags, " ")
}

// declaredStampVariables refuses `stamp-unbound` unless the main package in source declares
// every variable the release builder stamps, since the linker silently drops an -X for an
// undeclared variable and the binary would then report nothing of what the manifest records.
func declaredStampVariables(source string, binary Binary) error {
	wanted := binary.stampVariables()
	if len(wanted) == 0 {
		return nil
	}
	directory := filepath.Join(source, filepath.FromSlash(binary.Package))
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	declared := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(directory, entry.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			return fmt.Errorf("%s: %w", binary.Name, err)
		}
		if file.Name.Name != "main" {
			continue
		}
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.VAR {
				continue
			}
			for _, spec := range general.Specs {
				for _, name := range spec.(*ast.ValueSpec).Names {
					declared[name.Name] = true
				}
			}
		}
	}
	var missing []string
	for _, name := range wanted {
		if !declared[name] {
			missing = append(missing, "main."+name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("stamp-unbound: %s does not declare %s, so the release stamp would be dropped; declare it, or mark the binary stamp none in the catalog if another contract owns its --version", binary.Package, strings.Join(missing, ", "))
	}
	return nil
}

func buildTarget(ctx context.Context, root string, binary Binary, target Target, version, build, commit, output string) error {
	ldflags := stampFlags(binary, version, build, commit)
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", ldflags, "-o", output, binary.Package)
	command.Dir = root
	command.Env = buildEnvironment(target)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("build-failed %s %s: %w: %s", binary.Name, target.id(), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// verifyBuildInfo binds the compiled bytes to the clean commit, the target and -trimpath from
// metadata inside the artifact, so a stale or foreign binary cannot be archived.
func verifyBuildInfo(path string, target Target, commit, goVersion string) error {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return err
	}
	settings := map[string]string{"go": info.GoVersion}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	expected := map[string]string{"go": goVersion, "GOOS": target.GOOS, "GOARCH": target.GOARCH, "CGO_ENABLED": "0", "-trimpath": "true", "vcs.revision": commit, "vcs.modified": "false"}
	keys := make([]string, 0, len(expected))
	for key := range expected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var mismatches []string
	for _, key := range keys {
		if settings[key] != expected[key] {
			mismatches = append(mismatches, fmt.Sprintf("%s=%q want %q", key, settings[key], expected[key]))
		}
	}
	if len(mismatches) > 0 {
		return errors.New("buildinfo-mismatch: " + strings.Join(mismatches, ", "))
	}
	return nil
}

func hostGoVersion(ctx context.Context, root string) (string, error) {
	command := exec.CommandContext(ctx, "go", "env", "GOVERSION")
	command.Dir = root
	command.Env = buildEnvironment(Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH})
	out, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("go env GOVERSION: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func loadLegalFiles(root string, catalog Catalog) ([]archivebuild.File, error) {
	var files []archivebuild.File
	for _, name := range catalog.LegalFiles {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return nil, fmt.Errorf("legal-file-missing: %w", err)
		}
		if int64(len(raw)) > archivewire.HardMaxContentBytes {
			return nil, fmt.Errorf("legal file %s exceeds %d bytes", name, archivewire.HardMaxContentBytes)
		}
		files = append(files, archivebuild.File{Name: name, Mode: 0o644, Data: raw})
	}
	return files, nil
}

// assembleArchive lays out root/<binary>, root/<legal files> and root/SHA256SUMS exactly as
// the corvint archives do, through the shared deterministic encoders.
func assembleArchive(name string, target Target, binaryName string, binary []byte, legal []archivebuild.File) ([]byte, error) {
	archive := archiveName(name, target)
	root := strings.TrimSuffix(strings.TrimSuffix(archive, ".tar.gz"), ".zip")
	members := []archivebuild.File{{Name: root + "/" + binaryName, Mode: 0o755, Data: binary}}
	for _, file := range legal {
		members = append(members, archivebuild.File{Name: root + "/" + file.Name, Mode: file.Mode, Data: file.Data})
	}
	checksum := digestOf(binary) + "  " + binaryName + "\n"
	members = append(members, archivebuild.File{Name: root + "/SHA256SUMS", Mode: 0o644, Data: []byte(checksum)})
	if target.GOOS == "windows" {
		return archivebuild.ZIP(members)
	}
	return archivebuild.TarGzip(members)
}

// runBuild builds one binary for its targets from an exported copy of the clean HEAD in a
// private scratch directory, assembles and digests the archives, then retains them, the
// per-binary SHA256SUMS and the entry without replacing any existing file. It compiles nothing
// but the named package.
func runBuild(ctx context.Context, options BuildOptions) (entry Entry, err error) {
	binary, known := options.Catalog.find(options.Binary)
	if !known {
		return Entry{}, fmt.Errorf("unknown-binary: %q is not in the catalog", options.Binary)
	}
	if binary.Role != "product" {
		return Entry{}, fmt.Errorf("support-binary: %q is release tooling and is never archived", options.Binary)
	}
	if !versionPattern.MatchString(options.Version) {
		return Entry{}, fmt.Errorf("bad-version: %q is not MAJOR.MINOR.PATCH[-pre]", options.Version)
	}
	targets := options.Catalog.Targets
	if options.HostOnly {
		targets = nil
		for _, target := range options.Catalog.Targets {
			if target.GOOS == runtime.GOOS && target.GOARCH == runtime.GOARCH {
				targets = append(targets, target)
			}
		}
		if len(targets) == 0 {
			return Entry{}, fmt.Errorf("host %s/%s is not a catalog target", runtime.GOOS, runtime.GOARCH)
		}
	}
	if err := refuseOutputInsideRepository(options.Root, options.Output); err != nil {
		return Entry{}, err
	}
	commit, build, err := cleanHead(ctx, options.Root)
	if err != nil {
		return Entry{}, err
	}
	if binary.VersionSet != "" {
		// A core-set binary reports the compiled VERSION; an independent binary is stamped.
		if err := checkCompiledVersion(options.Root, options.Version); err != nil {
			return Entry{}, err
		}
	}
	if err := declaredStampVariables(options.Root, binary); err != nil {
		return Entry{}, err
	}
	scratch, err := os.MkdirTemp("", "release-binary-")
	if err != nil {
		return Entry{}, err
	}
	defer os.RemoveAll(scratch)
	source := filepath.Join(scratch, "src")
	if err := exportHead(ctx, options.Root, commit, source); err != nil {
		return Entry{}, err
	}
	goVersion, err := hostGoVersion(ctx, source)
	if err != nil {
		return Entry{}, err
	}
	legal, err := loadLegalFiles(source, options.Catalog)
	if err != nil {
		return Entry{}, err
	}
	if err := os.MkdirAll(options.Output, 0o700); err != nil {
		return Entry{}, err
	}
	entry = Entry{Schema: entrySchema, Name: binary.Name, Version: options.Version, SourceCommit: commit, Build: build, GoVersion: goVersion, VersionSet: binary.VersionSet}
	archives := map[string][]byte{}
	for _, target := range targets {
		binaryName := binary.Name + target.Suffix
		built := filepath.Join(scratch, target.GOOS+"-"+target.GOARCH, binaryName)
		if err := os.MkdirAll(filepath.Dir(built), 0o700); err != nil {
			return Entry{}, err
		}
		if err := buildTarget(ctx, source, binary, target, options.Version, build, commit, built); err != nil {
			return Entry{}, err
		}
		if err := verifyBuildInfo(built, target, commit, goVersion); err != nil {
			return Entry{}, fmt.Errorf("%s %s: %w", binary.Name, target.id(), err)
		}
		raw, err := os.ReadFile(built)
		if err != nil {
			return Entry{}, err
		}
		if int64(len(raw)) > archivewire.HardMaxContentBytes {
			return Entry{}, fmt.Errorf("%s %s: binary exceeds %d bytes", binary.Name, target.id(), archivewire.HardMaxContentBytes)
		}
		archive, err := assembleArchive(binary.Name, target, binaryName, raw, legal)
		if err != nil {
			return Entry{}, err
		}
		if int64(len(archive)) > archivewire.HardMaxArchiveBytes {
			return Entry{}, fmt.Errorf("%s: archive exceeds %d bytes", archiveName(binary.Name, target), archivewire.HardMaxArchiveBytes)
		}
		name := archiveName(binary.Name, target)
		archives[name] = archive
		entry.Archives = append(entry.Archives, ArchiveEntry{GOOS: target.GOOS, GOARCH: target.GOARCH, Name: name, SHA256: digestOf(archive), Bytes: int64(len(archive)), BinarySHA256: digestOf(raw)})
	}
	if !options.HostOnly {
		if err := (Record{Version: entry.Version, SourceCommit: entry.SourceCommit, Build: entry.Build, GoVersion: entry.GoVersion, Archives: entry.Archives}).validate(entry.Name, options.Catalog); err != nil {
			return Entry{}, err
		}
	}
	// Refuse before writing anything when any output already exists; a failure after the first
	// write removes what this run wrote.
	files := &outputs{directory: options.Output}
	names := []string{binary.Name + ".SHA256SUMS", binary.Name + entrySuffix}
	for _, archive := range entry.Archives {
		names = append(names, archive.Name)
	}
	if err := files.reserve(names...); err != nil {
		return Entry{}, err
	}
	defer func() {
		if err != nil {
			files.discard()
		}
	}()
	var checksums strings.Builder
	for _, archive := range entry.Archives {
		checksums.WriteString(archive.SHA256 + "  " + archive.Name + "\n")
		if err = files.write(archive.Name, archives[archive.Name], 0o644); err != nil {
			return Entry{}, err
		}
	}
	if err = files.write(binary.Name+".SHA256SUMS", []byte(checksums.String()), 0o644); err != nil {
		return Entry{}, err
	}
	raw, err := canonicalJSON(entry)
	if err != nil {
		return Entry{}, err
	}
	if err = files.write(binary.Name+entrySuffix, raw, 0o644); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// checkCompiledVersion refuses a release version that differs from the VERSION file the
// core-set binaries compile in, so the manifest never labels one with a version it does not
// report. Independent binaries are stamped with their version instead and are not checked
// here. A tree without VERSION (a fixture) is not checked.
func checkCompiledVersion(root, version string) error {
	raw, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	compiled := strings.TrimSpace(string(raw))
	if compiled != version {
		return fmt.Errorf("version-mismatch: --version %s but VERSION compiles %s; bump VERSION in the release commit first", version, compiled)
	}
	return nil
}
