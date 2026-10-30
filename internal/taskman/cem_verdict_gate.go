package taskman

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
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

// The closed refusal codes (TCB-V0-007 to TCB-V0-013, TCB-V0-017).
const (
	CEMVerdictInputInvalid         = "CEM_VERDICT_INPUT_INVALID"
	CEMVerdictMapMissing           = "CEM_VERDICT_MAP_MISSING"
	CEMVerdictMapAmbiguous         = "CEM_VERDICT_MAP_AMBIGUOUS"
	CEMVerdictProducerMismatch     = "CEM_VERDICT_PRODUCER_MISMATCH"
	CEMVerdictUnverifiable         = "CEM_VERDICT_UNVERIFIABLE"
	CEMVerdictPredicateFailed      = "CEM_VERDICT_PREDICATE_FAILED"
	CEMVerdictVerifierDisagreement = "CEM_VERDICT_VERIFIER_DISAGREEMENT"
)

// sealedMapDir holds the sealed maps `make dogfood-seal` names after their
// bind commit (.corvint/changes/<target>.cem.json).
const sealedMapDir = ".corvint/changes/"

// The portable consumer's one route and result profile (TCB-V0-017).
const (
	portableRoute   = "verify-stable-canonical"
	portableProfile = "cem-stable-verification/1"
)

// CEMVerdictInput carries only native facts (TCB-V0-007): Root is the
// absolute, lexically clean checkout that is both the Git repository and the
// artifact root; BaseCommit is the attempt's native base; CandidateCommit is
// the commit whose tree is the attempt candidate; TouchPaths is the claim-era
// ticket's effects.touchPaths. Nothing in it comes from the map.
// PortableConsumer is the absolute path of the independent interop/cem01-go
// executable that must recompute the same verdict (TCB-V0-017); it is an
// operator fact, not evidence, so the document never names it.
type CEMVerdictInput struct {
	Root             string
	BaseCommit       string
	CandidateCommit  string
	TouchPaths       []string
	Predicate        string
	PortableConsumer string
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
	Portable      *cemPortablePin   `json:"portable"`
	Citation      *verify.Citation  `json:"citation"`
	Governed      []cemVerdictHunk  `json:"governed"`
	Paths         []cemVerdictPath  `json:"paths"`
	// Uncertainty is present only when a governed hunk is satisfied on the
	// runner-input basis (CEM-V1-634, CEM-V1-635), so every other document
	// encodes as before.
	Uncertainty *cemVerdictUncertainty `json:"uncertainty,omitempty"`
}

