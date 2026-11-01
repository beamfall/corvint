package changee2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/jsonend"
)

// OracleAcceptanceSchema is the recorded, independent acceptance of an outcome the change edited
// (CCE-V0-051).
const OracleAcceptanceSchema = "e2e-oracle-acceptance/0"

// maxOracleAcceptanceBytes bounds the acceptance document.
const maxOracleAcceptanceBytes = 1 << 20

// oracleAcceptanceDoc is the closed e2e-oracle-acceptance/0 document. It lives outside the
// repository, so the change under test cannot author it.
type oracleAcceptanceDoc struct {
	Schema      string `json:"schema"`
	Base        string `json:"base"`
	Head        string `json:"head"`
	Acceptances []struct {
		AssertionID  string `json:"assertion_id"`
		OracleDigest string `json:"oracle_digest"`
		AcceptedBy   string `json:"accepted_by"`
		Reason       string `json:"reason"`
	} `json:"acceptances"`
}

// oracleAcceptance is one acceptance as Prove applies it.
type oracleAcceptance struct {
	DocumentDigest string `json:"document_digest"`
	AcceptedBy     string `json:"accepted_by"`
	Reason         string `json:"reason"`
	// Status is `accepted`, or why the acceptance does not lift the cap: `oracle-digest-mismatch`
	// (the oracle changed after acceptance) or `acceptor-is-head-author`.
	Status string `json:"status"`
}

// oracleAcceptances maps an assertion ID to its acceptance.
type oracleAcceptances struct {
	digest      string
	headAuthors map[string]bool
	byAssertion map[string]struct{ digest, by, reason string }
}

// readOracleAcceptance reads the acceptance document at path (CCE-V0-051). It refuses
// `e2e-oracle-acceptance-invalid` for a path inside the repository, an unreadable or oversized
// file, any schema error, a duplicate or empty entry, or base and head other than the proof's.
func readOracleAcceptance(ctx context.Context, root, base, head, path string) (*oracleAcceptances, error) {
	if path == "" {
		return nil, nil
	}
	if _, err := CheckOut(root, path); err != nil {
		return nil, refuse("e2e-oracle-acceptance-invalid", "%s: the acceptance must be recorded outside the repository", path)
	}
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, refuse("e2e-oracle-acceptance-invalid", "%v", err)
	}
	defer f.Close()
	var buf bytes.Buffer
	if n, err := buf.ReadFrom(io.LimitReader(f, maxOracleAcceptanceBytes+1)); err != nil || n > maxOracleAcceptanceBytes {
		return nil, refuse("e2e-oracle-acceptance-invalid", "%s: unreadable or over %d bytes", path, maxOracleAcceptanceBytes)
	}
	data := buf.Bytes()
	var doc oracleAcceptanceDoc
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil || !jsonend.Done(dec) {
		return nil, refuse("e2e-oracle-acceptance-invalid", "%s: %v", path, err)
	}
	if doc.Schema != OracleAcceptanceSchema || doc.Base != base || doc.Head != head {
		return nil, refuse("e2e-oracle-acceptance-invalid", "%s: schema %q, base and head must be %s and %s", path, doc.Schema, base, head)
	}
	a := &oracleAcceptances{digest: "sha256:" + sha256Hex(data), headAuthors: map[string]bool{},
		byAssertion: map[string]struct{ digest, by, reason string }{}}
	for _, e := range doc.Acceptances {
		if !nonEmptyText(e.AssertionID) || !strings.HasPrefix(e.OracleDigest, "sha256:") || !nonEmptyText(e.AcceptedBy) || !nonEmptyText(e.Reason) {
			return nil, refuse("e2e-oracle-acceptance-invalid", "%s: incomplete acceptance %q", path, e.AssertionID)
		}
		if _, dup := a.byAssertion[e.AssertionID]; dup {
			return nil, refuse("e2e-oracle-acceptance-invalid", "%s: duplicate acceptance %q", path, e.AssertionID)
		}
		a.byAssertion[e.AssertionID] = struct{ digest, by, reason string }{e.OracleDigest, strings.TrimSpace(e.AcceptedBy), e.Reason}
	}
	out, err := git(ctx, root, "log", "-1", "--format=%an%x00%ae%x00%cn%x00%ce", head)
	if err != nil {
		return nil, refuse("e2e-revision-invalid", "read the head author: %v", err)
	}
	for _, id := range strings.Split(strings.TrimSpace(string(out)), "\x00") {
		if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
			a.headAuthors[id] = true
		}
	}
	return a, nil
}

// oracleDigest is the digest an acceptance names: the scenario's own test text at head, which
// holds the edited outcome (CCE-V0-051).
func oracleDigest(sc Scenario) string {
	return "sha256:" + sha256Hex(sc.Text)
}

