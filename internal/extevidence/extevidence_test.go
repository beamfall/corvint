package extevidence

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// repository builds a two-commit repository plus an orphan commit and returns
// its root with the commit ids and the blob id of pkg/main.go at head.
type repository struct {
	root, first, head, orphan, mainBlob string
}

func newRepository(t *testing.T) repository {
	t.Helper()
	root := t.TempDir()
	write := func(relative, content string) {
		full := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		command := exec.Command("git", args...)
		command.Dir = root
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "-q")
	git("config", "user.email", "corvint@example.test")
	git("config", "user.name", "Corvint Test")
	write("pkg/main.go", "package main\n")
	write("pkg/main_test.go", "package main\n")
	git("add", ".")
	git("commit", "-qm", "first")
	first := git("rev-parse", "HEAD")
	write("docs/guide.md", "# guide\n")
	git("add", ".")
	git("commit", "-qm", "second")
	head := git("rev-parse", "HEAD")
	mainBlob := git("rev-parse", "HEAD:pkg/main.go")
	branch := git("rev-parse", "--abbrev-ref", "HEAD")
	git("checkout", "-q", "--orphan", "island")
	git("rm", "-rqf", ".")
	write("island.txt", "island\n")
	git("add", ".")
	git("commit", "-qm", "island")
	orphan := git("rev-parse", "HEAD")
	git("checkout", "-q", "-f", branch)
	return repository{root: root, first: first, head: head, orphan: orphan, mainBlob: mainBlob}
}

func (r repository) index() *contextindex.Index {
	return &contextindex.Index{
		Root: r.root, CommitRevision: r.head,
		Tracked: map[string]struct{}{"pkg/main.go": {}, "pkg/main_test.go": {}, "docs/guide.md": {}},
		Sources: map[string]contextindex.Source{"pkg/main.go": {Path: "pkg/main.go", BlobHash: r.mainBlob}},
	}
}

func fixture(t *testing.T, revision string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "mock-provider.json"))
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Replace(data, []byte(strings.Repeat("0", 40)), []byte(revision), 1)
}

