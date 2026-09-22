package adapters

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/dashboard/authority"
	"github.com/Beamfall/corvint/internal/dashboard/model"
	"github.com/Beamfall/corvint/internal/dashboard/source"
)

const (
	testRevision = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testTree     = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	testBlob     = "cccccccccccccccccccccccccccccccccccccccc"
)

type fakeAuthority struct {
	result       TraceAuthorityResult
	revision     string
	paths        []string
	qualifyCalls int
	finishCalls  int
}

func TestTraceAdapterConsumesStableReadBytesWithoutReopen(t *testing.T) {
	directory := t.TempDir()
	body := traceFixture(t, "safe task", "passed", []string{"internal/a.go"}, nil, nil)
	if err := os.WriteFile(filepath.Join(directory, "trace.jsonl"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	called := 0
	var summary TraceSummary
	var code AdapterIssueCode
	var failure *ScanError
	result := source.ReadOrdinal(root, "trace.jsonl", 7, source.KindLocalTrace, source.VerifierLocalTraceV1, source.NewBudget(source.MaxAggregateBytes), func(content source.StableContent) {
		called++
		summary, code, failure = ValidateStableTraceContent(context.Background(), content, testRevision, qualifiedAuthority())
	})
	if called != 1 || result.Validity != source.ValidityStable {
		t.Fatalf("consumer=%d result=%+v", called, result)
	}
	if failure != nil || code != "" || summary.RetainedRows != 1 {
		t.Fatalf("stable validation = %+v, %s, %v", summary, code, failure)
	}
	if result.Evidence == nil || result.Evidence.ConfiguredOrdinal() != 7 {
		t.Fatalf("stable-read evidence = %+v", result.Evidence)
	}
}

func (fake *fakeAuthority) QualifyTrace(_ context.Context, revision string, paths []string) TraceAuthorityResult {
	fake.qualifyCalls++
	fake.revision = revision
	fake.paths = append([]string(nil), paths...)
	result := fake.result
	if result.Code == AuthorityQualified && result.Witnesses == nil {
		result.Witnesses = authorityWitnesses(qualifiedWitnesses(revision, len(paths) != 0))
	}
	if result.Code == AuthorityQualified && result.PathWitnesses == nil && len(paths) != 0 {
		for _, witness := range result.Witnesses {
			if witness.Kind != "TRACE_PATH_OBJECT" {
				continue
			}
			for _, tracePath := range paths {
				result.PathWitnesses = append(result.PathWitnesses, TracePathWitness{Path: tracePath, Witness: witness})
			}
			break
		}
	}
	return result
}

func TestStoredRowSecretPatternParityCorpus(t *testing.T) {
	type parityCase struct {
		Name  string `json:"name"`
		Text  string `json:"text"`
		Match bool   `json:"match"`
	}
	var corpus struct {
		Baseline   []parityCase `json:"baseline"`
		WriterOnly []parityCase `json:"writer_only"`
	}
	raw, err := os.ReadFile("../../secretscreen/testdata/parity.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, test := range corpus.Baseline {
		t.Run(test.Name, func(t *testing.T) {
			if got := secretPattern.MatchString(test.Text); got != test.Match {
				t.Fatalf("stored-row pattern match for %q = %v, want %v", test.Text, got, test.Match)
			}
		})
	}
	for _, test := range corpus.WriterOnly {
		t.Run(test.Name, func(t *testing.T) {
			if secretPattern.MatchString(test.Text) {
				t.Fatalf("stored-row reader unexpectedly rejected writer-only shape %q", test.Text)
			}
		})
	}
}

func (fake *fakeAuthority) Snapshot() authority.Snapshot {
	return authority.Snapshot{
		DirtyPathsSHA256: "sha256:" + strings.Repeat("0", 64), HeadRevision: testRevision,
		ObjectFormat: "sha1", TreeRevision: testTree, WorktreeState: authority.WorktreeClean,
	}
}

func (fake *fakeAuthority) Finish(context.Context) authority.FinishResult {
	fake.finishCalls++
	return authority.FinishResult{Code: authority.FinishStable}
}

var _ authority.Authority = (*fakeAuthority)(nil)

func authorityWitnesses(input []model.RepositoryWitness) []authority.Witness {
	result := make([]authority.Witness, len(input))
	for index, witness := range input {
		result[index] = authority.Witness{
			Kind: witness.Kind, ObjectFormat: witness.ObjectFormat, ObjectID: witness.ObjectID,
			ObjectType: witness.ObjectType, Revision: cloneOptional(witness.Revision),
		}
	}
	return result
}

func qualifiedAuthority() *fakeAuthority {
	return &fakeAuthority{result: TraceAuthorityResult{
		Code: AuthorityQualified, ObjectFormat: "sha1", TreeRevision: testTree,
	}}
}

func snapshotHeadWitness() model.RepositoryWitness {
	return model.RepositoryWitness{Kind: "SNAPSHOT_HEAD", ObjectFormat: "sha1", ObjectID: testRevision, ObjectType: "commit"}
}

func qualifiedWitnesses(revision string, withPath bool) []model.RepositoryWitness {
	result := []model.RepositoryWitness{
		snapshotHeadWitness(),
		{Kind: "TRACE_REVISION", ObjectFormat: "sha1", ObjectID: revision, ObjectType: "commit", Revision: pointer(revision)},
	}
	if withPath {
		result = append(result, model.RepositoryWitness{Kind: "TRACE_PATH_OBJECT", ObjectFormat: "sha1", ObjectID: testBlob, ObjectType: "blob", Revision: pointer(revision)})
	}
	return result
}

func traceFixture(t *testing.T, task, outcome string, opened, changed, commands []string) []byte {
	t.Helper()
	normalized := map[string]any{
		"schema_version": 1, "revision": testRevision,
		"task": strings.TrimSpace(task), "opened_paths": sortedUnique(opened),
		"changed_paths": sortedUnique(changed), "verification": normalizedTestCommands(commands),
		"outcome": outcome,
	}
	encoded, err := contextindex.CanonicalJSON(normalized)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(encoded)
	withID := make(map[string]any, len(normalized)+1)
	for key, value := range normalized {
		withID[key] = value
	}
	withID["trace_id"] = hex.EncodeToString(digest[:])
	encoded, err = contextindex.CanonicalJSON(withID)
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func normalizedTestCommands(values []string) []string {
	values = sortedUnique(values)
	for index := range values {
		values[index] = strings.TrimSpace(values[index])
	}
	return values
}

func TestTraceAdapterValidatesProductionShapeAndEmitsOnlyCounts(t *testing.T) {
	first := traceFixture(t, " private incident narrative café ", "passed",
		[]string{"internal/confidential.go", "internal/confidential.go"},
		[]string{"internal/confidential_test.go"}, []string{" go test ./internal/... "})
	second := traceFixture(t, "another task", "blocked", nil, nil, []string{})
	authority := qualifiedAuthority()
	summary, code, failure := ValidateTraceArtifact(context.Background(), append(first, second...), testRevision, authority)
	if failure != nil || code != "" {
		t.Fatalf("ValidateTraceArtifact code = %s, failure = %v", code, failure)
	}
	if summary.RetainedRows != 2 || len(summary.Outcomes) != 2 || summary.Outcomes[0] != (TraceOutcomeCount{"blocked", 1}) || summary.Outcomes[1] != (TraceOutcomeCount{"passed", 1}) {
		t.Fatalf("summary = %+v", summary)
	}
	if got := strings.Join(authority.paths, ","); got != "internal/confidential.go,internal/confidential_test.go" {
		t.Fatalf("authority paths = %q", got)
	}
	wire, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"private incident", "confidential", "go test", "trace_id", "café"} {
		if strings.Contains(string(wire), forbidden) {
			t.Fatalf("summary disclosed raw trace data %q: %s", forbidden, wire)
		}
	}
}

func TestTraceAdapterKeepsDistinctUnicodePathBytesInRawOrder(t *testing.T) {
	composed := "internal/caf\u00e9.go"
	decomposed := "internal/cafe\u0301.go"
	authority := qualifiedAuthority()
	body := traceFixture(t, "safe task", "passed", []string{composed, decomposed}, nil, nil)
	if _, code, failure := ValidateTraceArtifact(context.Background(), body, testRevision, authority); failure != nil || code != "" {
		t.Fatalf("code = %s", code)
	}
	want := []string{decomposed, composed}
	if got := authority.paths; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("raw byte path order = %q, want %q", got, want)
	}
}

func TestTraceAdapterRejectsHostileAndInvalidRows(t *testing.T) {
	valid := traceFixture(t, "safe task", "failed", []string{"internal/a.go"}, nil, []string{"go test ./..."})
	cases := map[string]struct {
		body     []byte
		revision string
		want     AdapterIssueCode
	}{
		"wrong filename revision": {valid, strings.Repeat("d", 40), IssueSourceInvalidIdentity},
		"invalid filename":        {valid, "HEAD", IssueSourceInvalidIdentity},
		"duplicate row":           {append(append([]byte(nil), valid...), valid...), testRevision, IssueVerifierRejected},
		"empty line":              {[]byte("\n"), testRevision, IssueSourceInvalidSchema},
		"duplicate key":           {[]byte(`{"schema_version":1,"schema_version":1}`), testRevision, IssueSourceInvalidSchema},
		"oversized store":         {make([]byte, maxTraceStoreBytes+1), testRevision, IssueTraceStoreBound},
		"secret task":             {traceFixture(t, "password=do-not-store", "passed", nil, nil, nil), testRevision, IssueVerifierRejected},
		"secret path":             {traceFixture(t, "safe", "passed", []string{"internal/token=abcdefghij"}, nil, nil), testRevision, IssueVerifierRejected},
		"oversized path":          {traceFixture(t, "safe", "passed", []string{strings.Repeat("a", maxTracePathBytes+1)}, nil, nil), testRevision, IssueVerifierRejected},
		"control path":            {traceFixture(t, "safe", "passed", []string{"internal/a\x00.go"}, nil, nil), testRevision, IssueVerifierRejected},
		"volume-prefixed path":    {traceFixture(t, "safe", "passed", []string{"C:/internal/a.go"}, nil, nil), testRevision, IssueVerifierRejected},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			_, got, _ := ValidateTraceArtifact(context.Background(), test.body, test.revision, qualifiedAuthority())
			if got != test.want {
				t.Fatalf("code = %s, want %s", got, test.want)
			}
		})
	}
}

