package taskman

import (
	"bytes"
	"context"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CEMVerdictGateProfile names the one canonical evidence document the sealed
// cem/1.0 verdict gate emits (TCB-V0-012).
const CEMVerdictGateProfile = "taskman-cem-verdict-gate/0"

// The closed predicate vocabulary (TCB-V0-011).
const (
	PredicateTouchedAnchored = "touched-anchored"
	PredicateAllAnchored     = "all-anchored"
)

// The closed refusal codes (TCB-V0-007 to TCB-V0-013).
const (
	CEMVerdictInputInvalid     = "CEM_VERDICT_INPUT_INVALID"
	CEMVerdictMapMissing       = "CEM_VERDICT_MAP_MISSING"
	CEMVerdictMapAmbiguous     = "CEM_VERDICT_MAP_AMBIGUOUS"
	CEMVerdictProducerMismatch = "CEM_VERDICT_PRODUCER_MISMATCH"
	CEMVerdictUnverifiable     = "CEM_VERDICT_UNVERIFIABLE"
	CEMVerdictPredicateFailed  = "CEM_VERDICT_PREDICATE_FAILED"
)

// sealedMapDir holds the sealed maps `make dogfood-seal` names after their
// bind commit (.corvint/changes/<target>.cem.json).
const sealedMapDir = ".corvint/changes/"

// CEMVerdictInput carries only native facts (TCB-V0-007): Root is the
// absolute, lexically clean checkout that is both the Git repository and the
// artifact root; BaseCommit is the attempt's native base; CandidateCommit is
// the commit whose tree is the attempt candidate; TouchPaths is the claim-era
// ticket's effects.touchPaths. Nothing in it comes from the map.
type CEMVerdictInput struct {
	Root            string
	BaseCommit      string
	CandidateCommit string
	TouchPaths      []string
	Predicate       string
}

// CEMVerdictResult is one evaluation. Document is the canonical gate evidence
// the caller pins; Code is empty only when Granted.
type CEMVerdictResult struct {
	Granted  bool
	Code     string
	Document []byte
}

type cemVerdictDocument struct {
	Profile       string            `json:"profile"`
	Outcome       string            `json:"outcome"`
	Code          *string           `json:"code"`
	Reason        *string           `json:"reason"`
	Inputs        *cemVerdictInputs `json:"inputs"`
	CandidateTree *string           `json:"candidateTree"`
	SealedMaps    []string          `json:"sealedMaps"`
	Map           *cemVerdictMap    `json:"map"`
	Verdict       *cemVerdictPin    `json:"verdict"`
	Citation      *verify.Citation  `json:"citation"`
	Governed      []cemVerdictHunk  `json:"governed"`
}

type cemVerdictInputs struct {
	BaseCommit      string   `json:"baseCommit"`
	CandidateCommit string   `json:"candidateCommit"`
	Predicate       string   `json:"predicate"`
	TouchPaths      []string `json:"touchPaths"`
}

type cemVerdictMap struct {
	Path      string  `json:"path"`
	Target    string  `json:"target"`
	Blob      *string `json:"blob"`
	MapSha256 *string `json:"mapSha256"`
}

// cemVerdictPin is the consumer's recomputed verdict, never the producer's.
type cemVerdictPin struct {
	Outcome       string   `json:"outcome"`
	Stage         string   `json:"stage"`
	Code          *string  `json:"code"`
	IssueCodes    []string `json:"issueCodes"`
	Sidecar       string   `json:"sidecar"`
	VerdictSha256 string   `json:"verdictSha256"`
}

type cemVerdictHunk struct {
	HunkID    string `json:"hunkId"`
	Path      string `json:"path"`
	Tier      string `json:"tier"`
	Satisfied bool   `json:"satisfied"`
}

// EvaluateCEMVerdictGate is the sealed cem/1.0 verdict gate (decision 0525
// step 8). It reads the one sealed map the candidate adds from Git objects,
// cross-checks every producer-named field against native facts, recomputes the
// verdict through the in-process stable consumer, and evaluates one closed
// predicate over the recomputed citation tiers. It writes nothing and never
// raises a tier; completion authority stays with the native store.
func EvaluateCEMVerdictGate(ctx context.Context, in CEMVerdictInput) (CEMVerdictResult, error) {
	g := &verdictGate{doc: cemVerdictDocument{Profile: CEMVerdictGateProfile, Outcome: "REFUSE", SealedMaps: []string{}, Governed: []cemVerdictHunk{}}}
	g.evaluate(ctx, in)
	raw, err := encode(g.doc)
	if err != nil {
		return CEMVerdictResult{}, err
	}
	result := CEMVerdictResult{Granted: g.doc.Outcome == "GRANT", Document: raw}
	if g.doc.Code != nil {
		result.Code = *g.doc.Code
	}
	return result, nil
}

// ReverifyCEMVerdictGate is any consumer's check of pinned gate evidence
// (TCB-V0-013): it recomputes the gate from in, which a consumer with no Tasks
// store reads from the pinned document's own inputs, and admits the pinned
// bytes only when they are exactly that recomputation. Document is always the
// recomputation; any difference refuses as a producer mismatch.
func ReverifyCEMVerdictGate(ctx context.Context, in CEMVerdictInput, pinned []byte) (CEMVerdictResult, error) {
	result, err := EvaluateCEMVerdictGate(ctx, in)
	if err != nil {
		return result, err
	}
	if !bytes.Equal(result.Document, pinned) {
		result.Granted, result.Code = false, CEMVerdictProducerMismatch
	}
	return result, nil
}

type verdictGate struct{ doc cemVerdictDocument }

func (g *verdictGate) refuse(code, reason string) {
	g.doc.Outcome = "REFUSE"
	g.doc.Code = &code
	if reason != "" {
		g.doc.Reason = &reason
	}
}

func (g *verdictGate) evaluate(ctx context.Context, in CEMVerdictInput) {
	touch, ok := admitVerdictInput(in)
	if !ok {
		g.refuse(CEMVerdictInputInvalid, "invalid-input")
		return
	}
	g.doc.Inputs = &cemVerdictInputs{BaseCommit: in.BaseCommit, CandidateCommit: in.CandidateCommit, Predicate: in.Predicate, TouchPaths: touch}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		g.refuse(CEMVerdictUnverifiable, "unsupported-platform")
		return
	}
	git := func(args ...string) ([]byte, error) {
		return runRead(ctx, "git", in.Root, append([]string{"--no-optional-locks", "-c", "core.quotePath=false"}, args...))
	}
	// The probe separates an unreadable repository from an absent commit:
	// after it succeeds, a quiet --verify fails only for a missing object.
	if _, err := git("rev-parse", "--git-dir"); err != nil {
		g.refuse(CEMVerdictUnverifiable, "git-read-failed")
		return
	}
	resolves := func(oid, kind string) bool {
		out, err := git("rev-parse", "--verify", "--quiet", "--end-of-options", oid+"^{"+kind+"}")
		return err == nil && strings.TrimSuffix(string(out), "\n") == oid
	}
	if !resolves(in.BaseCommit, "commit") || !resolves(in.CandidateCommit, "commit") {
		g.refuse(CEMVerdictInputInvalid, "commit-unavailable")
		return
	}
	tree, err := git("rev-parse", "--verify", "--end-of-options", in.CandidateCommit+"^{tree}")
	if err != nil {
		g.refuse(CEMVerdictUnverifiable, "git-read-failed")
		return
	}
	candidateTree := strings.TrimSuffix(string(tree), "\n")
	g.doc.CandidateTree = &candidateTree
	added, err := git("diff-tree", "-r", "-z", "--no-renames", "--no-commit-id", "--diff-filter=A", "--name-only", in.BaseCommit, in.CandidateCommit, "--", sealedMapDir)
	if err != nil {
		g.refuse(CEMVerdictUnverifiable, "git-read-failed")
		return
	}
	for _, path := range strings.Split(string(added), "\x00") {
		if target := sealedTarget(path); wire.IsGitOid(target) && len(target) == len(in.BaseCommit) {
			g.doc.SealedMaps = append(g.doc.SealedMaps, path)
		}
	}
	sort.Strings(g.doc.SealedMaps)
	switch len(g.doc.SealedMaps) {
	case 0:
		g.refuse(CEMVerdictMapMissing, "")
		return
	case 1:
	default:
		g.refuse(CEMVerdictMapAmbiguous, "")
		return
	}
	sealed := g.doc.SealedMaps[0]
	target := sealedTarget(sealed)
	g.doc.Map = &cemVerdictMap{Path: sealed, Target: target}
	if !resolves(target, "commit") {
		g.refuse(CEMVerdictProducerMismatch, "sealed-target-unavailable")
		return
	}
	// The candidate must be the named target with only its sidecar moved, byte
	// for byte, to the sealed name: the verdict covers nothing else.
	delta, err := git("diff-tree", "-r", "-z", "--no-renames", "--no-commit-id", target, in.CandidateCommit)
	if err != nil {
		g.refuse(CEMVerdictUnverifiable, "git-read-failed")
		return
	}
	blob, ok := sealedRename(string(delta), sealed)
	if !ok {
		g.refuse(CEMVerdictProducerMismatch, "sealed-tree-mismatch")
		return
	}
	g.doc.Map.Blob = &blob
	raw, err := git("cat-file", "blob", blob)
	if err != nil {
		g.refuse(CEMVerdictUnverifiable, "git-read-failed")
		return
	}
	mapSha256 := sum(raw)
	g.doc.Map.MapSha256 = &mapSha256
	document, parseErr := wire.ParseStable(raw)
	if parseErr == nil && document.Change.BaseRevision != in.BaseCommit {
		g.refuse(CEMVerdictProducerMismatch, "sealed-base-mismatch")
		return
	}
	r, _ := verify.Stable(ctx, raw, verify.StableOptions{Repository: in.Root, ExpectedBase: in.BaseCommit, Target: target, ArtifactRoot: in.Root})
	verdict, err := r.VerdictDigest()
	if err != nil {
		g.refuse(CEMVerdictUnverifiable, "stable-not-accepted")
		return
	}
	issues := append([]string{}, r.IssueCodes...)
	g.doc.Verdict = &cemVerdictPin{Outcome: r.Outcome, Stage: r.Stage, Code: r.Code, IssueCodes: issues, Sidecar: r.Sidecar, VerdictSha256: verdict}
	if parseErr != nil || r.Outcome != "ACCEPT" || r.Sidecar != "EXACT" || r.Citation == nil {
		g.refuse(CEMVerdictUnverifiable, "stable-not-accepted")
		return
	}
	g.doc.Citation = r.Citation
	g.predicate(in.Predicate, touch, document.Change.Hunks, r.Citation)
}

