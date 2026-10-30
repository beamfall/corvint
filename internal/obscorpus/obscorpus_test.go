package obscorpus

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const fixtureRevision = "0123456789abcdef0123456789abcdef01234567"

// pinnedSealID pins the canonical seal identity of testdata/replay.json; a
// change means the sealed form changed and every prior corpus is a new corpus.
const pinnedSealID = "446a534476e1201e6ff9fd5f2087f571a29c0e1cf41a2a340a2df97f381bb090"

type replayFixture struct {
	Corpus  Corpus          `json:"corpus"`
	SealID  string          `json:"sealId"`
	Results []rawResult     `json:"results"`
	Want    json.RawMessage `json:"want"`
}

type rawResult struct {
	SealID    string       `json:"sealId"`
	SubjectID string       `json:"subjectId"`
	Outcome   string       `json:"outcome"`
	Telemetry RawTelemetry `json:"telemetry"`
}

func TestCaptureFixturesBindBoundedFieldsAndNotObserved(t *testing.T) {
	var cases []struct {
		Name string          `json:"name"`
		Raw  RawTelemetry    `json:"raw"`
		Want json.RawMessage `json:"want"`
	}
	decodeFixture(t, "capture.json", &cases)
	for _, fixture := range cases {
		got := mustJSON(t, Capture(fixture.Raw))
		want := canonicalJSON(t, fixture.Want)
		if !bytes.Equal(got, want) {
			t.Errorf("%s:\n got %s\nwant %s", fixture.Name, got, want)
		}
	}
}

func TestCaptureFromFailsOpenOnErrorAndPanic(t *testing.T) {
	sources := map[string]func() (RawTelemetry, error){
		"error": func() (RawTelemetry, error) {
			return RawTelemetry{Identity: "turn-1"}, errors.New("telemetry pipe closed")
		},
		"panic": func() (RawTelemetry, error) { panic("harness exploded") },
	}
	for name, source := range sources {
		observation := CaptureFrom(source)
		if observation.Status != StatusNotObserved || observation.Reason != ReasonCaptureFailed || observation.InputTokens.Value != nil {
			t.Errorf("%s: got %+v, want NOT_OBSERVED capture-failed with no values", name, observation)
		}
	}
}

func TestCaptureAllDropsPastCountBound(t *testing.T) {
	raws := make([]RawTelemetry, MaxObservations+7)
	for index := range raws {
		raws[index] = RawTelemetry{Identity: "turn-" + strconv.Itoa(index)}
	}
	batch := CaptureAll(raws)
	if len(batch.Observations) != MaxObservations || batch.Dropped != 7 || batch.DroppedReason != ReasonCountBound {
		t.Fatalf("kept=%d dropped=%d reason=%q", len(batch.Observations), batch.Dropped, batch.DroppedReason)
	}
	if batch.Observations[MaxObservations-1].Identity.Value != "turn-"+strconv.Itoa(MaxObservations-1) {
		t.Fatalf("batch did not keep input order")
	}
}

func TestEncodedObservationStaysWithinRowBound(t *testing.T) {
	tokens, latency := int64(MaxTokens), int64(MaxLatencyMillis)
	worst := Capture(RawTelemetry{Identity: strings.Repeat("a", MaxIdentityBytes), Revision: strings.Repeat("f", 64), InputTokens: &tokens, OutputTokens: &tokens, LatencyMillis: &latency})
	if !worst.Complete() {
		t.Fatalf("worst case was not fully observed: %+v", worst)
	}
	if encoded := mustJSON(t, worst); len(encoded) > 1024 {
		t.Fatalf("worst-case observation is %d bytes, want <= 1024", len(encoded))
	}
	oversized := Capture(RawTelemetry{Identity: strings.Repeat("a", MaxIdentityBytes+1)})
	if oversized.Status != StatusNotObserved {
		t.Fatalf("oversized identity was bound: %+v", oversized)
	}
}

