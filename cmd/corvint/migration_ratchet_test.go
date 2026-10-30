package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/migrationratchet"
)

func TestMigrationRatchetCLIExitAndDeterministicReceipt(t *testing.T) {
	profile := migrationRatchetProfile()
	path := writeMigrationRatchetProfile(t, profile)
	var first, firstErr bytes.Buffer
	if exit := run([]string{"migration-ratchet", "--profile", path}, strings.NewReader(""), &first, &firstErr); exit != 0 || firstErr.Len() != 0 {
		t.Fatalf("first exit=%d stderr=%s", exit, firstErr.String())
	}
	var second, secondErr bytes.Buffer
	if exit := run([]string{"migration-ratchet", "--profile", path}, strings.NewReader(""), &second, &secondErr); exit != 0 || secondErr.Len() != 0 {
		t.Fatalf("second exit=%d stderr=%s", exit, secondErr.String())
	}
	if first.String() != second.String() || !strings.Contains(first.String(), `"verdict":"pass"`) {
		t.Fatalf("nondeterministic or non-pass receipt: %s / %s", first.String(), second.String())
	}

	orphan := migrationRatchetRecord("test-execution", "test:orphan", "reviewed")
	profile.Candidate.Records = append(profile.Candidate.Records, orphan)
	profile.Candidate.ArtifactSHA256 = migrationratchet.SnapshotDigest(profile.Candidate)
	path = writeMigrationRatchetProfile(t, profile)
	var failed bytes.Buffer
	if exit := run([]string{"migration-ratchet", "--profile", path}, strings.NewReader(""), &failed, &bytes.Buffer{}); exit != 1 || !strings.Contains(failed.String(), `"verdict":"fail"`) {
		t.Fatalf("failed exit/receipt: exit=%d receipt=%s", exit, failed.String())
	}
}

func TestMigrationRatchetCLIRefusesMutableOrMalformedBinding(t *testing.T) {
	profile := migrationRatchetProfile()
	profile.Candidate.Repository.Revision = "main"
	profile.Candidate.ArtifactSHA256 = migrationratchet.SnapshotDigest(profile.Candidate)
	path := writeMigrationRatchetProfile(t, profile)
	var stderr bytes.Buffer
	if exit := run([]string{"migration-ratchet", "--profile", path}, strings.NewReader(""), &bytes.Buffer{}, &stderr); exit != 2 || !strings.Contains(stderr.String(), `"ok":false`) {
		t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
	}
}

func TestMigrationRatchetCLIHelperForms(t *testing.T) {
	var schema bytes.Buffer
	if exit := run([]string{"migration-ratchet", "schema"}, strings.NewReader(""), &schema, &bytes.Buffer{}); exit != 0 {
		t.Fatalf("schema exit=%d", exit)
	}
	var description migrationratchet.SchemaDescription
	if err := json.Unmarshal(schema.Bytes(), &description); err != nil || description.Schema != migrationratchet.SchemaDescriptionSchema || len(description.Refusals) == 0 {
		t.Fatalf("schema = %v %s", err, schema.String())
	}
	example := writeMigrationRatchetProfile(t, description.Example)
	if exit := run([]string{"migration-ratchet", "--profile", example}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); exit != 0 {
		t.Fatalf("schema example exit=%d", exit)
	}

	// An unsealed profile is refused with the field named; seal repairs only
	// self-digests and order, writing to stdout without touching the file.
	profile := migrationRatchetProfile()
	profile.Candidate.Records[0].SHA256 = strings.Repeat("0", 64)
	path := writeMigrationRatchetProfile(t, profile)
	before, _ := os.ReadFile(path)
	var refused bytes.Buffer
	if exit := run([]string{"migration-ratchet", "--profile", path}, strings.NewReader(""), &bytes.Buffer{}, &refused); exit != 2 || !strings.Contains(refused.String(), "candidate digest refused: records[") {
		t.Fatalf("unsealed exit=%d stderr=%s", exit, refused.String())
	}
	var sealed bytes.Buffer
	if exit := run([]string{"migration-ratchet", "seal", "--profile", path}, strings.NewReader(""), &sealed, &bytes.Buffer{}); exit != 0 {
		t.Fatalf("seal exit=%d", exit)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Fatal("seal mutated its input")
	}
	sealedPath := filepath.Join(t.TempDir(), "sealed.json")
	if err := os.WriteFile(sealedPath, sealed.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if exit := run([]string{"migration-ratchet", "--profile", sealedPath}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); exit != 0 {
		t.Fatalf("sealed profile exit=%d", exit)
	}
	snapshot, _ := migrationratchet.Encode(profile.Candidate)
	snapshotPath := filepath.Join(t.TempDir(), "snapshot.json")
	if err := os.WriteFile(snapshotPath, snapshot, 0600); err != nil {
		t.Fatal(err)
	}
	var sealedSnapshot bytes.Buffer
	want := migrationratchet.SealSnapshot(profile.Candidate).ArtifactSHA256
	if exit := run([]string{"migration-ratchet", "seal", "--snapshot", snapshotPath}, strings.NewReader(""), &sealedSnapshot, &bytes.Buffer{}); exit != 0 || !strings.Contains(sealedSnapshot.String(), want) {
		t.Fatalf("seal snapshot exit=%d out=%s", exit, sealedSnapshot.String())
	}

	root := cliRepository(t)
	writeFixtureFile(t, root, "case_test.go", "package x\nfunc TestA() {}\nfunc TestB() {}\n")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "tests")
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	var span bytes.Buffer
	if exit := run([]string{"--root", root, "migration-ratchet", "span-digest", "--revision", head, "--path", "case_test.go", "--lines", "3:3"}, strings.NewReader(""), &span, &bytes.Buffer{}); exit != 0 {
		t.Fatalf("span exit=%d out=%s", exit, span.String())
	}
	sum := sha256.Sum256([]byte("func TestB() {}"))
	if !strings.Contains(span.String(), hex.EncodeToString(sum[:])) || !strings.Contains(span.String(), `"schema":"corvint-migration-span-digest/1"`) || !strings.Contains(span.String(), `"start":3`) {
		t.Fatalf("span digest = %s", span.String())
	}

	for _, args := range [][]string{
		{"migration-ratchet"},
		{"migration-ratchet", "schema", "--profile", path},
		{"migration-ratchet", "seal"},
		{"migration-ratchet", "seal", "--profile", path, "--snapshot", snapshotPath},
		{"migration-ratchet", "--profile", path, "--profile", path},
		{"migration-ratchet", "span-digest", "--revision", head, "--path", "case_test.go", "--lines", "3"},
		{"migration-ratchet", "span-digest", "--revision", head, "--path", "case_test.go", "--lines", "3:2"},
		{"--root", root, "migration-ratchet", "span-digest", "--revision", "HEAD", "--path", "case_test.go", "--lines", "1:1"},
		{"--root", root, "migration-ratchet", "span-digest", "--revision", head, "--path", "case_test.go", "--lines", "1:9"},
	} {
		var out, stderr bytes.Buffer
		if exit := run(args, strings.NewReader(""), &out, &stderr); exit != 2 || out.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("%v: exit=%d out=%s stderr=%s", args, exit, out.String(), stderr.String())
		}
	}
}