// lookup returns the acceptance of a CHANGED_IN_DIFF scenario, or nil when none names it.
func (a *oracleAcceptances) lookup(sc Scenario) *oracleAcceptance {
	if a == nil || sc.OracleOrigin != OriginChangedInDiff {
		return nil
	}
	e, ok := a.byAssertion[sc.AssertionID]
	if !ok {
		return nil
	}
	r := &oracleAcceptance{DocumentDigest: a.digest, AcceptedBy: e.by, Reason: e.reason, Status: "accepted"}
	switch {
	case e.digest != oracleDigest(sc):
		r.Status = "oracle-digest-mismatch"
	case a.headAuthors[strings.ToLower(e.by)]:
		r.Status = "acceptor-is-head-author"
	}
	return r
}

// LocatorRepair is a proposed replacement for an outcome locator that did not resolve at head
// (CCE-V0-052). It is evidence for a reviewer and is never applied: the verdict and the
// scenario are unchanged.
type LocatorRepair struct {
	Locator    string           `json:"locator"`
	Proposed   string           `json:"proposed,omitempty"`
	Unproposed string           `json:"unproposed,omitempty"` // `no-rename-in-diff` or `repair-ambiguous`
	Applied    bool             `json:"applied"`              // always false
	Evidence   []repairEvidence `json:"evidence"`
}

type repairEvidence struct {
	Path      string `json:"path"`
	BaseLine  int    `json:"base_line"`
	HeadLine  int    `json:"head_line"`
	Candidate string `json:"candidate"`
}

var (
	repairHunk = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)
	testIDAttr = regexp.MustCompile(`data-testid=(?:"([^"\\]{1,128})"|'([^'\\]{1,128})')`)
)

// headFailsUnresolved is true when every head repeat failed at the outcome assertion because its
// locator did not resolve.
func headFailsUnresolved(head sideProof) bool {
	if len(head.Outcomes) == 0 {
		return false
	}
	for _, o := range head.Outcomes {
		if !o.AtOutcomeAssertion || o.FailureKind != FailureLocatorUnresolved {
			return false
		}
	}
	return true
}

// proposeLocatorRepair reads the base-to-head diff for a line pair that renames the
// `data-testid` the outcome locator names and is otherwise identical (CCE-V0-052). One distinct
// candidate is proposed with each line pair as evidence; several are `repair-ambiguous`; none is
// `no-rename-in-diff`.
func proposeLocatorRepair(ctx context.Context, root, base, head, locator string) *LocatorRepair {
	r := &LocatorRepair{Locator: locator, Evidence: []repairEvidence{}}
	out, err := git(ctx, root, "diff", "-U0", "--no-color", "--no-ext-diff", "--no-renames", "--text", base, head)
	if err != nil {
		r.Unproposed = "diff-unavailable"
		return r
	}
	var file string
	var removed, added []string
	var baseStart, headStart int
	flush := func() {
		for i := 0; i < len(removed) && i < len(added); i++ {
			if c, ok := renamedTestID(removed[i], added[i], locator); ok {
				r.Evidence = append(r.Evidence, repairEvidence{Path: file, BaseLine: baseStart + i, HeadLine: headStart + i, Candidate: c})
			}
		}
		removed, added = nil, nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "+++ "):
			flush()
			file = strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/")
		case strings.HasPrefix(line, "@@"):
			flush()
			if m := repairHunk.FindStringSubmatch(line); m != nil {
				baseStart, _ = strconv.Atoi(m[1])
				headStart, _ = strconv.Atoi(m[2])
			}
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "--- "):
			removed = append(removed, line[1:])
		case strings.HasPrefix(line, "+"):
			added = append(added, line[1:])
		}
	}
	flush()
	candidates := map[string]bool{}
	for _, e := range r.Evidence {
		candidates[e.Candidate] = true
	}
	switch len(candidates) {
	case 0:
		r.Unproposed = "no-rename-in-diff"
	case 1:
		r.Proposed = r.Evidence[0].Candidate
	default:
		r.Unproposed = "repair-ambiguous"
	}
	sort.SliceStable(r.Evidence, func(i, j int) bool {
		return r.Evidence[i].Path < r.Evidence[j].Path || (r.Evidence[i].Path == r.Evidence[j].Path && r.Evidence[i].BaseLine < r.Evidence[j].BaseLine)
	})
	return r
}

// renamedTestID reports the new test ID when before names locator as its data-testid and after
// is the same line with only that value changed.
func renamedTestID(before, after, locator string) (string, bool) {
	m := testIDAttr.FindStringSubmatchIndex(before)
	if m == nil || attrValue(before, m) != locator {
		return "", false
	}
	n := testIDAttr.FindStringSubmatchIndex(after)
	if n == nil {
		return "", false
	}
	candidate := attrValue(after, n)
	if candidate == locator || before[:m[0]] != after[:n[0]] || before[m[1]:] != after[n[1]:] {
		return "", false
	}
	return candidate, true
}

func attrValue(s string, m []int) string {
	if m[2] >= 0 {
		return s[m[2]:m[3]]
	}
	return s[m[4]:m[5]]
}
