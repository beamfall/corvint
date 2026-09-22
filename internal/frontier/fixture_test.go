package frontier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/lrf"
)

// The fixtures below are authored from docs/specs/change-frontier-v0.md. They
// drive the real LRF evaluator through the real closure table; only the shared
// CEM/OCM verifier and the TCQ recomputer are doubles, because both need a
// repository and internal/tcq does not exist yet.

func hexOf(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

func oidOf(seed string) string { return hexOf(seed)[:40] }

func hunkRef(name string) string     { return wire.HunkPrefix + hexOf("hunk/"+name) }
func evidenceRef(name string) string { return wire.EvidencePrefix + hexOf("evidence/"+name) }
func claimRef(name string) string    { return "claim:sha256:" + hexOf("claim/"+name) }
func tcqRef(seed string) string      { return "tcq:sha256:" + hexOf("tcq/"+seed) }

// manyTerms builds a body that overflows the LRF 256-term ceiling, which is
// how a fixture reaches the `subject-term-bound-exceeded` abstention.
func manyTerms(count int) string {
	var out strings.Builder
	for index := 0; index < count; index++ {
		// Distinct all-letter words: LRF splits an identifier at every
		// letter/digit boundary, so a numeric suffix would collapse to one term.
		out.WriteByte('q')
		out.WriteByte(byte('a' + index/676%26))
		out.WriteByte(byte('a' + index/26%26))
		out.WriteByte(byte('a' + index%26))
		out.WriteByte('z')
		out.WriteByte(' ')
	}
	return out.String()
}

type evidenceSpec struct {
	name string
	path string
	body string
}

type hunkSpec struct {
	name        string
	disposition string
	reason      string
	body        string
	newPath     string // empty means a deletion: LRF sees NewPath == nil
	oldPath     string
	bases       []string // evidence names, all under the `specification` relation
}

type obligationSpec struct {
	id          string
	disposition string
	reason      string
	statement   string
	hunks       []string
	claims      []string
	claimPaths  []string
}

type scenario struct {
	evidence    []evidenceSpec
	hunks       []hunkSpec
	obligations []obligationSpec
	claims      map[string][]TCQClaimResult
	intentPath  string
	intentSpan  Span
	patchSeed   string
	tcqSeed     string
}

func (s scenario) universe(t *testing.T) VerifiedUniverse {
	t.Helper()
	intentPath := s.intentPath
	if intentPath == "" {
		intentPath = "docs/specs/change-frontier-v0.md"
	}
	span := s.intentSpan
	if span.End == 0 {
		span = Span{Start: 0, End: 512}
	}
	patchSeed := s.patchSeed
	if patchSeed == "" {
		patchSeed = "patch"
	}

	cemMap := &wire.Map{
		Spec:         wire.Spec02,
		BaseRevision: oidOf("base"),
		PatchSha256:  hexOf(patchSeed),
		ExcludedPath: ExcludedPath,
	}
	request := lrf.Request{Context: lrf.Context{
		CEMSpec:          wire.Spec02,
		CEMMapSHA256:     hexOf("cem-map"),
		PatchSource:      "canonical-derived",
		BaseRevision:     oidOf("base"),
		TargetRevision:   pointer(oidOf("target")),
		PatchSHA256:      hexOf(patchSeed),
		ExcludedPath:     pointer(ExcludedPath),
		OCMSpec:          pointer(lrf.OCMSpec),
		OCMMapSHA256:     pointer(hexOf("ocm-map")),
		IntentPath:       pointer(intentPath),
		IntentBlobOID:    pointer(oidOf("intent-blob")),
		IntentStart:      pointer(span.Start),
		IntentEnd:        pointer(span.End),
		IntentSpanSHA256: pointer(hexOf("intent-span")),
	}}

	for _, item := range s.evidence {
		cemMap.Evidence = append(cemMap.Evidence, wire.Evidence{
			ID: evidenceRef(item.name), BlobOid: oidOf(item.name), Path: item.path,
			Span: wire.Span{Start: 0, End: int64(len(item.body))}, SpanSha256: hexOf(item.body),
		})
		request.Evidence = append(request.Evidence, lrf.Evidence{
			ID: evidenceRef(item.name), Path: item.path, Span: []byte(item.body),
		})
	}

	for ordinal, item := range s.hunks {
		bases := make([]wire.Basis, 0, len(item.bases))
		for _, name := range item.bases {
			bases = append(bases, wire.Basis{EvidenceID: evidenceRef(name), Relation: "specification"})
		}
		path := item.newPath
		if path == "" {
			path = item.oldPath
		}
		cemMap.Hunks = append(cemMap.Hunks, wire.Hunk{
			ID: hunkRef(item.name), Path: path, Disposition: item.disposition,
			Reason: item.reason, Basis: bases,
		})
		projected := lrf.Hunk{
			ID: hunkRef(item.name), Added: []byte(item.body),
			Disposition: item.disposition, Basis: bases, Ordinal: ordinal,
		}
		if item.newPath != "" {
			projected.NewPath = pointer(item.newPath)
		}
		if item.oldPath != "" {
			projected.OldPath = pointer(item.oldPath)
		}
		request.Hunks = append(request.Hunks, projected)
	}

	obligations := make([]Obligation, 0, len(s.obligations))
	linkedOrdinal := 0
	for _, item := range s.obligations {
		obligations = append(obligations, Obligation{
			ID: item.id, Disposition: item.disposition, Reason: item.reason,
			HunkIDs: refs(item.hunks, hunkRef), ClaimIDs: refs(item.claims, claimRef),
		})
		if item.disposition != OCMLinked {
			continue
		}
		request.Obligations = append(request.Obligations, lrf.Obligation{
			ID:         item.id,
			Statement:  []byte("- `" + item.id + "`: " + item.statement),
			HunkIDs:    refs(item.hunks, hunkRef),
			ClaimPaths: item.claimPaths,
			Ordinal:    linkedOrdinal,
		})
		linkedOrdinal++
	}

	return VerifiedUniverse{
		CEM:            cemMap,
		Obligations:    obligations,
		BaseRevision:   oidOf("base"),
		TargetRevision: oidOf("target"),
		ObjectFormat:   "sha1",
		PatchSHA256:    hexOf(patchSeed),
		Intent: IntentScope{
			BlobOID: oidOf("intent-blob"), Path: intentPath,
			Span: span, SpanSHA256: hexOf("intent-span"),
		},
		LRFRequest: request,
	}
}

func refs(names []string, encode func(string) string) []string {
	if names == nil {
		return nil
	}
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, encode(name))
	}
	return out
}

