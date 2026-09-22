package lrf

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

type corpusDocument struct {
	Spec  string       `json:"spec"`
	Cases []corpusCase `json:"cases"`
}

type corpusCase struct {
	CoverageID string          `json:"coverageId"`
	Input      json.RawMessage `json:"input"`
	Expected   json.RawMessage `json:"expected"`
}

type corpusManifest struct {
	Kind    string     `json:"kind"`
	Spec    string     `json:"spec"`
	Entries [][]string `json:"entries"`
}

type corpusRegistration struct {
	ManifestSHA256 string `json:"manifestSha256"`
	Spec           string `json:"spec"`
}

type actionInput struct {
	Action         string            `json:"action"`
	Request        *requestDTO       `json:"request"`
	Limits         map[string]string `json:"limits"`
	Fault          string            `json:"fault"`
	Subcases       []json.RawMessage `json:"subcases"`
	CEM            string            `json:"cem"`
	OCM            string            `json:"ocm"`
	Patch          string            `json:"patch"`
	PatchDefaulted bool              `json:"patchDefaulted"`
	CommitOIDs     []string          `json:"commitOids"`
	Subject        json.RawMessage   `json:"subject"`
}

type requestDTO struct {
	Context     contextDTO      `json:"context"`
	Hunks       []hunkDTO       `json:"hunks"`
	Evidence    []evidenceDTO   `json:"evidence"`
	Obligations []obligationDTO `json:"obligations"`
}

type contextDTO struct {
	CEMSpec          string  `json:"cem_spec"`
	CEMMapSHA256     string  `json:"cem_map_sha256"`
	PatchSource      string  `json:"patch_source"`
	BaseRevision     string  `json:"base_revision"`
	TargetRevision   *string `json:"target_revision"`
	PatchSHA256      string  `json:"patch_sha256"`
	ExcludedPath     *string `json:"excluded_path"`
	OCMSpec          *string `json:"ocm_spec"`
	OCMMapSHA256     *string `json:"ocm_map_sha256"`
	IntentPath       *string `json:"intent_path"`
	IntentBlobOID    *string `json:"intent_blob_oid"`
	IntentStart      *string `json:"intent_start"`
	IntentEnd        *string `json:"intent_end"`
	IntentSpanSHA256 *string `json:"intent_span_sha256"`
}

type hunkDTO struct {
	ID          string     `json:"id"`
	OldPath     *string    `json:"old_path"`
	NewPath     *string    `json:"new_path"`
	Added       string     `json:"added"`
	Disposition string     `json:"disposition"`
	Basis       []basisDTO `json:"basis"`
	Ordinal     int        `json:"ordinal"`
}

type basisDTO struct {
	EvidenceID string `json:"evidence_id"`
	Relation   string `json:"relation"`
}

type evidenceDTO struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Span string `json:"span"`
}

type obligationDTO struct {
	ID         string   `json:"id"`
	Statement  string   `json:"statement"`
	HunkIDs    []string `json:"hunk_ids"`
	ClaimPaths []string `json:"claim_paths"`
	Ordinal    int      `json:"ordinal"`
}

