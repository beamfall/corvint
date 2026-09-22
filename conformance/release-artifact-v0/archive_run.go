package main

import (
	"bufio"
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/conformance/release-artifact-v0/archivebuild"
	"github.com/Beamfall/corvint/conformance/release-artifact-v0/archivewire"
)

const (
	archiveFramingAllowance   = int64(1 << 20)
	maximumArchiveReportBytes = int64(1 << 20)
)

type gitState struct {
	commit string
	tree   string
}

type treeEntry struct {
	mode string
	oid  string
	name string
}

func buildArchives(ctx context.Context, options ArchiveOptions) (report ArchiveReport, returnedErr error) {
	report = ArchiveReport{Schema: archiveReportSchema, Verdict: statusFail, Targets: []ArchiveTargetReport{}, Reasons: []ArchiveReason{}}
	manifestPath := options.ManifestPath
	if !filepath.IsAbs(manifestPath) {
		manifestPath = filepath.Join(options.Root, manifestPath)
	}
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return failArchive(report, "manifest-invalid", "", err)
	}
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return failArchive(report, "manifest-unreadable", "", err)
	}
	report.ManifestSHA256 = digestBytes(manifestBytes)
	parent, err := validateArchiveDestination(ctx, options.Root, options.Output)
	if err != nil {
		return failArchive(report, "output-boundary-invalid", "", err)
	}
	scratch, err := os.MkdirTemp(parent, ".corvint-release-go-scratch-")
	if err != nil {
		return failArchive(report, "scratch-create-failed", "", err)
	}
	if err := os.Chmod(scratch, 0o700); err != nil {
		_ = os.RemoveAll(scratch)
		return failArchive(report, "scratch-permission-failed", "", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(scratch); cleanupErr != nil {
			returnedErr = errors.Join(returnedErr, fmt.Errorf("scratch cleanup: %w", cleanupErr))
		}
	}()
	state, err := resolveCleanState(ctx, options, scratch)
	if err != nil {
		return failArchive(report, "git-state-invalid", "", err)
	}
	report.Revision, report.Tree = state.commit, state.tree
	committedManifest, err := closedGit(ctx, options.Root, scratch, "show", state.commit+":"+repoRelative(options.Root, manifestPath))
	if err != nil || !bytes.Equal(committedManifest, manifestBytes) {
		return failArchive(report, "manifest-commit-mismatch", "", err)
	}
	if err := checkArchiveToolchain(ctx, options.Root, scratch, manifest); err != nil {
		return failArchive(report, "toolchain-mismatch", "", err)
	}
	report.Toolchain = manifest.Toolchain.GoVersion
	looseOutput := filepath.Join(scratch, "existing-loose-gate")
	for _, directory := range []string{filepath.Join(looseOutput, "build-a"), filepath.Join(looseOutput, "build-b")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return failArchive(report, "loose-gate-output-failed", "", err)
		}
	}
	looseReport, err := verify(ctx, Options{Root: options.Root, ManifestPath: manifestPath, Output: looseOutput, Writer: io.Discard})
	if err != nil || looseReport.Verdict != statusPass || looseReport.Commit != state.commit || looseReport.Tree != state.tree || len(looseReport.Targets) != len(manifest.Targets) {
		if err == nil {
			err = fmt.Errorf("existing loose reproducibility gate did not PASS the selected commit")
		}
		return failArchive(report, "loose-gate-failed", "", err)
	}
	sourceA := filepath.Join(scratch, "source-a")
	sourceB := filepath.Join(scratch, "source-b")
	if err := exportRawCommit(ctx, options.Root, scratch, state.commit, sourceA); err != nil {
		return failArchive(report, "raw-export-a-failed", "", err)
	}
	if err := exportRawCommit(ctx, options.Root, scratch, state.commit, sourceB); err != nil {
		return failArchive(report, "raw-export-b-failed", "", err)
	}
	legalA, err := loadCommittedLegal(sourceA, manifest)
	if err != nil {
		return failArchive(report, "legal-bytes-invalid", "", err)
	}
	legalB, err := loadCommittedLegal(sourceB, manifest)
	if err != nil || !equalLegal(legalA, legalB) {
		return failArchive(report, "legal-export-mismatch", "", err)
	}
	retained := map[string][]byte{}
	gitDirectoryBytes, err := closedGit(ctx, options.Root, scratch, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return failArchive(report, "git-directory-invalid", "", err)
	}
	gitDirectory := strings.TrimSpace(string(gitDirectoryBytes))
	verifierPath := filepath.Join(scratch, "offline-verifier", "archive-verifier")
	if err := buildOfflineVerifier(ctx, sourceA, gitDirectory, state.commit, scratch, manifest, verifierPath); err != nil {
		return failArchive(report, "offline-verifier-build-failed", "", err)
	}
	var maximumArchive, maximumTotal int64
	for index, target := range manifest.Targets {
		entry, archiveBytes, archiveLimit, err := buildArchiveTarget(ctx, options, scratch, sourceA, sourceB, gitDirectory, verifierPath, state, manifestBytes, manifest, target, legalA, looseReport.Targets[index])
		if err != nil {
			report.Targets = append(report.Targets, entry)
			return failArchive(report, classifyArchiveError(err), target.id(), err)
		}
		report.Targets = append(report.Targets, entry)
		retained[entry.ArchiveName] = archiveBytes
		if archiveLimit > maximumArchive {
			maximumArchive = archiveLimit
		}
		maximumTotal += archiveLimit
	}
	report.Limits = ArchiveLimits{Targets: len(manifest.Targets), MembersPerArchive: archivewire.MaxMembers, MaximumArchiveBytes: maximumArchive, MaximumTotalBytes: maximumTotal, MaximumReportBytes: maximumArchiveReportBytes, MaximumWitnessBytes: maxWitnessBytes}
	if err := recheckCleanState(ctx, options, scratch, state); err != nil {
		return failArchive(report, "git-state-changed", "", err)
	}
	report.Verdict = statusPass
	if err := retainArchiveOutput(ctx, options, parent, report, retained, scratch, state); err != nil {
		report.Verdict = statusFail
		return failArchive(report, "retention-failed", "", err)
	}
	return report, nil
}

