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
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// The fixture builds a real temporary Git repository: real commits, a real
// cem/1.0 producer map from the real CEM workflow (CEM-V1-214), a real OCM
// artifact bound to it, and a real intent scope read out of a committed
// Markdown blob. stable_test.go converts it into the committed stable map.

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
	command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
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

// newFixture builds base and target commits, runs the real CEM two-phase
// workflow to produce the cem/1.0 producer map, and then writes an OCM artifact
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

func encodeOCM(t *testing.T, document map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}
