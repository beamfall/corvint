package frontierrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/frontier"
	"github.com/Beamfall/corvint/internal/tcq"
)

// These tests run a COMPLETE frontier computation over a real temporary Git
// repository: real commits, a real canonical `cem/0.2` sidecar produced by the
// real CEM workflow, a real OCM artifact bound to it, and a real intent scope
// read out of a committed Markdown blob. Nothing is stubbed — the point is that
// both seams are now implementations rather than doubles, so a defect in the
// shared verifier composition surfaces here and not in a mock's expectations.

// requirementID is the one obligation the fixture's intent scope declares. It
// appears verbatim in the intent Markdown, in the OCM obligation row, and
// inside the test claim's anchor, because the OCM verifier re-derives all three
// and refuses a claim whose anchor lacks the exact obligation ID.
const requirementID = "CFINT-001"

const intentDocument = "# Intent\n" +
	"\n" +
	"## Requirements\n" +
	"\n" +
	"- `" + requirementID + "`: Widget renderer changes remain relevant.\n" +
	"\n" +
	"## Next\n"

// claimAnchor is the requirement ID and nothing else. Two independent rules
// force that: the OCM verifier requires the claim anchor to contain the exact
// obligation ID with no identifier character beside it, and TCQ-V0-018 admits a
// Go table case only when its value matches `[A-Za-z0-9_-]{1,96}` — a set in
// which every character IS identifier-adjacent. The ID can therefore only sit
// at the end of an admitted case value, and the shortest such value is the ID.
const claimAnchor = requirementID

const testDocument = "package tests\n" +
	"\n" +
	"import \"testing\"\n" +
	"\n" +
	"func TestWidget(t *testing.T) {\n" +
	"\t_ = []struct{ name string }{{name: \"" + claimAnchor + "\"}}\n" +
	"}\n"

const pythonPost39Document = "def test_python_widget(value):\n" +
	"    \"\"\"" + requirementID + "\"\"\"\n" +
	"    match value:\n" +
	"        case 1:\n" +
	"            return 1\n"

const pythonCRLFDocument = "def test_python_crlf():\r\n" +
	"    \"\"\"" + requirementID + "\"\"\"\r\n" +
	"    assert True\r\n"

// claimSelector is the exact selector the extractor re-derives from the anchor:
// the enclosing Go test name plus the anchor's word slug. A guessed selector
// would be rejected as not re-extractable, which is the point of the profile.
const claimSelector = "test:TestWidget/case:cfint"

type fixture struct {
	root        string
	base        string
	target      string
	cemRaw      []byte
	ocmRaw      []byte
	patchSHA256 string
	intent      intentSpan
	hunkID      string
}

type intentSpan struct {
	blobOID    string
	start      int
	end        int
	spanSHA256 string
}

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "HOME="+t.TempDir(), "XDG_CONFIG_HOME="+t.TempDir(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func digestHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// newFixture builds base and target commits, runs the real CEM 0.2 two-phase
// workflow to produce the canonical sidecar, and then writes an OCM artifact
// bound to that exact map and patch. `linked` selects the two OCM dispositions
// the frontier projects differently: a linked obligation carries a test claim,
// an unknown one carries a bounded reason and no references.
func newFixture(t *testing.T, linked bool) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "docs/rule.txt", "widget renderer authority\n")
	writeFile(t, root, "docs/intent.md", intentDocument)
	writeFile(t, root, "src/app.txt", "alpha\n")
	writeFile(t, root, "tests/widget_test.go", testDocument)
	writeFile(t, root, "tests/test_python_widget.py", pythonPost39Document)
	writeFile(t, root, "tests/test_python_crlf.py", pythonCRLFDocument)
	// Uppercase extension on purpose: internal/tcq dispatches Python analysis on
	// a case-sensitive ".py", so this path exercises the undispatched branch.
	writeFile(t, root, "tests/test_python_upper.PY", pythonPost39Document)
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "src/app.txt", "alpha\nwidget renderer\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "target")
	target := gitCmd(t, root, "rev-parse", "HEAD")

	patchBytes := canonicalPatch(t, root, base, target)
	cemRaw := buildCEM(t, root, base, target)
	hunkID := firstHunkID(t, cemRaw)
	intent := intentScopeOf(t, root, target)
	ocmRaw := buildOCM(t, root, target, cemRaw, patchBytes, intent, hunkID, linked)

	return fixture{
		root: root, base: base, target: target,
		cemRaw: cemRaw, ocmRaw: ocmRaw,
		patchSHA256: digestHex(patchBytes), intent: intent, hunkID: hunkID,
	}
}