// predicate reads tiers only from the recomputed citation block; hunk paths
// come from the map the consumer just admitted against the recomputed patch.
func (g *verdictGate) predicate(name string, touch []string, hunks []wire.Hunk, citation *verify.Citation) {
	paths := map[string]string{}
	for _, hunk := range hunks {
		paths[hunk.ID] = hunk.Path
	}
	satisfied := true
	for _, row := range citation.Hunks {
		path := paths[row.HunkID]
		if name == PredicateTouchedAnchored && !underTouchPath(path, touch) {
			continue
		}
		ok := row.Tier == verify.TierAnchored || row.Tier == verify.TierMechanicalProved || row.Tier == verify.TierHeld
		g.doc.Governed = append(g.doc.Governed, cemVerdictHunk{HunkID: row.HunkID, Path: path, Tier: row.Tier, Satisfied: ok})
		satisfied = satisfied && ok
	}
	switch {
	case len(g.doc.Governed) == 0:
		g.refuse(CEMVerdictPredicateFailed, "no-governed-hunk")
	case !satisfied:
		g.refuse(CEMVerdictPredicateFailed, "tier-below-anchored")
	default:
		g.doc.Outcome = "GRANT"
	}
}

// admitVerdictInput returns the sorted, deduplicated touch paths of a
// well-formed input.
func admitVerdictInput(in CEMVerdictInput) ([]string, bool) {
	if !filepath.IsAbs(in.Root) || filepath.Clean(in.Root) != in.Root {
		return nil, false
	}
	if !wire.IsGitOid(in.BaseCommit) || !wire.IsGitOid(in.CandidateCommit) || len(in.BaseCommit) != len(in.CandidateCommit) {
		return nil, false
	}
	if in.Predicate != PredicateTouchedAnchored && in.Predicate != PredicateAllAnchored || len(in.TouchPaths) > 4096 {
		return nil, false
	}
	touch := append([]string{}, in.TouchPaths...)
	sort.Strings(touch)
	unique := []string{}
	for _, path := range touch {
		if !validPath(path) {
			return nil, false
		}
		if len(unique) == 0 || path != unique[len(unique)-1] {
			unique = append(unique, path)
		}
	}
	if in.Predicate == PredicateTouchedAnchored && len(unique) == 0 {
		return nil, false
	}
	return unique, true
}

