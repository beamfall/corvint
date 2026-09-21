package releasecandidate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/companionrelease"
)

const (
	manifestProfile      = "corvint-qualified-release-candidate/0"
	qualificationProfile = "corvint-release-qualification/0"
)

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+a[0-9]+$`)

func Assemble(ctx context.Context, options Options) (_ *Result, err error) {
	if !versionPattern.MatchString(options.Version) {
		return nil, fmt.Errorf("invalid alpha version %q", options.Version)
	}
	for name, value := range map[string]string{"core directory": options.CoreDirectory, "companion directory": options.CompanionDirectory, "source root": options.SourceRoot, "scratch": options.Scratch, "output parent": options.OutputParent} {
		if value == "" || !filepath.IsAbs(value) {
			return nil, fmt.Errorf("%s must be absolute", name)
		}
	}
	if err := validateScratchSeparation(options); err != nil {
		return nil, err
	}
	name := "corvint-v" + options.Version + "-qualified"
	target := filepath.Join(options.OutputParent, name)
	if _, statErr := os.Lstat(target); statErr == nil {
		return nil, fmt.Errorf("refusing to overwrite existing release candidate %s", target)
	} else if !os.IsNotExist(statErr) {
		return nil, statErr
	}
	core, coreBytes, err := verifyCore(options.CoreDirectory)
	if err != nil {
		return nil, err
	}
	companion, err := companionrelease.VerifyRetainedBundle(options.CompanionDirectory)
	if err != nil {
		return nil, fmt.Errorf("companion bundle: %w", err)
	}
	corvintCommit, corvintTree, corvintSourcePath, err := companion.CorvintSourceIdentity()
	if err != nil {
		return nil, err
	}
	if core.Revision != corvintCommit || core.Tree != corvintTree || companion.Manifest.GoVersion != core.Toolchain {
		return nil, fmt.Errorf("core and companion source or toolchain identities disagree")
	}
	tasks, tasksSourcePath, err := companionTasksIdentity(companion)
	if err != nil {
		return nil, err
	}
	buildNumber, versionOutput, err := verifyVersionIdentity(ctx, options, companion, corvintCommit, corvintTree)
	if err != nil {
		return nil, err
	}
	corvintSource, ok := companion.Entry(corvintSourcePath)
	if !ok {
		return nil, fmt.Errorf("companion lacks %s", corvintSourcePath)
	}
	tasksSource, ok := companion.Entry(tasksSourcePath)
	if !ok {
		return nil, fmt.Errorf("companion lacks %s", tasksSourcePath)
	}
	files := map[string][]byte{}
	roles := map[string]string{}
	add := func(path, role string, raw []byte) {
		files[path], roles[path] = append([]byte(nil), raw...), role
	}
	for _, archive := range shippedCoreArchives {
		add("core/"+archive, "core-archive", coreBytes[archive])
	}
	add("evidence/core-SHA256SUMS", "core-gate-checksums", coreBytes["SHA256SUMS"])
	add("evidence/core-verification-report.json", "core-gate-report", coreBytes["verification-report.json"])
	for _, item := range []struct{ path, role, source string }{
		{"companion/" + filepath.Base(companion.ArchivePath), "companion-archive", companion.ArchivePath},
		{"companion/" + filepath.Base(companion.ChecksumPath), "companion-checksum", companion.ChecksumPath},
		{"evidence/" + filepath.Base(companion.SmokePath), "companion-installed-smoke", companion.SmokePath},
	} {
		raw, readErr := readRegular(item.source, maxInputBytes)
		if readErr != nil {
			return nil, readErr
		}
		add(item.path, item.role, raw)
	}
	add("source/corvint-src.tar.gz", "corvint-source", corvintSource)
	add("source/corvint-tasks-src.tar.gz", "corvint-tasks-source", tasksSource)
	qualification := buildQualification(companion)
	qualificationRaw, err := canonicalJSON(qualification)
	if err != nil {
		return nil, err
	}
	add("QUALIFICATION.json", "qualification-receipt", qualificationRaw)
	add("README.md", "release-notes", []byte(candidateReadme(options.Version)))
	manifest := Manifest{Profile: manifestProfile, Version: options.Version, BuildNumber: buildNumber, CorvintVersion: versionOutput, GoVersion: core.Toolchain, GitVersion: companion.Manifest.GitVersion, Sources: []SourceIdentity{{Name: "corvint", Commit: corvintCommit, Tree: corvintTree}, tasks}}
	manifest.Assets = assetsFor(files, roles)
	manifestRaw, err := canonicalJSON(manifest)
	if err != nil {
		return nil, err
	}
	add("MANIFEST.json", "release-manifest", manifestRaw)
	add("SHA256SUMS", "candidate-checksums", renderChecksums(files))
	if err := os.MkdirAll(options.OutputParent, 0o700); err != nil {
		return nil, err
	}
	staging, err := os.MkdirTemp(options.OutputParent, ".release-candidate-")
	if err != nil {
		return nil, err
	}
	defer func() { err = joinCleanup(err, os.RemoveAll(staging)) }()
	if err := writeCandidate(staging, files); err != nil {
		return nil, err
	}
	if _, err := VerifyContext(ctx, staging); err != nil {
		return nil, fmt.Errorf("verify completed release candidate: %w", err)
	}
	if err := promoteNoReplace(options.OutputParent, filepath.Base(staging), name); err != nil {
		return nil, fmt.Errorf("retain release candidate: %w", err)
	}
	return &Result{Directory: target, Manifest: manifest, Qualification: qualification}, nil
}

func companionTasksIdentity(bundle *companionrelease.VerifiedRetainedBundle) (SourceIdentity, string, error) {
	for _, component := range bundle.Manifest.Components {
		if component.Module == "corvint" {
			continue
		}
		if component.SourceArchivePath != "source/corvint-tasks-src.tar.gz" {
			return SourceIdentity{}, "", fmt.Errorf("unexpected Tasks source path %s", component.SourceArchivePath)
		}
		return SourceIdentity{Name: "corvint-tasks", Commit: component.Commit, Tree: component.Tree}, component.SourceArchivePath, nil
	}
	return SourceIdentity{}, "", fmt.Errorf("companion has no Corvint Tasks component")
}

func verifyVersionIdentity(ctx context.Context, options Options, bundle *companionrelease.VerifiedRetainedBundle, commit, tree string) (string, string, error) {
	git := "/usr/bin/git"
	if runtime.GOOS == "linux" {
		git = "git"
	}
	runGit := func(arguments ...string) (string, error) {
		command := exec.CommandContext(ctx, git, append([]string{"-C", options.SourceRoot}, arguments...)...)
		command.Env = []string{"PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin", "HOME=" + options.Scratch, "LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_NO_REPLACE_OBJECTS=1"}
		out, err := command.Output()
		return strings.TrimSpace(string(out)), err
	}
	actualCommit, err := runGit("rev-parse", "--verify", commit+"^{commit}")
	if err != nil || actualCommit != commit {
		return "", "", fmt.Errorf("source root lacks exact Corvint commit")
	}
	actualTree, err := runGit("rev-parse", "--verify", commit+"^{tree}")
	if err != nil || actualTree != tree {
		return "", "", fmt.Errorf("source root Corvint tree disagrees")
	}
	version, err := runGit("show", commit+":VERSION")
	if err != nil || version != options.Version {
		return "", "", fmt.Errorf("source VERSION does not equal %s", options.Version)
	}
	build, err := runGit("rev-list", "--first-parent", "--count", commit)
	if err != nil || build == "" {
		return "", "", fmt.Errorf("derive Corvint build number")
	}
	probe, err := os.MkdirTemp(options.Scratch, ".release-version-probe-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(probe)
	extracted := filepath.Join(probe, "bundle")
	if err := bundle.Extract(extracted); err != nil {
		return "", "", err
	}
	command := exec.CommandContext(ctx, filepath.Join(extracted, "bin", "corvint"), "--version")
	command.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + options.Scratch, "LANG=C", "LC_ALL=C"}
	out, err := command.Output()
	observed := strings.TrimSpace(string(out))
	expected := "Corvint " + options.Version + " (build " + build + ")"
	if err != nil || observed != expected {
		return "", "", fmt.Errorf("installed Corvint version %q, expected %q", observed, expected)
	}
	return build, observed, nil
}

func buildQualification(bundle *companionrelease.VerifiedRetainedBundle) Qualification {
	installed := map[string]bool{}
	for _, step := range bundle.SmokeSteps {
		installed[step.Name] = step.OK
	}
	workflows := []string{"version-identity", "affected-selection", "playwright-external-discovery", "documentation-corpus-discovery", "work-queue-observation"}
	stepNames := map[string]string{
		"version-identity": "corvint-version-identity", "affected-selection": "corvint-affected-selection", "playwright-external-discovery": "corvint-playwright-external-discovery", "documentation-corpus-discovery": "corvint-documentation-corpus-discovery", "work-queue-observation": "corvint-work-queue-observation",
	}
	platforms := []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64"}
	rows := []QualificationRow{}
	for _, platform := range platforms {
		rows = append(rows, QualificationRow{Platform: platform, Workflow: "core-archive", Status: "PASS", Evidence: "evidence/core-verification-report.json"})
		companionStatus, evidence := "NOT_RUN", "companion target unavailable"
		if platform == "darwin/arm64" {
			companionStatus, evidence = "PASS", "companion/"+filepath.Base(bundle.ArchivePath)
		}
		rows = append(rows, QualificationRow{Platform: platform, Workflow: "companion-bundle", Status: companionStatus, Evidence: evidence})
		for _, workflow := range workflows {
			status, detail := "NOT_RUN", "installed workflow unavailable on this platform"
			if platform == "darwin/arm64" && installed[stepNames[workflow]] {
				status, detail = "PASS", "evidence/"+filepath.Base(bundle.SmokePath)
			}
			rows = append(rows, QualificationRow{Platform: platform, Workflow: workflow, Status: status, Evidence: detail})
		}
	}
	return Qualification{Profile: qualificationProfile, Rows: rows}
}

func assetsFor(files map[string][]byte, roles map[string]string) []Asset {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	assets := make([]Asset, 0, len(paths))
	for _, path := range paths {
		assets = append(assets, Asset{Path: path, Role: roles[path], SHA256: digest(files[path]), SizeBytes: int64(len(files[path]))})
	}
	return assets
}

func writeCandidate(root string, files map[string][]byte) error {
	for path, raw := range files {
		if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "..") {
			return fmt.Errorf("unsafe candidate path %s", path)
		}
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(full, raw, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func canonicalJSON(value any) ([]byte, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func candidateReadme(version string) string {
	return "# Corvint " + version + " qualified local candidate\n\nExperimental prerelease candidate. `QUALIFICATION.json` is authoritative: NOT_RUN is not PASS. This directory was assembled locally and is not a publication, tag, signature, upload, promotion, or installed-path replacement. Windows is excluded. Verify every retained file with `shasum -a 256 -c SHA256SUMS`.\n\nEvidence authority defaults to the local Git executable and object database described by the CEM trust contract. Performance is unmeasured, and hosted CI is unavailable/NOT_RUN. Optional workflow and host features remain experimental unless their exact row is PASS.\n"
}

func validateScratchSeparation(options Options) error {
	scratch, err := resolveProspective(options.Scratch)
	if err != nil {
		return fmt.Errorf("resolve scratch: %w", err)
	}
	info, err := os.Lstat(options.Scratch)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("scratch must be an existing real directory")
	}
	inputs := map[string]string{"source root": options.SourceRoot, "core directory": options.CoreDirectory, "companion directory": options.CompanionDirectory}
	resolvedInputs := map[string]string{}
	for name, candidate := range inputs {
		resolved, resolveErr := resolveProspective(candidate)
		if resolveErr != nil {
			return fmt.Errorf("resolve %s: %w", name, resolveErr)
		}
		resolvedInputs[name] = resolved
		if pathOverlap(scratch, resolved) {
			return fmt.Errorf("scratch overlaps %s", name)
		}
	}
	output, err := resolveProspective(options.OutputParent)
	if err != nil {
		return fmt.Errorf("resolve output parent: %w", err)
	}
	if pathOverlap(scratch, output) {
		return fmt.Errorf("scratch overlaps output parent")
	}
	for name, resolved := range resolvedInputs {
		if pathOverlap(output, resolved) {
			return fmt.Errorf("output parent overlaps %s", name)
		}
	}
	return nil
}

func resolveProspective(candidate string) (string, error) {
	candidate = filepath.Clean(candidate)
	current := candidate
	var suffix []string
	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for index := len(suffix) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, suffix[index])
			}
			return resolved, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no existing path ancestor")
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func pathOverlap(left, right string) bool {
	within := func(parent, child string) bool {
		relative, err := filepath.Rel(parent, child)
		return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
	}
	return within(left, right) || within(right, left)
}

func joinCleanup(primary, cleanup error) error {
	if cleanup == nil {
		return primary
	}
	if primary == nil {
		return cleanup
	}
	return fmt.Errorf("%v; cleanup: %w", primary, cleanup)
}
