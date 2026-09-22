package compactionkernel

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

const (
	testRevision  = "1111111111111111111111111111111111111111"
	agentsBlob    = "2222222222222222222222222222222222222222"
	tsvBlob       = "3333333333333333333333333333333333333333"
	specBlob      = "4444444444444444444444444444444444444444"
	changedBlob   = "5555555555555555555555555555555555555555"
	pinnedSpec    = "docs/specs/compaction-kernel-v0.md"
	worktreeOnly  = "docs/specs/moved-elsewhere-v0.md"
	requirementID = "CKN-V0-001"
)

// pinnedTSV is the requirement index as the pinned blob states it. The
// worktree's own REQUIREMENTS.tsv is deliberately not consulted by the package,
// so this table is the only thing Build may follow.
const pinnedTSV = "id\tfile\tline\ttitle\n" +
	"CKN-V0-001\tdocs/specs/compaction-kernel-v0.md\t42\tKernel MUST pin authority\n" +
	"CKN-V0-002\tdocs/specs/compaction-kernel-v0.md\t48\tKernel MUST stay bounded\n" +
	"CKN-V0-003\tdocs/specs/compaction-kernel-v0.md\t54\tRequirements MUST resolve from pinned blobs\n" +
	"CKN-V0-004\tdocs/specs/compaction-kernel-v0.md\t60\tDigest MUST cover every other member\n" +
	"CKN-V0-009\tdocs/specs/moved-elsewhere-v0.md\t9\tSpec absent from this revision\n"

func fixture() *contextindex.Index {
	sources := map[string]string{
		"AGENTS.md":           agentsBlob,
		requirementsIndexPath: tsvBlob,
		pinnedSpec:            specBlob,
	}
	index := &contextindex.Index{Root: "/fixture", Revision: testRevision, Sources: map[string]contextindex.Source{}}
	for path, blob := range sources {
		body := "# " + path + "\n"
		if path == requirementsIndexPath {
			body = pinnedTSV
		}
		index.Sources[path] = contextindex.Source{Path: path, BlobHash: blob, Data: []byte(body)}
	}
	return index
}

func governing() []Entry {
	return []Entry{{Authority: "project-instructions", BlobHash: agentsBlob, Line: 1, Path: "AGENTS.md", Relation: "governing"}}
}

func build(t *testing.T, requirements ...string) Kernel {
	t.Helper()
	kernel, err := Build(fixture(), governing(), requirements)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return kernel
}

// kernelJSON returns the canonical document line out of a rendered envelope,
// which is the byte-bounded artifact CKN-V0-002 governs.
func kernelJSON(t *testing.T, kernel Kernel) string {
	t.Helper()
	lines := strings.Split(Render(kernel), "\n")
	if len(lines) < 5 || lines[0] != fence || lines[1] != beginMarker || lines[3] != endMarker || lines[4] != fence {
		t.Fatalf("envelope shape = %q", Render(kernel))
	}
	return lines[2]
}

// CKN-V0-001: the kernel pins the governing authority to the indexed blob and
// refuses an authority this index never read.
func TestBuildPinsGoverningAuthority(t *testing.T) {
	kernel := build(t)
	if len(kernel.Authorities) != 1 || kernel.Authorities[0].Path != "AGENTS.md" || kernel.Authorities[0].BlobHash != agentsBlob {
		t.Fatalf("authorities = %+v", kernel.Authorities)
	}
	if kernel.Revision != testRevision {
		t.Fatalf("revision = %q", kernel.Revision)
	}
	untracked := []Entry{{Authority: "project-instructions", Path: "ELSEWHERE.md", Relation: "governing"}}
	if _, err := Build(fixture(), untracked, nil); err == nil {
		t.Fatal("expected an unpinned governing authority to be refused")
	}
	if _, err := Build(fixture(), nil, nil); err == nil {
		t.Fatal("expected an empty governance array to be refused")
	}
}