func buildArchiveTarget(ctx context.Context, options ArchiveOptions, scratch, sourceA, sourceB, gitDirectory, verifierPath string, state gitState, manifestBytes []byte, manifest Manifest, target Target, legal []archivewire.ExpectedFile, looseTarget TargetReport) (ArchiveTargetReport, []byte, int64, error) {
	binaryName := manifest.Profile.BinaryName + target.Suffix
	archiveName, format := archiveTargetName(target)
	root := strings.TrimSuffix(strings.TrimSuffix(archiveName, ".gz"), ".tar")
	if format == "zip" {
		root = strings.TrimSuffix(archiveName, ".zip")
	}
	targetID := target.GOOS + "-" + target.GOARCH
	buildAPath := filepath.Join(scratch, "build-a", targetID, binaryName)
	buildBPath := filepath.Join(scratch, "build-b", targetID, binaryName)
	if err := archiveBuildOnce(ctx, sourceA, gitDirectory, state.commit, manifest, target, buildAPath, filepath.Join(scratch, "cache-a", targetID), filepath.Join(scratch, "tmp-a", targetID)); err != nil {
		return ArchiveTargetReport{GOOS: target.GOOS, GOARCH: target.GOARCH, BinaryName: binaryName, ArchiveName: archiveName}, nil, 0, err
	}
	if err := archiveBuildOnce(ctx, sourceB, gitDirectory, state.commit, manifest, target, buildBPath, filepath.Join(scratch, "cache-b", targetID), filepath.Join(scratch, "tmp-b", targetID)); err != nil {
		return ArchiveTargetReport{GOOS: target.GOOS, GOARCH: target.GOARCH, BinaryName: binaryName, ArchiveName: archiveName}, nil, 0, err
	}
	binaryA, err := os.ReadFile(buildAPath)
	if err != nil {
		return ArchiveTargetReport{}, nil, 0, err
	}
	binaryB, err := os.ReadFile(buildBPath)
	if err != nil {
		return ArchiveTargetReport{}, nil, 0, err
	}
	if err := bindLooseGateTarget(looseTarget, target, binaryA); err != nil {
		return ArchiveTargetReport{}, nil, 0, err
	}
	membersA := archiveMembers(root, binaryName, binaryA, legal)
	membersB := archiveMembers(root, binaryName, binaryB, legal)
	assemblyAPath := filepath.Join(scratch, "assembly-a", targetID, archiveName)
	assemblyBPath := filepath.Join(scratch, "assembly-b", targetID, archiveName)
	archiveA, err := assembleArchiveTo(format, membersA, assemblyAPath)
	if err != nil {
		return ArchiveTargetReport{}, nil, 0, err
	}
	archiveB, err := assembleArchiveTo(format, membersB, assemblyBPath)
	if err != nil {
		return ArchiveTargetReport{}, nil, 0, err
	}
	legalBytes := int64(0)
	for _, file := range legal {
		legalBytes += int64(len(file.Data))
	}
	contentLimit := int64(len(binaryA)) + legalBytes + 128
	archiveLimit := contentLimit + archiveFramingAllowance
	verified, err := runOfflineVerifier(ctx, verifierPath, scratch, archivewire.Request{
		Schema: archivewire.RequestSchema, ArchiveName: archiveName, Format: format, Root: root, BinaryName: binaryName,
		GOOS: target.GOOS, GOARCH: target.GOARCH, Commit: state.commit, Tree: state.tree,
		GoVersion: manifest.Toolchain.GoVersion, PackagePath: manifest.Profile.ModulePath + "/" + strings.TrimPrefix(manifest.Profile.Package, "./"), ManifestBytes: manifestBytes,
		ArchiveAPath: assemblyAPath, ArchiveBPath: assemblyBPath, BinaryAPath: buildAPath, BinaryBPath: buildBPath, LooseGateBinaryPath: looseTarget.Artifact,
		LegalFiles: legal, MaximumArchiveBytes: archiveLimit, MaximumContentBytes: contentLimit,
	}, targetID)
	entry := ArchiveTargetReport{
		GOOS: target.GOOS, GOARCH: target.GOARCH, BinaryName: binaryName, ArchiveName: archiveName,
		BuildA: digestReport(binaryA), BuildB: digestReport(binaryB),
		AssemblyA: digestReport(archiveA), AssemblyB: digestReport(archiveB),
	}
	if err != nil {
		return entry, nil, archiveLimit, err
	}
	entry.RetainedBinary = ArchiveDigest{SHA256: verified.BinarySHA256, Bytes: verified.BinaryBytes}
	entry.RetainedArchive = ArchiveDigest{SHA256: verified.ArchiveSHA256, Bytes: verified.ArchiveBytes}
	return entry, archiveA, archiveLimit, nil
}

