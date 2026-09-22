package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/conformance/release-artifact-v0/archivewire"
)

var archiveRepositoryFixture struct {
	once       sync.Once
	directory  string
	repository string
	err        error
	output     []byte
}

func TestMain(m *testing.M) {
	code := m.Run()
	if archiveRepositoryFixture.directory != "" {
		_ = os.RemoveAll(archiveRepositoryFixture.directory)
	}
	os.Exit(code)
}

func testRawCommitExportBuildCarriesExactVCSIdentity(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/releasefixture\n\ngo 1.27.1\n")
	mustWrite(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	mustWrite(t, filepath.Join(root, "unrelated.txt"), "committed\n")
	runGitTest(t, root, "init", "-q")
	runGitTest(t, root, "add", "go.mod", "main.go", "unrelated.txt")
	runGitTest(t, root, "-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.invalid", "commit", "-qm", "fixture")
	scratch := t.TempDir()
	commitRaw, err := closedGit(context.Background(), root, scratch, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	commit := strings.TrimSpace(string(commitRaw))
	mustWrite(t, filepath.Join(root, "unrelated.txt"), "staged\n")
	runGitTest(t, root, "add", "unrelated.txt")
	liveIndex := filepath.Join(root, ".git", "index")
	indexBefore, err := os.ReadFile(liveIndex)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(scratch, "source")
	if err := exportRawCommit(context.Background(), root, scratch, commit, source); err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{Profile: Profile{
		Package:    ".",
		BinaryName: "fixture",
		BuildFlags: []string{"-trimpath"},
		Environment: map[string]string{
			"CGO_ENABLED": "0", "GOENV": "off", "GOFLAGS": "-mod=readonly",
			"GOPROXY": "off", "GOSUMDB": "off", "GOTOOLCHAIN": "local",
		},
	}}
	if content, err := os.ReadFile(filepath.Join(source, "unrelated.txt")); err != nil || string(content) != "committed\n" {
		t.Fatalf("raw export used staged content: %q %v", content, err)
	}
	target := Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	if target.GOOS == "windows" {
		target.Suffix = ".exe"
	}
	gitDirectoryRaw, err := closedGit(context.Background(), root, scratch, "rev-parse", "--absolute-git-dir")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(scratch, "out", manifest.Profile.BinaryName+target.Suffix)
	if err := archiveBuildOnce(context.Background(), source, strings.TrimSpace(string(gitDirectoryRaw)), commit, manifest, target, output, filepath.Join(scratch, "cache"), filepath.Join(scratch, "tmp")); err != nil {
		t.Fatal(err)
	}
	info, err := readBuildInfo(output)
	if err != nil {
		t.Fatal(err)
	}
	if info["vcs.revision"] != commit || info["vcs.modified"] != "false" {
		status := exec.Command("git", "--git-dir="+strings.TrimSpace(string(gitDirectoryRaw)), "--work-tree="+source, "status", "--porcelain=v1", "--untracked-files=all")
		statusOutput, statusErr := status.CombinedOutput()
		t.Logf("export status err=%v output=%q", statusErr, statusOutput)
		t.Fatalf("build info revision=%q modified=%q", info["vcs.revision"], info["vcs.modified"])
	}
	indexAfter, err := os.ReadFile(liveIndex)
	if err != nil || !bytes.Equal(indexAfter, indexBefore) {
		t.Fatalf("raw export build changed live index: %v", err)
	}
}

func TestArchiveDestinationRequiresTrustedAbsentExternalChild(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(parent, "release")
	if got, err := validateArchiveDestination(context.Background(), root, destination); err != nil || got != parent {
		t.Fatalf("valid boundary got=%q err=%v", got, err)
	}
	if err := os.Chmod(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := validateArchiveDestination(context.Background(), root, destination); err == nil {
		t.Fatal("world-readable parent accepted")
	}
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := validateArchiveDestination(context.Background(), root, destination); err == nil {
		t.Fatal("existing destination accepted")
	}
}

func TestArchiveReportAndWitnessAreCanonicalAndPathFree(t *testing.T) {
	report := ArchiveReport{
		Schema: archiveReportSchema, Revision: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40), ManifestSHA256: strings.Repeat("c", 64), Toolchain: "go1.27.1", Verdict: statusPass,
		Targets: []ArchiveTargetReport{{GOOS: "linux", GOARCH: "amd64", BinaryName: "corvint", ArchiveName: "corvint_linux_amd64.tar.gz", RetainedArchive: ArchiveDigest{SHA256: strings.Repeat("d", 64), Bytes: 10}}}, Reasons: []ArchiveReason{},
	}
	first, err := canonicalJSON(report)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := canonicalJSON(report)
	if string(first) != string(second) || !strings.HasSuffix(string(first), "\n") || strings.Contains(string(first), t.TempDir()) {
		t.Fatalf("noncanonical or path-bearing report %s", first)
	}
	var decoded map[string]any
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatal(err)
	}
	witness := report.witness()
	if witness.Schema != archiveWitnessSchema || len(witness.Archives) != 1 || witness.Archives[0].Name != report.Targets[0].ArchiveName {
		t.Fatalf("witness %+v", witness)
	}
}

func TestArchiveWitnessStrictSchemaAndPassInvariants(t *testing.T) {
	names := []string{
		"corvint_darwin_amd64.tar.gz", "corvint_darwin_arm64.tar.gz",
		"corvint_linux_amd64.tar.gz", "corvint_linux_arm64.tar.gz",
		"corvint_windows_amd64.zip",
	}
	witness := ArchiveWitness{Schema: archiveWitnessSchema, Revision: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40), Verdict: statusPass, Archives: []ArchiveWitnessDigest{}, Reasons: []string{}}
	for _, name := range names {
		witness.Archives = append(witness.Archives, ArchiveWitnessDigest{Name: name, SHA256: strings.Repeat("c", 64), Bytes: 1})
	}
	content, err := canonicalJSON(witness)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "witness.json")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readArchiveWitness(path); err != nil {
		t.Fatalf("valid witness rejected: %v", err)
	}
	for name, mutate := range map[string]func([]byte) []byte{
		"unknown": func(raw []byte) []byte {
			return []byte(strings.Replace(string(raw), `"schema":`, `"unknown":1,"schema":`, 1))
		},
		"duplicate": func(raw []byte) []byte {
			return []byte(strings.Replace(string(raw), `"schema":`, `"schema":"corvint.release-go-archive-witness.v1","schema":`, 1))
		},
		"trailing":    func(raw []byte) []byte { return append(raw, ' ') },
		"wrong-order": func(raw []byte) []byte { return []byte(strings.Replace(string(raw), names[0], names[1], 1)) },
	} {
		t.Run(name, func(t *testing.T) {
			changed := mutate(append([]byte(nil), content...))
			if err := os.WriteFile(path, changed, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := readArchiveWitness(path); err == nil {
				t.Fatal("malformed witness accepted")
			}
		})
	}
}