func pointer[T any](value T) *T { return &value }

// stubVerifier stands in for the shared OCM-consuming CEM 0.2 verifier.
type stubVerifier struct {
	universe VerifiedUniverse
	err      error
}

func (s stubVerifier) Verify(context.Context, VerifyRequest) (VerifiedUniverse, error) {
	return s.universe, s.err
}

// stubTCQ stands in for internal/tcq. CF-V0-002 forbids accepting a
// caller-supplied TCQ document, so Frontier holds a recomputer seam and this
// double sits behind it.
type stubTCQ struct {
	result  TCQResult
	err     error
	request *TCQRequest
}

func (s *stubTCQ) Recompute(request TCQRequest) (TCQResult, error) {
	s.request = &request
	return s.result, s.err
}

// codedError is an upstream producer error carrying an explicit public code,
// which is the only thing CF-V0-022 is allowed to match on.
type codedError struct{ value string }

func (e codedError) Error() string { return "upstream failure" }
func (e codedError) Code() string  { return e.value }

const validCEMBytes = `{"spec":"cem/0.2"}`
const validOCMBytes = `{"cem":{"spec":"cem/0.2"},"spec":"ocm/0.1-experimental"}`

func (s scenario) run(t *testing.T) (Document, []byte) {
	t.Helper()
	document, encoded, err := s.compute(t)
	if err != nil {
		t.Fatalf("compute returned %q, want a valid result", CodeOf(err))
	}
	return document, encoded
}

func (s scenario) compute(t *testing.T) (Document, []byte, error) {
	t.Helper()
	claims := []TCQClaimResult{}
	for _, obligation := range s.obligations {
		for _, edge := range s.claims[obligation.id] {
			claims = append(claims, edge)
		}
	}
	seed := s.tcqSeed
	if seed == "" {
		seed = "static"
	}
	return Compute(context.Background(), Request{
		CEMBytes:     []byte(validCEMBytes),
		OCMBytes:     []byte(validOCMBytes),
		ExpectedBase: oidOf("base"),
		Target:       oidOf("target"),
		Verifier:     stubVerifier{universe: s.universe(t)},
		TCQ:          &stubTCQ{result: TCQResult{ID: tcqRef(seed), Claims: claims}},
	})
}

// callerReported builds one selected claim edge. Every TCQ V0 edge is
// CALLER_REPORTED (CF-V0-015), so the fixture never varies that field except
// in the test that proves an upgrade is refused.
func callerReported(obligation, claim string, reasons ...string) TCQClaimResult {
	return TCQClaimResult{
		ObligationID: obligation, ClaimID: claimRef(claim),
		Reasons: reasons, AuthorityClass: AuthorityCallerReported,
	}
}

func itemsOfKind(document Document, kind string) []Item {
	out := []Item{}
	for _, item := range document.Items {
		if item.Kind == kind {
			out = append(out, item)
		}
	}
	return out
}

func findItem(t *testing.T, document Document, kind, subject string) Item {
	t.Helper()
	for _, item := range document.Items {
		if item.Kind == kind && item.SubjectID == subject {
			return item
		}
	}
	t.Fatalf("no %s item for subject %s", kind, subject)
	return Item{}
}

func requireNoItem(t *testing.T, document Document, kind, subject string) {
	t.Helper()
	for _, item := range document.Items {
		if item.Kind == kind && item.SubjectID == subject {
			t.Fatalf("unexpected %s item for subject %s", kind, subject)
		}
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