func bindLooseGateTarget(loose TargetReport, target Target, binary []byte) error {
	want := digestReport(binary)
	if loose.GOOS != target.GOOS || loose.GOARCH != target.GOARCH || !loose.ByteIdentical ||
		loose.BuildA.SHA256 != loose.BuildB.SHA256 || loose.BuildA.Bytes != loose.BuildB.Bytes ||
		loose.SHA256 != want.SHA256 || loose.BuildA.SHA256 != want.SHA256 || loose.BuildA.Bytes != want.Bytes {
		return fmt.Errorf("loose-gate-report-mismatch")
	}
	looseBinary, err := os.ReadFile(loose.Artifact)
	if err != nil || !bytes.Equal(looseBinary, binary) {
		return fmt.Errorf("loose-gate-binary-mismatch")
	}
	return nil
}

func archiveBuildOnce(ctx context.Context, source, gitDirectory, commit string, manifest Manifest, target Target, output, cache, temporary string) error {
	if err := os.WriteFile(filepath.Join(source, ".git"), []byte("gitdir: "+gitDirectory+"\n"), 0o600); err != nil {
		return err
	}
	for _, directory := range []string{filepath.Dir(output), cache, temporary} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
	}
	if err := initializeBuildIndex(ctx, source, gitDirectory, commit, temporary); err != nil {
		return err
	}
	build, err := closedGit(ctx, source, temporary, "--git-dir="+gitDirectory, "rev-list", "--count", "--first-parent", commit)
	if err != nil {
		return err
	}
	arguments := buildArguments(manifest, strings.TrimSpace(string(build)), output)
	environment := archiveBuildEnvironment(manifest, target, source, gitDirectory, cache, temporary)
	_, stderr, err := runContained(ctx, "go", arguments, environment, source)
	if err != nil {
		return fmt.Errorf("go build: %w: %s", err, strings.TrimSpace(string(stderr)))
	}
	return nil
}