func TestTraceAdapterRejectsRowAndAuthorityBounds(t *testing.T) {
	row := traceFixture(t, "safe task", "passed", nil, nil, nil)
	many := make([]byte, 0, len(row)*(maxTraceRows+1))
	for range maxTraceRows + 1 {
		many = append(many, row...)
	}
	if _, code, _ := ValidateTraceArtifact(context.Background(), many, testRevision, qualifiedAuthority()); code != IssueTraceStoreBound {
		t.Fatalf("row bound code = %s", code)
	}
	for authorityCode, want := range map[AuthorityCode]AdapterIssueCode{
		AuthorityUnavailable:     IssueRepositoryObjectUnavailable,
		AuthorityAncestryBound:   IssueTraceAncestryBound,
		AuthorityPathUntracked:   IssueVerifierRejected,
		AuthorityInvalidIdentity: IssueSourceInvalidIdentity,
	} {
		authority := qualifiedAuthority()
		authority.result.Code = authorityCode
		if _, code, _ := ValidateTraceArtifact(context.Background(), row, testRevision, authority); code != want {
			t.Fatalf("authority %s code = %s, want %s", authorityCode, code, want)
		}
	}
}

func TestTraceAdapterRejectsMalformedQualifiedIdentity(t *testing.T) {
	row := traceFixture(t, "safe task", "passed", nil, nil, nil)
	authority := qualifiedAuthority()
	authority.result.TreeRevision = "HEAD"
	if _, code, _ := ValidateTraceArtifact(context.Background(), row, testRevision, authority); code != IssueSourceInvalidIdentity {
		t.Fatalf("code = %s", code)
	}
}

