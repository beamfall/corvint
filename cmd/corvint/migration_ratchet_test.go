package main

import (
	"bytes"
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