func buildOfflineVerifier(ctx context.Context, source, gitDirectory, commit, scratch string, manifest Manifest, output string) error {
	if err := os.WriteFile(filepath.Join(source, ".git"), []byte("gitdir: "+gitDirectory+"\n"), 0o600); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o700); err != nil {
		return err
	}
	if err := initializeBuildIndex(ctx, source, gitDirectory, commit, filepath.Join(scratch, "verifier-tmp")); err != nil {
		return err
	}
	target := Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	environment := archiveBuildEnvironment(manifest, target, source, gitDirectory, filepath.Join(scratch, "verifier-cache"), filepath.Join(scratch, "verifier-tmp"))
	_, stderr, err := runContained(ctx, "go", []string{"build", "-trimpath", "-o", output, "./conformance/release-artifact-v0/archive-verifier"}, environment, source)
	if err != nil {
		return fmt.Errorf("build offline verifier: %w: %s", err, strings.TrimSpace(string(stderr)))
	}
	return nil
}

func runOfflineVerifier(ctx context.Context, verifierPath, scratch string, request archivewire.Request, targetID string) (archivewire.Result, error) {
	directory := filepath.Join(scratch, "verify-"+targetID)
	requestPath := filepath.Join(directory, "request.json")
	resultPath := filepath.Join(directory, "result.json")
	requestBytes, err := canonicalJSON(request)
	if err != nil {
		return archivewire.Result{}, err
	}
	if err := writePrivateFile(requestPath, requestBytes, 0o600); err != nil {
		return archivewire.Result{}, err
	}
	_, stderr, runErr := runContained(ctx, verifierPath, []string{"--request", requestPath, "--result", resultPath}, []string{"HOME=" + directory, "LC_ALL=C", "TMPDIR=" + directory}, directory)
	resultBytes, readErr := os.ReadFile(resultPath)
	if readErr != nil {
		return archivewire.Result{}, errors.Join(runErr, readErr)
	}
	var result archivewire.Result
	if err := archivewire.DecodeStrict(resultBytes, &result); err != nil {
		return archivewire.Result{}, err
	}
	canonical, err := canonicalJSON(result)
	if err != nil || !bytes.Equal(canonical, resultBytes) || result.Schema != archivewire.ResultSchema {
		return archivewire.Result{}, fmt.Errorf("offline-verifier-result-invalid")
	}
	if err := validateOfflineVerifierResult(request, result); err != nil {
		return archivewire.Result{}, err
	}
	if runErr != nil || result.Verdict != statusPass {
		return result, fmt.Errorf("%s: %s", result.Reason, strings.TrimSpace(string(stderr)))
	}
	return result, nil
}

func validateOfflineVerifierResult(request archivewire.Request, result archivewire.Result) error {
	if result.Schema != archivewire.ResultSchema || result.Verdict != statusPass || result.Reason != "" ||
		!digestPattern.MatchString(result.ArchiveSHA256) || !digestPattern.MatchString(result.BinarySHA256) ||
		result.ArchiveBytes <= 0 || result.ArchiveBytes > request.MaximumArchiveBytes ||
		result.BinaryBytes <= 0 || result.BinaryBytes > request.MaximumContentBytes {
		return fmt.Errorf("offline-verifier-result-invalid")
	}
	archive, err := os.ReadFile(request.ArchiveAPath)
	if err != nil || int64(len(archive)) != result.ArchiveBytes || digestBytes(archive) != result.ArchiveSHA256 {
		return fmt.Errorf("offline-verifier-archive-result-mismatch")
	}
	binary, err := os.ReadFile(request.BinaryAPath)
	if err != nil || int64(len(binary)) != result.BinaryBytes || digestBytes(binary) != result.BinarySHA256 {
		return fmt.Errorf("offline-verifier-binary-result-mismatch")
	}
	return nil
}