func TestSealedReplayFixtureIsDeterministic(t *testing.T) {
	fixture := loadReplay(t)
	sealed, err := Seal(fixture.Corpus)
	if err != nil {
		t.Fatal(err)
	}
	if sealed.ID() != pinnedSealID || fixture.SealID != pinnedSealID {
		t.Fatalf("seal id %s, fixture %s, pinned %s", sealed.ID(), fixture.SealID, pinnedSealID)
	}
	results := captureResults(fixture.Results)
	first := mustReplay(t, sealed, results)
	if want := canonicalJSON(t, fixture.Want); !bytes.Equal(first, want) {
		t.Fatalf("replay:\n got %s\nwant %s", first, want)
	}

	reordered := fixture.Corpus
	reordered.Subjects = append([]Subject(nil), fixture.Corpus.Subjects...)
	sort.Slice(reordered.Subjects, func(i, j int) bool { return reordered.Subjects[i].ID > reordered.Subjects[j].ID })
	resealed, err := Seal(reordered)
	if err != nil {
		t.Fatal(err)
	}
	reversed := append([]Result(nil), results...)
	sort.Slice(reversed, func(i, j int) bool { return reversed[i].SubjectID > reversed[j].SubjectID })
	if second := mustReplay(t, resealed, reversed); !bytes.Equal(first, second) {
		t.Fatalf("replay is order-dependent:\n%s\n%s", first, second)
	}
}

