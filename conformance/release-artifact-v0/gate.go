package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Options struct {
	Root         string
	ManifestPath string
	Output       string
	ReportPath   string
	Writer       io.Writer
}

// verify runs the whole gate. Preflight refusals short-circuit: a dirty tree or
// a wrong toolchain makes every later artifact meaningless, so nothing is built.
func verify(ctx context.Context, options Options) (Report, error) {
	manifest, err := loadManifest(options.ManifestPath)
	if err != nil {
		return Report{}, err
	}
	report := Report{Schema: manifestSchema, Pending: manifest.Pending, Profile: ProfileReport{
		Package: manifest.Profile.Package, BinaryName: manifest.Profile.BinaryName,
		BuildFlags: manifest.Profile.BuildFlags, Environment: manifest.Profile.Environment,
	}}
	if err := checkOutputLocation(options, &report); err != nil {
		return report, err
	}
	if err := preflight(ctx, options, manifest, &report); err != nil {
		return report, err
	}
	if len(report.Reasons) != 0 {
		report.Verdict = statusFail
		return report, nil
	}
	if err := buildTargets(ctx, options, manifest, &report); err != nil {
		return report, err
	}
	report.Verdict = verdict(report)
	return report, nil
}

// checkOutputLocation refuses to write build outputs inside the repository, so
// a gate run can never leave a binary where it could be committed.
func checkOutputLocation(options Options, report *Report) error {
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return err
	}
	output, err := filepath.Abs(options.Output)
	if err != nil {
		return err
	}
	if output == root || strings.HasPrefix(output, root+string(os.PathSeparator)) {
		report.fail(reasonOutputInsideRepo, "", "output directory "+output+" is inside the repository")
		report.Verdict = statusFail
		return fmt.Errorf("output directory must be outside the repository")
	}
	return os.MkdirAll(output, 0o700)
}

func preflight(ctx context.Context, options Options, manifest Manifest, report *Report) error {
	if err := checkToolchain(ctx, options.Root, manifest, report); err != nil {
		return err
	}
	if err := checkCommit(ctx, options.Root, report); err != nil {
		return err
	}
	if err := checkLegalFiles(options.Root, manifest, report); err != nil {
		return err
	}
	return checkModuleRequirements(options.Root, report)
}