func archiveBuildEnvironment(manifest Manifest, target Target, source, gitDirectory, cache, temporary string) []string {
	values := map[string]string{
		"GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_SYSTEM": os.DevNull,
		"GIT_DIR": gitDirectory, "GIT_INDEX_FILE": filepath.Join(temporary, "git-index"), "GIT_NO_REPLACE_OBJECTS": "1", "GIT_OPTIONAL_LOCKS": "0",
		"GIT_TERMINAL_PROMPT": "0", "GIT_WORK_TREE": source,
		"GOARCH": target.GOARCH, "GOCACHE": cache, "GOMODCACHE": filepath.Join(temporary, "modcache"),
		"GOOS": target.GOOS, "HOME": temporary, "LC_ALL": "C", "PATH": os.Getenv("PATH"), "TMPDIR": temporary,
	}
	for key, value := range manifest.Profile.Environment {
		values[key] = value
	}
	environment := make([]string, 0, len(values))
	for key, value := range values {
		environment = append(environment, key+"="+value)
	}
	sort.Strings(environment)
	return environment
}

func initializeBuildIndex(ctx context.Context, source, gitDirectory, commit, temporary string) error {
	if !gitObjectPattern.MatchString(commit) {
		return fmt.Errorf("build commit is not a full immutable object ID")
	}
	if err := os.MkdirAll(temporary, 0o700); err != nil {
		return err
	}
	index := filepath.Join(temporary, "git-index")
	environment := []string{
		"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=" + os.DevNull,
		"GIT_DIR=" + gitDirectory, "GIT_INDEX_FILE=" + index, "GIT_LITERAL_PATHSPECS=1",
		"GIT_NO_REPLACE_OBJECTS=1", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0",
		"GIT_WORK_TREE=" + source, "HOME=" + temporary, "LC_ALL=C", "PATH=" + os.Getenv("PATH"),
		"TMPDIR=" + temporary, "XDG_CONFIG_HOME=" + temporary,
	}
	_, stderr, err := runContained(ctx, "git", []string{"--no-replace-objects", "-c", "core.attributesFile=" + os.DevNull, "read-tree", commit}, environment, source)
	if err != nil {
		return fmt.Errorf("initialize private build index: %w: %s", err, strings.TrimSpace(string(stderr)))
	}
	if err := os.Chmod(index, 0o600); err != nil {
		return fmt.Errorf("protect private build index: %w", err)
	}
	return nil
}

func archiveMembers(root, binaryName string, binary []byte, legal []archivewire.ExpectedFile) []archivebuild.File {
	files := []archivebuild.File{
		{Name: path.Join(root, binaryName), Mode: 0o755, Data: binary},
		{Name: path.Join(root, "SHA256SUMS"), Mode: 0o644, Data: []byte(digestBytes(binary) + "  " + binaryName + "\n")},
	}
	for _, file := range legal {
		files = append(files, archivebuild.File{Name: path.Join(root, file.Name), Mode: 0o644, Data: file.Data})
	}
	return files
}

func assembleArchive(format string, files []archivebuild.File) ([]byte, error) {
	if format == "zip" {
		return archivebuild.ZIP(files)
	}
	return archivebuild.TarGzip(files)
}

func assembleArchiveTo(format string, files []archivebuild.File, output string) ([]byte, error) {
	content, err := assembleArchive(format, files)
	if err != nil {
		return nil, err
	}
	if err := writePrivateFile(output, content, 0o600); err != nil {
		return nil, err
	}
	return content, nil
}

func archiveTargetName(target Target) (string, string) {
	base := "corvint_" + target.GOOS + "_" + target.GOARCH
	if target.GOOS == "windows" {
		return base + ".zip", "zip"
	}
	return base + ".tar.gz", "tar.gz"
}