func TestConformanceCorpus(t *testing.T) {
	document := loadCorpus(t)
	expectedIDs := []string{
		"token.identifier-split", "token.id-redaction", "token.id-redaction-hunk", "token.basename",
		"token.stop-terms", "cem.candidate", "cem.deletion", "cem.span-cap", "cem.span-cap-at-limit",
		"cem.self-reference", "cem.self-reference-create", "cem.no-overlap", "cem.path-only-overlap",
		"cem.extra-basis", "cem.allowed-relations", "cem.relation-per-basis", "ocm.candidate", "ocm.no-cem-witness",
		"ocm.nonmaterial", "ocm.intent-path", "ocm.requirement-prefix", "ocm.no-overlap",
		"ocm.id-only", "ocm.no-links", "context.01-explicit", "context.01-default",
		"context.02-cem", "context.02-ocm", "context.unsupported", "failure.bound",
		"failure.operational", "failure.structural", "determinism.repeat", "determinism.duplicate-collapse", "dogfood.ocm-12",
		"dogfood.cem-broad", "commitment.mutation",
		"claim.no-upgrade", "claim.reuse-inert", "claim.reuse-issue-order", "execution.not-observed",
	}
	if document.Spec != "lrf-conformance-cases/0" || len(document.Cases) != len(expectedIDs) {
		t.Fatalf("closed corpus shape = %q/%d, want lrf-conformance-cases/0/%d", document.Spec, len(document.Cases), len(expectedIDs))
	}
	for index, testCase := range document.Cases {
		if testCase.CoverageID != expectedIDs[index] {
			t.Fatalf("coverage ID %d = %q, want %q", index, testCase.CoverageID, expectedIDs[index])
		}
		t.Run(testCase.CoverageID, func(t *testing.T) {
			first := observeCase(t, testCase)
			second := observeCase(t, testCase)
			if !bytes.Equal(first, second) {
				t.Fatal("two evaluations in one process produced different bytes")
			}
			want := compactWithLF(t, testCase.Expected)
			if !bytes.Equal(first, want) {
				t.Fatalf("canonical bytes differ\ngot:  %s\nwant: %s", first, want)
			}
		})
	}
}

func TestDeletionDoesNotUseOldBasenameTerms(t *testing.T) {
	context := canonicalOCMContext()
	hunkID := "hunk:sha256:" + strings.Repeat("1", 64)
	evidenceID := "evidence:sha256:" + strings.Repeat("a", 64)
	oldPath := "src/alpha_bravo.go"
	request := Request{
		Context:  context,
		Evidence: []Evidence{{ID: evidenceID, Path: "docs/alpha.md", Span: []byte("alpha")}},
		Hunks: []Hunk{{
			ID: hunkID, OldPath: &oldPath, Disposition: "supported", Ordinal: 0,
			Basis: []wire.Basis{{EvidenceID: evidenceID, Relation: "specification"}},
		}},
		Obligations: []Obligation{{
			ID: "LRF-V0-001", Statement: []byte("- `LRF-V0-001`: alpha.\n"),
			HunkIDs: []string{hunkID}, Ordinal: 0,
		}},
	}
	limits := DefaultLimits()
	limits.Terms = 1
	result, err := EvaluateWithLimits(request, limits)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Results()[1][5]; got != "rejected" {
		t.Fatalf("deletion OCM outcome = %q, want rejected", got)
	}
	if got := result.Issues()[1][4]; got != "obligation-hunk-mismatch" {
		t.Fatalf("deletion OCM issue = %q, want obligation-hunk-mismatch", got)
	}
}