func TestTraceAdapterRejectsMalformedSemanticWitnesses(t *testing.T) {
	withoutPaths := traceFixture(t, "safe task", "passed", nil, nil, nil)
	withPaths := traceFixture(t, "safe task", "passed", []string{"internal/a.go"}, nil, nil)
	cases := map[string]struct {
		body      []byte
		witnesses []model.RepositoryWitness
	}{
		"missing head": {withoutPaths, qualifiedWitnesses(testRevision, false)[1:]},
		"wrong revision": {withoutPaths, []model.RepositoryWitness{
			snapshotHeadWitness(),
			{Kind: "TRACE_REVISION", ObjectFormat: "sha1", ObjectID: strings.Repeat("d", 40), ObjectType: "commit", Revision: pointer(strings.Repeat("d", 40))},
		}},
		"path witness without paths": {withoutPaths, qualifiedWitnesses(testRevision, true)},
		"missing path witness":       {withPaths, qualifiedWitnesses(testRevision, false)},
		"tree path witness": {withPaths, []model.RepositoryWitness{
			snapshotHeadWitness(),
			{Kind: "TRACE_REVISION", ObjectFormat: "sha1", ObjectID: testRevision, ObjectType: "commit", Revision: pointer(testRevision)},
			{Kind: "TRACE_PATH_OBJECT", ObjectFormat: "sha1", ObjectID: testBlob, ObjectType: "tree", Revision: pointer(testRevision)},
		}},
		"mixed format": {withoutPaths, []model.RepositoryWitness{
			snapshotHeadWitness(),
			{Kind: "TRACE_REVISION", ObjectFormat: "sha256", ObjectID: strings.Repeat("d", 64), ObjectType: "commit", Revision: pointer(strings.Repeat("d", 64))},
		}},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			authority := qualifiedAuthority()
			authority.result.Witnesses = authorityWitnesses(test.witnesses)
			if _, code, _ := ValidateTraceArtifact(context.Background(), test.body, testRevision, authority); code != IssueSourceInvalidIdentity {
				t.Fatalf("code = %s", code)
			}
		})
	}
}