func resolveCleanState(ctx context.Context, options ArchiveOptions, scratch string) (gitState, error) {
	commitBytes, err := closedGit(ctx, options.Root, scratch, "rev-parse", "--verify", options.Revision+"^{commit}")
	if err != nil {
		return gitState{}, err
	}
	commit := strings.TrimSpace(string(commitBytes))
	if !gitObjectPattern.MatchString(commit) {
		return gitState{}, fmt.Errorf("revision did not resolve to one full immutable commit")
	}
	headBytes, err := closedGit(ctx, options.Root, scratch, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil || strings.TrimSpace(string(headBytes)) != commit {
		return gitState{}, fmt.Errorf("selected revision is not the checked-out HEAD")
	}
	treeBytes, err := closedGit(ctx, options.Root, scratch, "rev-parse", "--verify", commit+"^{tree}")
	if err != nil {
		return gitState{}, err
	}
	status, err := closedGit(ctx, options.Root, scratch, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil || len(status) != 0 {
		return gitState{}, fmt.Errorf("worktree is not clean")
	}
	return gitState{commit: commit, tree: strings.TrimSpace(string(treeBytes))}, nil
}

func recheckCleanState(ctx context.Context, options ArchiveOptions, scratch string, want gitState) error {
	got, err := resolveCleanState(ctx, options, scratch)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("commit or tree changed before retention")
	}
	return nil
}

func checkArchiveToolchain(ctx context.Context, root, scratch string, manifest Manifest) error {
	environment := closedToolEnvironment(scratch)
	stdout, stderr, err := runContained(ctx, "go", []string{"env", "GOVERSION"}, environment, root)
	if err != nil {
		return fmt.Errorf("go env GOVERSION: %w: %s", err, strings.TrimSpace(string(stderr)))
	}
	if strings.TrimSpace(string(stdout)) != manifest.Toolchain.GoVersion {
		return fmt.Errorf("selected Go version differs from manifest")
	}
	return nil
}

func closedToolEnvironment(home string) []string {
	return []string{"GOENV=off", "GOFLAGS=-mod=readonly", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "HOME=" + home, "LC_ALL=C", "PATH=" + os.Getenv("PATH"), "TMPDIR=" + home}
}

func closedGit(ctx context.Context, root, home string, arguments ...string) ([]byte, error) {
	argv := append([]string{"--no-replace-objects", "-c", "advice.graftFileDeprecated=false", "-c", "core.attributesFile=" + os.DevNull}, arguments...)
	environment := []string{"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_LITERAL_PATHSPECS=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE=" + os.DevNull, "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "HOME=" + home, "LC_ALL=C", "PATH=" + os.Getenv("PATH"), "TMPDIR=" + home, "XDG_CONFIG_HOME=" + home}
	stdout, stderr, err := runContained(ctx, "git", argv, environment, root)
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(string(stderr)))
	}
	return stdout, nil
}

func exportRawCommit(ctx context.Context, root, home, commit, destination string) error {
	if err := os.Mkdir(destination, 0o700); err != nil {
		return err
	}
	raw, err := closedGit(ctx, root, home, "ls-tree", "-r", "-z", "--full-tree", commit)
	if err != nil {
		return err
	}
	records := bytes.Split(raw, []byte{0})
	entries := make([]treeEntry, 0, len(records))
	var objectInput strings.Builder
	for _, record := range records {
		if len(record) == 0 {
			continue
		}
		space := bytes.IndexByte(record, ' ')
		secondSpace := bytes.IndexByte(record[space+1:], ' ')
		if secondSpace >= 0 {
			secondSpace += space + 1
		}
		tab := bytes.IndexByte(record, '\t')
		if space <= 0 || secondSpace <= space+1 || tab <= secondSpace+1 || string(record[space+1:secondSpace]) != "blob" {
			return fmt.Errorf("invalid ls-tree record")
		}
		entry := treeEntry{mode: string(record[:space]), oid: string(record[secondSpace+1 : tab]), name: string(record[tab+1:])}
		if (entry.mode != "100644" && entry.mode != "100755") || path.Clean(entry.name) != entry.name || strings.HasPrefix(entry.name, "../") {
			return fmt.Errorf("unsupported raw tree entry")
		}
		entries = append(entries, entry)
		objectInput.WriteString(entry.oid + "\n")
	}
	return catFileBatchExport(ctx, root, home, destination, entries, []byte(objectInput.String()))
}