func TestReuseRefusesChangedCorpusInputs(t *testing.T) {
	fixture := loadReplay(t)
	sealed, err := Seal(fixture.Corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := sealed.Reuse(fixture.Corpus); err != nil {
		t.Fatalf("identical corpus refused: %v", err)
	}
	changes := map[string]func(*Corpus){
		"label":     func(c *Corpus) { c.Labels["s-pass"] = "reject" },
		"revision":  func(c *Corpus) { c.Subjects[0].Revision = strings.Repeat("e", 40) },
		"evaluator": func(c *Corpus) { c.EvaluatorRevision = strings.Repeat("d", 40) },
		"subject":   func(c *Corpus) { c.Subjects = append(c.Subjects, Subject{ID: "s-new", Revision: fixtureRevision}) },
	}
	for name, change := range changes {
		candidate := cloneCorpus(fixture.Corpus)
		change(&candidate)
		if err := sealed.Reuse(candidate); !errors.Is(err, ErrChangedCorpus) {
			t.Errorf("%s change: got %v, want ErrChangedCorpus", name, err)
		}
	}
	foreign := captureResults(fixture.Results)
	foreign[0].SealID = strings.Repeat("0", 64)
	if _, err := sealed.Replay(foreign); !errors.Is(err, ErrSealMismatch) {
		t.Fatalf("foreign seal result: got %v, want ErrSealMismatch", err)
	}
}

func TestReplayReportIsOwnerLabelledEvidenceNotAuthority(t *testing.T) {
	sealed, err := Seal(Corpus{Subjects: []Subject{{ID: "s-1", Revision: fixtureRevision}}, Labels: map[string]string{"s-1": "accept"}, EvaluatorRevision: fixtureRevision, ScoringRule: ScoringExactLabel})
	if err != nil {
		t.Fatal(err)
	}
	report, err := sealed.Replay([]Result{{SealID: sealed.ID(), SubjectID: "s-1", Outcome: "accept", Telemetry: completeTelemetry("s-1")}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Overall != VerdictPass {
		t.Fatalf("overall %s, want PASS", report.Overall)
	}
	encoded := string(mustJSON(t, report))
	for _, marker := range []string{`"evidence":"owner-labelled"`, `"authority":"none"`, `"independentValidation":false`} {
		if !strings.Contains(encoded, marker) {
			t.Errorf("all-PASS report lacks non-authority marker %s: %s", marker, encoded)
		}
	}
}

func TestMissingLabelTelemetryOrVerifierNeverCountsAsSuccess(t *testing.T) {
	subjects := []Subject{{ID: "no-label", Revision: fixtureRevision}, {ID: "no-verifier", Revision: fixtureRevision}, {ID: "no-telemetry", Revision: fixtureRevision}, {ID: "no-result", Revision: fixtureRevision}}
	labels := map[string]string{"no-verifier": "accept", "no-telemetry": "accept", "no-result": "accept"}
	sealed, err := Seal(Corpus{Subjects: subjects, Labels: labels, EvaluatorRevision: fixtureRevision, ScoringRule: ScoringExactLabel})
	if err != nil {
		t.Fatal(err)
	}
	report, err := sealed.Replay([]Result{
		{SealID: sealed.ID(), SubjectID: "no-label", Outcome: "accept", Telemetry: completeTelemetry("no-label")},
		{SealID: sealed.ID(), SubjectID: "no-verifier", Telemetry: completeTelemetry("no-verifier")},
		{SealID: sealed.ID(), SubjectID: "no-telemetry", Outcome: "accept", Telemetry: Capture(RawTelemetry{Identity: "no-telemetry"})},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Pass != 0 || report.Denominator != 4 || report.NotObserved != 3 || report.NotRun != 1 || report.Overall != VerdictNotObserved {
		t.Fatalf("missing evidence counted as success: %+v", report)
	}
}

func TestCaptureDropsPromptSourceAndSecretText(t *testing.T) {
	wantFields := []string{"Identity", "Revision", "InputTokens", "OutputTokens", "LatencyMillis"}
	if got := fieldNames(reflect.TypeOf(RawTelemetry{})); !reflect.DeepEqual(got, wantFields) {
		t.Fatalf("RawTelemetry fields %v, want closed set %v", got, wantFields)
	}
	forbidden := []string{"AKIAABCDEFGHIJKLMNOP", "fix the login bug\nfunc main() {}", "password=hunter2"}
	for _, text := range forbidden {
		observation := Capture(RawTelemetry{Identity: text, Revision: text})
		if observation.Status != StatusNotObserved || bytes.Contains(mustJSON(t, observation), []byte(text)) {
			t.Errorf("identity %q was bound or retained: %+v", text, observation)
		}
		corpus := Corpus{Subjects: []Subject{{ID: "s-1", Revision: fixtureRevision}}, Labels: map[string]string{"s-1": text}, EvaluatorRevision: fixtureRevision, ScoringRule: ScoringExactLabel}
		if _, err := Seal(corpus); !errors.Is(err, ErrInvalidCorpus) || strings.Contains(err.Error(), text) {
			t.Errorf("label %q: got %v, want ErrInvalidCorpus without the text", text, err)
		}
	}
	sealed := sealOne(t)
	report, err := sealed.Replay([]Result{{SealID: sealed.ID(), SubjectID: "s-1", Outcome: "AKIAABCDEFGHIJKLMNOP", Telemetry: completeTelemetry("s-1")}})
	if err != nil || report.Verdicts[0].Reason != ReasonSecretScreened || bytes.Contains(mustJSON(t, report), []byte("AKIA")) {
		t.Fatalf("secret outcome retained or scored: %+v %v", report, err)
	}
}

func TestPackageIsPureAndUnwired(t *testing.T) {
	allowed := map[string]bool{"crypto/sha256": true, "encoding/hex": true, "encoding/json": true, "errors": true, "fmt": true, "regexp": true, "sort": true, "github.com/Beamfall/corvint/internal/secretscreen": true}
	for _, path := range productionFiles(t, ".") {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			if imported, _ := strconv.Unquote(spec.Path.Value); !allowed[imported] {
				t.Errorf("%s imports %s; the package must open no file, network, or process", path, imported)
			}
		}
	}
	self := []byte(`"github.com/Beamfall/corvint/internal/obscorpus"`)
	for _, tree := range []string{"../../cmd", "../../internal", "../../tools", "../../benchmarks", "../../conformance", "../../experiments"} {
		for _, path := range productionFiles(t, tree) {
			if data, err := os.ReadFile(path); err == nil && bytes.Contains(data, self) {
				t.Errorf("%s imports obscorpus; wiring OCA-V0 into a product path needs owner acceptance", path)
			}
		}
	}
}

func productionFiles(t *testing.T, root string) []string {
	t.Helper()
	skip := map[string]bool{".git": true, "node_modules": true, "testdata": true, "vendor": true, ".corvint": true}
	var files []string
	if _, err := os.Stat(root); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && path != root && (skip[entry.Name()] || (root == "." && path != ".")) {
			return filepath.SkipDir
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func sealOne(t *testing.T) Sealed {
	t.Helper()
	sealed, err := Seal(Corpus{Subjects: []Subject{{ID: "s-1", Revision: fixtureRevision}}, Labels: map[string]string{"s-1": "accept"}, EvaluatorRevision: fixtureRevision, ScoringRule: ScoringExactLabel})
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}

func completeTelemetry(identity string) Observation {
	tokens := int64(10)
	return Capture(RawTelemetry{Identity: identity, Revision: fixtureRevision, InputTokens: &tokens, OutputTokens: &tokens, LatencyMillis: &tokens})
}

func captureResults(raws []rawResult) []Result {
	results := make([]Result, 0, len(raws))
	for _, raw := range raws {
		results = append(results, Result{SealID: raw.SealID, SubjectID: raw.SubjectID, Outcome: raw.Outcome, Telemetry: Capture(raw.Telemetry)})
	}
	return results
}

func cloneCorpus(corpus Corpus) Corpus {
	clone := corpus
	clone.Subjects = append([]Subject(nil), corpus.Subjects...)
	clone.Labels = make(map[string]string, len(corpus.Labels))
	for key, value := range corpus.Labels {
		clone.Labels[key] = value
	}
	return clone
}

func fieldNames(typ reflect.Type) []string {
	names := make([]string, 0, typ.NumField())
	for index := 0; index < typ.NumField(); index++ {
		names = append(names, typ.Field(index).Name)
	}
	return names
}

func loadReplay(t *testing.T) replayFixture {
	t.Helper()
	var fixture replayFixture
	decodeFixture(t, "replay.json", &fixture)
	return fixture
}

func mustReplay(t *testing.T, sealed Sealed, results []Result) []byte {
	t.Helper()
	report, err := sealed.Replay(results)
	if err != nil {
		t.Fatal(err)
	}
	return mustJSON(t, report)
}

func decodeFixture(t *testing.T, name string, target any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func canonicalJSON(t *testing.T, raw json.RawMessage) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, raw); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

// Revealed results arrive as JSON, so a telemetry block can claim OBSERVED for
// fields it never carried or carry values Capture would have refused.
func TestForgedTelemetryStatusNeverCountsAsSuccess(t *testing.T) {
	sealed, err := Seal(Corpus{Subjects: []Subject{{ID: "forged", Revision: fixtureRevision}}, Labels: map[string]string{"forged": "accept"}, EvaluatorRevision: fixtureRevision, ScoringRule: ScoringExactLabel})
	if err != nil {
		t.Fatal(err)
	}
	var result Result
	forged := `{"sealId":"` + sealed.ID() + `","subjectId":"forged","outcome":"accept","telemetry":{"status":"OBSERVED",` +
		`"identity":{"status":"OBSERVED"},"revision":{"status":"OBSERVED","value":"main"},` +
		`"inputTokens":{"status":"OBSERVED"},"outputTokens":{"status":"OBSERVED","value":-5},"latencyMillis":{"status":"OBSERVED"}}}`
	if err := json.Unmarshal([]byte(forged), &result); err != nil {
		t.Fatal(err)
	}
	report, err := sealed.Replay([]Result{result})
	if err != nil {
		t.Fatal(err)
	}
	if report.Pass != 0 || report.NotObserved != 1 || report.Verdicts[0].Reason != ReasonTelemetryAbsent {
		t.Fatalf("forged telemetry counted as success: %+v", report)
	}
}
