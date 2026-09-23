package releasecandidate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/companionrelease"
)

func TestPUBV0023ClosedManifestRunsIsolatedHostProbe(t *testing.T) {
	t.Run("PUB-V0-023 closed manifest and source inventory", testPUBV0023ClosedManifestRunsIsolatedHostProbe)
}

func testPUBV0023ClosedManifestRunsIsolatedHostProbe(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("host probe fixture requires a POSIX shell")
	}
	hostArchive := "corvint_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	hostShipped := false
	for _, archive := range shippedCoreArchives {
		hostShipped = hostShipped || archive == hostArchive
	}
	if !hostShipped {
		t.Skip("host target is outside the qualified core set")
	}

	versionOutput := "Corvint 0.5.0a1 (build 9)"
	hostBinary := []byte("#!/bin/sh\nprintf '%s\\n' '" + versionOutput + "'\n")
	coreBinary := func(target coreTarget) []byte {
		if target.GOOS == runtime.GOOS && target.GOARCH == runtime.GOARCH {
			return hostBinary
		}
		return []byte("binary-" + target.BinaryName + "-" + target.GOOS + "-" + target.GOARCH)
	}
	previousCore := verifyCoreBinary
	verifyCoreBinary = func(_ []byte, target coreTarget) ([]byte, error) { return coreBinary(target), nil }
	t.Cleanup(func() { verifyCoreBinary = previousCore })

	corvintCommit, corvintTree := strings.Repeat("a", 40), strings.Repeat("b", 40)
	tasks := SourceIdentity{Name: "corvint-tasks", Commit: strings.Repeat("c", 40), Tree: strings.Repeat("d", 40)}
	corvintSource, tasksSource := []byte("corvint source\n"), []byte("tasks source\n")
	previousCompanion := loadCompanionEvidence
	loadCompanionEvidence = func(directory string) (*candidateCompanionEvidence, error) {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return nil, err
		}
		if len(entries) != 3 {
			t.Fatalf("companion verifier received %d files, expected 3", len(entries))
		}
		return &candidateCompanionEvidence{
			manifest: companionManifestFixture(), corvintCommit: corvintCommit, corvintTree: corvintTree,
			corvintSourcePath: "source/corvint-src.tar.gz", tasks: tasks, tasksSourcePath: "source/corvint-tasks-src.tar.gz",
			entries: map[string][]byte{"source/corvint-src.tar.gz": corvintSource, "source/corvint-tasks-src.tar.gz": tasksSource},
		}, nil
	}
	t.Cleanup(func() { loadCompanionEvidence = previousCompanion })

	directory := closedCandidateFixture(t, versionOutput, corvintCommit, corvintTree, tasks, corvintSource, tasksSource, coreBinary)
	verified, err := VerifyContext(context.Background(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Manifest.CorvintVersion != versionOutput || len(verified.Qualification.Rows) != 28 {
		t.Fatalf("unexpected verified candidate: %#v", verified)
	}
}

func TestPRSV1005CoreOnlyCandidateVerifiesAndInstalls(t *testing.T) {
	t.Run("PRS-V1-005 Core-only candidate needs no companion input", testPRSV1005CoreOnlyCandidateVerifiesAndInstalls)
}

func testPRSV1005CoreOnlyCandidateVerifiesAndInstalls(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("host probe fixture requires a POSIX shell")
	}
	platform := runtime.GOOS + "_" + runtime.GOARCH
	versionOutput := "Corvint 0.5.0a1 (build 9)"
	hostProgram := "#!/bin/sh\nprintf '%s\\n' '" + versionOutput + "'\n"
	isHost := func(target coreTarget) bool { return target.GOOS == runtime.GOOS && target.GOARCH == runtime.GOARCH }
	coreBinary := func(target coreTarget) []byte {
		if isHost(target) {
			return []byte(hostProgram)
		}
		return []byte("binary-" + target.BinaryName + "-" + target.GOOS + "-" + target.GOARCH)
	}
	hostArchive := coreScriptArchive(t, platform, hostProgram)
	coreArchive := func(target coreTarget) []byte {
		if isHost(target) {
			return hostArchive
		}
		return []byte("archive-" + target.ArchiveName)
	}
	previousCore := verifyCoreBinary
	verifyCoreBinary = func(_ []byte, target coreTarget) ([]byte, error) { return coreBinary(target), nil }
	t.Cleanup(func() { verifyCoreBinary = previousCore })
	previousCompanion := loadCompanionEvidence
	loadCompanionEvidence = func(string) (*candidateCompanionEvidence, error) {
		t.Fatal("Core-only candidate consulted companion evidence")
		return nil, nil
	}
	t.Cleanup(func() { loadCompanionEvidence = previousCompanion })

	directory := coreOnlyCandidateFixture(t, versionOutput, strings.Repeat("a", 40), strings.Repeat("b", 40), coreBinary, coreArchive, nil)
	verified, err := VerifyContext(t.Context(), directory)
	if err != nil {
		t.Fatalf("Core-only candidate refused: %v", err)
	}
	for _, row := range verified.Qualification.Rows {
		if row.Workflow == "companion-bundle" && (row.Status != "NOT_RUN" || row.Evidence != "companion not present") {
			t.Fatalf("absent companion reported as %s/%s", row.Status, row.Evidence)
		}
	}
	installed, err := InstallCore(t.Context(), directory, filepath.Join(canonicalTemp(t), "store"))
	if err != nil || !strings.HasSuffix(installed, filepath.Join("0.5.0a1", runtime.GOOS+"-"+runtime.GOARCH)) {
		t.Fatalf("Core-only candidate not installed: %q %v", installed, err)
	}

	companionClaim := verified.Qualification
	companionClaim.Rows = append([]QualificationRow(nil), companionClaim.Rows...)
	companionClaim.Rows[1] = QualificationRow{Platform: "darwin/amd64", Workflow: "companion-bundle", Status: "PASS", Evidence: "README.md"}
	if err := validateQualification(companionClaim, candidateProfiles[coreManifestProfile]); err == nil {
		t.Fatal("Core-only receipt claiming a companion PASS accepted")
	}
	if err := os.WriteFile(filepath.Join(directory, "core", "corvint_linux_arm64.tar.gz"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyContext(t.Context(), directory); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("Core-only candidate with a drifted core archive accepted: %v", err)
	}
}

func TestPRSV1005CoreOnlyCandidateRefusals(t *testing.T) {
	t.Run("PRS-V1-005 Core-only candidate refuses companion and inventory drift", testPRSV1005CoreOnlyCandidateRefusals)
}

func testPRSV1005CoreOnlyCandidateRefusals(t *testing.T) {
	previousCore := verifyCoreBinary
	verifyCoreBinary = func(_ []byte, target coreTarget) ([]byte, error) {
		return []byte("binary-" + target.GOOS + "-" + target.GOARCH), nil
	}
	t.Cleanup(func() { verifyCoreBinary = previousCore })
	previousCompanion := loadCompanionEvidence
	loadCompanionEvidence = func(string) (*candidateCompanionEvidence, error) {
		t.Fatal("refused candidate consulted companion evidence")
		return nil, nil
	}
	t.Cleanup(func() { loadCompanionEvidence = previousCompanion })
	commit, tree := strings.Repeat("a", 40), strings.Repeat("b", 40)
	coreBinary := func(target coreTarget) []byte { return []byte("binary-" + target.GOOS + "-" + target.GOARCH) }
	coreArchive := func(target coreTarget) []byte { return []byte("archive-" + target.ArchiveName) }
	tasks := SourceIdentity{Name: "corvint-tasks", Commit: strings.Repeat("c", 40), Tree: strings.Repeat("d", 40)}
	type mutation = func(files map[string][]byte, roles map[string]string, manifest *Manifest, qualification *Qualification)
	move := func(from, to, role string) mutation {
		return func(files map[string][]byte, roles map[string]string, _ *Manifest, _ *Qualification) {
			files[to], roles[to] = files[from], role
			delete(files, from)
			delete(roles, from)
		}
	}
	for _, test := range []struct {
		name, want string
		directory  func() string
	}{
		{"combined profile on Core inventory", "manifest identity is invalid", func() string {
			return coreOnlyCandidateFixture(t, "Corvint 0.5.0a1 (build 9)", commit, tree, coreBinary, coreArchive, func(_ map[string][]byte, _ map[string]string, manifest *Manifest, _ *Qualification) {
				manifest.Profile = manifestProfile
			})
		}},
		{"Core profile on combined inventory", "manifest identity is invalid", func() string {
			directory := closedCandidateFixture(t, "Corvint 0.5.0a1 (build 9)", commit, tree, tasks, []byte("corvint source\n"), []byte("tasks source\n"), coreBinary)
			resealCandidateProfile(t, directory, coreManifestProfile)
			return directory
		}},
		{"Core profile with two sources", "manifest identity is invalid", func() string {
			return coreOnlyCandidateFixture(t, "Corvint 0.5.0a1 (build 9)", commit, tree, coreBinary, coreArchive, func(_ map[string][]byte, _ map[string]string, manifest *Manifest, _ *Qualification) {
				manifest.Sources = append(manifest.Sources, tasks)
			})
		}},
		{"Core profile listing a companion archive", "role companion-archive is not admitted", func() string {
			return coreOnlyCandidateFixture(t, "Corvint 0.5.0a1 (build 9)", commit, tree, coreBinary, coreArchive, func(files map[string][]byte, roles map[string]string, _ *Manifest, _ *Qualification) {
				files["companion/corvint-companion.tar.gz"], roles["companion/corvint-companion.tar.gz"] = []byte("companion archive\n"), "companion-archive"
			})
		}},
		{"Corvint source on a Tasks archive path", "role corvint-source is not at source/corvint-src.tar.gz", func() string {
			return coreOnlyCandidateFixture(t, "Corvint 0.5.0a1 (build 9)", commit, tree, coreBinary, coreArchive, move("source/corvint-src.tar.gz", "companion/corvint-tasks_darwin_arm64.tar.gz", "corvint-source"))
		}},
		{"release notes on a companion smoke path", "role release-notes is not at README.md", func() string {
			return coreOnlyCandidateFixture(t, "Corvint 0.5.0a1 (build 9)", commit, tree, coreBinary, coreArchive, move("README.md", "evidence/corvint-companion.smoke.json", "release-notes"))
		}},
		{"companion row with other NOT_RUN evidence", "invalid qualification row darwin/amd64/companion-bundle", func() string {
			return coreOnlyCandidateFixture(t, "Corvint 0.5.0a1 (build 9)", commit, tree, coreBinary, coreArchive, func(_ map[string][]byte, _ map[string]string, _ *Manifest, qualification *Qualification) {
				qualification.Rows[1].Evidence = "companion target unavailable"
			})
		}},
		{"missing qualification row", "qualification profile or row count is invalid", func() string {
			return coreOnlyCandidateFixture(t, "Corvint 0.5.0a1 (build 9)", commit, tree, coreBinary, coreArchive, func(_ map[string][]byte, _ map[string]string, _ *Manifest, qualification *Qualification) {
				qualification.Rows = qualification.Rows[:len(qualification.Rows)-1]
			})
		}},
		{"duplicate qualification row", "invalid qualification row darwin/amd64/core-archive", func() string {
			return coreOnlyCandidateFixture(t, "Corvint 0.5.0a1 (build 9)", commit, tree, coreBinary, coreArchive, func(_ map[string][]byte, _ map[string]string, _ *Manifest, qualification *Qualification) {
				qualification.Rows[len(qualification.Rows)-1] = qualification.Rows[0]
			})
		}},
		{"bogus qualification status", "invalid qualification row darwin/amd64/version-identity", func() string {
			return coreOnlyCandidateFixture(t, "Corvint 0.5.0a1 (build 9)", commit, tree, coreBinary, coreArchive, func(_ map[string][]byte, _ map[string]string, _ *Manifest, qualification *Qualification) {
				qualification.Rows[2].Status = "SKIPPED"
			})
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := VerifyContext(t.Context(), test.directory()); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected refusal containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestPUBV0022VerifyCoreRequiresClosedReproducibleChecksummedSet(t *testing.T) {
	t.Run("PUB-V0-022 closed reproducible core set", testPUBV0022VerifyCoreRequiresClosedReproducibleChecksummedSet)
}

func testPUBV0022VerifyCoreRequiresClosedReproducibleChecksummedSet(t *testing.T) {
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
	t.Run("PUB-V0-026 failed input retains no candidate", testPUBV0026FailedInputRetainsNoCandidate)
}

func testPUBV0026FailedInputRetainsNoCandidate(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "output")
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := Assemble(t.Context(), Options{
		CoreDirectory: validCoreFixture(t), CompanionDirectory: filepath.Join(root, "missing"),
		SourceRoot: root, Scratch: root, OutputParent: output, Version: "0.5.0a1",
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
	notes := candidateReadme("0.5.0a1")
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

func companionManifestFixture() companionrelease.BundleManifest {
	return companionrelease.BundleManifest{GoVersion: "go1.27.1", GitVersion: "git version fixture"}
}

func closedCandidateFixture(t *testing.T, versionOutput, corvintCommit, corvintTree string, tasks SourceIdentity, corvintSource, tasksSource []byte, coreBinary func(coreTarget) []byte) string {
	t.Helper()
	files, roles := map[string][]byte{}, map[string]string{}
	add := func(name, role string, raw []byte) {
		files[name], roles[name] = raw, role
	}
	addCoreCandidateFixture(t, add, corvintCommit, corvintTree, coreBinary, func(target coreTarget) []byte { return []byte("archive-" + target.ArchiveName) })
	add("companion/corvint-companion.tar.gz", "companion-archive", []byte("companion archive\n"))
	add("companion/corvint-companion.tar.gz.sha256", "companion-checksum", []byte("companion checksum\n"))
	add("evidence/corvint-companion.smoke.json", "companion-installed-smoke", []byte("companion smoke\n"))
	add("source/corvint-src.tar.gz", "corvint-source", corvintSource)
	add("source/corvint-tasks-src.tar.gz", "corvint-tasks-source", tasksSource)

	qualification := Qualification{Profile: qualificationProfile}
	workflows := []string{"core-archive", "companion-bundle", "version-identity", "affected-selection", "playwright-external-discovery", "documentation-corpus-discovery", "work-queue-observation"}
	for _, platform := range []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64"} {
		for _, workflow := range workflows {
			status, evidence := "NOT_RUN", "workflow unavailable"
			if workflow == "core-archive" {
				status, evidence = "PASS", "evidence/core-verification-report.json"
			} else if platform == "darwin/arm64" {
				status, evidence = "PASS", "evidence/corvint-companion.smoke.json"
				if workflow == "companion-bundle" {
					evidence = "companion/corvint-companion.tar.gz"
				}
			}
			qualification.Rows = append(qualification.Rows, QualificationRow{Platform: platform, Workflow: workflow, Status: status, Evidence: evidence})
		}
	}
	qualificationRaw, err := canonicalJSON(qualification)
	if err != nil {
		t.Fatal(err)
	}
	add("QUALIFICATION.json", "qualification-receipt", qualificationRaw)
	add("README.md", "release-notes", []byte(candidateReadme("0.5.0a1")))
	manifest := Manifest{
		Profile: manifestProfile, Version: "0.5.0a1", BuildNumber: "9", CorvintVersion: versionOutput, GoVersion: "go1.27.1", GitVersion: "git version fixture",
		Sources: []SourceIdentity{{Name: "corvint", Commit: corvintCommit, Tree: corvintTree}, tasks}, Assets: assetsFor(files, roles),
	}
	manifestRaw, err := canonicalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	files["MANIFEST.json"] = manifestRaw
	files["SHA256SUMS"] = renderChecksums(files)
	directory := t.TempDir()
	if err := writeCandidate(directory, files); err != nil {
		t.Fatal(err)
	}
	return directory
}

func addCoreCandidateFixture(t *testing.T, add func(name, role string, raw []byte), corvintCommit, corvintTree string, coreBinary, coreArchive func(coreTarget) []byte) {
	t.Helper()
	report := coreReport{
		Schema: "corvint.release-go-archive-report.v2", Revision: corvintCommit, Tree: corvintTree, ManifestSHA256: strings.Repeat("e", 64), Toolchain: "go1.27.1", Verdict: "PASS",
		Limits: coreLimits{Targets: 5, MembersPerArchive: 6, MaximumArchiveBytes: 1 << 20, MaximumTotalBytes: 5 << 20, MaximumReportBytes: 1 << 20, MaximumWitnessBytes: 1 << 16}, Reasons: []coreReason{},
	}
	targets := []struct{ goos, goarch, binary, archive string }{
		{"darwin", "amd64", "corvint", "corvint_darwin_amd64.tar.gz"},
		{"darwin", "arm64", "corvint", "corvint_darwin_arm64.tar.gz"},
		{"linux", "amd64", "corvint", "corvint_linux_amd64.tar.gz"},
		{"linux", "arm64", "corvint", "corvint_linux_arm64.tar.gz"},
		{"windows", "amd64", "corvint.exe", "corvint_windows_amd64.zip"},
	}
	coreArchives := map[string][]byte{}
	for _, item := range targets {
		target := coreTarget{GOOS: item.goos, GOARCH: item.goarch, BinaryName: item.binary, ArchiveName: item.archive}
		raw := coreArchive(target)
		coreArchives[item.archive] = raw
		binary := coreBinary(target)
		target.BuildA = coreDigest{SHA256: digest(binary), Bytes: int64(len(binary))}
		target.BuildB, target.RetainedBinary = target.BuildA, target.BuildA
		target.AssemblyA = coreDigest{SHA256: digest(raw), Bytes: int64(len(raw))}
		target.AssemblyB, target.RetainedArchive = target.AssemblyA, target.AssemblyA
		report.Targets = append(report.Targets, target)
	}
	for _, archive := range shippedCoreArchives {
		add("core/"+archive, "core-archive", coreArchives[archive])
	}
	add("evidence/core-SHA256SUMS", "core-gate-checksums", renderChecksums(coreArchives))
	reportRaw, err := canonicalJSON(report)
	if err != nil {
		t.Fatal(err)
	}
	add("evidence/core-verification-report.json", "core-gate-report", reportRaw)
}

func coreOnlyCandidateFixture(t *testing.T, versionOutput, corvintCommit, corvintTree string, coreBinary, coreArchive func(coreTarget) []byte, mutate func(files map[string][]byte, roles map[string]string, manifest *Manifest, qualification *Qualification)) string {
	t.Helper()
	files, roles := map[string][]byte{}, map[string]string{}
	add := func(name, role string, raw []byte) {
		files[name], roles[name] = raw, role
	}
	addCoreCandidateFixture(t, add, corvintCommit, corvintTree, coreBinary, coreArchive)
	add("source/corvint-src.tar.gz", "corvint-source", []byte("corvint source\n"))
	add("README.md", "release-notes", []byte(candidateReadme("0.5.0a1")))
	qualification := Qualification{Profile: qualificationProfile}
	workflows := []string{"core-archive", "companion-bundle", "version-identity", "affected-selection", "playwright-external-discovery", "documentation-corpus-discovery", "work-queue-observation"}
	for _, platform := range []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64"} {
		for _, workflow := range workflows {
			status, evidence := "NOT_RUN", "companion not present"
			if workflow == "core-archive" {
				status, evidence = "PASS", "evidence/core-verification-report.json"
			}
			qualification.Rows = append(qualification.Rows, QualificationRow{Platform: platform, Workflow: workflow, Status: status, Evidence: evidence})
		}
	}
	manifest := Manifest{
		Profile: coreManifestProfile, Version: "0.5.0a1", BuildNumber: "9", CorvintVersion: versionOutput, GoVersion: "go1.27.1", GitVersion: "git version fixture",
		Sources: []SourceIdentity{{Name: "corvint", Commit: corvintCommit, Tree: corvintTree}},
	}
	if mutate != nil {
		mutate(files, roles, &manifest, &qualification)
	}
	qualificationRaw, err := canonicalJSON(qualification)
	if err != nil {
		t.Fatal(err)
	}
	add("QUALIFICATION.json", "qualification-receipt", qualificationRaw)
	manifest.Assets = assetsFor(files, roles)
	manifestRaw, err := canonicalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	files["MANIFEST.json"] = manifestRaw
	files["SHA256SUMS"] = renderChecksums(files)
	directory := t.TempDir()
	if err := writeCandidate(directory, files); err != nil {
		t.Fatal(err)
	}
	return directory
}

// resealCandidateProfile rewrites only the manifest profile and reseals the
// candidate checksums, so the profile check alone decides admission.
func resealCandidateProfile(t *testing.T, directory, profile string) {
	t.Helper()
	files, _, err := readCandidateFiles(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := decodeClosed(files["MANIFEST.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Profile = profile
	if files["MANIFEST.json"], err = canonicalJSON(manifest); err != nil {
		t.Fatal(err)
	}
	delete(files, "SHA256SUMS")
	files["SHA256SUMS"] = renderChecksums(files)
	if err := writeCandidate(directory, files); err != nil {
		t.Fatal(err)
	}
}