func catFileBatchExport(ctx context.Context, root, home, destination string, entries []treeEntry, objectInput []byte) error {
	command := exec.CommandContext(ctx, "git", "--no-replace-objects", "cat-file", "--batch")
	command.Dir = root
	command.Env = []string{"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_NO_REPLACE_OBJECTS=1", "GIT_OPTIONAL_LOCKS=0", "HOME=" + home, "LC_ALL=C", "PATH=" + os.Getenv("PATH"), "TMPDIR=" + home}
	command.Stdin = bytes.NewReader(objectInput)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	stderr := &boundedBuffer{limit: maximumCommandOutput}
	command.Stderr = stderr
	if err := configureContainedCommand(command); err != nil {
		return err
	}
	if err := command.Start(); err != nil {
		return err
	}
	finished := false
	defer func() {
		if !finished {
			_ = cleanupContainedCommand(command, time.Second)
			_ = command.Wait()
		}
	}()
	reader := bufio.NewReader(stdout)
	for _, entry := range entries {
		header, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[0] != entry.oid || fields[1] != "blob" {
			return fmt.Errorf("cat-file response mismatch")
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || size < 0 || size > 128<<20 {
			return fmt.Errorf("cat-file blob size invalid")
		}
		content := make([]byte, size)
		if _, err := io.ReadFull(reader, content); err != nil {
			return err
		}
		terminator, err := reader.ReadByte()
		if err != nil || terminator != '\n' {
			return fmt.Errorf("cat-file response terminator invalid")
		}
		destinationPath := filepath.Join(destination, filepath.FromSlash(entry.name))
		if err := os.MkdirAll(filepath.Dir(destinationPath), 0o700); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if entry.mode == "100755" {
			mode = 0o755
		}
		if err := os.WriteFile(destinationPath, content, mode); err != nil {
			return err
		}
	}
	waitErr := command.Wait()
	finished = true
	cleanupErr := cleanupContainedCommand(command, time.Second)
	if waitErr != nil {
		return fmt.Errorf("git cat-file: %w: %s", waitErr, stderr.String())
	}
	return cleanupErr
}

func loadCommittedLegal(source string, manifest Manifest) ([]archivewire.ExpectedFile, error) {
	files := make([]archivewire.ExpectedFile, 0, len(manifest.Legal))
	for _, legal := range manifest.Legal {
		content, err := os.ReadFile(filepath.Join(source, legal.Path))
		if err != nil {
			return nil, err
		}
		if digestBytes(content) != legal.SHA256 {
			return nil, fmt.Errorf("legal digest mismatch")
		}
		files = append(files, archivewire.ExpectedFile{Name: legal.Path, Mode: 0o644, Data: content})
	}
	return files, nil
}

func equalLegal(left, right []archivewire.ExpectedFile) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Name != right[index].Name || !bytes.Equal(left[index].Data, right[index].Data) {
			return false
		}
	}
	return true
}

func validateArchiveDestination(ctx context.Context, root, destination string) (string, error) {
	parent := filepath.Dir(destination)
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	parentReal, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	if parentReal == rootReal || strings.HasPrefix(parentReal, rootReal+string(os.PathSeparator)) {
		return "", fmt.Errorf("output parent is inside repository")
	}
	gitHome, err := os.MkdirTemp("", "corvint-release-git-boundary-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(gitHome)
	gitDirectoryBytes, err := closedGit(ctx, root, gitHome, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	gitDirectoryReal, err := filepath.EvalSymlinks(strings.TrimSpace(string(gitDirectoryBytes)))
	if err != nil {
		return "", err
	}
	if parentReal == gitDirectoryReal || strings.HasPrefix(parentReal, gitDirectoryReal+string(os.PathSeparator)) {
		return "", fmt.Errorf("output parent is inside the private Git directory")
	}
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
		return "", fmt.Errorf("output parent must be an existing real 0700 directory")
	}
	if err := invokingUserOwns(info); err != nil {
		return "", err
	}
	if _, err := os.Lstat(destination); err == nil || !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("output destination must not exist")
	}
	return parent, nil
}