func canonicalPatch(t *testing.T, root, base, target string) []byte {
	t.Helper()
	repository, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	patchBytes, err := repository.CanonicalDiff(context.Background(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	return patchBytes
}

// buildCEM runs the real workflow: prepare derives the candidate map from the
// canonical patch, and one cite records the basis that makes the single hunk
// `supported` — which a linked OCM obligation requires of every hunk it names.
func buildCEM(t *testing.T, root, base, target string) []byte {
	t.Helper()
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Prepare(context.Background(), workflow.PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	session, err = workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Cite(context.Background(), workflow.CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1",
		EvidencePath: "docs/rule.txt", Lines: "1:1", Relation: "specification",
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath)))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func firstHunkID(t *testing.T, cemRaw []byte) string {
	t.Helper()
	var document struct {
		Hunks []struct {
			ID          string `json:"id"`
			Disposition string `json:"disposition"`
		} `json:"hunks"`
	}
	if err := json.Unmarshal(cemRaw, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Hunks) != 1 || document.Hunks[0].Disposition != "supported" {
		t.Fatalf("fixture expects one supported hunk, got %+v", document.Hunks)
	}
	return document.Hunks[0].ID
}

// intentScopeOf derives the exact scope the OCM verifier re-derives: the byte
// range from the single `## Requirements` heading to the next `## ` heading.
func intentScopeOf(t *testing.T, root, target string) intentSpan {
	t.Helper()
	body := []byte(intentDocument)
	start := bytes.Index(body, []byte("## Requirements"))
	end := start + bytes.Index(body[start:], []byte("## Next"))
	return intentSpan{
		blobOID:    gitCmd(t, root, "rev-parse", target+":docs/intent.md"),
		start:      start,
		end:        end,
		spanSHA256: digestHex(body[start:end]),
	}
}

func buildOCM(t *testing.T, root, target string, cemRaw, patchBytes []byte, intent intentSpan, hunkID string, linked bool) []byte {
	t.Helper()
	claims := []any{}
	hunkIDs := []string{}
	claimIDs := []string{}
	disposition, reason := "unknown", "unassessed"
	if linked {
		claim := buildClaim(t, root, target)
		claims = []any{claim}
		hunkIDs = []string{hunkID}
		claimIDs = []string{claim["id"].(string)}
		disposition, reason = "linked", "change-and-test-linked"
	}
	document := map[string]any{
		"spec": "ocm/0.1-experimental", "targetRevision": target,
		"intentScope": map[string]any{
			"path": "docs/intent.md", "blobOid": intent.blobOID,
			"span":       map[string]any{"start": intent.start, "end": intent.end},
			"spanSha256": intent.spanSHA256,
		},
		"cem":    map[string]any{"mapSha256": digestHex(cemRaw), "patchSha256": digestHex(patchBytes)},
		"claims": claims,
		"obligations": []any{map[string]any{
			"id": requirementID, "disposition": disposition, "reason": reason,
			"hunkIds": hunkIDs, "claimIds": claimIDs,
		}},
	}
	return encodeOCM(t, document)
}

// buildClaim mirrors the OCM claim identity rule: the id digests the canonical
// body without the id, so a hand-written claim binds exactly its own fields.
func buildClaim(t *testing.T, root, target string) map[string]any {
	t.Helper()
	body := []byte(testDocument)
	start := bytes.Index(body, []byte(claimAnchor))
	if start < 0 {
		t.Fatal("fixture claim anchor is absent from the test document")
	}
	fields := map[string]any{
		"extractor": "corvint-test-claim/1", "path": "tests/widget_test.go",
		"blobOid":    gitCmd(t, root, "rev-parse", target+":tests/widget_test.go"),
		"selector":   claimSelector,
		"span":       map[string]any{"start": start, "end": start + len(claimAnchor)},
		"spanSha256": digestHex(body[start : start+len(claimAnchor)]),
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	claim := make(map[string]any, len(fields)+1)
	for key, value := range fields {
		claim[key] = value
	}
	claim["id"] = "claim:sha256:" + digestHex(encoded)
	return claim
}

func buildPythonClaim(t *testing.T, root, target, path, body, testName string) map[string]any {
	t.Helper()
	anchor := []byte(`"""` + requirementID + `"""`)
	start := bytes.Index([]byte(body), anchor)
	if start < 0 {
		t.Fatal("fixture claim anchor is absent from the Python document")
	}
	fields := map[string]any{
		"extractor": "corvint-test-claim/1", "path": path,
		"blobOid":    gitCmd(t, root, "rev-parse", target+":"+path),
		"selector":   "test:" + testName + "#doc",
		"span":       map[string]any{"start": start, "end": start + len(anchor)},
		"spanSha256": digestHex(anchor),
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	claim := make(map[string]any, len(fields)+1)
	for key, value := range fields {
		claim[key] = value
	}
	claim["id"] = "claim:sha256:" + digestHex(encoded)
	return claim
}

func addClaims(t *testing.T, raw []byte, added ...map[string]any) []byte {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	claims := document["claims"].([]any)
	claimIDs := document["obligations"].([]any)[0].(map[string]any)["claimIds"].([]any)
	for _, claim := range added {
		claims = append(claims, claim)
		claimIDs = append(claimIDs, claim["id"].(string))
	}
	sort.Slice(claims, func(left, right int) bool {
		return claims[left].(map[string]any)["id"].(string) < claims[right].(map[string]any)["id"].(string)
	})
	sort.Slice(claimIDs, func(left, right int) bool {
		return claimIDs[left].(string) < claimIDs[right].(string)
	})
	document["claims"] = claims
	document["obligations"].([]any)[0].(map[string]any)["claimIds"] = claimIDs
	return encodeOCM(t, document)
}

func encodeOCM(t *testing.T, document map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func computeFrontier(t *testing.T, f fixture) (frontier.Document, []byte, error) {
	t.Helper()
	ctx := context.Background()
	adapter := New(ctx, f.root)
	return frontier.Compute(ctx, frontier.Request{
		CEMBytes:     f.cemRaw,
		OCMBytes:     f.ocmRaw,
		ExpectedBase: f.base,
		Target:       f.target,
		Verifier:     adapter,
		TCQ:          adapter,
	})
}

// TestLinkedObligationProducesCompleteDocument is the end-to-end proof: a real
// repository in, one complete CF-V0-018 document out, checked against the wire
// contract and the CF-V0-004 state law rather than against a golden blob.
func TestLinkedObligationProducesCompleteDocument(t *testing.T) {
	f := newFixture(t, true)
	document, encoded, err := computeFrontier(t, f)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	// CF-V0-018 shape, CF-V0-019 codec and identity, CF-V0-020 orders and
	// CF-V0-023 bounds, all through the implementation's own verifier.
	if verifyErr := frontier.VerifyDocument(encoded); verifyErr != nil {
		t.Fatalf("emitted document is not canonical: %v\n%s", verifyErr, encoded)
	}
	checkStateLaw(t, document)
	if document.FrontierState != frontier.StateOpen {
		t.Fatalf("a linked obligation with no test report leaves work open, got %s", document.FrontierState)
	}

	// CF-V0-018 inputs: each digest binds exactly the bytes the clause names.
	if document.Inputs.CEMSHA256 != digestHex(f.cemRaw) || document.Inputs.OCMSHA256 != digestHex(f.ocmRaw) {
		t.Error("inputs do not digest the verified bounded raw artifact copies")
	}
	if document.Inputs.TestMode != frontier.TestModeStatic {
		t.Errorf("testMode = %q, want STATIC for an absent dynamic tuple", document.Inputs.TestMode)
	}
	if !strings.HasPrefix(document.Inputs.TCQID, "tcq:sha256:") {
		t.Errorf("tcqId = %q is not a recomputed TCQ identity", document.Inputs.TCQID)
	}

	// CF-V0-018 scope: every field is the value the shared verifier resolved,
	// not a value copied out of an artifact.
	checkScope(t, document.Scope, f)

	// CF-V0-006: the universe identity must rebind from the emitted scope alone.
	universeID, idErr := frontier.UniverseID(document.Scope)
	if idErr != nil || universeID != document.UniverseID {
		t.Errorf("universeId does not rebind from the emitted scope: %v", idErr)
	}

	// CF-V0-015: every structurally linked obligation emits one INTENT_TEST item,
	// and a caller-reported TCQ edge never closes it. With no dynamic tuple the
	// claim is unmatched, which CF-V0-016 maps to TEST_NOT_MATCHED.
	testItem := itemOf(t, document, frontier.KindIntentTest, requirementID)
	if testItem.AuthorityClass != frontier.AuthorityCallerReported {
		t.Errorf("INTENT_TEST authorityClass = %q", testItem.AuthorityClass)
	}
	if len(testItem.Reasons) == 0 || testItem.Reasons[0] != frontier.ReasonTestNotMatched {
		t.Errorf("INTENT_TEST reasons = %v, want TEST_NOT_MATCHED first", testItem.Reasons)
	}
	if testItem.NextAction != frontier.ActionSupplyTestObservation {
		t.Errorf("INTENT_TEST nextAction = %q", testItem.NextAction)
	}
	// The related IDs name the selected claim edge, and nothing names a body.
	if len(testItem.RelatedIDs) != 1 || !strings.HasPrefix(testItem.RelatedIDs[0], "claim:sha256:") {
		t.Errorf("INTENT_TEST relatedIds = %v, want the one selected claim", testItem.RelatedIDs)
	}

	// CF-V0-008: the one hunk carries a qualifying lexical basis over the cited
	// evidence, so it is closed and emits no HUNK_BASIS item. That leaves exactly
	// the two intent obligations, which is what makes this a real closure test
	// rather than a document-shape test.
	if len(document.Items) != 2 {
		t.Errorf("items = %d, want the two intent obligations only", len(document.Items))
	}
	for _, item := range document.Items {
		if item.Kind == frontier.KindHunkBasis {
			t.Errorf("a hunk closed by a qualifying basis still emitted an item: %+v", item)
		}
	}

	// CF-V0-012: a lexical OCM candidate is non-closing, so a linked obligation
	// whose only change witness is lexical stays open pending real authority.
	changeItem := itemOf(t, document, frontier.KindIntentChange, requirementID)
	if len(changeItem.Reasons) != 1 || changeItem.Reasons[0] != frontier.ReasonLexicalCandidateNonclosing {
		t.Errorf("INTENT_CHANGE reasons = %v, want LEXICAL_CANDIDATE_NONCLOSING", changeItem.Reasons)
	}
	if changeItem.ResolutionClass != frontier.ResolutionAuthorityRequired ||
		changeItem.NextAction != frontier.ActionEstablishChangeWitness {
		t.Errorf("INTENT_CHANGE disposition = %s/%s", changeItem.ResolutionClass, changeItem.NextAction)
	}
	if len(changeItem.RelatedIDs) != 1 || !strings.HasPrefix(changeItem.RelatedIDs[0], "hunk:sha256:") {
		t.Errorf("INTENT_CHANGE relatedIds = %v, want the one linked hunk", changeItem.RelatedIDs)
	}

	// CF-V0-024: no source, diff, command, or report body reaches the output.
	for _, secret := range []string{"widget renderer authority", "alpha", "@@ ", "func TestWidget"} {
		if bytes.Contains(encoded, []byte(secret)) {
			t.Errorf("body text %q leaked into the frontier document", secret)
		}
	}
}

// TestUnknownObligationEmitsNoTestItem pins CF-V0-011: an OCM-unknown
// obligation is an INTENT_CHANGE item carrying its mapped reason, and
// fabricating an INTENT_TEST item for it would be inventing a test obligation
// the producer never declared.
func TestUnknownObligationEmitsNoTestItem(t *testing.T) {
	f := newFixture(t, false)
	document, encoded, err := computeFrontier(t, f)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if verifyErr := frontier.VerifyDocument(encoded); verifyErr != nil {
		t.Fatalf("emitted document is not canonical: %v", verifyErr)
	}
	checkStateLaw(t, document)
	changeItem := itemOf(t, document, frontier.KindIntentChange, requirementID)
	if len(changeItem.Reasons) != 1 || changeItem.Reasons[0] != frontier.ReasonObligationUnassessed {
		t.Errorf("INTENT_CHANGE reasons = %v, want the mapped unknown reason", changeItem.Reasons)
	}
	if changeItem.AuthorityClass == frontier.AuthorityCallerReported {
		t.Error("only an INTENT_TEST item may be CALLER_REPORTED")
	}
	for _, item := range document.Items {
		if item.Kind == frontier.KindIntentTest {
			t.Errorf("an OCM-unknown obligation produced a fabricated INTENT_TEST item: %+v", item)
		}
	}
}

// TestTCQSeamsRunStandalone drives internal/tcq directly through the two seams
// this package implements, with no memo to consult. It is what proves the
// upstream verifier and the bounded tree reader are real implementations and
// not shapes that only work when Frontier has already done the work.
func TestTCQSeamsRunStandalone(t *testing.T) {
	f := newFixture(t, true)
	adapter := New(context.Background(), f.root)
	repository, err := adapter.tree()
	if err != nil {
		t.Fatalf("tree reader: %v", err)
	}
	result, err := tcq.Evaluate(repository, adapter, tcq.Request{
		CEM: f.cemRaw, OCM: f.ocmRaw, ExpectedBase: f.base, Target: f.target,
	})
	if err != nil {
		t.Fatalf("standalone tcq: %v", err)
	}
	if adapter.shared != nil {
		t.Error("a standalone TCQ run must not populate the shared-verification memo")
	}
	if !strings.HasPrefix(result.ID(), "tcq:sha256:") {
		t.Errorf("tcq id = %q", result.ID())
	}
	claims := result.Claims()
	if len(claims) != 1 {
		t.Fatalf("selected %d edges, want exactly the one linked claim", len(claims))
	}
	if claims[0].ObligationID != requirementID || claims[0].AuthorityClass != "CALLER_REPORTED" {
		t.Errorf("edge = %+v", claims[0])
	}
	// The bounded reader resolves only the target tree, so a path that exists
	// nowhere in it is reported absent rather than searched for elsewhere.
	entry, err := repository.TreeEntry(f.target, "tests/widget_test.go")
	if err != nil || !entry.Found || entry.Type != "blob" {
		t.Errorf("target tree entry = %+v, %v", entry, err)
	}
	missing, err := repository.TreeEntry(f.target, "tests/absent_test.go")
	if err != nil || missing.Found {
		t.Errorf("an absent path must be reported missing, got %+v, %v", missing, err)
	}
}

// TestTCQV0047PythonGrammarAbstainsPerEdge pins DR-0014 at the Frontier-owned
// seam. Python 3.10 match syntax is outside frozen python-ast/1: its selected
// edge abstains with source evidence, while the sibling Go edge still reaches
// ordinary TCQ processing and the whole universe remains usable.
func TestTCQV0047PythonGrammarAbstainsPerEdge(t *testing.T) {
	f := newFixture(t, true)
	pythonClaim := buildPythonClaim(t, f.root, f.target,
		"tests/test_python_widget.py", pythonPost39Document, "test_python_widget")
	f.ocmRaw = addClaims(t, f.ocmRaw, pythonClaim)

	adapter := New(context.Background(), f.root)
	result, err := adapter.Recompute(frontier.TCQRequest{
		CEMBytes: f.cemRaw, OCMBytes: f.ocmRaw,
		BaseRevision: f.base, TargetRevision: f.target,
	})
	if err != nil {
		t.Fatalf("recompute: %v", err)
	}
	if len(result.Claims) != 2 {
		t.Fatalf("selected %d edges, want Python and Go", len(result.Claims))
	}
	unamendedAdapter := New(context.Background(), f.root)
	repository, err := unamendedAdapter.tree()
	if err != nil {
		t.Fatalf("tree reader: %v", err)
	}
	unamended, err := tcq.Evaluate(repository, unamendedAdapter, tcq.Request{
		CEM: f.cemRaw, OCM: f.ocmRaw, ExpectedBase: f.base, Target: f.target,
	})
	if err != nil {
		t.Fatalf("unamended tcq: %v", err)
	}
	if result.ID != unamended.ID() {
		t.Error("Frontier changed the TCQ identity after TCQ retained the Python anchor profile")
	}
	pythonEdge := projectedClaimOf(t, result.Claims, pythonClaim["id"].(string))
	if len(pythonEdge.Reasons) != 1 || pythonEdge.Reasons[0] != "unsupported-python-grammar" {
		t.Errorf("Python reasons = %v, want unsupported-python-grammar", pythonEdge.Reasons)
	}
	goEdge := projectedClaimOf(t, result.Claims, buildClaim(t, f.root, f.target)["id"].(string))
	if len(goEdge.Reasons) != 1 || goEdge.Reasons[0] != "test-not-matched" {
		t.Errorf("Go reasons = %v, want test-not-matched", goEdge.Reasons)
	}
	if len(adapter.pythonClaims) != 1 {
		t.Fatalf("source evidence = %+v, want one Python claim", adapter.pythonClaims)
	}
	evidence := adapter.pythonClaims[0]
	if evidence.ClaimID != pythonEdge.ClaimID || evidence.Path != "tests/test_python_widget.py" ||
		evidence.BlobOID == "" || evidence.AnchorProfile != "python-docstring/1" ||
		evidence.Reason != "unsupported-python-grammar" {
		t.Errorf("source evidence = %+v", evidence)
	}

	document, encoded, err := computeFrontier(t, f)
	if err != nil {
		t.Fatalf("frontier compute: %v", err)
	}
	if verifyErr := frontier.VerifyDocument(encoded); verifyErr != nil {
		t.Fatalf("emitted document is not canonical: %v", verifyErr)
	}
	item := itemOf(t, document, frontier.KindIntentTest, requirementID)
	if !contains(item.Reasons, frontier.ReasonTestClaimUnsupported) ||
		!contains(item.Reasons, frontier.ReasonTestNotMatched) {
		t.Errorf("INTENT_TEST reasons = %v, want unsupported Python and processed Go", item.Reasons)
	}
	if bytes.Contains(encoded, []byte("tests/test_python_widget.py")) {
		t.Error("internal Python source evidence leaked into the Frontier wire")
	}
}

// TestTCQV0047DoesNotUseTheOCMGrammarAsAProfileGate guards the opposite side:
// CRLF is valid frozen Python source and TCQ explicitly supports it. The OCM
// syntax approximation rejects CR bytes, but that must not force either a
// command refusal or a blanket Python abstention.
func TestTCQV0047DoesNotUseTheOCMGrammarAsAProfileGate(t *testing.T) {
	f := newFixture(t, true)
	pythonClaim := buildPythonClaim(t, f.root, f.target,
		"tests/test_python_crlf.py", pythonCRLFDocument, "test_python_crlf")
	f.ocmRaw = addClaims(t, f.ocmRaw, pythonClaim)
	adapter := New(context.Background(), f.root)
	result, err := adapter.Recompute(frontier.TCQRequest{
		CEMBytes: f.cemRaw, OCMBytes: f.ocmRaw,
		BaseRevision: f.base, TargetRevision: f.target,
	})
	if err != nil {
		t.Fatalf("recompute: %v", err)
	}
	pythonEdge := projectedClaimOf(t, result.Claims, pythonClaim["id"].(string))
	if len(pythonEdge.Reasons) != 1 || pythonEdge.Reasons[0] != "test-not-matched" {
		t.Errorf("CRLF Python reasons = %v, want ordinary associated processing", pythonEdge.Reasons)
	}
}

// TestTCQV0047DoesNotClaimAGrammarFailureForAnUndispatchedExtension guards the
// seam between the two extension tests. internal/tcq.analyzeBlob dispatches to
// the Python analyzer on a case-sensitive ".py", so a ".PY" path is never given
// Python treatment and fails for the ordinary reason that it produced no anchor
// candidates. The Python-claim evidence path must agree: reporting
// unsupported-python-grammar here would name a grammar failure that no analyzer
// ever performed, which is the invented certainty AGENTS.md invariant 2 forbids.
func TestTCQV0047DoesNotClaimAGrammarFailureForAnUndispatchedExtension(t *testing.T) {
	f := newFixture(t, true)
	pythonClaim := buildPythonClaim(t, f.root, f.target,
		"tests/test_python_upper.PY", pythonPost39Document, "test_python_widget")
	f.ocmRaw = addClaims(t, f.ocmRaw, pythonClaim)

	adapter := New(context.Background(), f.root)
	result, err := adapter.Recompute(frontier.TCQRequest{
		CEMBytes: f.cemRaw, OCMBytes: f.ocmRaw,
		BaseRevision: f.base, TargetRevision: f.target,
	})
	if err != nil {
		t.Fatalf("recompute: %v", err)
	}
	edge := projectedClaimOf(t, result.Claims, pythonClaim["id"].(string))
	if len(edge.Reasons) != 1 || edge.Reasons[0] != "unsupported-anchor-profile" {
		t.Errorf("reasons = %v, want the undispatched extension's own reason", edge.Reasons)
	}
	if len(adapter.pythonClaims) != 0 {
		t.Errorf("source evidence = %+v, want no Python claim for an undispatched extension", adapter.pythonClaims)
	}
}

func projectedClaimOf(t *testing.T, claims []frontier.TCQClaimResult, claimID string) frontier.TCQClaimResult {
	t.Helper()
	for _, claim := range claims {
		if claim.ClaimID == claimID {
			return claim
		}
	}
	t.Fatalf("claim %s is absent from %+v", claimID, claims)
	return frontier.TCQClaimResult{}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

// TestSharedVerificationRunsExactlyOnce pins CF-V0-021 step 5: one invocation
// makes ONE shared OCM-consuming verification call, and TCQ consumes that call
// rather than opening a second one over the same bytes.
func TestSharedVerificationRunsExactlyOnce(t *testing.T) {
	f := newFixture(t, true)
	adapter := New(context.Background(), f.root)
	if _, _, err := frontier.Compute(context.Background(), frontier.Request{
		CEMBytes: f.cemRaw, OCMBytes: f.ocmRaw, ExpectedBase: f.base, Target: f.target,
		Verifier: adapter, TCQ: adapter,
	}); err != nil {
		t.Fatalf("compute: %v", err)
	}
	shared := adapter.shared
	if shared == nil {
		t.Fatal("the shared verification recorded no outcome")
	}
	// Frontier hands TCQ the RESOLVED revisions, not its own inputs, so the memo
	// must recognise both spellings of the same two commits.
	if !shared.covers(f.cemRaw, f.ocmRaw, shared.resolved.BaseRevision, shared.resolved.TargetRevision) {
		t.Error("the resolved revisions do not name the shared verification")
	}
	if !shared.covers(f.cemRaw, f.ocmRaw, f.base, f.target) {
		t.Error("the requested revisions do not name the shared verification")
	}
	// Different bytes are a different universe and must fall through to a real
	// verification instead of inheriting this outcome.
	if shared.covers(append(append([]byte(nil), f.cemRaw...), ' '), f.ocmRaw, f.base, f.target) {
		t.Error("the memo accepted CEM bytes the shared call never verified")
	}
	if shared.covers(f.cemRaw, f.ocmRaw, f.base, f.base) {
		t.Error("the memo accepted a target the shared call never verified")
	}
}

// TestUpstreamCodesReachTheClosedAllowlist proves the CF-V0-022 translation
// path end to end: a genuine upstream refusal must arrive at Frontier as the
// exact inherited code, not as `frontier-internal-error`. That is exactly what
// the code-normalizing wrapper in errors.go exists for — both producers declare
// their code as a struct field, which the translation cannot see on its own.
func TestUpstreamCodesReachTheClosedAllowlist(t *testing.T) {
	f := newFixture(t, true)
	tests := []struct {
		name string
		edit func(*frontier.Request)
		want string
	}{
		{
			// The independent expected base is not the map's baseRevision.
			name: "expected base mismatch",
			edit: func(request *frontier.Request) { request.ExpectedBase = f.target },
			want: "base-revision-mismatch",
		},
		{
			// The caller target names the base commit. Native CEM precedence
			// decides this one: canonical derivation over base..base yields an
			// empty patch, so the digest check refuses before the OCM's own
			// target binding is ever consulted. CF-V0-021 puts that precedence
			// ahead of every later Frontier check, and this pins it.
			name: "caller target names the base",
			edit: func(request *frontier.Request) { request.Target = f.base },
			want: "patch-digest-mismatch",
		},
		{
			// A revision that does not resolve in this repository at all.
			name: "unresolvable target",
			edit: func(request *frontier.Request) { request.Target = strings.Repeat("0", 40) },
			want: "git-read-failed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter := New(context.Background(), f.root)
			request := frontier.Request{
				CEMBytes: f.cemRaw, OCMBytes: f.ocmRaw, ExpectedBase: f.base, Target: f.target,
				Verifier: adapter, TCQ: adapter,
			}
			test.edit(&request)
			_, encoded, err := frontier.Compute(context.Background(), request)
			if err == nil {
				t.Fatal("expected an operational failure")
			}
			if len(encoded) != 0 {
				t.Error("CF-V0-022 forbids frontier output on operational failure")
			}
			if got := frontier.CodeOf(err); got != test.want {
				t.Errorf("code = %q, want %q", got, test.want)
			}
			// CF-V0-024: the error carries the code and nothing else.
			if err.Error() != test.want {
				t.Errorf("error text %q is not the bare code", err.Error())
			}
		})
	}
}

// TestLegacyCEMProfileFailsOperationally pins CF-V0-001 and TCQ-V0-002 at the
// composition level: `cem/0.1` is refused operationally, never as an abstention
// or an open item.
func TestLegacyCEMProfileFailsOperationally(t *testing.T) {
	f := newFixture(t, true)
	legacy := bytes.Replace(f.cemRaw, []byte(`"cem/0.2"`), []byte(`"cem/0.1"`), 1)
	if bytes.Equal(legacy, f.cemRaw) {
		t.Fatal("fixture CEM does not declare cem/0.2")
	}
	adapter := New(context.Background(), f.root)
	document, encoded, err := frontier.Compute(context.Background(), frontier.Request{
		CEMBytes: legacy, OCMBytes: f.ocmRaw, ExpectedBase: f.base, Target: f.target,
		Verifier: adapter, TCQ: adapter,
	})
	if err == nil {
		t.Fatalf("cem/0.1 produced a frontier result: %+v", document)
	}
	if len(encoded) != 0 {
		t.Error("CF-V0-022 forbids frontier output on operational failure")
	}
	if got := frontier.CodeOf(err); got != frontier.CodeUnsupportedContext {
		t.Errorf("code = %q, want %q", got, frontier.CodeUnsupportedContext)
	}
}

// checkStateLaw is the CF-V0-004 invariant: EMPTY requires exact `items: []`,
// OPEN requires at least one item, and the exit code follows the state.
func checkStateLaw(t *testing.T, document frontier.Document) {
	t.Helper()
	switch document.FrontierState {
	case frontier.StateEmpty:
		if len(document.Items) != 0 || document.ExitCode() != 0 {
			t.Errorf("EMPTY carries %d items and exit %d", len(document.Items), document.ExitCode())
		}
	case frontier.StateOpen:
		if len(document.Items) == 0 || document.ExitCode() != 1 {
			t.Errorf("OPEN carries %d items and exit %d", len(document.Items), document.ExitCode())
		}
	default:
		t.Errorf("frontierState = %q is not an admitted state", document.FrontierState)
	}
}

func checkScope(t *testing.T, scope frontier.Scope, f fixture) {
	t.Helper()
	if scope.BaseRevision != f.base || scope.TargetRevision != f.target {
		t.Errorf("scope revisions = %s..%s, want %s..%s",
			scope.BaseRevision, scope.TargetRevision, f.base, f.target)
	}
	if scope.PatchSHA256 != f.patchSHA256 {
		t.Error("scope patchSha256 is not the canonical derived patch digest")
	}
	if scope.ObjectFormat != "sha1" && scope.ObjectFormat != "sha256" {
		t.Errorf("objectFormat = %q", scope.ObjectFormat)
	}
	if scope.IntentPath != "docs/intent.md" || scope.IntentBlobOID != f.intent.blobOID {
		t.Errorf("intent = %s@%s", scope.IntentPath, scope.IntentBlobOID)
	}
	if scope.IntentSpan.Start != int64(f.intent.start) || scope.IntentSpan.End != int64(f.intent.end) {
		t.Errorf("intent span = %d:%d, want %d:%d",
			scope.IntentSpan.Start, scope.IntentSpan.End, f.intent.start, f.intent.end)
	}
	if scope.IntentSpanSHA256 != f.intent.spanSHA256 {
		t.Error("intent spanSha256 does not digest the verified scope")
	}
}

func itemOf(t *testing.T, document frontier.Document, kind, subject string) frontier.Item {
	t.Helper()
	found := []frontier.Item{}
	for _, item := range document.Items {
		if item.Kind == kind && item.SubjectID == subject {
			found = append(found, item)
		}
	}
	if len(found) != 1 {
		t.Fatalf("found %d %s items for %s, want exactly one", len(found), kind, subject)
	}
	return found[0]
}
