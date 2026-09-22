package verify

// Corpus harness: constructs the frozen interop and conformance repositories
// per their manifests and runs every valid, invalid, drift, and mutation
// vector through the full verification flow.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readFixture(t *testing.T, parts ...string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func gitAt(t *testing.T, dir, author, email, date string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "HOME="+os.TempDir(), "XDG_CONFIG_HOME="+os.TempDir(),
		"GIT_AUTHOR_NAME="+author, "GIT_AUTHOR_EMAIL="+email, "GIT_AUTHOR_DATE="+date,
		"GIT_COMMITTER_NAME="+author, "GIT_COMMITTER_EMAIL="+email, "GIT_COMMITTER_DATE="+date)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func copyTree(t *testing.T, source, destination string) {
	t.Helper()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// buildRepo constructs a corpus repository per the frozen recipe and asserts
// the resulting commit OID.
func buildRepo(t *testing.T, baseDir, objectFormat, author, email, date, message, wantOID string) string {
	t.Helper()
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	initArgs := []string{"init", "-q", "-b", "main"}
	if objectFormat == "sha256" {
		initArgs = append(initArgs, "--object-format=sha256")
	}
	command := exec.Command("git", initArgs...)
	command.Dir = root
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+os.TempDir())
	if out, initErr := command.CombinedOutput(); initErr != nil {
		if objectFormat == "sha256" {
			t.Skipf("git lacks sha256 support: %s", out)
		}
		t.Fatalf("init: %v %s", initErr, out)
	}
	copyTree(t, baseDir, root)
	gitAt(t, root, author, email, date, "add", ".")
	gitAt(t, root, author, email, date, "commit", "-qm", message)
	head := gitAt(t, root, author, email, date, "rev-parse", "HEAD")
	if wantOID != "" && head != wantOID {
		t.Fatalf("constructed %s repository head %s, manifest requires %s", objectFormat, head, wantOID)
	}
	return root
}

type manifest struct {
	Repository struct {
		Root      string            `json:"root"`
		Author    string            `json:"author"`
		Timestamp string            `json:"timestamp"`
		Message   string            `json:"message"`
		Revisions map[string]string `json:"revisions"`
	} `json:"repository"`
	Valid []struct {
		Name         string `json:"name"`
		ObjectFormat string `json:"objectFormat"`
		Map          string `json:"map"`
		Patch        string `json:"patch"`
	} `json:"valid"`
	Invalid []struct {
		Name         string `json:"name"`
		Map          string `json:"map"`
		Patch        string `json:"patch"`
		PatchRecipe  string `json:"patchRecipe"`
		Class        string `json:"class"`
		ExpectedCode string `json:"expectedCode"`
	} `json:"invalid"`
	Drift []struct {
		Name           string          `json:"name"`
		Map            string          `json:"map"`
		Patch          string          `json:"patch"`
		TargetPatch    *string         `json:"targetPatch"`
		Timestamp      *string         `json:"timestamp"`
		Message        *string         `json:"message"`
		TargetRevision string          `json:"targetRevision"`
		Accept         bool            `json:"accept"`
		Status         string          `json:"status"`
		TargetBlobOid  *string         `json:"targetBlobOid"`
		TargetSpan     json.RawMessage `json:"targetSpan"`
	} `json:"drift"`
}

func loadManifest(t *testing.T) (manifest, string) {
	t.Helper()
	interop := filepath.Join(repoRoot(t), "interop", "cem-0.1")
	var m manifest
	if err := json.Unmarshal(readFixture(t, interop, "manifest.json"), &m); err != nil {
		t.Fatal(err)
	}
	return m, interop
}

func splitAuthor(t *testing.T, combined string) (string, string) {
	t.Helper()
	name, rest, found := strings.Cut(combined, " <")
	email, hasClose := strings.CutSuffix(rest, ">")
	if !found || !hasClose {
		t.Fatalf("malformed author %q", combined)
	}
	return name, email
}