// TestMigrationRatchetCLIVerifiesRecordSourceSpans pins MER-V0-018: a record
// source span is re-read from the checkout at compare time, a matching digest
// passes, and a mismatched digest or an unreadable path refuses with exit 2.
func TestMigrationRatchetCLIVerifiesRecordSourceSpans(t *testing.T) {
	root := cliRepository(t)
	writeFixtureFile(t, root, "checkout.spec.ts", "test('pays', async () => {\n  await pay();\n});\n")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "base")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	writeFixtureFile(t, root, "notes.txt", "unrelated\n")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "candidate")
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	sum := sha256.Sum256([]byte("test('pays', async () => {\n  await pay();\n});"))
	body := hex.EncodeToString(sum[:])
	build := func(baseContent, path string) string {
		profile := migrationRatchetProfile()
		sides := []*migrationratchet.Snapshot{&profile.Baseline, &profile.Candidate}
		for index, commit := range []string{base, head} {
			snapshot, content := sides[index], body
			if index == 0 {
				content = baseContent
			}
			snapshot.Repository.Revision = commit
			snapshot.Records = append([]migrationratchet.Record{}, snapshot.Records...)
			snapshot.Records = append(snapshot.Records, migrationratchet.Record{Kind: "coverage-target", ID: "coverage:pays", ContentSHA256: content, State: "reviewed", Links: []migrationratchet.Link{},
				Source: &migrationratchet.SourceSpan{Commit: commit, Path: path, Start: 1, End: 3}})
			*snapshot = migrationratchet.SealSnapshot(*snapshot)
		}
		return writeMigrationRatchetProfile(t, profile)
	}
	var out, stderr bytes.Buffer
	if exit := run([]string{"--root", root, "migration-ratchet", "--profile", build(body, "checkout.spec.ts")}, strings.NewReader(""), &out, &stderr); exit != 0 || !strings.Contains(out.String(), "2 record source spans were re-read from Git") {
		t.Fatalf("verified exit=%d out=%s stderr=%s", exit, out.String(), stderr.String())
	}
	// GVA-V0-005: a source-backed comparison analyses the source root, so a
	// --log there refuses and the tracked file stays unwritten.
	out.Reset()
	stderr.Reset()
	tracked := filepath.Join(root, "notes.txt")
	if exit := run([]string{"--root", root, "migration-ratchet", "--profile", build(body, "checkout.spec.ts"), "--log", tracked}, strings.NewReader(""), &out, &stderr); exit != 2 || out.Len() != 0 || !strings.Contains(stderr.String(), "agreement-log-inside-repository") {
		t.Fatalf("log inside the source root: exit=%d out=%s stderr=%s", exit, out.String(), stderr.String())
	}
	if data, err := os.ReadFile(tracked); err != nil || string(data) != "unrelated\n" {
		t.Fatalf("tracked log target changed: %q %v", data, err)
	}
	stale := migrationratchet.Digest("stale")
	for name, test := range map[string]struct {
		path    string
		content string
		want    string
	}{
		"stale digest": {"checkout.spec.ts", stale, "baseline source refused: records[0].source (coverage-target coverage:pays) content_sha256 does not match Git span digest " + body},
		"missing path": {"gone.spec.ts", body, "baseline source unreadable: records[0].source (coverage-target coverage:pays): source missing"},
	} {
		var out, stderr bytes.Buffer
		if exit := run([]string{"--root", root, "migration-ratchet", "--profile", build(test.content, test.path)}, strings.NewReader(""), &out, &stderr); exit != 2 || out.Len() != 0 || !strings.Contains(stderr.String(), test.want) {
			t.Fatalf("%s exit=%d out=%s stderr=%s", name, exit, out.String(), stderr.String())
		}
	}
}