func retainArchiveOutput(ctx context.Context, options ArchiveOptions, parent string, report ArchiveReport, archives map[string][]byte, scratch string, state gitState) (returnedErr error) {
	parentRoot, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer parentRoot.Close()
	stage, err := mkdirPrivateTempRoot(parentRoot, ".corvint-release-go-stage-")
	if err != nil {
		return err
	}
	keepStage := false
	defer func() {
		if !keepStage {
			if cleanupErr := parentRoot.RemoveAll(stage); cleanupErr != nil {
				returnedErr = errors.Join(returnedErr, fmt.Errorf("stage cleanup: %w", cleanupErr))
			}
		}
	}()
	names := make([]string, 0, len(archives))
	for name := range archives {
		names = append(names, name)
	}
	sort.Strings(names)
	var sums strings.Builder
	for _, name := range names {
		content := archives[name]
		if err := writeSyncedExclusiveRoot(parentRoot, path.Join(stage, name), content, 0o644); err != nil {
			return err
		}
		sums.WriteString(digestBytes(content) + "  " + name + "\n")
	}
	if err := writeSyncedExclusiveRoot(parentRoot, path.Join(stage, "SHA256SUMS"), []byte(sums.String()), 0o644); err != nil {
		return err
	}
	if err := validateArchiveReport(report); err != nil {
		return err
	}
	reportBytes, err := canonicalJSON(report)
	if err != nil || int64(len(reportBytes)) > maximumArchiveReportBytes {
		return fmt.Errorf("archive report exceeds bound")
	}
	if err := writeSyncedExclusiveRoot(parentRoot, path.Join(stage, archiveReportName), reportBytes, 0o644); err != nil {
		return err
	}
	if err := syncRootSubdirectory(parentRoot, stage); err != nil {
		return err
	}
	if err := recheckCleanState(ctx, options, scratch, state); err != nil {
		return err
	}
	if _, err := validateArchiveDestination(ctx, options.Root, options.Output); err != nil {
		return err
	}
	parentInfo, err := parentRoot.Lstat(".")
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode().Perm() != 0o700 || invokingUserOwns(parentInfo) != nil {
		return fmt.Errorf("held output parent changed before retention")
	}
	pathInfo, err := os.Lstat(parent)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(parentInfo, pathInfo) {
		return fmt.Errorf("output parent path no longer names held directory")
	}
	stageInfo, err := parentRoot.Lstat(stage)
	if err != nil || !stageInfo.IsDir() || stageInfo.Mode().Perm() != 0o700 || invokingUserOwns(stageInfo) != nil {
		return fmt.Errorf("staged output identity or permissions changed")
	}
	destinationName := filepath.Base(options.Output)
	if _, err := parentRoot.Lstat(destinationName); err == nil || !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("output destination appeared before retention")
	}
	if err := renameRootNoReplace(parentRoot, stage, destinationName); err != nil {
		return err
	}
	keepStage = true
	if err := syncRootDirectory(parentRoot); err != nil {
		_ = parentRoot.RemoveAll(destinationName)
		_ = syncRootDirectory(parentRoot)
		return err
	}
	return nil
}

func mkdirPrivateTempRoot(root *os.Root, prefix string) (string, error) {
	for range 100 {
		var random [16]byte
		if _, err := cryptorand.Read(random[:]); err != nil {
			return "", err
		}
		name := prefix + hex.EncodeToString(random[:])
		if err := root.Mkdir(name, 0o700); err == nil {
			return name, nil
		} else if !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("stage name collision limit reached")
}

func syncRootSubdirectory(root *os.Root, name string) error {
	directory, err := root.Open(name)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func syncRootDirectory(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func writePrivateFile(name string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		return err
	}
	return os.WriteFile(name, content, mode)
}

func writeSyncedExclusive(name string, content []byte, mode os.FileMode) error {
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(content)
	syncErr := file.Sync()
	closeErr := file.Close()
	return errors.Join(writeErr, syncErr, closeErr)
}

func writeSyncedExclusiveRoot(root *os.Root, name string, content []byte, mode os.FileMode) error {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(content)
	syncErr := file.Sync()
	closeErr := file.Close()
	return errors.Join(writeErr, syncErr, closeErr)
}

func syncDirectory(name string) error {
	directory, err := os.Open(name)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func failArchive(report ArchiveReport, code, target string, err error) (ArchiveReport, error) {
	report.Verdict = statusFail
	report.Reasons = append(report.Reasons, ArchiveReason{Code: code, Target: target})
	if err == nil {
		err = errors.New(code)
	}
	return report, err
}

func classifyArchiveError(err error) string {
	value := err.Error()
	if index := strings.IndexByte(value, ':'); index >= 0 {
		value = value[:index]
	}
	for _, allowed := range []string{"double-build-mismatch", "double-assembly-mismatch", "buildinfo-identity-mismatch", "archive-resource-limit", "member-resource-limit", "noncanonical-container-framing-or-metadata"} {
		if value == allowed {
			return allowed
		}
	}
	return "target-verification-failed"
}

func digestReport(content []byte) ArchiveDigest {
	return ArchiveDigest{SHA256: digestBytes(content), Bytes: int64(len(content))}
}

func digestBytes(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func repoRelative(root, filename string) string {
	relative, err := filepath.Rel(root, filename)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(relative)
}