func buildInterop(t *testing.T, objectFormat string) (*gitauth.Repository, string) {
	t.Helper()
	m, interop := loadManifest(t)
	name, email := splitAuthor(t, m.Repository.Author)
	root := buildRepo(t, filepath.Join(interop, "repository", "base"), objectFormat,
		name, email, m.Repository.Timestamp, m.Repository.Message, m.Repository.Revisions[objectFormat])
	repository, err := gitauth.Open(root, gitrun.NewBudget(4096, 120e9))
	if err != nil {
		t.Fatal(err)
	}
	return repository, root
}

func verifyFlow(t *testing.T, repository *gitauth.Repository, mapBytes, patchBytes []byte, target string) (*Outcome, error) {
	t.Helper()
	document, err := wire.ParseMap(mapBytes)
	if err != nil {
		return nil, err
	}
	return Exact(context.Background(), repository, document, patchBytes, ExactOptions{Target: target})
}

func TestInteropValidCases(t *testing.T) {
	m, interop := loadManifest(t)
	repositories := map[string]*gitauth.Repository{}
	for _, testCase := range m.Valid {
		repository, ok := repositories[testCase.ObjectFormat]
		if !ok {
			repository, _ = buildInterop(t, testCase.ObjectFormat)
			repositories[testCase.ObjectFormat] = repository
		}
		_, err := verifyFlow(t, repository,
			readFixture(t, interop, testCase.Map), readFixture(t, interop, testCase.Patch), "")
		if err != nil {
			t.Errorf("%s: %v", testCase.Name, err)
		}
	}
}

func TestInteropInvalidCases(t *testing.T) {
	m, interop := loadManifest(t)
	repository, _ := buildInterop(t, "sha1")
	for _, testCase := range m.Invalid {
		var patchBytes []byte
		switch {
		case testCase.PatchRecipe == "cem/0.1-lf-overflow-x-lines":
			patchBytes = bytes.Repeat([]byte{0x78, 0x0a}, 262145)
		case testCase.PatchRecipe != "":
			t.Fatalf("%s: unknown recipe %q", testCase.Name, testCase.PatchRecipe)
		default:
			patchBytes = readFixture(t, interop, testCase.Patch)
		}
		_, err := verifyFlow(t, repository, readFixture(t, interop, testCase.Map), patchBytes, "")
		if err == nil {
			t.Errorf("%s: accepted", testCase.Name)
			continue
		}
		if testCase.ExpectedCode != "" && cemcode.CodeOf(err) != testCase.ExpectedCode {
			t.Errorf("%s: code %q, want %q (%v)", testCase.Name, cemcode.CodeOf(err), testCase.ExpectedCode, err)
		}
	}
}

func TestInteropDriftCases(t *testing.T) {
	m, interop := loadManifest(t)
	repository, root := buildInterop(t, "sha1")
	name, email := splitAuthor(t, m.Repository.Author)
	for _, testCase := range m.Drift {
		if testCase.TargetPatch != nil {
			date := *testCase.Timestamp
			gitAt(t, root, name, email, date, "checkout", "-q", m.Repository.Revisions["sha1"])
			gitAt(t, root, name, email, date, "apply", filepath.Join(interop, *testCase.TargetPatch))
			gitAt(t, root, name, email, date, "add", "-A", ".")
			gitAt(t, root, name, email, date, "commit", "-qm", *testCase.Message)
			head := gitAt(t, root, name, email, date, "rev-parse", "HEAD")
			if head != testCase.TargetRevision {
				t.Fatalf("%s: constructed target %s, manifest requires %s", testCase.Name, head, testCase.TargetRevision)
			}
		}
		outcome, err := verifyFlow(t, repository,
			readFixture(t, interop, testCase.Map), readFixture(t, interop, testCase.Patch), testCase.TargetRevision)
		if testCase.Accept {
			if err != nil {
				t.Errorf("%s: rejected: %v", testCase.Name, err)
				continue
			}
		} else if err == nil || cemcode.CodeOf(err) != cemcode.EvidenceDrift {
			t.Errorf("%s: got %v, want evidence-drift rejection", testCase.Name, err)
			continue
		}
		if outcome != nil {
			assertDrift(t, testCase.Name, outcome, testCase.Status, testCase.TargetBlobOid, testCase.TargetSpan)
			if outcome.TargetRevision != testCase.TargetRevision {
				t.Errorf("%s: reported target %s", testCase.Name, outcome.TargetRevision)
			}
		}
	}
}