func migrationRatchetProfile() migrationratchet.Profile {
	policy := migrationratchet.Policy{
		ID: "policy:migration", States: []migrationratchet.StateRule{{Name: "unresolved", Rank: 0, Unresolved: true}, {Name: "reviewed", Rank: 1}, {Name: "terminal", Rank: 2, Terminal: true}},
		ForbidNewLegacy: true, RequireContractForNewOrChangedTests: true, TerminalStatesCannotRegress: true,
		InvalidateEvidenceOnContentChange: true, UnresolvedDenominatorCannotGrow: true,
	}
	policy.SHA256 = migrationratchet.PolicyDigest(policy)
	provider := migrationratchet.ProviderBinding{ID: "provider:migration", Schema: "fixture/1", SHA256: migrationratchet.Digest("provider")}
	records := []migrationratchet.Record{
		migrationRatchetRecord("legacy-case", "legacy:one", "unresolved"),
		migrationRatchetRecord("owner", "owner:migration", "reviewed"),
		migrationRatchetRecord("reviewer", "reviewer:primary", "reviewed"),
	}
	baseline := migrationratchet.Snapshot{Schema: migrationratchet.SnapshotSchema, Repository: migrationratchet.RepositoryBinding{ID: "repository:fixture", Revision: strings.Repeat("1", 40), Tree: strings.Repeat("2", 40)}, Policy: policy, Provider: provider, Complete: true, Fresh: true, Records: records, Exceptions: []migrationratchet.Exception{}}
	baseline.ArtifactSHA256 = migrationratchet.SnapshotDigest(baseline)
	candidate := baseline
	candidate.Repository.Revision = strings.Repeat("3", 40)
	candidate.Repository.Tree = strings.Repeat("4", 40)
	candidate.Records = append([]migrationratchet.Record{}, records...)
	candidate.Records[0].State = "terminal"
	candidate.Records[0].SHA256 = migrationratchet.RecordDigest(candidate.Records[0])
	candidate.ArtifactSHA256 = migrationratchet.SnapshotDigest(candidate)
	return migrationratchet.Profile{Schema: migrationratchet.ProfileSchema, EvaluatedOn: "2026-09-20", Baseline: baseline, Candidate: candidate}
}

func migrationRatchetRecord(kind, id, state string) migrationratchet.Record {
	record := migrationratchet.Record{Kind: kind, ID: id, ContentSHA256: migrationratchet.Digest(kind + id), State: state, Links: []migrationratchet.Link{}}
	record.SHA256 = migrationratchet.RecordDigest(record)
	return record
}

func writeMigrationRatchetProfile(t *testing.T, profile migrationratchet.Profile) string {
	t.Helper()
	encoded, err := migrationratchet.Encode(profile)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestMER098SealRefusesBadAcceptIncomparable proves seal applies the compare's policy validation
// (MER-V0-098).
func TestMER098SealRefusesBadAcceptIncomparable(t *testing.T) {
	profile := migrationRatchetProfile()
	profile.Baseline.Policy.AcceptIncomparable = []string{"bogus"}
	path := writeMigrationRatchetProfile(t, profile)
	var stderr bytes.Buffer
	if exit := run([]string{"migration-ratchet", "seal", "--profile", path}, strings.NewReader(""), &bytes.Buffer{}, &stderr); exit != 2 ||
		!strings.Contains(stderr.String(), "accept_incomparable refused") {
		t.Fatalf("seal exit=%d stderr=%s", exit, stderr.String())
	}
}

// TestMER099SourceRootUnavailableOutsideGit proves a sourced profile outside a Git checkout
// without --root names source-root-unavailable (MER-V0-099).
func TestMER099SourceRootUnavailableOutsideGit(t *testing.T) {
	profile := migrationRatchetProfile()
	profile.Candidate.Records[0].Source = &migrationratchet.SourceSpan{Commit: strings.Repeat("a", 40), Path: "x.go", Start: 1, End: 2}
	path := writeMigrationRatchetProfile(t, profile)
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer
	if exit := run([]string{"migration-ratchet", "--profile", path}, strings.NewReader(""), &bytes.Buffer{}, &stderr); exit != 2 ||
		!strings.Contains(stderr.String(), "source-root-unavailable") {
		t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
	}
}