func TestTraceAdapterCollapsesDuplicateSemanticWitnesses(t *testing.T) {
	body := traceFixture(t, "safe task", "passed", []string{"internal/a.go"}, nil, nil)
	authority := qualifiedAuthority()
	authority.result.Witnesses = authorityWitnesses(qualifiedWitnesses(testRevision, true))
	authority.result.Witnesses = append(authority.result.Witnesses, authority.result.Witnesses[len(authority.result.Witnesses)-1])
	summary, code, failure := ValidateTraceArtifact(context.Background(), body, testRevision, authority)
	if failure != nil || code != "" || len(summary.repositoryWitnesses) != 3 {
		t.Fatalf("summary/code = %+v, %s", summary, code)
	}
}

func TestTraceAdapterRequiresExactPrivatePathWitnessCoverage(t *testing.T) {
	paths := []string{"internal/a.go", "internal/b.go"}
	body := traceFixture(t, "safe task", "passed", paths, nil, nil)
	pathWitness := qualifiedWitnesses(testRevision, true)[2]
	otherWitness := pathWitness
	otherWitness.ObjectID = strings.Repeat("d", 40)
	tests := map[string]TraceAuthorityResult{
		"missing association": {
			Code: AuthorityQualified, ObjectFormat: "sha1", TreeRevision: testTree,
			Witnesses: authorityWitnesses(qualifiedWitnesses(testRevision, true)), PathWitnesses: []TracePathWitness{},
		},
		"incomplete association": {
			Code: AuthorityQualified, ObjectFormat: "sha1", TreeRevision: testTree,
			Witnesses:     authorityWitnesses(qualifiedWitnesses(testRevision, true)),
			PathWitnesses: []TracePathWitness{{Path: paths[0], Witness: authorityWitnesses([]model.RepositoryWitness{pathWitness})[0]}},
		},
		"wrong path association": {
			Code: AuthorityQualified, ObjectFormat: "sha1", TreeRevision: testTree,
			Witnesses: authorityWitnesses(qualifiedWitnesses(testRevision, true)),
			PathWitnesses: []TracePathWitness{{Path: paths[1], Witness: authorityWitnesses([]model.RepositoryWitness{pathWitness})[0]},
				{Path: paths[0], Witness: authorityWitnesses([]model.RepositoryWitness{pathWitness})[0]}},
		},
		"unlisted semantic witness": {
			Code: AuthorityQualified, ObjectFormat: "sha1", TreeRevision: testTree,
			Witnesses: authorityWitnesses(qualifiedWitnesses(testRevision, true)),
			PathWitnesses: []TracePathWitness{{Path: paths[0], Witness: authorityWitnesses([]model.RepositoryWitness{otherWitness})[0]},
				{Path: paths[1], Witness: authorityWitnesses([]model.RepositoryWitness{otherWitness})[0]}},
		},
		"unassociated extra semantic witness": {
			Code: AuthorityQualified, ObjectFormat: "sha1", TreeRevision: testTree,
			Witnesses: authorityWitnesses(append(qualifiedWitnesses(testRevision, true), otherWitness)),
			PathWitnesses: []TracePathWitness{{Path: paths[0], Witness: authorityWitnesses([]model.RepositoryWitness{pathWitness})[0]},
				{Path: paths[1], Witness: authorityWitnesses([]model.RepositoryWitness{pathWitness})[0]}},
		},
	}
	for name, result := range tests {
		t.Run(name, func(t *testing.T) {
			authority := &fakeAuthority{result: result}
			if _, code, _ := ValidateTraceArtifact(context.Background(), body, testRevision, authority); code != IssueSourceInvalidIdentity {
				t.Fatalf("code = %s", code)
			}
		})
	}
}