// cemVerdictUncertainty states what the runner-input basis never observed:
// whether the plan selected every test the change needs, and whether the map
// names every run (a run left out whole leaves bytes equal to a map without
// it). Both axes are always NOT_OBSERVED (CEM-V1-634).
type cemVerdictUncertainty struct {
	RunnerSelectionCompleteness string `json:"runnerSelectionCompleteness"`
	SourceInventoryCompleteness string `json:"sourceInventoryCompleteness"`
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

// cemPortablePin is what the independent portable consumer reported for the
// same map bytes: its own verdictSha256 claim and its citation block's digest,
// with the stage its result ended at and its closed result code (TCB-V0-040),
// so an UNSUPPORTED answer names its own reason in the document.
type cemPortablePin struct {
	Route          string  `json:"route"`
	Outcome        string  `json:"outcome"`
	Stage          string  `json:"stage"`
	Code           *string `json:"code"`
	VerdictSha256  string  `json:"verdictSha256"`
	CitationSha256 *string `json:"citationSha256"`
}

// cemVerdictHunk is one governed hunk. Tier is always the recomputed citation
// tier; Anchor names the runner-bound basis that satisfies a hunk below
// anchored (CEM-V1-606, CEM-V1-607) and Reason the closed reason an
// unsatisfied hunk names (CEM-V1-608). Both are omitted when empty, so a hunk
// satisfied by its tier encodes exactly as before.
type cemVerdictHunk struct {
	HunkID    string `json:"hunkId"`
	Path      string `json:"path"`
	Tier      string `json:"tier"`
	Satisfied bool   `json:"satisfied"`
	Anchor    string `json:"anchor,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// cemVerdictPath is one governed path's minimum recomputed tier (TCB-V0-018).
type cemVerdictPath struct {
	Path      string `json:"path"`
	Tier      string `json:"tier"`
	Satisfied bool   `json:"satisfied"`
}

// EvaluateCEMVerdictGate is the sealed cem/1.0 verdict gate (decision 0525
// step 8). It reads the one sealed map the candidate adds from Git objects,
// cross-checks every producer-named field against native facts, recomputes the
// verdict through the in-process stable consumer, requires the independent
// portable consumer to recompute the same verdict, and evaluates one closed
// predicate over the recomputed citation tiers. It writes nothing in the
// repository (the portable consumer reads the map bytes from one private
// temporary file it removes) and never raises a tier; completion authority
// stays with the native store.
func EvaluateCEMVerdictGate(ctx context.Context, in CEMVerdictInput) (CEMVerdictResult, error) {
	g := &verdictGate{doc: cemVerdictDocument{Profile: CEMVerdictGateProfile, Outcome: "REFUSE", SealedMaps: []string{}, Governed: []cemVerdictHunk{}, Paths: []cemVerdictPath{}}}
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
	if in.Predicate == PredicateTouchedAnchored {
		names, err := exactKeyNamesDirectory(git, touch, in.BaseCommit, in.CandidateCommit)
		if err != nil {
			g.refuse(CEMVerdictUnverifiable, "git-read-failed")
			return
		}
		if names {
			g.refuse(CEMVerdictInputInvalid, "touch-key-names-directory")
			return
		}
	}
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
	if !g.crossCheck(ctx, in, target, raw, r, verdict) {
		return
	}
	if parseErr != nil || r.Outcome != "ACCEPT" || r.Sidecar != "EXACT" || r.Citation == nil {
		g.refuse(CEMVerdictUnverifiable, "stable-not-accepted")
		return
	}
	g.doc.Citation = r.Citation
	g.predicate(in.Predicate, touch, document.Change.Hunks, r.Citation, newRunnerBinding(git, in.CandidateCommit, document).anchor)
}

// crossCheck runs the independent portable consumer over the same map bytes
// and admits only its agreement with the native recomputation (TCB-V0-017):
// its outcome, its verdictSha256 claim, the digest recomputed from its own
// result and its canonical citation block must all equal the native ones. A
// consumer that cannot run, emits no readable result or exits in a way its own
// outcome does not explain is unverifiable; one that answers UNSUPPORTED
// against a native verdict is unverifiable under the route's name
// (TCB-V0-041); any other readable difference is a disagreement, never a pass.
func (g *verdictGate) crossCheck(ctx context.Context, in CEMVerdictInput, target string, raw []byte, native verify.StableResult, verdict string) bool {
	stdout, status, err := runPortableConsumer(ctx, in, target, raw)
	if err != nil {
		g.refuse(CEMVerdictUnverifiable, "portable-consumer-failed")
		return false
	}
	reading, ok := readPortable(stdout)
	if !ok || !portableExitAgrees(reading.pin.Outcome, status) {
		g.refuse(CEMVerdictUnverifiable, "portable-consumer-failed")
		return false
	}
	g.doc.Portable = &reading.pin
	// A consumer that could not run its route on this host answers
	// UNSUPPORTED with its own closed code; that is the portable leg missing,
	// not two verdicts differing, so the refusal names the route and keeps
	// the pinned code (TCB-V0-041). Only a result whose verdictSha256 claim is
	// the digest recomputed from its own bytes is that consumer's answer: a
	// claim its result does not recompute to is a disagreement below, so the
	// named refusal never pins a false claim. Both legs answering
	// UNSUPPORTED still go through the comparison and the native refusal.
	if reading.pin.Outcome == "UNSUPPORTED" && native.Outcome != "UNSUPPORTED" && reading.pin.VerdictSha256 == reading.recomputed {
		g.refuse(CEMVerdictUnverifiable, "portable-route-unsupported")
		return false
	}
	citation, err := json.Marshal(native.Citation)
	if err != nil {
		g.refuse(CEMVerdictUnverifiable, "portable-consumer-failed")
		return false
	}
	nativeCitation, err := wire.Parse(citation)
	if err != nil {
		g.refuse(CEMVerdictUnverifiable, "portable-consumer-failed")
		return false
	}
	if reading.pin.Outcome != native.Outcome || reading.pin.VerdictSha256 != verdict || reading.recomputed != verdict || !bytes.Equal(reading.citation, wire.CanonicalValue(nativeCitation)) {
		reason := "verifier-disagreement"
		if verifierBuildsDiffer(verifierBuilds(in.PortableConsumer)) {
			reason = "verifier-build-mismatch"
		}
		g.refuse(CEMVerdictVerifierDisagreement, reason)
		return false
	}
	return true
}

// verifierBuild is the source identity a Go executable records: its VCS
// revision and whether the tree was modified. Revision is empty when the build
// recorded none (go test, a non-Git build or a non-Go wrapper).
type verifierBuild struct {
	Revision string
	Modified bool
}

// verifierBuilds reads the recorded build of this verifier and of the portable
// consumer executable (TCB-V0-045). It runs only after a disagreement and never
// executes the consumer.
var verifierBuilds = func(consumer string) (native, portable verifierBuild) {
	if info, ok := debug.ReadBuildInfo(); ok {
		native = recordedBuild(info)
	}
	if info, err := buildinfo.ReadFile(consumer); err == nil {
		portable = recordedBuild(info)
	}
	return native, portable
}

func recordedBuild(info *debug.BuildInfo) verifierBuild {
	var b verifierBuild
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			b.Revision = s.Value
		case "vcs.modified":
			b.Modified = s.Value == "true"
		}
	}
	return b
}

// verifierBuildsDiffer reports a disagreement that the two legs' builds
// explain (TCB-V0-045): both recorded a revision, and the revisions differ or
// either tree was modified, so the legs are not known to share their source.
// An unrecorded build on either side proves nothing and keeps
// verifier-disagreement.
func verifierBuildsDiffer(native, portable verifierBuild) bool {
	if native.Revision == "" || portable.Revision == "" {
		return false
	}
	return native.Revision != portable.Revision || native.Modified || portable.Modified
}

// runPortableConsumer hands the exact map bytes to the portable consumer
// through one private temporary file and returns its stdout and exit status.
func runPortableConsumer(ctx context.Context, in CEMVerdictInput, target string, raw []byte) ([]byte, int, error) {
	file, err := os.CreateTemp("", "corvint-cem-verdict-*.json")
	if err != nil {
		return nil, 0, err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(raw); err != nil {
		file.Close()
		return nil, 0, err
	}
	if err := file.Close(); err != nil {
		return nil, 0, err
	}
	return runPortable(ctx, in.PortableConsumer, in.Root, []string{portableRoute, "--repository", in.Root, "--map", file.Name(), "--expected-base", in.BaseCommit, "--target", target, "--artifacts", in.Root})
}

// portableStages and portableCodes are the closed `stage` and `code` sets of
// the portable consumer's result contract,
// protocol/cem-1.0/stable/RESULT.schema.json, which the gate pins verbatim
// (TCB-V0-040). TestTCBV0040PortablePinSetsMatchTheResultSchema pins them to
// the schema; a stage or code outside them is not a readable result, so no
// other bytes enter the document.
var portableStages = closedSet("arguments", "input", "wire", "references", "authority-arguments", "runtime", "repository", "binding", "verification", "drift", "artifacts", "complete")

var portableCodes = closedSet(
	"artifact-resource-limit", "artifact-role-closure", "artifact-unavailable", "artifacts-required",
	"duplicate-reference", "expected-base-required", "git-diff-failed", "git-read-failed", "git-timeout",
	"invalid-arguments", "invalid-criterion-identity", "invalid-excluded-path", "invalid-field", "invalid-json",
	"map-unavailable", "missing-field", "reference-coherence", "repository-object-unavailable", "target-required",
	"unknown-field", "unresolved-reference", "unsupported-object-alternates", "unsupported-patch-inventory",
	"unsupported-process-containment", "unsupported-repository-envelope", "unsupported-resource-limit",
	"unsupported-spec", "unsupported-structural-runtime", "unsupported-tree-mode", "verification-timeout",
)

func closedSet(members ...string) map[string]bool {
	set := make(map[string]bool, len(members))
	for _, m := range members {
		set[m] = true
	}
	return set
}

// portableExitAgrees reports whether the consumer's exit status matches its
// outcome: verify-stable-canonical exits 0 exactly on ACCEPT, and 1 or 2 on a
// REJECT or UNSUPPORTED result.
func portableExitAgrees(outcome string, status int) bool {
	if outcome == "ACCEPT" {
		return status == 0
	}
	return status == 1 || status == 2
}

type portableReading struct {
	pin        cemPortablePin
	recomputed string
	citation   []byte
}

// readPortable reads one cem-stable-verification/1 result and recomputes its
// verdict digest: the canonical value of the result minus runtime,
// repositoryEnvelope and verdictSha256 (CEM-V1-066).
func readPortable(stdout []byte) (portableReading, bool) {
	value, err := wire.Parse(stdout)
	if err != nil || value.Kind != wire.KindObject {
		return portableReading{}, false
	}
	member := func(key string, kind wire.Kind) (wire.Value, bool) {
		v, ok := value.Obj.Get(key)
		return v, ok && v.Kind == kind
	}
	profile, okProfile := member("profile", wire.KindString)
	outcome, okOutcome := member("outcome", wire.KindString)
	stage, okStage := member("stage", wire.KindString)
	code, okCode := value.Obj.Get("code")
	claimed, okClaimed := member("verdictSha256", wire.KindString)
	citation, okCitation := value.Obj.Get("citation")
	if !okProfile || !okOutcome || !okStage || !okCode || !okClaimed || !okCitation || profile.Str != portableProfile || !digest(claimed.Str) || citation.Kind != wire.KindObject && citation.Kind != wire.KindNull {
		return portableReading{}, false
	}
	// The stage and code are pinned verbatim, so they must be members of the
	// contract's closed sets: anything else is not a readable result
	// (TCB-V0-040).
	if !portableStages[stage.Str] || code.Kind != wire.KindNull && (code.Kind != wire.KindString || !portableCodes[code.Str]) {
		return portableReading{}, false
	}
	result := &wire.Object{Values: map[string]wire.Value{}}
	for _, key := range value.Obj.Keys {
		if key != "runtime" && key != "repositoryEnvelope" && key != "verdictSha256" {
			result.Keys = append(result.Keys, key)
			result.Values[key] = value.Obj.Values[key]
		}
	}
	reading := portableReading{
		pin:        cemPortablePin{Route: portableRoute, Outcome: outcome.Str, Stage: stage.Str, VerdictSha256: claimed.Str},
		recomputed: sum(wire.CanonicalValue(wire.Value{Kind: wire.KindObject, Obj: result})),
		citation:   wire.CanonicalValue(citation),
	}
	if code.Kind == wire.KindString {
		pinned := code.Str
		reading.pin.Code = &pinned
	}
	if citation.Kind == wire.KindObject {
		citationSha256 := sum(reading.citation)
		reading.pin.CitationSha256 = &citationSha256
	}
	return reading, true
}

// tierRank orders the recomputed tiers by precedence; a tier outside the
// closed set ranks lowest.
func tierRank(tier string) int {
	switch tier {
	case verify.TierHeld:
		return 4
	case verify.TierMechanicalProved:
		return 3
	case verify.TierAnchored:
		return 2
	case verify.TierCited:
		return 1
	}
	return 0
}

// predicate reads tiers only from the recomputed citation block; hunk paths
// come from the map the consumer just admitted against the recomputed patch.
// A hunk below anchored is satisfied only on a runner-bound basis anchor
// returns (CEM-V1-606 to CEM-V1-608); its tier is never raised. Each governed
// path reports its minimum tier and is satisfied only when every one of its
// governed hunks is (TCB-V0-018). A hunk satisfied on runner-input adds the
// document's uncertainty member, whatever the outcome (CEM-V1-634).
func (g *verdictGate) predicate(name string, touch []string, hunks []wire.Hunk, citation *verify.Citation, anchor runnerAnchor) {
	byID := map[string]wire.Hunk{}
	for _, hunk := range hunks {
		byID[hunk.ID] = hunk
	}
	satisfied := true
	minimum := map[string]cemVerdictPath{}
	for _, row := range citation.Hunks {
		hunk := byID[row.HunkID]
		path := hunk.Path
		if name == PredicateTouchedAnchored && !underTouchPath(path, touch) {
			continue
		}
		governed := cemVerdictHunk{HunkID: row.HunkID, Path: path, Tier: row.Tier}
		governed.Satisfied = row.Tier == verify.TierAnchored || row.Tier == verify.TierMechanicalProved || row.Tier == verify.TierHeld
		if !governed.Satisfied {
			governed.Reason = runnerReasonNoBinding
			if anchor != nil {
				bound, reason, err := anchor(hunk, row.Tier)
				if err != nil {
					g.doc.Governed, g.doc.Paths, g.doc.Uncertainty = []cemVerdictHunk{}, []cemVerdictPath{}, nil
					g.refuse(CEMVerdictUnverifiable, "git-read-failed")
					return
				}
				governed.Anchor, governed.Reason, governed.Satisfied = bound, reason, bound != ""
			}
		}
		g.doc.Governed = append(g.doc.Governed, governed)
		satisfied = satisfied && governed.Satisfied
		if governed.Anchor == runnerAnchorInput {
			g.doc.Uncertainty = &cemVerdictUncertainty{RunnerSelectionCompleteness: "NOT_OBSERVED", SourceInventoryCompleteness: "NOT_OBSERVED"}
		}
		seen, found := minimum[path]
		if !found || tierRank(row.Tier) < tierRank(seen.Tier) {
			seen.Path, seen.Tier = path, row.Tier
		}
		seen.Satisfied = governed.Satisfied && (!found || seen.Satisfied)
		minimum[path] = seen
	}
	for _, row := range minimum {
		g.doc.Paths = append(g.doc.Paths, row)
	}
	sort.Slice(g.doc.Paths, func(i, j int) bool { return g.doc.Paths[i].Path < g.doc.Paths[j].Path })
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
	if !filepath.IsAbs(in.PortableConsumer) || filepath.Clean(in.PortableConsumer) != in.PortableConsumer {
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

// exactKeyNamesDirectory reports whether a touch key without a trailing "/"
// names a directory in any of commits (TCB-V0-023). Under the Tasks PATH key
// rule such a key governs that one path and nothing beneath it, so the hunks
// under the directory would go ungoverned without notice. The gate refuses
// rather than widening the key, so a key means the same in Tasks and here.
func exactKeyNamesDirectory(git func(...string) ([]byte, error), touch []string, commits ...string) (bool, error) {
	exact := map[string]bool{}
	for _, key := range touch {
		if !strings.HasSuffix(key, "/") {
			exact[key] = true
		}
	}
	if len(exact) == 0 {
		return false, nil
	}
	for _, commit := range commits {
		out, err := git("ls-tree", "-r", "-d", "-z", "--full-tree", "--end-of-options", commit)
		if err != nil {
			return false, err
		}
		if len(out) == 0 {
			continue
		}
		listing, terminated := strings.CutSuffix(string(out), "\x00")
		if !terminated {
			return false, errors.New("unterminated directory listing")
		}
		for _, entry := range strings.Split(listing, "\x00") {
			meta, path, ok := strings.Cut(entry, "\t")
			if fields := strings.Fields(meta); !ok || path == "" || len(fields) != 3 || fields[1] != "tree" {
				return false, errors.New("unreadable directory listing")
			}
			if exact[path] {
				return true, nil
			}
		}
	}
	return false, nil
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