// CKN-V0-002: the canonical document stays under the byte bound, dropping
// requirement rows from the tail and counting what it dropped.
func TestKernelStaysUnderByteBound(t *testing.T) {
	kernel := build(t, "CKN-V0-001", "CKN-V0-002", "CKN-V0-003", "CKN-V0-004")
	encoded := kernelJSON(t, kernel)
	if len(encoded) > MaxKernelBytes {
		t.Fatalf("kernel document = %d bytes, bound %d", len(encoded), MaxKernelBytes)
	}
	if kernel.Omitted != 4-len(kernel.Requirements) {
		t.Fatalf("omitted = %d with %d retained", kernel.Omitted, len(kernel.Requirements))
	}
	if len(kernel.Requirements) == 0 {
		t.Fatal("the bound must retain at least one requirement beside one authority")
	}
	if bare := kernelJSON(t, build(t)); len(bare) > MaxKernelBytes {
		t.Fatalf("authority-only kernel = %d bytes", len(bare))
	}
}

// CKN-V0-003: requirement ids resolve through the pinned requirement index and
// the pinned spec blob, never the worktree.
func TestRequirementsResolveFromPinnedBlob(t *testing.T) {
	kernel := build(t, requirementID)
	if len(kernel.Requirements) != 1 {
		t.Fatalf("requirements = %+v", kernel.Requirements)
	}
	row := kernel.Requirements[0]
	if row.Path != pinnedSpec || row.Line != 42 || row.BlobHash != specBlob {
		t.Fatalf("requirement row = %+v", row)
	}
	// The pinned table names this spec, but the revision does not carry it, so
	// the kernel abstains rather than citing an unpinned path.
	if _, err := Build(fixture(), governing(), []string{"CKN-V0-009"}); err == nil {
		t.Fatalf("expected %s to be refused as unpinned", worktreeOnly)
	}
	if _, err := Build(fixture(), governing(), []string{"ZZZ-V0-001"}); err == nil {
		t.Fatal("expected an id absent from the pinned index to be refused")
	}
	// A repeated id is one requirement: a duplicate row would spend the byte
	// bound and push a distinct requested requirement into omitted.
	if repeated := build(t, requirementID, requirementID); len(repeated.Requirements) != 1 || repeated.Omitted != 0 {
		t.Fatalf("repeated id pinned %d rows, omitted %d", len(repeated.Requirements), repeated.Omitted)
	}
}

// CKN-V0-004: the digest is deterministic and covers every other member.
func TestDigestIsDeterministicAndCovering(t *testing.T) {
	first, second := build(t, requirementID), build(t, requirementID)
	if first.Digest != second.Digest {
		t.Fatalf("digest %q != %q", first.Digest, second.Digest)
	}
	if len(first.Digest) != 64 {
		t.Fatalf("digest = %q", first.Digest)
	}
	if bare := build(t); bare.Digest == first.Digest {
		t.Fatal("digest must change when the requirement list changes")
	}
	// Each member the payload carries must move the digest on its own, so a
	// recovered kernel cannot be edited in one field and still verify.
	for name, mutate := range map[string]func(*Kernel){
		"revision":    func(kernel *Kernel) { kernel.Revision = strings.Repeat("9", 40) },
		"authorities": func(kernel *Kernel) { kernel.Authorities[0].BlobHash = changedBlob },
		"requirement": func(kernel *Kernel) { kernel.Requirements[0].Line = 43 },
		"omitted":     func(kernel *Kernel) { kernel.Omitted = 7 },
	} {
		edited := build(t, requirementID)
		mutate(&edited)
		recomputed, err := digestOf(payload(edited))
		if err != nil {
			t.Fatalf("digestOf %s: %v", name, err)
		}
		if recomputed == first.Digest {
			t.Fatalf("digest must change when %s changes", name)
		}
	}
	moved := first
	moved.Revision = strings.Repeat("9", 40)
	if kernelJSON(t, moved) == kernelJSON(t, first) {
		t.Fatal("a different revision must serialize differently")
	}
}

