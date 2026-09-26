package releasecandidate

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type readinessFixture struct {
	candidate, source, evidence, commit, tree string
}

// newReadinessFixture binds a 1.0.0-rc.1 Core-only candidate to a one-commit
// source root carrying goMod, with the host toolchain probe pinned.
func newReadinessFixture(t *testing.T, goMod string) readinessFixture {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("host probe fixture requires a POSIX shell")
	}
	source := readinessSource(t, firstStableCandidate, goMod)
	commit, tree := readinessGit(t, source, "rev-parse", "HEAD"), readinessGit(t, source, "rev-parse", "HEAD^{tree}")
	versionOutput := "Corvint " + firstStableCandidate + " (build 1)"
	hostProgram := "#!/bin/sh\nprintf '%s\\n' '" + versionOutput + "'\n"
	isHost := func(target coreTarget) bool { return target.GOOS == runtime.GOOS && target.GOARCH == runtime.GOARCH }
	coreBinary := func(target coreTarget) []byte {
		if isHost(target) {
			return []byte(hostProgram)
		}
		return []byte("binary-" + target.GOOS + "-" + target.GOARCH)
	}
	hostArchive := coreScriptArchive(t, runtime.GOOS+"_"+runtime.GOARCH, hostProgram)
	coreArchive := func(target coreTarget) []byte {
		if isHost(target) {
			return hostArchive
		}
		return []byte("archive-" + target.ArchiveName)
	}
	previousCore, previousProbe := verifyCoreBinary, goToolchainProbe
	verifyCoreBinary = func(_ []byte, target coreTarget) ([]byte, error) { return coreBinary(target), nil }
	goToolchainProbe = func(context.Context, string) (string, error) { return pinnedToolchain, nil }
	t.Cleanup(func() { verifyCoreBinary, goToolchainProbe = previousCore, previousProbe })
	candidate := coreOnlyCandidateFixture(t, versionOutput, commit, tree, coreBinary, coreArchive, func(files map[string][]byte, _ map[string]string, manifest *Manifest, _ *Qualification) {
		manifest.Version, manifest.BuildNumber = firstStableCandidate, "1"
		files["README.md"] = []byte(candidateReadme(firstStableCandidate))
	})
	return readinessFixture{candidate: candidate, source: source, evidence: canonicalTemp(t), commit: commit, tree: tree}
}

func readinessSource(t *testing.T, version, goMod string) string {
	t.Helper()
	source := canonicalTemp(t)
	readinessGit(t, source, "init", "-q")
	for name, content := range map[string]string{"VERSION": version + "\n", "go.mod": goMod} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	readinessGit(t, source, "add", "VERSION", "go.mod")
	readinessGit(t, source, "commit", "-q", "-m", "fixture")
	return source
}

func readinessGit(t *testing.T, source string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", source}, arguments...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", arguments, err, out)
	}
	return strings.TrimSpace(string(out))
}

const cleanGoMod = "module example.invalid/core\n\ngo 1.27.1\n"

func (f readinessFixture) build(t *testing.T, evidence map[string]ReadinessEvidence) (*ReadinessRecord, []byte) {
	t.Helper()
	record, raw, err := BuildReadinessRecord(t.Context(), ReadinessOptions{CandidateDirectory: f.candidate, SourceRoot: f.source, Evidence: evidence})
	if err != nil {
		t.Fatalf("readiness record refused: %v", err)
	}
	return record, raw
}