func assertDrift(t *testing.T, name string, outcome *Outcome, status string, blobOid *string, span json.RawMessage) {
	t.Helper()
	if len(outcome.Drift) != 1 {
		t.Errorf("%s: %d drift items", name, len(outcome.Drift))
		return
	}
	item := outcome.Drift[0]
	if string(item.Status) != status {
		t.Errorf("%s: status %s, want %s", name, item.Status, status)
	}
	wantBlob := ""
	if blobOid != nil {
		wantBlob = *blobOid
	}
	if item.TargetBlobOid != wantBlob {
		t.Errorf("%s: target blob %q, want %q", name, item.TargetBlobOid, wantBlob)
	}
	var wantSpan *wire.Span
	if len(span) > 0 && string(span) != "null" {
		var decoded struct{ Start, End int64 }
		if err := json.Unmarshal(span, &decoded); err != nil {
			t.Fatal(err)
		}
		wantSpan = &wire.Span{Start: decoded.Start, End: decoded.End}
	}
	switch {
	case wantSpan == nil && item.TargetSpan != nil:
		t.Errorf("%s: unexpected target span %+v", name, *item.TargetSpan)
	case wantSpan != nil && (item.TargetSpan == nil || *item.TargetSpan != *wantSpan):
		t.Errorf("%s: target span %+v, want %+v", name, item.TargetSpan, wantSpan)
	}
}

// --- conformance/cem-0.1 mutation suite ---

type conformanceCases struct {
	Valid          string `json:"valid"`
	InvalidPatches []struct {
		Name          string `json:"name"`
		Patch         string `json:"patch"`
		PatchSha256   string `json:"patchSha256"`
		ReferenceCode string `json:"referenceCode"`
	} `json:"invalidPatches"`
	Invalid []struct {
		Name            string                     `json:"name"`
		Operation       string                     `json:"operation"`
		Pointer         string                     `json:"pointer"`
		Value           json.RawMessage            `json:"value"`
		Values          map[string]json.RawMessage `json:"values"`
		ReferenceCode   string                     `json:"referenceCode"`
		AcceptableCodes []string                   `json:"acceptableCodes"`
	} `json:"invalid"`
}

func applyPointer(t *testing.T, document any, pointer string, operation string, value json.RawMessage) any {
	t.Helper()
	segments := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	return mutate(t, document, segments, operation, value)
}

func mutate(t *testing.T, node any, segments []string, operation string, value json.RawMessage) any {
	t.Helper()
	if len(segments) == 0 {
		t.Fatal("empty pointer")
	}
	key := segments[0]
	switch typed := node.(type) {
	case map[string]any:
		if len(segments) == 1 {
			if operation == "remove" {
				delete(typed, key)
			} else {
				typed[key] = decodeRaw(t, value)
			}
			return typed
		}
		typed[key] = mutate(t, typed[key], segments[1:], operation, value)
		return typed
	case []any:
		index, err := strconv.Atoi(key)
		if err != nil || index < 0 || index >= len(typed) {
			t.Fatalf("pointer index %q", key)
		}
		if len(segments) == 1 {
			if operation == "remove" {
				return append(typed[:index], typed[index+1:]...)
			}
			typed[index] = decodeRaw(t, value)
			return typed
		}
		typed[index] = mutate(t, typed[index], segments[1:], operation, value)
		return typed
	default:
		t.Fatalf("pointer descends into non-container %T", node)
		return nil
	}
}