func TestWitnessWriteFailureDoesNotChangeComputedVerdict(t *testing.T) {
	t.Parallel()
	report := ArchiveReport{Schema: archiveReportSchema, Revision: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40), Verdict: statusPass, Targets: []ArchiveTargetReport{}, Reasons: []ArchiveReason{}}
	err := writeArchiveWitnessBestEffort(t.TempDir(), report.witness())
	if err == nil {
		t.Fatal("non-repository witness root unexpectedly succeeded")
	}
	if report.Verdict != statusPass {
		t.Fatalf("witness failure relabelled verdict: %s", report.Verdict)
	}
}

func TestRetainArchiveOutputIsAtomicAndRefusesCollision(t *testing.T) {
	t.Parallel()
	root := initArchiveTestRepository(t)
	parent := t.TempDir()
	_ = os.Chmod(parent, 0o700)
	output := filepath.Join(parent, "retained")
	options := ArchiveOptions{Root: root, Output: output, Revision: "HEAD"}
	scratch := t.TempDir()
	state, err := resolveCleanState(context.Background(), options, scratch)
	if err != nil {
		t.Fatal(err)
	}
	report, archives := validArchiveReportForTest(state)
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(output, "sentinel"), "do-not-overwrite")
	err = retainArchiveOutput(context.Background(), options, parent, report, archives, scratch, state)
	if err == nil {
		t.Fatal("collision accepted")
	}
	if content, err := os.ReadFile(filepath.Join(output, "sentinel")); err != nil || string(content) != "do-not-overwrite" {
		t.Fatalf("collision target changed: %q %v", content, err)
	}
	entries, _ := os.ReadDir(parent)
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "stage") {
			t.Fatalf("staging residue %s", entry.Name())
		}
	}
}

