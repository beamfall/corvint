package cemcandidate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/verify"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
	tw "github.com/Beamfall/corvint/internal/tasks/wire"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

const StableAssemblyProfile = "cem-stable-assembly/1"

// StableRequest is distinct from the experimental candidate request. The snapshot
// artifact is an independently pinned file; its bytes are not a native identity.
type StableRequest struct {
	Profile              string  `json:"profile"`
	Repository           string  `json:"repository"`
	ExpectedBase         string  `json:"expectedBase"`
	Target               string  `json:"target"`
	TicketID             string  `json:"ticketId"`
	AttemptID            string  `json:"attemptId"`
	SourcePrefix         string  `json:"sourcePrefix"`
	SourceMap            FileRef `json:"sourceMap"`
	Capture              FileRef `json:"capture"`
	Verification         FileRef `json:"verification"`
	SnapshotHeadArtifact FileRef `json:"snapshotHeadArtifact"`
	RunnerPlan           FileRef `json:"runnerPlan"`
	RunnerReceipt        FileRef `json:"runnerReceipt"`
	Links                []Link  `json:"links"`
}

type StableAssemblyResult struct {
	Profile                           string              `json:"profile"`
	StablePath                        string              `json:"stablePath"`
	StableSha256                      string              `json:"stableSha256"`
	Verification                      verify.StableResult `json:"verification"`
	TargetRevision                    string              `json:"targetRevision"`
	DeclaredInputGitBinding           string              `json:"declaredInputGitBinding"`
	InputInventorySha256              string              `json:"inputInventorySha256"`
	NativeCaptureSemanticVerification string              `json:"nativeCaptureSemanticVerification"`
	ReceiptAuthority                  string              `json:"receiptAuthority"`
	SourceInventoryCompleteness       string              `json:"sourceInventoryCompleteness"`
	ExecutionAtCommit                 string              `json:"executionAtCommit"`
}

func (r StableRequest) inputs() []FileRef {
	return []FileRef{r.SourceMap, r.Capture, r.Verification, r.SnapshotHeadArtifact, r.RunnerPlan, r.RunnerReceipt}
}
func (r StableRequest) validate() error {
	if r.Profile != StableAssemblyProfile || !filepath.IsAbs(r.Repository) || !cw.IsGitOid(r.ExpectedBase) || !cw.IsGitOid(r.Target) || len(r.ExpectedBase) != len(r.Target) || r.TicketID == "" || r.AttemptID == "" || len(r.Links) == 0 || len(r.Links) > 256 {
		return fmt.Errorf("incomplete stable assembly request")
	}
	if r.SourcePrefix != "" && !literal(r.SourcePrefix) {
		return fmt.Errorf("invalid source prefix")
	}
	for _, pin := range r.inputs() {
		if !filepath.IsAbs(pin.Path) || filepath.Clean(pin.Path) != pin.Path || !cw.IsSha256(pin.Sha256) {
			return fmt.Errorf("absolute pinned input required")
		}
	}
	for _, l := range r.Links {
		if l.CriterionIndex < 0 || l.CriterionIndex >= 256 || len(l.HunkIDs) == 0 || len(l.HunkIDs) > 32 || len(l.EvidenceIDs) == 0 || len(l.EvidenceIDs) > 32 {
			return fmt.Errorf("invalid explicit criterion link")
		}
	}
	return nil
}