// CKN-V0-006: Verify locates the envelope inside arbitrary text and reports the
// four states with the specific mismatches.
func TestVerifyReportsEachState(t *testing.T) {
	index := fixture()
	block, err := InjectionBlock(index, governing())
	if err != nil {
		t.Fatalf("InjectionBlock: %v", err)
	}
	summary := "Earlier we discussed the index.\n\n" + block + "\nThen the conversation continued.\n"

	intact, err := Verify(summary, index)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if intact.State != "intact" || len(intact.Mismatches) != 0 || intact.Revision != testRevision {
		t.Fatalf("intact verdict = %+v", intact)
	}

	moved := fixture()
	moved.Sources["AGENTS.md"] = contextindex.Source{Path: "AGENTS.md", BlobHash: changedBlob, Data: []byte("# changed\n")}
	stale, err := Verify(summary, moved)
	if err != nil {
		t.Fatalf("Verify stale: %v", err)
	}
	if stale.State != "stale" || len(stale.Mismatches) != 1 {
		t.Fatalf("stale verdict = %+v", stale)
	}
	if stale.Mismatches[0].Path != "AGENTS.md" || stale.Mismatches[0].Actual != changedBlob || stale.Mismatches[0].Expected != agentsBlob {
		t.Fatalf("stale mismatch = %+v", stale.Mismatches[0])
	}

	untracked := fixture()
	delete(untracked.Sources, "AGENTS.md")
	gone, err := Verify(summary, untracked)
	if err != nil {
		t.Fatalf("Verify untracked: %v", err)
	}
	if gone.State != "stale" || len(gone.Mismatches) != 1 {
		t.Fatalf("untracked verdict = %+v", gone)
	}
	if gone.Mismatches[0].Reason != "authority-unavailable" || gone.Mismatches[0].Path != "AGENTS.md" {
		t.Fatalf("untracked mismatch = %+v", gone.Mismatches[0])
	}
	if gone.Mismatches[0].Actual != "" || gone.Mismatches[0].Expected != agentsBlob {
		t.Fatalf("untracked mismatch hashes = %+v", gone.Mismatches[0])
	}

	missing, err := Verify("a compaction summary that kept nothing pinned", index)
	if err != nil {
		t.Fatalf("Verify missing: %v", err)
	}
	if missing.State != "missing" {
		t.Fatalf("missing verdict = %+v", missing)
	}

	tampered := strings.Replace(summary, `"omitted":0`, `"omitted":7`, 1)
	if tampered == summary {
		t.Fatal("expected the kernel body to carry an omitted member")
	}
	for name, text := range map[string]string{
		"edited-body":  tampered,
		"unterminated": strings.Replace(summary, endMarker, "", 1),
		"not-json":     strings.Replace(summary, `{"authorities"`, `not json {"authorities"`, 1),
		// Re-spaced and duplicate-member bodies decode to the same members, so
		// the digest alone still matches; only the canonical bytes differ.
		"non-canonical":    strings.Replace(summary, `"omitted":0`, `"omitted": 0`, 1),
		"duplicate-member": strings.Replace(summary, `{"authorities"`, `{"revision":"forged","authorities"`, 1),
		// A self-digested body that pins nothing must never verify as intact.
		"no-authorities": unpinnedKernel(t),
		// Nor may one that names no revision, which Build refuses to pin.
		"no-revision": selfDigested(t, Kernel{Authorities: build(t).Authorities, Requirements: []Requirement{}}),
	} {
		verdict, verifyErr := Verify(text, index)
		if verifyErr != nil {
			t.Fatalf("Verify %s: %v", name, verifyErr)
		}
		if verdict.State != "corrupt" || len(verdict.Mismatches) != 1 {
			t.Fatalf("%s verdict = %+v", name, verdict)
		}
	}
}