// checkToolchain proves the selected local toolchain is exactly the pinned
// version. GOTOOLCHAIN=local makes selecting or downloading another impossible
// rather than merely unlikely.
func checkToolchain(ctx context.Context, root string, manifest Manifest, report *Report) error {
	selected, err := goEnv(ctx, root, "GOVERSION")
	if err != nil {
		return err
	}
	toolchainEnv, err := goEnv(ctx, root, "GOTOOLCHAIN")
	if err != nil {
		return err
	}
	directive, toolchainLines, err := readGoModDirectives(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	report.Toolchain = ToolchainReport{
		SelectedGoVersion: selected, ExpectedGoVersion: manifest.Toolchain.GoVersion,
		GoToolchainEnv: toolchainEnv, GoModDirective: directive, ToolchainLines: toolchainLines,
		Status: statusPass,
	}
	if selected != manifest.Toolchain.GoVersion {
		report.fail(reasonToolchainMismatch, "", fmt.Sprintf("selected toolchain %q want %q", selected, manifest.Toolchain.GoVersion))
	}
	if directive != manifest.Toolchain.GoModDirective {
		report.fail(reasonGoModDirective, "", fmt.Sprintf("go.mod go directive %q want %q", directive, manifest.Toolchain.GoModDirective))
	}
	if toolchainLines != 0 {
		report.fail(reasonGoModToolchainLine, "", fmt.Sprintf("go.mod carries %d toolchain directives; Go 1.27 normalizes an equal directive away", toolchainLines))
	}
	if len(report.Reasons) != 0 {
		report.Toolchain.Status = statusFail
	}
	return nil
}

// goEnv always runs under GOTOOLCHAIN=local so the query itself cannot trigger
// a toolchain download.
func goEnv(ctx context.Context, root, key string) (string, error) {
	command := exec.CommandContext(ctx, "go", "env", key)
	command.Dir = root
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOPROXY=off")
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("go env %s: %w", key, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func readGoModDirectives(path string) (string, int, error) {
	handle, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer handle.Close()
	directive, toolchainLines := "", 0
	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[0] == "go" && directive == "" {
			directive = fields[1]
		}
		if len(fields) != 0 && fields[0] == "toolchain" {
			toolchainLines++
		}
	}
	return directive, toolchainLines, scanner.Err()
}

// checkCommit requires one exact clean commit. A dirty worktree is refused
// before any build, because its bytes would not correspond to any commit.
func checkCommit(ctx context.Context, root string, report *Report) error {
	commit, err := gitOutput(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	tree, err := gitOutput(ctx, root, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return err
	}
	status, err := gitOutput(ctx, root, "status", "--porcelain")
	if err != nil {
		return err
	}
	build, err := buildNumber(ctx, root, "HEAD")
	if err != nil {
		return err
	}
	report.Commit, report.Tree, report.build = commit, tree, build
	if status != "" {
		report.fail(reasonDirtyTree, "", "worktree is dirty:\n"+status)
	}
	return nil
}

func gitOutput(ctx context.Context, root string, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", arguments...)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(arguments, " "), err)
	}
	return strings.TrimSpace(string(output)), nil
}

// checkLegalFiles proves the applicable legal files are present and their bytes
// are exactly the pinned ones GPK-V0-020 forbids the migration to alter.
func checkLegalFiles(root string, manifest Manifest, report *Report) error {
	for _, file := range manifest.Legal {
		entry := LegalReport{Path: file.Path, Expected: file.SHA256, Status: statusPass}
		content, err := os.ReadFile(filepath.Join(root, file.Path))
		if err != nil {
			entry.Status = statusFail
			report.fail(reasonLegalFileMissing, "", file.Path+": "+err.Error())
			report.LegalFiles = append(report.LegalFiles, entry)
			continue
		}
		digest := sha256.Sum256(content)
		entry.SHA256 = hex.EncodeToString(digest[:])
		if entry.SHA256 != file.SHA256 {
			entry.Status = statusFail
			report.fail(reasonLegalFileAltered, "", fmt.Sprintf("%s sha256 %s want %s", file.Path, entry.SHA256, file.SHA256))
		}
		report.LegalFiles = append(report.LegalFiles, entry)
	}
	return nil
}

// checkModuleRequirements records the declared module requirements. The binary
// dependency list is filled in per target from the embedded build info.
func checkModuleRequirements(root string, report *Report) error {
	content, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	requirements := 0
	inBlock := false
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "require (":
			inBlock = true
		case inBlock && trimmed == ")":
			inBlock = false
		case inBlock && trimmed != "" && !strings.HasPrefix(trimmed, "//"):
			requirements++
		case strings.HasPrefix(trimmed, "require ") && !inBlock:
			requirements++
		}
	}
	report.Dependencies = DependencyReport{ModuleRequirements: requirements, BinaryDependencies: []string{}, Status: statusPass}
	return nil
}

func buildTargets(ctx context.Context, options Options, manifest Manifest, report *Report) error {
	for _, target := range manifest.Targets {
		entry, err := buildTarget(ctx, options, manifest, target, report)
		if err != nil {
			return err
		}
		report.Targets = append(report.Targets, entry)
	}
	recordBinaryDependencies(report)
	return writeChecksums(filepath.Join(options.Output, "SHA256SUMS"), report.Targets)
}

func buildTarget(ctx context.Context, options Options, manifest Manifest, target Target, report *Report) (TargetReport, error) {
	name := manifest.Profile.BinaryName + "-" + target.GOOS + "-" + target.GOARCH + target.Suffix
	entry := TargetReport{GOOS: target.GOOS, GOARCH: target.GOARCH, Native: isNative(target),
		Smoke: SmokeReport{Status: statusNotRun, Reason: manifest.Smoke.CrossCompileReason}}
	first := filepath.Join(options.Output, "build-a", name)
	second := filepath.Join(options.Output, "build-b", name)
	entry.Artifact = first
	if err := runBuildPair(ctx, options, manifest, target, report.build, first, second); err != nil {
		entry.Smoke.Reason = "target did not build"
		report.fail(reasonBuildFailed, target.id(), err.Error())
		return entry, nil
	}
	if err := recordDigests(&entry, first, second); err != nil {
		return entry, err
	}
	if !entry.ByteIdentical {
		entry.Divergence = describeDivergence(first, second)
		report.fail(reasonNotByteIdentical, target.id(), entry.Divergence)
	}
	return finishTarget(ctx, options, manifest, target, entry, report)
}