// underTouchPath applies the Tasks PATH key rule: a key ending in "/" governs
// every path below it, any other key governs exactly itself.
func underTouchPath(path string, touch []string) bool {
	for _, key := range touch {
		if path == key || strings.HasSuffix(key, "/") && strings.HasPrefix(path, key) {
			return true
		}
	}
	return false
}

func sealedTarget(path string) string {
	name, ok := strings.CutPrefix(path, sealedMapDir)
	if !ok {
		return ""
	}
	target, ok := strings.CutSuffix(name, ".cem.json")
	if !ok {
		return ""
	}
	return target
}

// sealedRename admits exactly the seal delta: the regular-file sidecar deleted
// and the same blob added as a regular file at the sealed name.
func sealedRename(delta, sealed string) (string, bool) {
	fields := strings.Split(strings.TrimSuffix(delta, "\x00"), "\x00")
	if len(fields) != 4 {
		return "", false
	}
	var removed, added string
	for i := 0; i < len(fields); i += 2 {
		meta, ok := strings.CutPrefix(fields[i], ":")
		parts := strings.Fields(meta)
		if !ok || len(parts) != 5 {
			return "", false
		}
		switch {
		case fields[i+1] == wire.ExcludedCEMPath && parts[4] == "D" && parts[0] == "100644":
			removed = parts[2]
		case fields[i+1] == sealed && parts[4] == "A" && parts[1] == "100644":
			added = parts[3]
		}
	}
	return added, added != "" && added == removed
}