func writeRecord(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func mutate(t *testing.T, data []byte, edit func(record map[string]any)) []byte {
	t.Helper()
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	edit(record)
	out, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func relations(record map[string]any) []any { return record["relations"].([]any) }

func TestProviderRecordSchemaStrict(t *testing.T) {
	t.Parallel()
	valid := fixture(t, strings.Repeat("a", 40))
	if _, err := Decode(valid); err != nil {
		t.Fatalf("fixture must decode: %v", err)
	}
	cases := map[string]func(record map[string]any){
		"unknown top-level member": func(r map[string]any) { r["authority"] = "authoritative" },
		"unknown nested member":    func(r map[string]any) { relations(r)[0].(map[string]any)["weight"] = 1 },
		"wrong schema":             func(r map[string]any) { r["schema"] = "external-evidence-provider/1" },
		"duplicate entity id": func(r map[string]any) {
			r["entities"] = append(r["entities"].([]any), r["entities"].([]any)[0])
		},
		"reserved provider id": func(r map[string]any) { r["provider"].(map[string]any)["id"] = "path" },
		"oversized summary": func(r map[string]any) {
			r["entities"].([]any)[0].(map[string]any)["summary"] = strings.Repeat("x", maxText+1)
		},
		"uppercase relation type": func(r map[string]any) { relations(r)[0].(map[string]any)["type"] = "Implements" },
		"malformed blob":          func(r map[string]any) { relations(r)[0].(map[string]any)["blob"] = "abc" },
	}
	for name, edit := range cases {
		if _, err := Decode(mutate(t, valid, edit)); err == nil {
			t.Errorf("%s: record must be invalid", name)
		}
	}
	if _, err := Decode(append(valid, []byte("{}")...)); err == nil {
		t.Error("trailing content must be invalid")
	}
	if _, err := Decode(bytes.Replace(valid, []byte("capability-map"), []byte{0xff, 0xfe}, 1)); err == nil {
		t.Error("invalid UTF-8 must be invalid")
	}
	if _, err := Decode(bytes.Repeat([]byte(" "), MaxRecordBytes+1)); err == nil {
		t.Error("oversized record must be invalid")
	}
}

func TestProviderSectionDeterministicAndPinned(t *testing.T) {
	t.Parallel()
	repo := newRepository(t)
	source := writeRecord(t, t.TempDir(), "mock.json", fixture(t, repo.head))
	first, err := contextindex.CanonicalJSON(Section(context.Background(), repo.index(), []string{source}, nil, []string{"pkg/main.go"}, 10))
	if err != nil {
		t.Fatal(err)
	}
	second, _ := contextindex.CanonicalJSON(Section(context.Background(), repo.index(), []string{source}, nil, []string{"pkg/main.go"}, 10))
	if !bytes.Equal(first, second) {
		t.Fatalf("section must be deterministic:\n%s\n%s", first, second)
	}
	var section map[string]any
	if err := json.Unmarshal(first, &section); err != nil {
		t.Fatal(err)
	}
	provider := section["providers"].([]any)[0].(map[string]any)
	for key, want := range map[string]any{
		"source": source, "id": "mockdocs", "revision": "2026-09-18.1", "repository_revision": repo.head,
		"freshness": FreshnessEqual, "state": StateLoaded,
	} {
		if provider[key] != want {
			t.Errorf("provider[%s] = %v, want %v", key, provider[key], want)
		}
	}
	if digest, _ := provider["sha256"].(string); len(digest) != 64 {
		t.Errorf("sha256 must pin the record bytes, got %q", digest)
	}
	// A relative source resolves against the index root (EEP-V0-002).
	writeRecord(t, repo.root, "relative.json", fixture(t, repo.head))
	relative := Section(context.Background(), repo.index(), []string{"relative.json"}, nil, nil, 10)
	if state := relative["providers"].([]any)[0].(map[string]any)["state"]; state != StateLoaded {
		t.Fatalf("relative source must load against the root, got %v", state)
	}
}

func TestProviderUnavailableAndInvalidAreStructured(t *testing.T) {
	t.Parallel()
	repo := newRepository(t)
	dir := t.TempDir()
	invalid := writeRecord(t, dir, "invalid.json", []byte(`{"schema":"external-evidence-provider/0","extra":1}`))
	missing := filepath.Join(dir, "absent.json")
	section := Section(context.Background(), repo.index(), []string{missing, invalid}, nil, []string{"pkg/main.go"}, 10)
	rows := section["providers"].([]any)
	if got := rows[0].(map[string]any)["state"]; got != StateUnavailable {
		t.Errorf("missing file state = %v, want %s", got, StateUnavailable)
	}
	if reason := rows[0].(map[string]any)["reason"].(string); strings.Contains(reason, dir) {
		t.Errorf("reason must not repeat the path: %q", reason)
	}
	if got := rows[1].(map[string]any)["state"]; got != StateInvalid {
		t.Errorf("unknown member state = %v, want %s", got, StateInvalid)
	}
	for _, key := range []string{"results", "downstream", "verification", "unknowns"} {
		if len(section[key].([]any)) != 0 {
			t.Errorf("%s must be empty for failed providers", key)
		}
	}
}

func loaded(t *testing.T, repo repository, data []byte, changed ...string) composition {
	t.Helper()
	record, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	set := make(map[string]struct{}, len(changed))
	for _, path := range changed {
		set[path] = struct{}{}
	}
	return compose(record, set, repositoryTree(context.Background(), indexRoot(repo.index()), []provider{{state: StateLoaded, record: record}}))
}

func TestEndpointIdentitiesResolve(t *testing.T) {
	t.Parallel()
	repo := newRepository(t)
	data := mutate(t, fixture(t, repo.head), func(r map[string]any) {
		r["relations"] = append(relations(r),
			map[string]any{"from": "path:/etc/passwd", "to": "mockdocs:cap-stable-value", "type": "reads", "evidence": "declared", "rule": "r", "reference": "x"},
			map[string]any{"from": "path:../secret", "to": "mockdocs:cap-stable-value", "type": "reads", "evidence": "declared", "rule": "r", "reference": "x"},
			map[string]any{"from": "mockdocs:undeclared", "to": "mockdocs:cap-stable-value", "type": "reads", "evidence": "declared", "rule": "r", "reference": "x"},
			map[string]any{"from": "noprefix", "to": "mockdocs:cap-stable-value", "type": "reads", "evidence": "declared", "rule": "r", "reference": "x"},
		)
	})
	out := loaded(t, repo, data, "pkg/main.go")
	unresolved := 0
	for _, entry := range out.unknowns {
		if entry.state == unknownUnresolved {
			unresolved++
		}
	}
	if unresolved != 5 {
		t.Fatalf("expected 5 unresolved relations (foreign provider, absolute, traversal, undeclared, prefixless), got %d: %+v", unresolved, out.unknowns)
	}
	if len(out.results) != 1 || out.results[0].entity.ID != "cap-stable-value" {
		t.Fatalf("resolvable relation must still produce its result: %+v", out.results)
	}
}

func TestEvidenceKindLearnedExcluded(t *testing.T) {
	t.Parallel()
	repo := newRepository(t)
	out := loaded(t, repo, fixture(t, repo.head), "pkg/main.go")
	var learned *unknown
	for i := range out.unknowns {
		if out.unknowns[i].relation.Evidence == "learned" {
			learned = &out.unknowns[i]
		}
	}
	if learned == nil || learned.state != unknownExcluded {
		t.Fatalf("learned relation must be excluded: %+v", out.unknowns)
	}
	for _, entry := range out.downstream {
		if entry.entity.ID == "gap-no-negative-test" {
			t.Fatal("a learned relation must not admit a downstream entity")
		}
	}
	for _, entry := range append(out.results, out.downstream...) {
		if entry.toMap()["authority"] != Authority {
			t.Fatalf("Core must assign authority %q", Authority)
		}
	}
}

// TestImpactProviderEvaluation reports precision, recall, false-positive
// relationships, abstention accuracy, latency, and receipt size for the
// provider-to-impact workflow (`context.external`, EEP-V0-011) over the
// mock-provider fixture's five labelled relations: a `declared` path-to-entity
// relation and its `inferred` downstream neighbor and `observed` verification
// must all be admitted; the `learned` relation (EEP-V0-007) and the
// foreign-provider endpoint (EEP-V0-006) must both be excluded to `unknowns`.
// This is a synthetic single-record fixture, not an adopter corpus.
func TestImpactProviderEvaluation(t *testing.T) {
	t.Parallel()
	repo := newRepository(t)
	source := writeRecord(t, t.TempDir(), "mock.json", fixture(t, repo.head))
	// The fixture carries exactly 3 relations that must be admitted (declared
	// implements, inferred enables, observed verifies) and 2 that must be
	// excluded (learned, foreign-provider); see the fixture at
	// testdata/mock-provider.json.
	const relevantRelations = 3
	admitted := map[string]bool{"mockdocs:cap-stable-value": true, "mockdocs:journey-first-run": true}
	forbidden := []string{"mockdocs:gap-no-negative-test", "cap-1"}

	started := time.Now()
	out, err := contextindex.CanonicalJSON(Section(context.Background(), repo.index(), []string{source}, nil, []string{"pkg/main.go"}, 10))
	latency := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	var section map[string]any
	if err := json.Unmarshal(out, &section); err != nil {
		t.Fatal(err)
	}

	var admittedCount, truePositive, falsePositive, learnedAdmitted int
	for _, list := range []string{"results", "downstream", "verification"} {
		for _, raw := range section[list].([]any) {
			row := raw.(map[string]any)
			admittedCount++
			entity, _ := row["entity"].(string)
			if admitted[entity] {
				truePositive++
			} else {
				falsePositive++
			}
			for _, id := range forbidden {
				if strings.Contains(entity, id) {
					falsePositive++
				}
			}
			if relation, ok := row["relation"].(map[string]any); ok && relation["evidence"] == "learned" {
				learnedAdmitted++
			}
		}
	}
	recall := float64(truePositive) / float64(relevantRelations)
	precision := float64(truePositive) / float64(max(admittedCount, 1))

	var excludedLearned, excludedForeign bool
	for _, raw := range section["unknowns"].([]any) {
		row := raw.(map[string]any)
		relation, _ := row["relation"].(map[string]any)
		if relation["evidence"] == "learned" && row["state"] == "excluded" {
			excludedLearned = true
		}
		if relation["evidence"] == "declared" && row["state"] == "unresolved" {
			excludedForeign = true
		}
	}
	abstainCorrect := 0
	if excludedLearned {
		abstainCorrect++
	}
	if excludedForeign {
		abstainCorrect++
	}

	t.Logf("relations=%d precision=%.3f (%d/%d) recall=%.3f (%d/%d) false-positive-relations=%d abstention-accuracy=%d/2 latency=%s receipt-bytes=%d",
		relevantRelations+2, precision, truePositive, admittedCount, recall, truePositive, relevantRelations, falsePositive, abstainCorrect, latency, len(out))
	if falsePositive != 0 {
		t.Fatalf("zero false-positive relationships required, got %d", falsePositive)
	}
	if learnedAdmitted != 0 {
		t.Fatalf("zero learned-evidence admission required, got %d", learnedAdmitted)
	}
	if abstainCorrect != 2 {
		t.Fatalf("both the learned and foreign-provider relations must be reported as unknowns, got %d/2", abstainCorrect)
	}
}

func TestRelationTypesPreserved(t *testing.T) {
	t.Parallel()
	repo := newRepository(t)
	out := loaded(t, repo, fixture(t, repo.head), "pkg/main.go")
	if len(out.downstream) != 1 {
		t.Fatalf("expected one downstream entity, got %+v", out.downstream)
	}
	if got := out.downstream[0].toMap()["relation"].(map[string]any)["type"]; got != "mockdocs:enables" {
		t.Fatalf("namespaced type must be preserved verbatim, got %v", got)
	}
	if got := out.results[0].link.relation.Type; got != "implements" {
		t.Fatalf("type must be preserved verbatim, got %v", got)
	}
}

func TestFreshnessStatesFromAncestry(t *testing.T) {
	t.Parallel()
	repo := newRepository(t)
	ctx := context.Background()
	cases := map[string]string{
		repo.head:               FreshnessEqual,
		repo.first:              FreshnessRepositoryAhead,
		repo.orphan:             FreshnessUnrelatedHistory,
		strings.Repeat("f", 40): FreshnessRevisionUnavailable,
		"HEAD":                  FreshnessRevisionUnavailable,
	}
	for provider, want := range cases {
		if got := Freshness(ctx, repo.root, repo.head, provider); got != want {
			t.Errorf("Freshness(head, %s) = %s, want %s", provider, got, want)
		}
	}
	if got := Freshness(ctx, repo.root, repo.first, repo.head); got != FreshnessProviderAhead {
		t.Errorf("provider at a descendant = %s, want %s", got, FreshnessProviderAhead)
	}
}

func TestReferenceVerificationStates(t *testing.T) {
	t.Parallel()
	repo := newRepository(t)
	testBlob := strings.TrimSpace(gitOutput(t, repo.root, "rev-parse", repo.head+":pkg/main_test.go"))
	data := mutate(t, fixture(t, repo.head), func(r map[string]any) {
		r["relations"] = []any{
			map[string]any{"from": "path:pkg/main.go", "to": "mockdocs:cap-stable-value", "type": "implements", "evidence": "declared", "rule": "r", "reference": "x", "blob": repo.mainBlob},
			map[string]any{"from": "path:pkg/main.go", "to": "mockdocs:journey-first-run", "type": "implements", "evidence": "declared", "rule": "r", "reference": "x", "blob": strings.Repeat("1", 40)},
			map[string]any{"from": "path:pkg/main.go", "to": "mockdocs:gap-no-negative-test", "type": "implements", "evidence": "declared", "rule": "r", "reference": "x"},
			map[string]any{"from": "path:pkg/main_test.go", "to": "mockdocs:cap-stable-value", "type": "verifies", "evidence": "observed", "rule": "r", "reference": "x", "blob": testBlob},
			map[string]any{"from": "path:pkg/absent_test.go", "to": "mockdocs:cap-stable-value", "type": "verifies", "evidence": "observed", "rule": "r", "reference": "x"},
		}
	})
	out := loaded(t, repo, data, "pkg/main.go")
	states := map[string]string{}
	for _, entry := range out.results {
		states[entry.entity.ID] = entry.state
	}
	for _, entry := range out.verification {
		states[entry.path] = entry.state
	}
	want := map[string]string{
		"cap-stable-value":     VerificationVerified, // pinned blob equals the index blob
		"journey-first-run":    VerificationStale,    // pinned blob differs
		"gap-no-negative-test": VerificationVerified, // tracked, no pin
		"pkg/main_test.go":     VerificationVerified, // pinned blob resolved by Git, not the index
		"pkg/absent_test.go":   VerificationMissing,
	}
	for key, state := range want {
		if states[key] != state {
			t.Errorf("%s = %q, want %q", key, states[key], state)
		}
	}
}

func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}

func TestResultCompositionDirectDownstreamVerification(t *testing.T) {
	t.Parallel()
	repo := newRepository(t)
	out := loaded(t, repo, fixture(t, repo.head), "pkg/main.go")
	if len(out.results) != 1 || out.results[0].entity.ID != "cap-stable-value" || out.results[0].path != "pkg/main.go" {
		t.Fatalf("results = %+v", out.results)
	}
	if !strings.Contains(out.results[0].reason, "changed path pkg/main.go") || !strings.Contains(out.results[0].reason, "declared relation implements") {
		t.Fatalf("result reason must name the path, evidence, and type: %q", out.results[0].reason)
	}
	if len(out.downstream) != 1 || out.downstream[0].entity.ID != "journey-first-run" {
		t.Fatalf("downstream = %+v", out.downstream)
	}
	if len(out.verification) != 1 || out.verification[0].path != "pkg/main_test.go" || out.verification[0].link.relation.Type != "verifies" {
		t.Fatalf("verification = %+v", out.verification)
	}
	// A changed test path is a result, never a verification (EEP-V0-011).
	both := loaded(t, repo, fixture(t, repo.head), "pkg/main.go", "pkg/main_test.go")
	if len(both.verification) != 0 || len(both.results) != 2 {
		t.Fatalf("changed test path must join results: results=%d verification=%d", len(both.results), len(both.verification))
	}
}

// EEP-V0-011: two providers declaring the same entity id group by provider, not by relation.
func TestItemOrderGroupsByProvider(t *testing.T) {
	t.Parallel()
	at := func(provider, entity, relationType string) item {
		return item{provider: provider, entity: Entity{ID: entity}, link: link{relation: Relation{From: "path:pkg/main.go", To: entity, Type: relationType}}}
	}
	items := []item{at("b", "cap-x", "covers"), at("a", "cap-y", "covers"), at("a", "cap-x", "implements")}
	sortItems(items)
	var got []string
	for _, entry := range items {
		got = append(got, entry.provider+"/"+entry.entity.ID)
	}
	if strings.Join(got, " ") != "a/cap-x a/cap-y b/cap-x" {
		t.Fatalf("order = %v", got)
	}
}

func TestLimitsAndOmissions(t *testing.T) {
	t.Parallel()
	repo := newRepository(t)
	data := mutate(t, fixture(t, repo.head), func(r map[string]any) {
		r["relations"] = append(relations(r),
			map[string]any{"from": "path:pkg/main.go", "to": "mockdocs:journey-first-run", "type": "touches", "evidence": "declared", "rule": "r", "reference": "x"},
			map[string]any{"from": "path:pkg/main.go", "to": "mockdocs:gap-no-negative-test", "type": "touches", "evidence": "declared", "rule": "r", "reference": "x"},
		)
	})
	source := writeRecord(t, t.TempDir(), "mock.json", data)
	section := Section(context.Background(), repo.index(), []string{source}, nil, []string{"pkg/main.go"}, 1)
	results := section["results"].([]any)
	omitted := section["omitted"].(map[string]any)
	if len(results) != 1 || omitted["results"] != 2 {
		t.Fatalf("limit 1 must keep one result and count two omitted: kept=%d omitted=%v", len(results), omitted)
	}
	if results[0].(map[string]any)["entity"] != "mockdocs:cap-stable-value" {
		t.Fatalf("kept result must be first in entity order, got %v", results[0])
	}
	if omitted["unknowns"] != 1 {
		t.Fatalf("two unknowns under limit 1 must count one omitted, got %v", omitted["unknowns"])
	}
	oversized := mutate(t, fixture(t, repo.head), func(r map[string]any) {
		entities := make([]any, 0, maxEntities+1)
		for i := 0; i <= maxEntities; i++ {
			entities = append(entities, map[string]any{"id": "e" + strconv.Itoa(i), "kind": "k", "summary": "s"})
		}
		r["entities"] = entities
	})
	if _, err := Decode(oversized); err == nil || !strings.Contains(err.Error(), "entities exceed") {
		t.Fatalf("entity bound must reject the record, got %v", err)
	}
}