func TestAtomicNoReplacePromotionPreservesRacingDestination(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("atomic no-replace promotion is supported on Darwin and Linux")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := root.Mkdir("stage", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := root.Mkdir("destination", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("destination/sentinel", []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := renameRootNoReplace(root, "stage", "destination"); err == nil {
		t.Fatal("atomic no-replace promotion overwrote racing destination")
	}
	content, err := root.ReadFile("destination/sentinel")
	if err != nil || string(content) != "unchanged" {
		t.Fatalf("racing destination changed: %q %v", content, err)
	}
	if info, err := root.Lstat("stage"); err != nil || !info.IsDir() {
		t.Fatalf("failed promotion consumed stage: %v %v", info, err)
	}
	if err := root.RemoveAll("destination"); err != nil {
		t.Fatal(err)
	}
	if err := renameRootNoReplace(root, "stage", "destination"); err != nil {
		t.Fatalf("absent-destination promotion failed: %v", err)
	}
}

func TestRetainArchiveOutputPublishesCompleteDirectory(t *testing.T) {
	t.Parallel()
	root := initArchiveTestRepository(t)
	parent := t.TempDir()
	_ = os.Chmod(parent, 0o700)
	options := ArchiveOptions{Root: root, Output: filepath.Join(parent, "retained"), Revision: "HEAD"}
	scratch := t.TempDir()
	state, err := resolveCleanState(context.Background(), options, scratch)
	if err != nil {
		t.Fatal(err)
	}
	report, archives := validArchiveReportForTest(state)
	if err := retainArchiveOutput(context.Background(), options, parent, report, archives, scratch, state); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(options.Output)
	if err != nil || len(entries) != 7 {
		t.Fatalf("retained entries=%d err=%v", len(entries), err)
	}
	info, err := os.Stat(options.Output)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("retained directory mode=%v err=%v", info.Mode().Perm(), err)
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o644 {
			t.Fatalf("retained member %s mode=%v err=%v", entry.Name(), info.Mode(), err)
		}
	}
}

func TestBuildArchivesCancellationRemovesScratchStageAndOutput(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = buildArchives(ctx, ArchiveOptions{Root: root, ManifestPath: "conformance/release-artifact-v0/manifest.json", Output: filepath.Join(parent, "retained"), Revision: "HEAD"})
	if err == nil {
		t.Fatal("canceled archive build succeeded")
	}
	entries, readErr := os.ReadDir(parent)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("canceled build residue=%v err=%v", entries, readErr)
	}
}

func TestArchiveBuildPathsProveSeparateColdInputs(t *testing.T) {
	manifest, err := loadManifest("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	target := Target{GOOS: "linux", GOARCH: "amd64"}
	environmentA := environmentMap(archiveBuildEnvironment(manifest, target, "/source-a", "/git", "/cache-a", "/tmp-a"))
	environmentB := environmentMap(archiveBuildEnvironment(manifest, target, "/source-b", "/git", "/cache-b", "/tmp-b"))
	for _, key := range []string{"GOCACHE", "GOMODCACHE", "HOME", "TMPDIR", "GIT_INDEX_FILE", "GIT_WORK_TREE"} {
		if environmentA[key] == "" || environmentA[key] == environmentB[key] {
			t.Fatalf("%s is not independently rooted: %q", key, environmentA[key])
		}
	}
	buildA := filepath.Join("/scratch", "build-a", target.id(), "corvint")
	buildB := filepath.Join("/scratch", "build-b", target.id(), "corvint")
	assemblyA := filepath.Join("/scratch", "assembly-a", target.id(), "corvint_linux_amd64.tar.gz")
	assemblyB := filepath.Join("/scratch", "assembly-b", target.id(), "corvint_linux_amd64.tar.gz")
	for name, pair := range map[string][2]string{"build": {buildA, buildB}, "assembly": {assemblyA, assemblyB}} {
		if filepath.Dir(pair[0]) == filepath.Dir(pair[1]) || pair[0] == pair[1] {
			t.Fatalf("%s outputs share identity", name)
		}
	}
}

func TestLooseReproducibilityGateBindingRejectsReportAndByteMismatch(t *testing.T) {
	binary := []byte("binary")
	digest := digestReport(binary)
	artifact := filepath.Join(t.TempDir(), "corvint-linux-amd64")
	if err := os.WriteFile(artifact, binary, 0o600); err != nil {
		t.Fatal(err)
	}
	loose := TargetReport{GOOS: "linux", GOARCH: "amd64", Artifact: artifact, BuildA: BuildReport{SHA256: digest.SHA256, Bytes: digest.Bytes}, BuildB: BuildReport{SHA256: digest.SHA256, Bytes: digest.Bytes}, ByteIdentical: true, SHA256: digest.SHA256}
	target := Target{GOOS: "linux", GOARCH: "amd64"}
	if err := bindLooseGateTarget(loose, target, binary); err != nil {
		t.Fatalf("valid loose gate binding rejected: %v", err)
	}
	changedReport := loose
	changedReport.SHA256 = strings.Repeat("a", 64)
	if err := bindLooseGateTarget(changedReport, target, binary); err == nil {
		t.Fatal("mismatched loose gate report accepted")
	}
	if err := os.WriteFile(artifact, []byte("other"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := bindLooseGateTarget(loose, target, binary); err == nil {
		t.Fatal("mismatched loose gate bytes accepted")
	}
}

func TestRunOfflineVerifierConsumesCanonicalBoundResult(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test verifier fixture uses /bin/sh")
	}
	scratch := t.TempDir()
	archivePath, binaryPath := filepath.Join(scratch, "archive-a", "archive"), filepath.Join(scratch, "binary-a", "binary")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, []byte("archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	request := archivewire.Request{Schema: archivewire.RequestSchema, ArchiveAPath: archivePath, BinaryAPath: binaryPath, MaximumArchiveBytes: 64, MaximumContentBytes: 64}
	result := archivewire.Result{Schema: archivewire.ResultSchema, Verdict: statusPass, ArchiveSHA256: digestBytes([]byte("archive")), ArchiveBytes: 7, BinarySHA256: digestBytes([]byte("binary")), BinaryBytes: 6}
	resultBytes, err := canonicalJSON(result)
	if err != nil {
		t.Fatal(err)
	}
	verifier := writeVerifierFixture(t, resultBytes, 0)
	if _, err := runOfflineVerifier(context.Background(), verifier, scratch, request, "integration-pass"); err != nil {
		t.Fatalf("canonical verifier result rejected: %v", err)
	}
	malformed := append(append([]byte(nil), resultBytes...), ' ')
	verifier = writeVerifierFixture(t, malformed, 0)
	if _, err := runOfflineVerifier(context.Background(), verifier, scratch, request, "integration-malformed"); err == nil {
		t.Fatal("malformed verifier result accepted")
	}
}

func TestOfflineVerifierResultInvariantMatrix(t *testing.T) {
	directory := t.TempDir()
	archivePath, binaryPath := filepath.Join(directory, "archive"), filepath.Join(directory, "binary")
	if err := os.WriteFile(archivePath, []byte("archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	request := archivewire.Request{ArchiveAPath: archivePath, BinaryAPath: binaryPath, MaximumArchiveBytes: 64, MaximumContentBytes: 64}
	valid := archivewire.Result{Schema: archivewire.ResultSchema, Verdict: statusPass, ArchiveSHA256: digestBytes([]byte("archive")), ArchiveBytes: 7, BinarySHA256: digestBytes([]byte("binary")), BinaryBytes: 6}
	if err := validateOfflineVerifierResult(request, valid); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*archivewire.Result){
		"reason":         func(result *archivewire.Result) { result.Reason = "unexpected" },
		"archive-digest": func(result *archivewire.Result) { result.ArchiveSHA256 = strings.Repeat("a", 64) },
		"binary-digest":  func(result *archivewire.Result) { result.BinarySHA256 = strings.Repeat("b", 64) },
		"archive-size":   func(result *archivewire.Result) { result.ArchiveBytes++ },
		"binary-size":    func(result *archivewire.Result) { result.BinaryBytes++ },
	} {
		t.Run(name, func(t *testing.T) {
			changed := valid
			mutate(&changed)
			if err := validateOfflineVerifierResult(request, changed); err == nil {
				t.Fatal("invalid verifier result accepted")
			}
		})
	}
}

func environmentMap(values []string) map[string]string {
	result := map[string]string{}
	for _, value := range values {
		key, body, _ := strings.Cut(value, "=")
		result[key] = body
	}
	return result
}

func writeVerifierFixture(t *testing.T, result []byte, exitCode int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "verifier")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s' '%s' > \"$4\"\nexit %d\n", string(result), exitCode)
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func validArchiveReportForTest(state gitState) (ArchiveReport, map[string][]byte) {
	rows := []struct{ goos, goarch, binary, archive string }{
		{"darwin", "amd64", "corvint", "corvint_darwin_amd64.tar.gz"},
		{"darwin", "arm64", "corvint", "corvint_darwin_arm64.tar.gz"},
		{"linux", "amd64", "corvint", "corvint_linux_amd64.tar.gz"},
		{"linux", "arm64", "corvint", "corvint_linux_arm64.tar.gz"},
		{"windows", "amd64", "corvint.exe", "corvint_windows_amd64.zip"},
	}
	report := ArchiveReport{Schema: archiveReportSchema, Revision: state.commit, Tree: state.tree, ManifestSHA256: strings.Repeat("a", 64), Toolchain: "go1.27.1", Verdict: statusPass, Limits: ArchiveLimits{Targets: 5, MembersPerArchive: 6, MaximumArchiveBytes: 100, MaximumTotalBytes: 500, MaximumReportBytes: maximumArchiveReportBytes, MaximumWitnessBytes: maxWitnessBytes}, Reasons: []ArchiveReason{}}
	archives := map[string][]byte{}
	for _, row := range rows {
		content := []byte(row.archive)
		archiveDigest := digestReport(content)
		binaryDigest := digestReport([]byte(row.binary))
		report.Targets = append(report.Targets, ArchiveTargetReport{GOOS: row.goos, GOARCH: row.goarch, BinaryName: row.binary, ArchiveName: row.archive, BuildA: binaryDigest, BuildB: binaryDigest, AssemblyA: archiveDigest, AssemblyB: archiveDigest, RetainedBinary: binaryDigest, RetainedArchive: archiveDigest})
		archives[row.archive] = content
	}
	return report, archives
}

func TestArchiveReportRejectsInvalidLimits(t *testing.T) {
	report, _ := validArchiveReportForTest(gitState{commit: strings.Repeat("a", 40), tree: strings.Repeat("b", 40)})
	if err := validateArchiveReport(report); err != nil {
		t.Fatalf("valid report rejected: %v", err)
	}
	for name, mutate := range map[string]func(*ArchiveReport){
		"targets":       func(report *ArchiveReport) { report.Limits.Targets = 0 },
		"members":       func(report *ArchiveReport) { report.Limits.MembersPerArchive = 0 },
		"archive-zero":  func(report *ArchiveReport) { report.Limits.MaximumArchiveBytes = 0 },
		"archive-hard":  func(report *ArchiveReport) { report.Limits.MaximumArchiveBytes = archivewire.HardMaxArchiveBytes + 1 },
		"total-small":   func(report *ArchiveReport) { report.Limits.MaximumTotalBytes = 1 },
		"total-hard":    func(report *ArchiveReport) { report.Limits.MaximumTotalBytes = archivewire.HardMaxArchiveBytes*5 + 1 },
		"report-bytes":  func(report *ArchiveReport) { report.Limits.MaximumReportBytes++ },
		"witness-bytes": func(report *ArchiveReport) { report.Limits.MaximumWitnessBytes++ },
	} {
		t.Run(name, func(t *testing.T) {
			changed := report
			mutate(&changed)
			if err := validateArchiveReport(changed); err == nil {
				t.Fatal("invalid report limits accepted")
			}
		})
	}
}

func TestArchiveManifestRejectsEveryAuthorityDrift(t *testing.T) {
	mutations := []struct{ old, new string }{
		{`"goVersion": "go1.27.1"`, `"goVersion": "go1.28.0"`},
		{`"goModDirective": "1.27.1"`, `"goModDirective": "1.28.0"`},
		{`"package": "./cmd/corvint"`, `"package": "./cmd/other"`},
		{`"modulePath": "github.com/Beamfall/corvint"`, `"modulePath": "example.com/corvint"`},
		{`"binaryName": "corvint"`, `"binaryName": "corvint2"`},
		{`"GOSUMDB": "off"`, `"GOSUMDB": "sum.golang.org"`},
		{`"binarySuffix": ".exe"`, `"binarySuffix": ""`},
	}
	for _, mutation := range mutations {
		body := strings.Replace(validManifest, mutation.old, mutation.new, 1)
		if body == validManifest {
			t.Fatalf("fixture missing %s", mutation.old)
		}
		if _, err := loadManifest(writeManifest(t, body)); err == nil {
			t.Fatalf("authority drift accepted: %s", mutation.new)
		}
	}
}

func TestClosedGitEnvironmentAndRawExportIgnoreAmbientState(t *testing.T) {
	root := initArchiveTestRepository(t)
	scratch := t.TempDir()
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "hostile"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "hostile-config"))
	state, err := resolveCleanState(context.Background(), ArchiveOptions{Root: root, Revision: "HEAD"}, scratch)
	if err != nil {
		t.Fatalf("closed Git environment admitted ambient state: %v", err)
	}
	infoAttributes := filepath.Join(root, ".git", "info", "attributes")
	if err := os.MkdirAll(filepath.Dir(infoAttributes), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(infoAttributes, []byte("tracked.txt export-ignore\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored.go"), []byte("package hostile\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	export := filepath.Join(scratch, "export")
	if err := exportRawCommit(context.Background(), root, scratch, state.commit, export); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(filepath.Join(export, "tracked.txt")); err != nil || string(content) != "tracked\n" {
		t.Fatalf("raw tracked blob missing: %q %v", content, err)
	}
	if _, err := os.Stat(filepath.Join(export, "ignored.go")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ignored ambient file entered raw export: %v", err)
	}
}

// ARTIFACT-GO-V0-009: Go stamps vcs.modified from `git status --porcelain`, so
// only a path Git ignores is disjoint from the build; a disjoint-looking
// untracked path still refuses.
func TestCleanStateAllowsIgnoredButRefusesAnyUntrackedPath(t *testing.T) {
	root := initArchiveTestRepository(t)
	scratch := t.TempDir()
	options := ArchiveOptions{Root: root, Revision: "HEAD"}
	mustWrite(t, filepath.Join(root, "ignored.go"), "package ignored\n")
	if _, err := resolveCleanState(context.Background(), options, scratch); err != nil {
		t.Fatalf("ignored path refused: %v", err)
	}
	mustWrite(t, filepath.Join(root, "notes.md"), "notes\n")
	if _, err := resolveCleanState(context.Background(), options, scratch); err == nil {
		t.Fatal("untracked path outside every Go package was admitted")
	}
}

func TestFinalGitRecheckRejectsDirtyStagedUntrackedAndHeadMovement(t *testing.T) {
	for _, mutation := range []string{"dirty", "staged", "untracked", "head"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			root := initArchiveTestRepository(t)
			scratch := t.TempDir()
			options := ArchiveOptions{Root: root, Revision: "HEAD"}
			state, err := resolveCleanState(context.Background(), options, scratch)
			if err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "dirty":
				mustWrite(t, filepath.Join(root, "tracked.txt"), "changed\n")
			case "staged":
				mustWrite(t, filepath.Join(root, "tracked.txt"), "changed\n")
				runGitTest(t, root, "add", "tracked.txt")
			case "untracked":
				mustWrite(t, filepath.Join(root, "new.txt"), "new\n")
			case "head":
				mustWrite(t, filepath.Join(root, "tracked.txt"), "next\n")
				runGitTest(t, root, "add", "tracked.txt")
				runGitTest(t, root, "-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.invalid", "commit", "-m", "next")
			}
			if err := recheckCleanState(context.Background(), options, scratch, state); err == nil {
				t.Fatalf("%s mutation survived final recheck", mutation)
			}
		})
	}
}

func initArchiveTestRepository(t *testing.T) string {
	t.Helper()
	fixture := sharedArchiveTestRepository(t)
	root := filepath.Join(t.TempDir(), "repository")
	runGitTest(t, ".", "clone", "-q", "--shared", fixture, root)
	return root
}

func sharedArchiveTestRepository(t *testing.T) string {
	t.Helper()
	archiveRepositoryFixture.once.Do(func() {
		archiveRepositoryFixture.directory, archiveRepositoryFixture.err = os.MkdirTemp("", "release-artifact-v0-")
		if archiveRepositoryFixture.err != nil {
			return
		}
		archiveRepositoryFixture.repository = filepath.Join(archiveRepositoryFixture.directory, "repository")
		archiveRepositoryFixture.err = os.Mkdir(archiveRepositoryFixture.repository, 0o700)
		if archiveRepositoryFixture.err != nil {
			return
		}
		commands := [][]string{
			{"init", archiveRepositoryFixture.repository},
			{"-C", archiveRepositoryFixture.repository, "add", ".gitignore", "tracked.txt"},
			{"-C", archiveRepositoryFixture.repository, "-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.invalid", "commit", "-m", "base"},
		}
		if err := os.WriteFile(filepath.Join(archiveRepositoryFixture.repository, ".gitignore"), []byte("ignored.go\n"), 0o600); err != nil {
			archiveRepositoryFixture.err = err
			return
		}
		if err := os.WriteFile(filepath.Join(archiveRepositoryFixture.repository, "tracked.txt"), []byte("tracked\n"), 0o600); err != nil {
			archiveRepositoryFixture.err = err
			return
		}
		for _, arguments := range commands {
			command := exec.Command("git", arguments...)
			command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
			archiveRepositoryFixture.output, archiveRepositoryFixture.err = command.CombinedOutput()
			if archiveRepositoryFixture.err != nil {
				return
			}
		}
	})
	if archiveRepositoryFixture.err != nil {
		t.Fatalf("initialize shared archive repository: %v: %s", archiveRepositoryFixture.err, archiveRepositoryFixture.output)
	}
	return archiveRepositoryFixture.repository
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runGitTest(t *testing.T, root string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(arguments, " "), err, output)
	}
}

func TestRawCommitExportBuildCarriesExactVCSIdentity(t *testing.T) {
	t.Run("GOC-V0-006 raw committed source identity", testRawCommitExportBuildCarriesExactVCSIdentity)
}