func TestContentIDBesideNonASCIIByteIsNotRedacted(t *testing.T) {
	hunkID := "hunk:sha256:" + strings.Repeat("1", 64)
	evidenceID := "evidence:sha256:" + strings.Repeat("a", 64)
	contentID := "hunk:sha256:cafe" + strings.Repeat("0", 60)
	newPath := "src/a.py"
	for name, added := range map[string]string{"leading": "\xc3\xa9" + contentID + "\n", "trailing": contentID + "\xc3\xa9\n"} {
		t.Run(name, func(t *testing.T) {
			result, err := Evaluate(Request{
				Context:  canonicalOCMContext(),
				Evidence: []Evidence{{ID: evidenceID, Path: "docs/id.md", Span: []byte("cafe")}},
				Hunks: []Hunk{{
					ID: hunkID, OldPath: &newPath, NewPath: &newPath, Added: []byte(added), Disposition: "supported",
					Basis: []wire.Basis{{EvidenceID: evidenceID, Relation: "specification"}},
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := result.Results()[0][5]; got != "cem-lexical-v0" {
				t.Fatalf("content ID beside a non-ASCII byte was redacted: outcome %q", got)
			}
		})
	}
}

func TestRequirementIDInAddedBytesSuppliesNoTerm(t *testing.T) {
	hunkID := "hunk:sha256:" + strings.Repeat("1", 64)
	evidenceID := "evidence:sha256:" + strings.Repeat("a", 64)
	newPath := "src/a.py"
	for added, want := range map[string]string{
		"see WIDGET-001\n":         "rejected",
		"see WIDGET-001-0023\n":    "cem-lexical-v0",
		"see xWIDGET-001\n":        "cem-lexical-v0",
		"see \xc3\xa9WIDGET-001\n": "rejected",
	} {
		result, err := Evaluate(Request{
			Context:  canonicalOCMContext(),
			Evidence: []Evidence{{ID: evidenceID, Path: "docs/id.md", Span: []byte("widget")}},
			Hunks: []Hunk{{
				ID: hunkID, OldPath: &newPath, NewPath: &newPath, Added: []byte(added), Disposition: "supported",
				Basis: []wire.Basis{{EvidenceID: evidenceID, Relation: "specification"}},
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := result.Results()[0][5]; got != want {
			t.Fatalf("added %q: outcome %q, want %q", added, got, want)
		}
	}
}

func TestFrozenDefaultsAndIssuedBytes(t *testing.T) {
	mutated := DefaultLimits()
	mutated.Terms = maxTerms + 1
	if got := DefaultLimits().Terms; got != maxTerms {
		t.Fatalf("fresh default term cap = %d, want %d", got, maxTerms)
	}
	if raw := CanonicalBytes(Result{}); raw != nil {
		t.Fatalf("caller-constructed result serialized as %q", raw)
	}
	tooSmall := DefaultLimits()
	tooSmall.OutputBytes = 1
	_, err := EvaluateWithLimits(Request{Context: canonicalOCMContext()}, tooSmall)
	typed, ok := err.(*Error)
	if !ok || typed.Code != "invalid-lrf-request" {
		t.Fatalf("too-small bound-document limit error = %v", err)
	}
}

func canonicalOCMContext() Context {
	target := strings.Repeat("2", 40)
	excluded := wire.ExcludedCEMPath
	ocmSpec := OCMSpec
	ocmDigest := strings.Repeat("c", 64)
	intentPath := "docs/intent.md"
	intentBlob := strings.Repeat("3", 40)
	start, end := int64(0), int64(32)
	intentDigest := strings.Repeat("d", 64)
	return Context{
		CEMSpec: wire.Spec02, CEMMapSHA256: strings.Repeat("a", 64), PatchSource: "canonical-derived",
		BaseRevision: strings.Repeat("1", 40), TargetRevision: &target, PatchSHA256: strings.Repeat("b", 64),
		ExcludedPath: &excluded, OCMSpec: &ocmSpec, OCMMapSHA256: &ocmDigest, IntentPath: &intentPath,
		IntentBlobOID: &intentBlob, IntentStart: &start, IntentEnd: &end, IntentSpanSHA256: &intentDigest,
	}
}

func loadCorpus(t *testing.T) corpusDocument {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test source")
	}
	directory := filepath.Join(filepath.Dir(source), "..", "..", "conformance", "lrf-v0")
	raw, err := os.ReadFile(filepath.Join(directory, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document corpusDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	validateCorpusBindings(t, directory, document)
	return document
}

func validateCorpusBindings(t *testing.T, directory string, document corpusDocument) {
	t.Helper()
	manifestRaw, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	registrationRaw, err := os.ReadFile(filepath.Join(directory, "registration.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest corpusManifest
	var registration corpusRegistration
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(registrationRaw, &registration); err != nil {
		t.Fatal(err)
	}
	if manifest.Kind != "conformance-plan" || manifest.Spec != "lrf-eval-manifest/0" || len(manifest.Entries) != len(document.Cases) {
		t.Fatal("conformance manifest has the wrong closed shape")
	}
	for index, testCase := range document.Cases {
		entry := manifest.Entries[index]
		input := compactWithLF(t, testCase.Input)
		expected := compactWithLF(t, testCase.Expected)
		if len(entry) != 3 || entry[0] != testCase.CoverageID ||
			entry[1] != domainHash("conformance-input", input) ||
			entry[2] != domainHash("conformance-output", expected) {
			t.Fatalf("%s: conformance payload binding mismatch", testCase.CoverageID)
		}
	}
	manifestValue, err := wire.Parse(manifestRaw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(manifestRaw, canonicalCorpusValue(manifestValue)) {
		t.Fatal("conformance manifest is not canonical JSON")
	}
	if registration.Spec != "lrf-conformance-registration/0" ||
		registration.ManifestSHA256 != domainHash("conformance-plan", manifestRaw) ||
		!bytes.Equal(registrationRaw, compactWithLF(t, registrationRaw)) {
		t.Fatal("conformance registration binding mismatch")
	}
}

func observeCase(t *testing.T, testCase corpusCase) []byte {
	t.Helper()
	input := decodeAction(t, testCase.Input)
	switch input.Action {
	case "core":
		return append(observeCore(t, input), '\n')
	case "matrix":
		return observeMatrix(t, input)
	case "repeat-process":
		return observeRepeat(t, input)
	case "repository":
		return append(observeRepository(t, input), '\n')
	case "repository-zero-links":
		return observeZeroLinks(t, input)
	case "commitment-mutation":
		return observeMutation(t, input)
	default:
		t.Fatalf("unknown action %q", input.Action)
		return nil
	}
}

func decodeAction(t *testing.T, raw []byte) actionInput {
	t.Helper()
	var input actionInput
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	return input
}

func observeCore(t *testing.T, input actionInput) []byte {
	t.Helper()
	if input.Request == nil {
		t.Fatal("core action lacks request")
	}
	request := input.Request.convert(t)
	limits := limitsFrom(t, input.Limits)
	result, err := evaluate(request, limits, input.Fault)
	if err != nil {
		code := ""
		if typed, ok := err.(*Error); ok {
			code = typed.Code
		}
		return []byte(`{"error":` + wire.CanonicalString(code) + `,"exitCode":"2","stdout":null}`)
	}
	stdout := bytes.TrimSuffix(canonicalResultBytes(t, result), []byte{'\n'})
	return append([]byte(`{"error":null,"exitCode":`+wire.CanonicalString(strconv.Itoa(result.ExitCode()))+`,"stdout":`), append(stdout, '}')...)
}

func observeMatrix(t *testing.T, input actionInput) []byte {
	t.Helper()
	var out strings.Builder
	out.WriteString(`{"subcases":[`)
	for index, raw := range input.Subcases {
		if index > 0 {
			out.WriteByte(',')
		}
		subcase := decodeAction(t, raw)
		var named struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &named); err != nil {
			t.Fatal(err)
		}
		out.WriteString(`{"name":`)
		out.WriteString(wire.CanonicalString(named.Name))
		out.WriteString(`,"observed":`)
		out.Write(observeCore(t, subcase))
		out.WriteByte('}')
	}
	out.WriteString("]}\n")
	return []byte(out.String())
}

func observeRepeat(t *testing.T, input actionInput) []byte {
	t.Helper()
	if input.Request == nil {
		t.Fatal("repeat action lacks request")
	}
	forward := input.Request.convert(t)
	reverse := input.Request.convert(t)
	reverseRequest(&reverse)
	first, err := Evaluate(forward)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Evaluate(reverse)
	if err != nil {
		t.Fatal(err)
	}
	firstBytes := canonicalResultBytes(t, first)
	secondBytes := canonicalResultBytes(t, second)
	freshForward := freshProcessBytes(t, input.Request, false)
	freshReverse := freshProcessBytes(t, input.Request, true)
	if !bytes.Equal(firstBytes, freshForward) || !bytes.Equal(firstBytes, freshReverse) {
		t.Fatal("fresh-process evaluation differs from same-process canonical bytes")
	}
	return []byte(`{"byteIdentical":` + strconv.FormatBool(bytes.Equal(firstBytes, secondBytes)) +
		`,"first":` + wire.CanonicalString(string(firstBytes)) + `,"second":` +
		wire.CanonicalString(string(secondBytes)) + "}\n")
}

func TestLRFFreshProcessHelper(t *testing.T) {
	encoded := os.Getenv("CORVINT_LRF_FRESH_REQUEST")
	if encoded == "" {
		return
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var dto requestDTO
	if err := json.Unmarshal(raw, &dto); err != nil {
		t.Fatal(err)
	}
	request := dto.convert(t)
	if os.Getenv("CORVINT_LRF_FRESH_REVERSE") == "1" {
		reverseRequest(&request)
	}
	result, err := Evaluate(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("CORVINT_LRF_FRESH_OUTPUT"), canonicalResultBytes(t, result), 0o600); err != nil {
		t.Fatal(err)
	}
}

func freshProcessBytes(t *testing.T, request *requestDTO, reverse bool) []byte {
	t.Helper()
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "result.json")
	command := exec.Command(os.Args[0], "-test.run=^TestLRFFreshProcessHelper$")
	command.Env = append(os.Environ(),
		"CORVINT_LRF_FRESH_REQUEST="+base64.StdEncoding.EncodeToString(raw),
		"CORVINT_LRF_FRESH_OUTPUT="+output,
	)
	if reverse {
		command.Env = append(command.Env, "CORVINT_LRF_FRESH_REVERSE=1")
	}
	if combined, err := command.CombinedOutput(); err != nil {
		t.Fatalf("fresh-process evaluator failed: %v\n%s", err, combined)
	}
	result, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func observeRepository(t *testing.T, input actionInput) []byte {
	t.Helper()
	document, err := wire.ParseMap([]byte(input.CEM))
	if err != nil {
		return []byte(`{"error":` + wire.CanonicalString(cemcode.CodeOf(err)) + `,"exitCode":"2","stdout":null}`)
	}
	if document.Spec != wire.Spec01 || len(input.CommitOIDs) != 2 {
		t.Fatal("repository harness case is outside the frozen projection fixture")
	}
	digest := sha256.Sum256([]byte(input.CEM))
	patchSource := "explicit-out-of-band"
	if input.PatchDefaulted {
		patchSource = "default-out-of-band"
	}
	target := input.CommitOIDs[1]
	request := Request{Context: Context{
		CEMSpec: document.Spec, CEMMapSHA256: hex.EncodeToString(digest[:]), PatchSource: patchSource,
		BaseRevision: document.BaseRevision, TargetRevision: &target, PatchSHA256: document.PatchSha256,
	}}
	result, err := Evaluate(request)
	if err != nil {
		t.Fatal(err)
	}
	stdout := bytes.TrimSuffix(canonicalResultBytes(t, result), []byte{'\n'})
	return append([]byte(`{"error":null,"exitCode":"0","stdout":`), append(stdout, '}')...)
}

func observeZeroLinks(t *testing.T, input actionInput) []byte {
	t.Helper()
	document, err := wire.ParseMap([]byte(input.CEM))
	if err != nil {
		t.Fatal(err)
	}
	var ocm struct {
		Spec           string `json:"spec"`
		TargetRevision string `json:"targetRevision"`
		CEM            struct {
			MapSHA256   string `json:"mapSha256"`
			PatchSHA256 string `json:"patchSha256"`
		} `json:"cem"`
		IntentScope struct {
			Path       string `json:"path"`
			BlobOID    string `json:"blobOid"`
			SpanSHA256 string `json:"spanSha256"`
			Span       struct {
				Start int64 `json:"start"`
				End   int64 `json:"end"`
			} `json:"span"`
		} `json:"intentScope"`
		Obligations []struct {
			Disposition string   `json:"disposition"`
			HunkIDs     []string `json:"hunkIds"`
		} `json:"obligations"`
	}
	if err := json.Unmarshal([]byte(input.OCM), &ocm); err != nil {
		t.Fatal(err)
	}
	linked := 0
	unknown := 0
	for _, obligation := range ocm.Obligations {
		if obligation.Disposition == "linked" {
			linked++
		} else if obligation.Disposition == "unknown" {
			unknown++
		}
	}
	if document.Spec != wire.Spec01 {
		t.Fatal("zero-links fixture must bind CEM 0.1")
	}
	digest := sha256.Sum256([]byte(input.CEM))
	ocmDigest := sha256.Sum256([]byte(input.OCM))
	ocmMapSHA256 := hex.EncodeToString(ocmDigest[:])
	request := Request{Context: Context{
		CEMSpec: document.Spec, CEMMapSHA256: hex.EncodeToString(digest[:]), PatchSource: "explicit-out-of-band",
		BaseRevision: document.BaseRevision, TargetRevision: &ocm.TargetRevision, PatchSHA256: document.PatchSha256,
		OCMSpec: &ocm.Spec, OCMMapSHA256: &ocmMapSHA256,
		IntentPath: &ocm.IntentScope.Path, IntentBlobOID: &ocm.IntentScope.BlobOID,
		IntentStart: &ocm.IntentScope.Span.Start, IntentEnd: &ocm.IntentScope.Span.End,
		IntentSpanSHA256: &ocm.IntentScope.SpanSHA256,
	}}
	_, compatibilityErr := Evaluate(request)
	typed, ok := compatibilityErr.(*Error)
	if !ok || typed.Code != "unsupported-lrf-context" {
		t.Fatalf("OCM plus CEM 0.1 compatibility error = %v", compatibilityErr)
	}
	return []byte(fmt.Sprintf(
		`{"error":%s,"exitCode":"2","linked":%s,"stdout":null,"unknown":%s}`+"\n",
		wire.CanonicalString(typed.Code),
		wire.CanonicalString(strconv.Itoa(linked)), wire.CanonicalString(strconv.Itoa(unknown)),
	))
}

func canonicalResultBytes(t *testing.T, result Result) []byte {
	t.Helper()
	raw := CanonicalBytes(result)
	if len(raw) < 2 || raw[len(raw)-1] != '\n' || raw[len(raw)-2] == '\n' {
		t.Fatal("canonical LRF result must have exactly one terminal LF")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw[:len(raw)-1]); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(compact.Bytes(), raw[:len(raw)-1]) {
		t.Fatal("LRF result is not compact JSON")
	}
	return raw
}

func observeMutation(t *testing.T, input actionInput) []byte {
	t.Helper()
	original := compactWithLF(t, input.Subject)
	mutated := append([]byte(nil), original...)
	index := len(mutated) - 2
	if mutated[index] == '0' {
		mutated[index] = '1'
	} else {
		mutated[index] = '0'
	}
	detected := domainHash("conformance-input", original) != domainHash("conformance-input", mutated)
	return []byte(`{"detected":` + strconv.FormatBool(detected) + `,"mutation":"single-byte-before-terminal-lf"}` + "\n")
}

func domainHash(kind string, payload []byte) string {
	hash := sha256.New()
	hash.Write([]byte("corvint-lrf-eval/0\x00"))
	hash.Write([]byte(kind))
	hash.Write([]byte{0})
	hash.Write(payload)
	return hex.EncodeToString(hash.Sum(nil))
}

func compactWithLF(t *testing.T, raw []byte) []byte {
	t.Helper()
	value, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return append(canonicalCorpusValue(value), '\n')
}

func canonicalCorpusValue(value wire.Value) []byte {
	switch value.Kind {
	case wire.KindNull:
		return []byte("null")
	case wire.KindBool:
		return []byte(strconv.FormatBool(value.Bool))
	case wire.KindInt:
		return []byte(strconv.FormatInt(value.Int, 10))
	case wire.KindString:
		return []byte(wire.CanonicalString(value.Str))
	case wire.KindArray:
		var out strings.Builder
		out.WriteByte('[')
		for index, item := range value.Arr {
			if index > 0 {
				out.WriteByte(',')
			}
			out.Write(canonicalCorpusValue(item))
		}
		out.WriteByte(']')
		return []byte(out.String())
	case wire.KindObject:
		keys := append([]string(nil), value.Obj.Keys...)
		sort.Strings(keys)
		var out strings.Builder
		out.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				out.WriteByte(',')
			}
			out.WriteString(wire.CanonicalString(key))
			out.WriteByte(':')
			out.Write(canonicalCorpusValue(value.Obj.Values[key]))
		}
		out.WriteByte('}')
		return []byte(out.String())
	default:
		panic("unknown strict JSON kind")
	}
}

func limitsFrom(t *testing.T, values map[string]string) Limits {
	t.Helper()
	limits := DefaultLimits()
	for name, raw := range values {
		value, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatal(err)
		}
		switch name {
		case "lexical_bytes":
			limits.LexicalBytes = value
		case "terms":
			limits.Terms = value
		case "edges":
			limits.Edges = value
		case "results":
			limits.Results = value
		case "issues":
			limits.Issues = value
		case "output_bytes":
			limits.OutputBytes = value
		case "evidence_bytes":
			limits.EvidenceBytes = value
		case "evidence_lines":
			limits.EvidenceLines = value
		default:
			t.Fatalf("unknown limit %q", name)
		}
	}
	return limits
}

func (dto requestDTO) convert(t *testing.T) Request {
	t.Helper()
	request := Request{Context: dto.Context.convert(t)}
	for _, item := range dto.Evidence {
		request.Evidence = append(request.Evidence, Evidence{ID: item.ID, Path: item.Path, Span: []byte(item.Span)})
	}
	for _, item := range dto.Hunks {
		hunk := Hunk{
			ID: item.ID, OldPath: item.OldPath, NewPath: item.NewPath, Added: []byte(item.Added),
			Disposition: item.Disposition, Ordinal: item.Ordinal,
		}
		for _, basis := range item.Basis {
			hunk.Basis = append(hunk.Basis, wire.Basis{EvidenceID: basis.EvidenceID, Relation: basis.Relation})
		}
		request.Hunks = append(request.Hunks, hunk)
	}
	for _, item := range dto.Obligations {
		request.Obligations = append(request.Obligations, Obligation{
			ID: item.ID, Statement: []byte(item.Statement), HunkIDs: append([]string(nil), item.HunkIDs...),
			ClaimPaths: append([]string(nil), item.ClaimPaths...), Ordinal: item.Ordinal,
		})
	}
	return request
}

func (dto contextDTO) convert(t *testing.T) Context {
	t.Helper()
	return Context{
		CEMSpec: dto.CEMSpec, CEMMapSHA256: dto.CEMMapSHA256, PatchSource: dto.PatchSource,
		BaseRevision: dto.BaseRevision, TargetRevision: dto.TargetRevision, PatchSHA256: dto.PatchSHA256,
		ExcludedPath: dto.ExcludedPath, OCMSpec: dto.OCMSpec, OCMMapSHA256: dto.OCMMapSHA256,
		IntentPath: dto.IntentPath, IntentBlobOID: dto.IntentBlobOID,
		IntentStart: parseOptionalInt(t, dto.IntentStart), IntentEnd: parseOptionalInt(t, dto.IntentEnd),
		IntentSpanSHA256: dto.IntentSpanSHA256,
	}
}

func parseOptionalInt(t *testing.T, raw *string) *int64 {
	t.Helper()
	if raw == nil {
		return nil
	}
	value, err := strconv.ParseInt(*raw, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return &value
}

func reverseRequest(request *Request) {
	reverseEvidence(request.Evidence)
	reverseHunks(request.Hunks)
	reverseObligations(request.Obligations)
	for index := range request.Hunks {
		reverseBasis(request.Hunks[index].Basis)
	}
	for index := range request.Obligations {
		reverseStrings(request.Obligations[index].HunkIDs)
	}
}

func reverseEvidence(values []Evidence) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func reverseHunks(values []Hunk) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func reverseObligations(values []Obligation) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func reverseBasis(values []wire.Basis) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func reverseStrings(values []string) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}
