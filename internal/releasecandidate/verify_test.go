package releasecandidate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPUBV0022VerifyCoreRequiresClosedReproducibleChecksummedSet(t *testing.T) {
	previous := verifyCoreBinary
	verifyCoreBinary = func(_ []byte, target coreTarget) ([]byte, error) {
		return []byte("binary-" + target.BinaryName), nil
	}
	t.Cleanup(func() { verifyCoreBinary = previous })
	directory := validCoreFixture(t)
	report, files, err := verifyCore(directory)
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "PASS" || len(files) != 7 {
		t.Fatalf("incomplete verified core: %#v files=%d", report, len(files))
	}

	path := filepath.Join(directory, "corvint_linux_amd64.tar.gz")
	if err := os.WriteFile(path, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := verifyCore(directory); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("changed core archive accepted: %v", err)
	}
}

func TestPUBV0022CoreArchiveVerifierRejectsNonArchiveBytes(t *testing.T) {
	if _, err := verifyCoreArchiveBinary([]byte("not an archive"), coreTarget{GOOS: "darwin", GOARCH: "arm64", BinaryName: "corvint", ArchiveName: "corvint_darwin_arm64.tar.gz"}); err == nil {
		t.Fatal("non-archive bytes accepted as a qualified core archive")
	}
}

func TestPUBV0026FailedInputRetainsNoCandidate(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "output")
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := Assemble(t.Context(), Options{
		CoreDirectory: validCoreFixture(t), CompanionDirectory: filepath.Join(root, "missing"),
		SourceRoot: root, Scratch: root, OutputParent: output, Version: "0.4.0a4",
	})
	if err == nil {
		t.Fatal("missing companion accepted")
	}
	entries, readErr := os.ReadDir(output)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("failed candidate retained output: %v %#v", readErr, entries)
	}
}

func TestPUBV0025PromotionNeverReplacesExistingCandidate(t *testing.T) {
	parent := t.TempDir()
	if err := os.Mkdir(filepath.Join(parent, "stage"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(parent, "candidate"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := promoteNoReplace(parent, "stage", "candidate"); err == nil {
		t.Fatal("existing candidate was replaced")
	}
}

func TestPUBV0026ScratchAndOutputCannotOverlapInputs(t *testing.T) {
	root := t.TempDir()
	for name, options := range map[string]Options{
		"scratch in source": {SourceRoot: root, CoreDirectory: filepath.Join(root, "core"), CompanionDirectory: filepath.Join(root, "companion"), Scratch: filepath.Join(root, "scratch"), OutputParent: filepath.Join(t.TempDir(), "out")},
		"output in source":  {SourceRoot: root, CoreDirectory: filepath.Join(root, "core"), CompanionDirectory: filepath.Join(root, "companion"), Scratch: t.TempDir(), OutputParent: filepath.Join(root, "out")},
	} {
		t.Run(name, func(t *testing.T) {
			for _, directory := range []string{options.CoreDirectory, options.CompanionDirectory, options.Scratch} {
				if err := os.MkdirAll(directory, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := validateScratchSeparation(options); err == nil {
				t.Fatal("overlapping release path accepted")
			}
		})
	}
}

func TestPUBV0007CandidateNotesPreserveRequiredDisclosures(t *testing.T) {
	notes := candidateReadme("0.4.0a4")
	for _, disclosure := range []string{"Experimental", "local Git executable and object database", "Performance is unmeasured", "hosted CI is unavailable/NOT_RUN"} {
		if !strings.Contains(notes, disclosure) {
			t.Fatalf("candidate notes omit %q", disclosure)
		}
	}
}

func validCoreFixture(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	files := map[string][]byte{}
	expected := []struct{ goos, goarch, binary, archive string }{
		{"darwin", "amd64", "corvint", "corvint_darwin_amd64.tar.gz"},
		{"darwin", "arm64", "corvint", "corvint_darwin_arm64.tar.gz"},
		{"linux", "amd64", "corvint", "corvint_linux_amd64.tar.gz"},
		{"linux", "arm64", "corvint", "corvint_linux_arm64.tar.gz"},
		{"windows", "amd64", "corvint.exe", "corvint_windows_amd64.zip"},
	}
	report := coreReport{
		Schema: "corvint.release-go-archive-report.v2", Revision: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40), ManifestSHA256: strings.Repeat("c", 64), Toolchain: "go1.27.1", Verdict: "PASS",
		Limits: coreLimits{Targets: 5, MembersPerArchive: 6, MaximumArchiveBytes: 1 << 20, MaximumTotalBytes: 5 << 20, MaximumReportBytes: 1 << 20, MaximumWitnessBytes: 1 << 16}, Reasons: []coreReason{},
	}
	for _, item := range expected {
		raw := []byte("archive-" + item.archive)
		files[item.archive] = raw
		archive := coreDigest{SHA256: digest(raw), Bytes: int64(len(raw))}
		binary := coreDigest{SHA256: digest([]byte("binary-" + item.binary)), Bytes: int64(len("binary-" + item.binary))}
		report.Targets = append(report.Targets, coreTarget{GOOS: item.goos, GOARCH: item.goarch, BinaryName: item.binary, ArchiveName: item.archive, BuildA: binary, BuildB: binary, AssemblyA: archive, AssemblyB: archive, RetainedBinary: binary, RetainedArchive: archive})
	}
	files["SHA256SUMS"] = renderChecksums(files)
	reportRaw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	files["verification-report.json"] = append(reportRaw, '\n')
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(directory, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}