func decodeRaw(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestConformanceMutationSuite(t *testing.T) {
	conformance := filepath.Join(repoRoot(t), "conformance", "cem-0.1")
	var cases conformanceCases
	if err := json.Unmarshal(readFixture(t, conformance, "cases.json"), &cases); err != nil {
		t.Fatal(err)
	}
	root := buildRepo(t, filepath.Join(conformance, "base"), "sha1",
		"CEM Conformance", "cem@example.invalid", "2000-01-01T00:00:00+0000",
		"CEM 0.1 base", "4ca153370afd9bd8c6034ad73acc3925150ab681")
	repository, err := gitauth.Open(root, gitrun.NewBudget(4096, 120e9))
	if err != nil {
		t.Fatal(err)
	}
	validMap := readFixture(t, conformance, cases.Valid)
	patchBytes := readFixture(t, conformance, "change.patch")
	if _, err := verifyFlow(t, repository, validMap, patchBytes, ""); err != nil {
		t.Fatalf("valid vector rejected: %v", err)
	}
	for _, testCase := range cases.Invalid {
		mutated := mutateMap(t, validMap, testCase.Operation, testCase.Pointer, testCase.Value, testCase.Values)
		_, err := verifyFlow(t, repository, mutated, patchBytes, "")
		if err == nil {
			t.Errorf("%s: accepted", testCase.Name)
			continue
		}
		if len(testCase.AcceptableCodes) > 0 && !contains(testCase.AcceptableCodes, cemcode.CodeOf(err)) {
			t.Errorf("%s: code %q outside acceptableCodes %v", testCase.Name, cemcode.CodeOf(err), testCase.AcceptableCodes)
		}
	}
	for _, testCase := range cases.InvalidPatches {
		mutated := mutateMap(t, validMap, "set", "/patchSha256",
			json.RawMessage(fmt.Sprintf("%q", testCase.PatchSha256)), nil)
		badPatch := readFixture(t, conformance, testCase.Patch)
		_, err := verifyFlow(t, repository, mutated, badPatch, "")
		if err == nil {
			t.Errorf("%s: accepted", testCase.Name)
		}
	}
}

// TestEvidenceBlobIdentityAndSpanEnd checks the ALGORITHMS.md evidence rule
// with self-consistent records: a blobOid other than the base entry's OID and
// an empty span are unavailable even when the span digest and ID recompute,
// and a half-open span ending exactly at end of file is in bounds.
func TestEvidenceBlobIdentityAndSpanEnd(t *testing.T) {
	conformance := filepath.Join(repoRoot(t), "conformance", "cem-0.1")
	root := buildRepo(t, filepath.Join(conformance, "base"), "sha1",
		"CEM Conformance", "cem@example.invalid", "2000-01-01T00:00:00+0000",
		"CEM 0.1 base", "4ca153370afd9bd8c6034ad73acc3925150ab681")
	repository, err := gitauth.Open(root, gitrun.NewBudget(4096, 120e9))
	if err != nil {
		t.Fatal(err)
	}
	validMap := readFixture(t, conformance, "valid.cem.json")
	patchBytes := readFixture(t, conformance, "change.patch")
	rule := readFixture(t, conformance, "base", "docs", "rule.txt")
	const ruleOID = "d8586f1d6c8e69f34f215d2f4cbf0e491fe795ca"
	withEvidence := func(blobOid string, span wire.Span) []byte {
		digest := sha256.Sum256(rule[span.Start:span.End])
		spanSha256 := hex.EncodeToString(digest[:])
		id := strconv.Quote(wire.EvidenceIdentity(blobOid, "docs/rule.txt", span, spanSha256))
		return mutateMap(t, validMap, "set-many", "", nil, map[string]json.RawMessage{
			"/evidence/0/blobOid":         json.RawMessage(strconv.Quote(blobOid)),
			"/evidence/0/span":            json.RawMessage(fmt.Sprintf(`{"end":%d,"start":%d}`, span.End, span.Start)),
			"/evidence/0/spanSha256":      json.RawMessage(strconv.Quote(spanSha256)),
			"/evidence/0/id":              json.RawMessage(id),
			"/hunks/0/basis/0/evidenceId": json.RawMessage(id),
		})
	}
	undeclared := withEvidence(strings.Repeat("1", 40), wire.Span{Start: 6, End: 36})
	if _, err := verifyFlow(t, repository, undeclared, patchBytes, ""); cemcode.CodeOf(err) != cemcode.EvidenceUnavailable {
		t.Errorf("undeclared blob: got %v, want %s", err, cemcode.EvidenceUnavailable)
	}
	empty := withEvidence(ruleOID, wire.Span{Start: 6, End: 6})
	if _, err := verifyFlow(t, repository, empty, patchBytes, ""); cemcode.CodeOf(err) != cemcode.EvidenceUnavailable {
		t.Errorf("empty span: got %v, want %s", err, cemcode.EvidenceUnavailable)
	}
	atEOF := withEvidence(ruleOID, wire.Span{Start: 6, End: int64(len(rule))})
	if _, err := verifyFlow(t, repository, atEOF, patchBytes, ""); err != nil {
		t.Errorf("span ending at end of file: %v", err)
	}
}

// TestClassifySpanCountsAMatchEndingAtTheEnd checks the ALGORITHMS.md drift
// rule: overlapping matches count, including one that ends at the last target
// byte, so two occurrences are ambiguous rather than relocated.
func TestClassifySpanCountsAMatchEndingAtTheEnd(t *testing.T) {
	status, span := ClassifySpan(true, false, []byte("xx"), []byte("x"), wire.Span{Start: 0, End: 1})
	if status != DriftAmbiguous || span != nil {
		t.Fatalf("ClassifySpan = %v, %v; want %v with no span", status, span, DriftAmbiguous)
	}
}

func mutateMap(t *testing.T, source []byte, operation, pointer string, value json.RawMessage, values map[string]json.RawMessage) []byte {
	t.Helper()
	document := decodeRaw(t, source)
	switch operation {
	case "set", "remove":
		document = applyPointer(t, document, pointer, operation, value)
	case "set-many":
		for member, raw := range values {
			document = applyPointer(t, document, member, "set", raw)
		}
	default:
		t.Fatalf("unknown operation %q", operation)
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// TestDriftSymlinkSameOIDIsStable is CEM-PILOT-019 (decision 0098): when the
// target path becomes a symlink whose target string equals the base blob
// bytes, the blob OID is unchanged and driftItem reports stable, because the
// frozen same-path rule tests OID identity before any entry kind.
func TestDriftSymlinkSameOIDIsStable(t *testing.T) {
	root := t.TempDir()
	const author, email, date = "A", "a@example.test", "2024-01-01T00:00:00Z"
	gitAt(t, root, author, email, date, "init", "-q")
	content := "target.txt"
	if err := os.WriteFile(filepath.Join(root, "f"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, root, author, email, date, "add", ".")
	gitAt(t, root, author, email, date, "commit", "-qm", "base")
	base := gitAt(t, root, author, email, date, "rev-parse", "HEAD^{tree}")
	blob := gitAt(t, root, author, email, date, "rev-parse", "HEAD:f")
	if err := os.Remove(filepath.Join(root, "f")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(content, filepath.Join(root, "f")); err != nil {
		t.Fatal(err)
	}
	gitAt(t, root, author, email, date, "add", "-A")
	gitAt(t, root, author, email, date, "commit", "-qm", "symlink")
	target := gitAt(t, root, author, email, date, "rev-parse", "HEAD^{tree}")

	repository, err := gitauth.Open(root, gitrun.NewBudget(4096, 120e9))
	if err != nil {
		t.Fatal(err)
	}
	item, err := driftItem(context.Background(), repository,
		wire.Evidence{ID: "e1", BlobOid: blob, Path: "f", Span: wire.Span{Start: 0, End: 6}}, base, target)
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != DriftStable {
		t.Errorf("status = %q, want %q (blob OID unchanged is stable per ALGORITHMS.md:154-155)", item.Status, DriftStable)
	}
	if item.TargetBlobOid != blob {
		t.Errorf("target blob oid = %q, want %q", item.TargetBlobOid, blob)
	}
}

// TestDriftNonRegularTargetEntryIsDeleted is CEM-PILOT-019 (decision 0098): a
// same-path target entry with a different OID that is not a regular-file blob
// (symlink, directory, gitlink) has no file content to search, so it reports
// the frozen `deleted` status with no target blob OID or span. The symlink's
// link text contains the cited bytes once, so a blob search would have said
// relocated.
func TestDriftNonRegularTargetEntryIsDeleted(t *testing.T) {
	const author, email, date = "A", "a@example.test", "2024-01-01T00:00:00Z"
	replacements := map[string]func(t *testing.T, root, path string){
		"symlink": func(t *testing.T, root, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("../evidence", path); err != nil {
				t.Fatal(err)
			}
			gitAt(t, root, author, email, date, "add", "-A")
		},
		"directory": func(t *testing.T, root, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(path, "x"), []byte("evidence"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitAt(t, root, author, email, date, "add", "-A")
		},
		"gitlink": func(t *testing.T, root, path string) {
			commit := gitAt(t, root, author, email, date, "rev-parse", "HEAD")
			gitAt(t, root, author, email, date, "rm", "-q", "--cached", "f")
			gitAt(t, root, author, email, date, "update-index", "--add", "--cacheinfo", "160000,"+commit+",f")
		},
	}
	for kind, replace := range replacements {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "f")
			gitAt(t, root, author, email, date, "init", "-q")
			if err := os.WriteFile(path, []byte("evidence bytes"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitAt(t, root, author, email, date, "add", ".")
			gitAt(t, root, author, email, date, "commit", "-qm", "base")
			base := gitAt(t, root, author, email, date, "rev-parse", "HEAD^{tree}")
			blob := gitAt(t, root, author, email, date, "rev-parse", "HEAD:f")
			replace(t, root, path)
			target := gitAt(t, root, author, email, date, "write-tree")

			repository, err := gitauth.Open(root, gitrun.NewBudget(4096, 120e9))
			if err != nil {
				t.Fatal(err)
			}
			item, err := driftItem(context.Background(), repository,
				wire.Evidence{ID: "e1", BlobOid: blob, Path: "f", Span: wire.Span{Start: 0, End: 8}}, base, target)
			if err != nil {
				t.Fatal(err)
			}
			if item.Status != DriftDeleted || item.TargetBlobOid != "" || item.TargetSpan != nil {
				t.Errorf("item = %+v, want deleted with no target blob OID or span", item)
			}
		})
	}
}

// TestCheckTargetSidecarRejectsDrifted checks CEM-CB-009: a present
// target-side .corvint/change.cem.json blob whose bytes differ from the
// verified CEM input is refused, not silently accepted.
func TestCheckTargetSidecarRejectsDrifted(t *testing.T) {
	root := t.TempDir()
	const author, email, date = "A", "a@example.test", "2024-01-01T00:00:00Z"
	gitAt(t, root, author, email, date, "init", "-q")
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	stored := []byte(`{"stored":true}`)
	if err := os.WriteFile(filepath.Join(root, ".corvint", "change.cem.json"), stored, 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, root, author, email, date, "add", ".")
	gitAt(t, root, author, email, date, "commit", "-qm", "sidecar")
	target := gitAt(t, root, author, email, date, "rev-parse", "HEAD^{tree}")

	repository, err := gitauth.Open(root, gitrun.NewBudget(4096, 120e9))
	if err != nil {
		t.Fatal(err)
	}
	err = checkTargetSidecar(context.Background(), repository, target, []byte(`{"verified":true}`))
	if cemcode.CodeOf(err) != cemcode.ExcludedArtifactMismatch {
		t.Fatalf("checkTargetSidecar = %v, want %s", err, cemcode.ExcludedArtifactMismatch)
	}
}

// TestInheritedRejectsBaseRevisionThatIsNotACommitOID checks that Resolve's
// `^{commit}` peel must equal document.BaseRevision exactly: an annotated tag
// OID used as baseRevision must be refused, not silently peeled and accepted.
func TestInheritedRejectsBaseRevisionThatIsNotACommitOID(t *testing.T) {
	root := t.TempDir()
	const author, email, date = "A", "a@example.test", "2024-01-01T00:00:00Z"
	gitAt(t, root, author, email, date, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, root, author, email, date, "add", ".")
	gitAt(t, root, author, email, date, "commit", "-qm", "base")
	gitAt(t, root, author, email, date, "tag", "-a", "-m", "annotated", "v1")
	tagOID := gitAt(t, root, author, email, date, "rev-parse", "v1")

	repository, err := gitauth.Open(root, gitrun.NewBudget(4096, 120e9))
	if err != nil {
		t.Fatal(err)
	}
	document := &wire.Map{BaseRevision: tagOID}
	if _, err := inherited(context.Background(), repository, document, nil, ""); cemcode.CodeOf(err) != cemcode.BaseRevisionMismatch {
		t.Fatalf("inherited = %v, want %s", err, cemcode.BaseRevisionMismatch)
	}
}

// TestCheckEvidenceAcceptsExecutableBlob checks that an evidence path
// resolving to a regular 100755 (executable) blob is accepted per
// ALGORITHMS.md, not refused as though only 100644 were a regular file.
func TestCheckEvidenceAcceptsExecutableBlob(t *testing.T) {
	root := t.TempDir()
	const author, email, date = "A", "a@example.test", "2024-01-01T00:00:00Z"
	gitAt(t, root, author, email, date, "init", "-q")
	content := []byte("#!/bin/sh\necho hi\n")
	path := filepath.Join(root, "run.sh")
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatal(err)
	}
	gitAt(t, root, author, email, date, "add", ".")
	gitAt(t, root, author, email, date, "commit", "-qm", "executable")
	base := gitAt(t, root, author, email, date, "rev-parse", "HEAD^{tree}")
	blob := gitAt(t, root, author, email, date, "rev-parse", "HEAD:run.sh")
	if mode := strings.Fields(gitAt(t, root, author, email, date, "ls-tree", "HEAD", "run.sh"))[0]; mode != "100755" {
		t.Fatalf("fixture mode = %s, want 100755 (core.fileMode must be honored)", mode)
	}

	repository, err := gitauth.Open(root, gitrun.NewBudget(4096, 120e9))
	if err != nil {
		t.Fatal(err)
	}
	span := wire.Span{Start: 0, End: int64(len(content))}
	digest := sha256.Sum256(content)
	spanSha256 := hex.EncodeToString(digest[:])
	document := &wire.Map{Evidence: []wire.Evidence{{
		ID:         wire.EvidenceIdentity(blob, "run.sh", span, spanSha256),
		BlobOid:    blob,
		Path:       "run.sh",
		Span:       span,
		SpanSha256: spanSha256,
	}}}
	if err := checkEvidence(context.Background(), repository, document, base); err != nil {
		t.Fatalf("checkEvidence rejected a 100755 evidence blob: %v", err)
	}
}

// TestMechanicalProvenRefusesIdenticalRemovedAndAdded checks that a hunk
// whose removed and added bytes are byte-identical never proves a mechanical
// claim: there is nothing to prove.
func TestMechanicalProvenRefusesIdenticalRemovedAndAdded(t *testing.T) {
	hunk := &patch.Hunk{Body: []patch.BodyLine{
		{Prefix: '-', Payload: []byte("same\n")},
		{Prefix: '+', Payload: []byte("same\n")},
	}}
	for _, reason := range []string{"whitespace-only", "line-ending-only"} {
		if mechanicalProven(hunk, reason) {
			t.Errorf("mechanicalProven(%q) = true for identical removed/added bytes", reason)
		}
	}
}