func runBuildPair(ctx context.Context, options Options, manifest Manifest, target Target, build, first, second string) error {
	suffix := target.GOOS + "-" + target.GOARCH
	if err := buildOnce(ctx, options.Root, manifest, target, build, first,
		filepath.Join(options.Output, "cache-a-"+suffix), filepath.Join(options.Output, "tmp-a-"+suffix)); err != nil {
		return fmt.Errorf("build A: %w", err)
	}
	if err := buildOnce(ctx, options.Root, manifest, target, build, second,
		filepath.Join(options.Output, "cache-b-"+suffix), filepath.Join(options.Output, "tmp-b-"+suffix)); err != nil {
		return fmt.Errorf("build B: %w", err)
	}
	return nil
}

func recordDigests(entry *TargetReport, first, second string) error {
	firstDigest, firstBytes, err := fileDigest(first)
	if err != nil {
		return err
	}
	secondDigest, secondBytes, err := fileDigest(second)
	if err != nil {
		return err
	}
	entry.BuildA = BuildReport{SHA256: firstDigest, Bytes: firstBytes, DistinctCache: "cache-a"}
	entry.BuildB = BuildReport{SHA256: secondDigest, Bytes: secondBytes, DistinctCache: "cache-b"}
	entry.ByteIdentical = firstDigest == secondDigest
	entry.SHA256 = firstDigest
	return nil
}

func finishTarget(ctx context.Context, options Options, manifest Manifest, target Target, entry TargetReport, report *Report) (TargetReport, error) {
	info, err := readBuildInfo(entry.Artifact)
	if err != nil {
		report.fail(reasonBuildInfoMismatch, target.id(), "unreadable build info: "+err.Error())
		return entry, nil
	}
	entry.BuildInfo = info
	if mismatches := verifyBuildInfo(manifest, target, report.Commit, info); len(mismatches) != 0 {
		report.fail(reasonBuildInfoMismatch, target.id(), strings.Join(mismatches, "; "))
	}
	if info["deps"] != "" {
		report.fail(reasonUndeclaredDependency, target.id(), "artifact carries module dependencies: "+info["deps"])
	}
	if !entry.Native {
		return entry, nil
	}
	entry.Smoke = smokeTest(ctx, entry.Artifact, manifest.Smoke, report.build, options.Output)
	if entry.Smoke.Status == statusFail {
		report.fail(reasonSmokeFailed, target.id(), entry.Smoke.Reason)
	}
	return entry, nil
}

func recordBinaryDependencies(report *Report) {
	for _, target := range report.Targets {
		if dependencies := target.BuildInfo["deps"]; dependencies != "" {
			report.Dependencies.BinaryDependencies = append(report.Dependencies.BinaryDependencies, dependencies)
		}
	}
	if report.Dependencies.ModuleRequirements != 0 || len(report.Dependencies.BinaryDependencies) != 0 {
		report.Dependencies.Status = statusFail
	}
}

func verdict(report Report) string {
	if len(report.Reasons) != 0 {
		return statusFail
	}
	return statusPass
}

// writeSummary prints the closed evidence line. pendingEvidence is part of the
// line because a PASS here is a build-reproducibility verdict, never a
// statement that the whole release gate is satisfied.
func writeSummary(writer io.Writer, report Report) {
	fmt.Fprintf(writer, "SUMMARY targets=%d byte-identical=%d of %d checksums=%d smoke-pass=%d smoke-not-run=%d smoke-fail=%d legal-files=%d module-requirements=%d undeclared-dependencies=%d verdict=%s pending-evidence=%s\n",
		len(report.Targets), report.byteIdenticalCount(), len(report.Targets), len(report.Targets),
		report.smokeCount(statusPass), report.smokeCount(statusNotRun), report.smokeCount(statusFail),
		len(report.LegalFiles), report.Dependencies.ModuleRequirements, len(report.Dependencies.BinaryDependencies),
		report.Verdict, strings.Join(report.Pending, ","))
}

func writeReport(path string, report Report) error {
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o600)
}