func (f readinessFixture) file(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(f.evidence, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readinessRowByID(record *ReadinessRecord, id string) ReadinessRow {
	for _, row := range record.Rows {
		if row.ID == id {
			return row
		}
	}
	return ReadinessRow{}
}

func index(record *ReadinessRecord, id string) int {
	for position, row := range record.Rows {
		if row.ID == id {
			return position
		}
	}
	return -1
}

func resealReadiness(t *testing.T, record ReadinessRecord, mutate func(*ReadinessRecord)) []byte {
	t.Helper()
	mutate(&record)
	raw, err := canonicalJSON(record)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestSRRV1001CanonicalClosedRecord(t *testing.T) {
	t.Run("SRR-V1-001 record is one canonical closed document", testSRRV1001CanonicalClosedRecord)
}

func testSRRV1001CanonicalClosedRecord(t *testing.T) {
	fixture := newReadinessFixture(t, cleanGoMod)
	record, raw := fixture.build(t, nil)
	if _, err := VerifyReadinessRecord(t.Context(), raw, fixture.candidate, nil); err != nil || record.Profile != readinessProfile {
		t.Fatalf("canonical record refused: %v", err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		t.Fatal(err)
	}
	for name, mutated := range map[string][]byte{
		"non-canonical": compact.Bytes(),
		"unknown field": bytes.Replace(raw, []byte(`"profile"`), []byte(`"extra": 1, "profile"`), 1),
		"trailing":      append(append([]byte(nil), raw...), []byte("{}\n")...),
		"profile":       resealReadiness(t, *record, func(r *ReadinessRecord) { r.Profile = "corvint-stable-readiness-record/0" }),
	} {
		if _, err := VerifyReadinessRecord(t.Context(), mutated, fixture.candidate, nil); err == nil {
			t.Fatalf("%s record admitted", name)
		}
	}
}

func TestSRRV1002IdentityBindsSourceRoot(t *testing.T) {
	t.Run("SRR-V1-002 identity is re-derived from the verified candidate and source root", testSRRV1002IdentityBindsSourceRoot)
}

func testSRRV1002IdentityBindsSourceRoot(t *testing.T) {
	fixture := newReadinessFixture(t, cleanGoMod)
	record, _ := fixture.build(t, nil)
	want := ReadinessIdentity{Version: firstStableCandidate, Tag: "v1.0.0-rc.1", BuildNumber: "1", GoVersion: pinnedToolchain, Commit: fixture.commit, Tree: fixture.tree}
	if record.Identity != want {
		t.Fatalf("identity = %#v, want %#v", record.Identity, want)
	}
	other := readinessSource(t, "1.0.0-rc.2", cleanGoMod)
	if _, _, err := BuildReadinessRecord(t.Context(), ReadinessOptions{CandidateDirectory: fixture.candidate, SourceRoot: other}); err == nil {
		t.Fatal("source root without the candidate commit admitted")
	}
	for name, mutate := range map[string]func(*ReadinessRecord){
		"build number": func(r *ReadinessRecord) { r.Identity.BuildNumber = "2" },
		"commit":       func(r *ReadinessRecord) { r.Identity.Commit = strings.Repeat("1", 40) },
		"tree":         func(r *ReadinessRecord) { r.Identity.Tree = strings.Repeat("2", 40) },
	} {
		if _, err := VerifyReadinessRecord(t.Context(), resealReadiness(t, *record, mutate), fixture.candidate, nil); err == nil {
			t.Fatalf("record with a foreign %s admitted", name)
		}
	}
}

func TestSRRV1003CoreBindsVerifiedCandidate(t *testing.T) {
	t.Run("SRR-V1-003 core digests come from the verified candidate only", testSRRV1003CoreBindsVerifiedCandidate)
}

func testSRRV1003CoreBindsVerifiedCandidate(t *testing.T) {
	fixture := newReadinessFixture(t, cleanGoMod)
	record, _ := fixture.build(t, nil)
	sums, err := os.ReadFile(filepath.Join(fixture.candidate, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	if record.Core.CandidateProfile != coreManifestProfile || record.Core.ChecksumsSHA256 != digest(sums) || record.Core.Reproducibility != "PASS" || len(record.Core.Archives) != len(shippedCoreArchives) {
		t.Fatalf("unexpected core binding %#v", record.Core)
	}
	for _, archive := range record.Core.Archives {
		if archive.Role != "core-archive" || !strings.HasPrefix(archive.Path, "core/") {
			t.Fatalf("unexpected archive row %#v", archive)
		}
	}
	tampered := resealReadiness(t, *record, func(r *ReadinessRecord) {
		r.Core.Archives = append([]Asset(nil), r.Core.Archives...)
		r.Core.Archives[0].SHA256 = strings.Repeat("0", 64)
	})
	if _, err := VerifyReadinessRecord(t.Context(), tampered, fixture.candidate, nil); err == nil {
		t.Fatal("record with a foreign archive digest admitted")
	}
}

func TestSRRV1004StoreReleaseBothOrNeither(t *testing.T) {
	t.Run("SRR-V1-004 store release binding records both fields or neither", testSRRV1004StoreReleaseBothOrNeither)
}

func testSRRV1004StoreReleaseBothOrNeither(t *testing.T) {
	fixture := newReadinessFixture(t, cleanGoMod)
	candidateSHA256 := strings.Repeat("a", 64)
	record, raw, err := BuildReadinessRecord(t.Context(), ReadinessOptions{CandidateDirectory: fixture.candidate, SourceRoot: fixture.source, StoreReleaseID: "v1-0", StoreCandidateSHA256: candidateSHA256})
	if err != nil || record.StoreRelease == nil || *record.StoreRelease != (ReadinessStoreRelease{ReleaseID: "v1-0", CandidateSHA256: candidateSHA256}) {
		t.Fatalf("store binding not recorded: %#v %v", record, err)
	}
	if _, err := VerifyReadinessRecord(t.Context(), raw, fixture.candidate, nil); err != nil {
		t.Fatalf("store-bound record refused: %v", err)
	}
	empty := resealReadiness(t, *record, func(r *ReadinessRecord) { r.StoreRelease = &ReadinessStoreRelease{} })
	if _, err := VerifyReadinessRecord(t.Context(), empty, fixture.candidate, nil); err == nil {
		t.Fatal("empty non-null store binding admitted")
	}
	for _, options := range []ReadinessOptions{{StoreReleaseID: "v1-0"}, {StoreCandidateSHA256: candidateSHA256}, {StoreReleaseID: "V1 0", StoreCandidateSHA256: candidateSHA256}} {
		options.CandidateDirectory, options.SourceRoot = fixture.candidate, fixture.source
		if _, _, err := BuildReadinessRecord(t.Context(), options); err == nil {
			t.Fatalf("partial store binding %#v admitted", options)
		}
	}
}

func TestSRRV1005MissingEvidenceIsNotRun(t *testing.T) {
	t.Run("SRR-V1-005 closed row catalogue records missing evidence as NOT_RUN or FALLBACK", testSRRV1005MissingEvidenceIsNotRun)
}

func testSRRV1005MissingEvidenceIsNotRun(t *testing.T) {
	fixture := newReadinessFixture(t, cleanGoMod)
	record, _ := fixture.build(t, nil)
	if len(record.Rows) != len(readinessCatalogue) {
		t.Fatalf("rows = %d, want %d", len(record.Rows), len(readinessCatalogue))
	}
	for index, row := range record.Rows {
		if row.ID != readinessCatalogue[index].id || row.SHA256 != "" || (row.Status != "NOT_RUN" && row.Status != "FALLBACK") || (row.Decision == "" && row.Reason == "") {
			t.Fatalf("row without evidence is not an explained NOT_RUN/FALLBACK: %#v", row)
		}
	}
	for name, mutate := range map[string]func(*ReadinessRecord){
		"reordered":  func(r *ReadinessRecord) { r.Rows = append([]ReadinessRow{r.Rows[1], r.Rows[0]}, r.Rows[2:]...) },
		"duplicated": func(r *ReadinessRecord) { r.Rows = append([]ReadinessRow{r.Rows[0], r.Rows[0]}, r.Rows[2:]...) },
		"truncated":  func(r *ReadinessRecord) { r.Rows = r.Rows[1:] },
	} {
		if _, err := VerifyReadinessRecord(t.Context(), resealReadiness(t, *record, mutate), fixture.candidate, nil); err == nil {
			t.Fatalf("%s row catalogue admitted", name)
		}
	}
	for _, id := range []string{"gate/unknown", "owner/tag"} {
		if _, _, err := BuildReadinessRecord(t.Context(), ReadinessOptions{CandidateDirectory: fixture.candidate, SourceRoot: fixture.source, Evidence: map[string]ReadinessEvidence{id: {Status: "NOT_RUN", Reason: "operator"}}}); err == nil {
			t.Fatalf("operator evidence for %s admitted", id)
		}
	}
}

func TestSRRV1006EvidenceDigestsReverified(t *testing.T) {
	t.Run("SRR-V1-006 status grammar and evidence digests are recomputed from operator files", testSRRV1006EvidenceDigestsReverified)
}

func testSRRV1006EvidenceDigestsReverified(t *testing.T) {
	fixture := newReadinessFixture(t, cleanGoMod)
	log := fixture.file(t, "full-gate.log", "make gate: ok\n")
	record, raw := fixture.build(t, map[string]ReadinessEvidence{"gate/full-gate": {Status: "PASS", Path: log}})
	if row := readinessRowByID(record, "gate/full-gate"); row.Status != "PASS" || row.SHA256 != digest([]byte("make gate: ok\n")) {
		t.Fatalf("full-gate row = %#v", row)
	}
	if _, err := VerifyReadinessRecord(t.Context(), raw, fixture.candidate, map[string]string{"gate/full-gate": log}); err != nil {
		t.Fatalf("reproducible evidence refused: %v", err)
	}
	extra := fixture.file(t, "extra.log", "extra\n")
	for name, evidence := range map[string]map[string]string{
		"missing": nil,
		"extra":   {"gate/full-gate": log, "gate/interop-gate": extra},
		"changed": {"gate/full-gate": extra},
	} {
		if _, err := VerifyReadinessRecord(t.Context(), raw, fixture.candidate, evidence); err == nil {
			t.Fatalf("%s evidence admitted", name)
		}
	}
	for name, supplied := range map[string]ReadinessEvidence{
		"PASS without file":     {Status: "PASS"},
		"NOT_RUN with file":     {Status: "NOT_RUN", Path: log, Reason: "operator"},
		"unexplained NOT_RUN":   {Status: "NOT_RUN"},
		"FALLBACK on gate row":  {Status: "FALLBACK", Reason: "operator"},
		"malformed decision id": {Status: "NOT_RUN", Decision: "420"},
		"unknown status":        {Status: "SKIPPED", Reason: "operator"},
	} {
		if _, _, err := BuildReadinessRecord(t.Context(), ReadinessOptions{CandidateDirectory: fixture.candidate, SourceRoot: fixture.source, Evidence: map[string]ReadinessEvidence{"gate/interop-gate": supplied}}); err == nil {
			t.Fatalf("%s admitted", name)
		}
	}
}

func TestSRRV1007PlatformRowsFallBackWithoutNativeEvidence(t *testing.T) {
	t.Run("SRR-V1-007 platform rows stay FALLBACK until native lifecycle evidence is supplied", testSRRV1007PlatformRowsFallBackWithoutNativeEvidence)
}

func testSRRV1007PlatformRowsFallBackWithoutNativeEvidence(t *testing.T) {
	fixture := newReadinessFixture(t, cleanGoMod)
	record, _ := fixture.build(t, nil)
	for id, decision := range map[string]string{"platform/darwin-arm64/lifecycle": "", "platform/darwin-arm64/host-lifecycle": "", "platform/linux-amd64/lifecycle": readinessDecision, "platform/linux-amd64/host-lifecycle": readinessDecision} {
		if row := readinessRowByID(record, id); row.Status != "FALLBACK" || row.Decision != decision {
			t.Fatalf("%s = %#v", id, row)
		}
	}
	for name, supplied := range map[string]map[string]ReadinessEvidence{
		"platform NOT_RUN":              {"platform/darwin-arm64/lifecycle": {Status: "NOT_RUN", Reason: "operator"}},
		"linux FALLBACK without 0420":   {"platform/linux-amd64/lifecycle": {Status: "FALLBACK", Reason: "operator"}},
		"linux FALLBACK other decision": {"platform/linux-amd64/lifecycle": {Status: "FALLBACK", Decision: "0419"}},
	} {
		if _, _, err := BuildReadinessRecord(t.Context(), ReadinessOptions{CandidateDirectory: fixture.candidate, SourceRoot: fixture.source, Evidence: supplied}); err == nil {
			t.Fatalf("builder admitted %s", name)
		}
	}
	for name, mutate := range map[string]func(*ReadinessRecord){
		"platform NOT_RUN":            func(r *ReadinessRecord) { r.Rows[index(r, "platform/darwin-arm64/lifecycle")].Status = "NOT_RUN" },
		"linux FALLBACK without 0420": func(r *ReadinessRecord) { r.Rows[index(r, "platform/linux-amd64/lifecycle")].Decision = "" },
	} {
		forged := resealReadiness(t, *record, func(r *ReadinessRecord) {
			r.Rows = append([]ReadinessRow(nil), r.Rows...)
			mutate(r)
		})
		if _, err := VerifyReadinessRecord(t.Context(), forged, fixture.candidate, nil); err == nil {
			t.Fatalf("verifier admitted %s", name)
		}
	}
	hosted := fixture.file(t, "linux-amd64-host-lifecycle.json", "{\"runner\":\"ubuntu-24.04\"}\n")
	record, raw := fixture.build(t, map[string]ReadinessEvidence{"platform/linux-amd64/host-lifecycle": {Status: "PASS", Path: hosted}})
	if row := readinessRowByID(record, "platform/linux-amd64/host-lifecycle"); row.Status != "PASS" {
		t.Fatalf("hosted evidence not recorded: %#v", row)
	}
	if _, err := VerifyReadinessRecord(t.Context(), raw, fixture.candidate, map[string]string{"platform/linux-amd64/host-lifecycle": hosted}); err != nil {
		t.Fatalf("hosted platform evidence refused: %v", err)
	}
}

func TestSRRV1008VulnerabilityRuleIsRequireFreeAndPinnedToolchain(t *testing.T) {
	t.Run("SRR-V1-008 vulnerability rule is require-free go.mod plus the pinned toolchain", testSRRV1008VulnerabilityRuleIsRequireFreeAndPinnedToolchain)
}

func testSRRV1008VulnerabilityRuleIsRequireFreeAndPinnedToolchain(t *testing.T) {
	if got := requireDirectives("module m\n// require x v1\nrequire y v1.0.0 // single\nrequire (\n\tz v1.0.0\n)\n"); got != 2 {
		t.Fatalf("require directives = %d, want 2", got)
	}
	if got := requireDirectives("module m\r\n\rrequire y v1.0.0\r\n"); got != 1 {
		t.Fatalf("carriage-return require directives = %d, want 1", got)
	}
	t.Setenv("GOTOOLCHAIN", "go1.99.0+path")
	if toolchain, err := probeLocalToolchain(t.Context(), t.TempDir()); err != nil || !strings.HasPrefix(toolchain, "go1.") {
		t.Fatalf("probe did not force the local toolchain: %q %v", toolchain, err)
	}
	fixture := newReadinessFixture(t, cleanGoMod)
	record, _ := fixture.build(t, nil)
	if record.Vulnerability != (ReadinessVulnerability{Decision: readinessDecision, RequireDirectives: 0, Toolchain: pinnedToolchain, Status: "PASS"}) {
		t.Fatalf("clean vulnerability = %#v", record.Vulnerability)
	}
	goToolchainProbe = func(context.Context, string) (string, error) { return "go1.27.2", nil }
	drifted, _ := fixture.build(t, nil)
	if drifted.Vulnerability.Status != "FAIL" || drifted.Vulnerability.Toolchain != "go1.27.2" {
		t.Fatalf("toolchain drift not FAIL: %#v", drifted.Vulnerability)
	}
	forged := resealReadiness(t, *drifted, func(r *ReadinessRecord) { r.Vulnerability.Status = "PASS" })
	if _, err := VerifyReadinessRecord(t.Context(), forged, fixture.candidate, nil); err == nil {
		t.Fatal("inconsistent vulnerability PASS admitted")
	}
	goToolchainProbe = func(context.Context, string) (string, error) { return pinnedToolchain, nil }
	required := newReadinessFixture(t, cleanGoMod+"\nrequire example.invalid/dep v1.0.0\n")
	record, _ = required.build(t, nil)
	if record.Vulnerability.Status != "FAIL" || record.Vulnerability.RequireDirectives != 1 {
		t.Fatalf("require directive not FAIL: %#v", record.Vulnerability)
	}
}

func TestSRRV1009PolicyRowsFollowDecision0420(t *testing.T) {
	t.Run("SRR-V1-009 rc.1 signing and native performance rows are fixed by decision 0420", testSRRV1009PolicyRowsFollowDecision0420)
}

func testSRRV1009PolicyRowsFollowDecision0420(t *testing.T) {
	fixture := newReadinessFixture(t, cleanGoMod)
	record, _ := fixture.build(t, nil)
	signing, performance := readinessRowByID(record, "policy/signing"), readinessRowByID(record, "policy/native-performance")
	if signing.Status != "NOT_RUN" || signing.Decision != readinessDecision || !strings.HasPrefix(signing.Reason, "No signing") || performance.Status != "NOT_RUN" || performance.Decision != readinessDecision {
		t.Fatalf("policy rows = %#v %#v", signing, performance)
	}
	if _, _, err := BuildReadinessRecord(t.Context(), ReadinessOptions{CandidateDirectory: fixture.candidate, SourceRoot: fixture.source, Evidence: map[string]ReadinessEvidence{"policy/signing": {Status: "PASS", Path: fixture.file(t, "sig", "sig\n")}}}); err == nil {
		t.Fatal("rc.1 signing override admitted")
	}
	stable, err := readinessRows(readinessRules("1.0.0"), map[string]ReadinessEvidence{"policy/signing": {Status: "NOT_RUN", Decision: "0421", Reason: "selection pending"}})
	if err != nil || stable[12].ID != "policy/signing" || stable[12].Decision != "0421" {
		t.Fatalf("stable signing selection not operator-owned: %#v %v", stable, err)
	}
}

func TestSRRV1010OwnerActionsStayNotRun(t *testing.T) {
	t.Run("SRR-V1-010 tag, publication and promotion stay NOT_RUN subsequent owner actions", testSRRV1010OwnerActionsStayNotRun)
}

func testSRRV1010OwnerActionsStayNotRun(t *testing.T) {
	fixture := newReadinessFixture(t, cleanGoMod)
	record, _ := fixture.build(t, nil)
	for _, id := range []string{"owner/tag", "owner/publication", "owner/promotion", "owner/toolchain-security-review"} {
		if row := readinessRowByID(record, id); row.Status != "NOT_RUN" || !strings.HasPrefix(row.Reason, "subsequent owner action") {
			t.Fatalf("%s = %#v", id, row)
		}
	}
	tag := fixture.file(t, "tag.log", "tagged\n")
	forged := resealReadiness(t, *record, func(r *ReadinessRecord) {
		r.Rows = append([]ReadinessRow(nil), r.Rows...)
		r.Rows[len(r.Rows)-3] = ReadinessRow{ID: "owner/tag", Status: "PASS", SHA256: digest([]byte("tagged\n"))}
	})
	if _, err := VerifyReadinessRecord(t.Context(), forged, fixture.candidate, map[string]string{"owner/tag": tag}); err == nil {
		t.Fatal("record claiming a tag admitted")
	}
}

func TestSRRV1011BuildAndVerifyWriteNothing(t *testing.T) {
	t.Run("SRR-V1-011 building and verifying leave candidate, source and evidence unchanged", testSRRV1011BuildAndVerifyWriteNothing)
}

func testSRRV1011BuildAndVerifyWriteNothing(t *testing.T) {
	fixture := newReadinessFixture(t, cleanGoMod)
	log := fixture.file(t, "full-gate.log", "ok\n")
	snapshot := func() string {
		var listing strings.Builder
		for _, root := range []string{fixture.candidate, fixture.source, fixture.evidence} {
			err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				info, err := entry.Info()
				if err != nil {
					return err
				}
				listing.WriteString(path + " " + info.Mode().String() + " " + info.ModTime().String() + "\n")
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		return listing.String()
	}
	before := snapshot()
	_, raw := fixture.build(t, map[string]ReadinessEvidence{"gate/full-gate": {Status: "PASS", Path: log}})
	if _, err := VerifyReadinessRecord(t.Context(), raw, fixture.candidate, map[string]string{"gate/full-gate": log}); err != nil {
		t.Fatal(err)
	}
	if after := snapshot(); after != before {
		t.Fatalf("readiness build or verify wrote state:\n%s\n---\n%s", before, after)
	}
}