// CKN-V0-006: a transcript may repeat the injected block, but envelopes that
// disagree leave no single kernel to verify, whichever comes first.
func TestVerifyRefusesDisagreeingEnvelopes(t *testing.T) {
	index := fixture()
	block, err := InjectionBlock(index, governing())
	if err != nil {
		t.Fatalf("InjectionBlock: %v", err)
	}
	repeated, err := Verify(block+"\nlater turn\n"+block, index)
	if err != nil || repeated.State != "intact" {
		t.Fatalf("repeated verdict = %+v, %v", repeated, err)
	}
	forged := strings.Replace(block, `"omitted":0`, `"omitted":7`, 1)
	disagreeing, err := Verify(block+"\nlater turn\n"+forged, index)
	if err != nil || disagreeing.State != "corrupt" || len(disagreeing.Mismatches) != 1 {
		t.Fatalf("disagreeing verdict = %+v, %v", disagreeing, err)
	}
}

// CKN-V0-006: Build emits `omitted` as a count and each requirement `line` as a
// positive TSV line, so a self-digested body with a negative, fractional or
// zero number is corrupt, and Build never pins a TSV row whose line is not
// positive.
func TestVerifyRefusesNumbersBuildCannotEmit(t *testing.T) {
	index := fixture()
	negative := build(t)
	negative.Omitted = -3
	zeroLine := build(t, requirementID)
	zeroLine.Requirements[0].Line = 0
	fractional := payload(build(t))
	fractional["omitted"] = json.Number("0.5")
	digest, err := digestOf(fractional)
	if err != nil {
		t.Fatalf("digestOf: %v", err)
	}
	fractional["digest"] = digest
	encoded, err := gokernel.CanonicalJSON(fractional)
	if err != nil {
		t.Fatalf("CanonicalJSON: %v", err)
	}
	for name, text := range map[string]string{
		"negative-omitted":   selfDigested(t, negative),
		"zero-line":          selfDigested(t, zeroLine),
		"fractional-omitted": fence + "\n" + beginMarker + "\n" + string(encoded) + "\n" + endMarker + "\n" + fence + "\n",
	} {
		verdict, verifyErr := Verify(text, index)
		if verifyErr != nil || verdict.State != "corrupt" {
			t.Fatalf("%s verdict = %+v, %v", name, verdict, verifyErr)
		}
	}
	unnumbered := fixture()
	unnumbered.Sources[requirementsIndexPath] = contextindex.Source{Path: requirementsIndexPath, BlobHash: tsvBlob,
		Data: []byte(pinnedTSV + "CKN-V0-005\tdocs/specs/compaction-kernel-v0.md\t0\tLine zero\n")}
	if _, err := Build(unnumbered, governing(), []string{"CKN-V0-005"}); err == nil {
		t.Fatal("Build pinned a requirement row whose line is not positive")
	}
}

func unpinnedKernel(t *testing.T) string {
	t.Helper()
	return selfDigested(t, Kernel{Authorities: []Authority{}, Requirements: []Requirement{}, Revision: testRevision})
}

// selfDigested renders a kernel Build never produced with a digest that covers
// it, so only the well-formedness check can refuse it.
func selfDigested(t *testing.T, kernel Kernel) string {
	t.Helper()
	digest, err := digestOf(payload(kernel))
	if err != nil {
		t.Fatalf("digestOf: %v", err)
	}
	kernel.Digest = digest
	return Render(kernel)
}

// CKN-V0-007: verification reads text only; a kernel carries no source content,
// so nothing in the envelope can leak a governed file's body.
func TestKernelCarriesNoSourceContent(t *testing.T) {
	encoded := kernelJSON(t, build(t, requirementID))
	if strings.Contains(encoded, "# AGENTS.md") || strings.Contains(encoded, "Kernel MUST pin authority") {
		t.Fatalf("kernel leaked source content: %s", encoded)
	}
}