// AssembleStable constructs a new typed stable document from a canonically
// verified source. Native semantic reads belong to the independently pinned
// operator harness; decoding retained artifacts here establishes coherence only.
func AssembleStable(ctx context.Context, r StableRequest, out string) (StableAssemblyResult, error) {
	opts := verify.StableOptions{Repository: r.Repository, ExpectedBase: r.ExpectedBase, Target: r.Target, ArtifactRoot: out}
	result := StableAssemblyResult{Profile: "cem-stable-assembly-result/1", TargetRevision: r.Target, Verification: verify.NewStableResult(opts), DeclaredInputGitBinding: "NOT_OBSERVED", NativeCaptureSemanticVerification: "NOT_OBSERVED", ReceiptAuthority: "NOT_OBSERVED", SourceInventoryCompleteness: "NOT_OBSERVED", ExecutionAtCommit: "NOT_OBSERVED"}
	if e := r.validate(); e != nil {
		return result, e
	}
	if e := legacyStableRepositoryEnvelope(r.Repository); e != nil {
		result.Verification.Refuse("repository", "unsupported-repository-envelope", true)
		return result, e
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	raws := make([][]byte, 0, 6)
	for _, p := range r.inputs() {
		b, e := readPinned(ctx, p)
		if e != nil {
			return result, e
		}
		raws = append(raws, b)
	}
	source, e := cw.ParseMap(raws[0])
	if e != nil {
		return result, e
	}
	if source.Spec != cw.Spec02 && source.Spec != cw.Spec03 {
		return result, fmt.Errorf("stable assembly requires canonical cem/0.2 or cem/0.3 source")
	}
	repo, e := gitauth.Open(r.Repository, gitrun.NewDefaultBudget())
	if e != nil {
		if cemcode.CodeOf(e) == cemcode.UnsupportedRepositoryAttributes {
			result.Verification.Refuse("repository", "unsupported-repository-envelope", true)
		}
		return result, e
	}
	if _, exists, e := repo.LookupTreeEntry(ctx, r.Target, cw.ExcludedCEMPath); e != nil {
		return result, e
	} else if exists {
		return result, fmt.Errorf("assembly target must have no committed CEM sidecar")
	}
	if _, _, e = verify.Canonical(ctx, repo, source, verify.CanonicalOptions{ExpectedBase: r.ExpectedBase, Target: r.Target, RawMapBytes: raws[0]}); e != nil {
		return result, e
	}
	// These existing private checks use only the independently supplied common
	// identity fields; no stable document is converted to a candidate document.
	common := Request{Repository: r.Repository, ExpectedBase: r.ExpectedBase, Target: r.Target, TicketID: r.TicketID, AttemptID: r.AttemptID, SourcePrefix: r.SourcePrefix}
	capture, binding, e := captureBinding(ctx, repo, common, raws[1], raws[2])
	if e != nil {
		return result, e
	}
	plan, e := runnerBinding(raws[4], raws[5])
	if e != nil {
		return result, e
	}
	inventory, e := sourceBinding(ctx, repo, common, plan)
	if e != nil {
		return result, e
	}
	artifacts := map[string][]byte{"artifacts/tasks-capture.json": raws[1], "artifacts/tasks-verification.json": raws[2], "artifacts/tasks-claimed-ticket.json": []byte(capture.ClaimedTicket), "artifacts/tasks-snapshot-head-receipt.json": raws[3], "artifacts/runner-plan.json": raws[4], "artifacts/runner-receipt.json": raws[5]}
	stable, e := stableBytes(r, source, binding, artifacts)
	if e != nil {
		return result, e
	}
	for _, p := range r.inputs() {
		if _, e := readPinned(ctx, p); e != nil {
			return result, e
		}
	}
	root, e := newBundle(r.Repository, out)
	if e != nil {
		return result, e
	}
	defer root.Close()
	if e := root.Mkdir("artifacts", 0700); e != nil {
		return result, e
	}
	for _, kind := range stableArtifactKinds {
		name := "artifacts/" + kind + ".json"
		if e := writeNew(root, name, artifacts[name]); e != nil {
			return result, e
		}
	}
	checked, status := verify.Stable(ctx, stable, opts)
	result.Verification = checked
	if status != 0 {
		return result, fmt.Errorf("stable verification refused at %s", checked.Stage)
	}
	for _, p := range r.inputs() {
		if _, e := readPinned(ctx, p); e != nil {
			return result, e
		}
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	if e := writeNew(root, "stable.json", stable); e != nil {
		return result, e
	}
	result.StablePath = filepath.Join(out, "stable.json")
	result.StableSha256 = tr.Digest(stable)
	result.DeclaredInputGitBinding = "DECLARED_INPUT_BYTES_MATCH_TARGET"
	result.InputInventorySha256 = inventory
	return result, nil
}

var stableArtifactKinds = []string{"tasks-capture", "tasks-verification", "tasks-claimed-ticket", "tasks-snapshot-head-receipt", "runner-plan", "runner-receipt"}

func legacyStableRepositoryEnvelope(root string) error {
	config, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		return nil
	}
	if bytes.HasPrefix(config, []byte("[extensions]\n\tobjectformat = sha256\n")) {
		return cemcode.New(cemcode.UnsupportedRepositoryAttributes, "legacy stable assembly does not admit leading object-format extension section")
	}
	return nil
}

func stableBytes(r StableRequest, source *cw.Map, b tw.CriterionBinding, artifacts map[string][]byte) ([]byte, error) {
	if source == nil || source.Spec != cw.Spec02 && source.Spec != cw.Spec03 {
		return nil, fmt.Errorf("canonical source required")
	}
	d := &cw.StableMap{Change: cw.StableChange{Spec: cw.StableSpec, BaseRevision: source.BaseRevision, PatchSha256: source.PatchSha256, ExcludedPath: source.ExcludedPath, Evidence: source.Evidence, Hunks: source.Hunks}, Criteria: []cw.StableCriterion{}, Receipts: []cw.StableReceipt{}, Links: []cw.StableLink{}, Artifacts: []cw.StableArtifact{}}
	digest := func(kind string) string { return tr.Digest(artifacts["artifacts/"+kind+".json"]) }
	hashes := make([]string, len(b.AcceptanceCriteria))
	for i, c := range b.AcceptanceCriteria {
		hashes[i] = tr.Digest([]byte(c))
	}
	acceptance, _ := json.Marshal(hashes)
	for _, l := range r.Links {
		if l.CriterionIndex < 0 || l.CriterionIndex >= len(hashes) {
			return nil, fmt.Errorf("criterion index exceeds captured criteria")
		}
		c := cw.StableCriterion{TicketID: b.TicketID.Raw, AcceptanceRevision: string(b.AcceptanceRevision), CriterionIndex: int64(l.CriterionIndex), AcceptanceSha256: tr.Digest(acceptance), CriterionSha256: hashes[l.CriterionIndex], CaptureSha256: digest("tasks-capture"), VerificationSha256: digest("tasks-verification"), ClaimTicketSha256: digest("tasks-claimed-ticket"), SnapshotHeadReceiptSha256: string(b.Snapshot.HeadReceiptSHA256), SnapshotHeadArtifactSha256: digest("tasks-snapshot-head-receipt")}
		raw, e := json.Marshal(c)
		if e != nil {
			return nil, e
		}
		var fields map[string]json.RawMessage
		if e = json.Unmarshal(raw, &fields); e != nil {
			return nil, e
		}
		delete(fields, "id")
		raw, e = json.Marshal(fields)
		if e != nil {
			return nil, e
		}
		value, e := cw.Parse(raw)
		if e != nil {
			return nil, e
		}
		c.ID = cw.CandidateCriterionPrefix + tr.Digest(cw.CanonicalValue(value))
		d.Criteria = append(d.Criteria, c)
		d.Links = append(d.Links, cw.StableLink{CriterionID: c.ID, HunkIDs: append([]string(nil), l.HunkIDs...), EvidenceIDs: append([]string(nil), l.EvidenceIDs...), RunnerReceiptSha256s: []string{digest("runner-receipt")}})
	}
	d.Receipts = append(d.Receipts, cw.StableReceipt{Sha256: digest("runner-receipt"), Profile: "corvint-test-runner-receipt/0", PlanSha256: digest("runner-plan"), SourceGitBinding: "NOT_OBSERVED", ExecutionAuthority: "CALLER_OBSERVED", DependencyClosure: "NOT_OBSERVED", Authentication: "NOT_OBSERVED"})
	for _, kind := range stableArtifactKinds {
		d.Artifacts = append(d.Artifacts, cw.StableArtifact{Kind: kind, Path: "artifacts/" + kind + ".json", Sha256: digest(kind)})
	}
	return cw.EncodeStable(d)
}
