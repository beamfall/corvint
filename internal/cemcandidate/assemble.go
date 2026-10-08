package cemcandidate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/verify"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
	tw "github.com/Beamfall/corvint/internal/tasks/wire"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

type Result struct {
	Profile                           string                 `json:"profile"`
	CandidatePath                     string                 `json:"candidatePath"`
	CandidateSha256                   string                 `json:"candidateSha256"`
	Verification                      verify.CandidateResult `json:"verification"`
	DeclaredInputGitBinding           string                 `json:"declaredInputGitBinding"`
	TargetRevision                    string                 `json:"targetRevision"`
	InputInventorySha256              string                 `json:"inputInventorySha256"`
	NativeCaptureSemanticVerification string                 `json:"nativeCaptureSemanticVerification"`
	ExecutionAtCommit                 string                 `json:"executionAtCommit"`
}

// Assemble writes a new candidate bundle only. Runtime records are decoded for
// reference coherence, never accepted as native authority or execution attestation.
func Assemble(ctx context.Context, r Request, out string) (Result, error) {
	result := Result{Profile: "cem-candidate-assembly-result/0", TargetRevision: r.Target, NativeCaptureSemanticVerification: "NOT_OBSERVED", ExecutionAtCommit: "NOT_OBSERVED", DeclaredInputGitBinding: "NOT_OBSERVED"}
	if e := r.validate(); e != nil {
		return result, e
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	raws := make([][]byte, 0, 5)
	for _, p := range r.inputs() {
		b, e := readPinned(ctx, p)
		if e != nil {
			return result, e
		}
		raws = append(raws, b)
	}
	repo, e := gitauth.Open(r.Repository, gitrun.NewDefaultBudget())
	if e != nil {
		return result, e
	}
	source, e := cw.ParseMap(raws[0])
	if e != nil {
		return result, e
	}
	if source.Spec != cw.Spec02 {
		return result, fmt.Errorf("assembly requires exact cem/0.2; no witness downgrade")
	}
	if _, exists, e := repo.LookupTreeEntry(ctx, r.Target, cw.ExcludedCEMPath); e != nil {
		return result, e
	} else if exists {
		return result, fmt.Errorf("assembly target must have no committed CEM sidecar")
	}
	if _, _, e := verify.Canonical(ctx, repo, source, verify.CanonicalOptions{ExpectedBase: r.ExpectedBase, Target: r.Target, RawMapBytes: raws[0]}); e != nil {
		return result, e
	}
	capture, binding, e := captureBinding(ctx, repo, r, raws[1], raws[2])
	if e != nil {
		return result, e
	}
	plan, e := runnerBinding(raws[3], raws[4])
	if e != nil {
		return result, e
	}
	inventory, e := sourceBinding(ctx, repo, r, plan)
	if e != nil {
		return result, e
	}
	artifacts := map[string][]byte{
		"artifacts/tasks-capture.json": raws[1], "artifacts/tasks-verification.json": raws[2], "artifacts/tasks-claimed-ticket.json": []byte(capture.ClaimedTicket), "artifacts/runner-plan.json": raws[3], "artifacts/runner-receipt.json": raws[4],
	}
	candidate, e := candidateBytes(r, raws[0], binding, artifacts)
	if e != nil {
		return result, e
	}
	if _, e := cw.ParseCandidate(candidate); e != nil {
		return result, e
	}
	// Re-read the original pinned bytes before allocating any output.
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
	for name, raw := range artifacts {
		if e := writeNew(root, name, raw); e != nil {
			return result, e
		}
	}
	checked, e := verify.ExperimentalCandidate(ctx, repo, candidate, verify.CandidateOptions{ExpectedBase: r.ExpectedBase, Target: r.Target, ArtifactRoot: out})
	if e != nil {
		return result, e
	}
	for _, p := range r.inputs() {
		if _, e := readPinned(ctx, p); e != nil {
			return result, e
		}
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	if e := writeNew(root, "candidate.json", candidate); e != nil {
		return result, e
	}
	result.CandidatePath = filepath.Join(out, "candidate.json")
	result.CandidateSha256 = tr.Digest(candidate)
	result.Verification = checked
	result.DeclaredInputGitBinding = "DECLARED_INPUT_BYTES_MATCH_TARGET"
	result.InputInventorySha256 = inventory
	return result, nil
}
func captureBinding(ctx context.Context, repo *gitauth.Repository, r Request, captureRaw, verificationRaw []byte) (tw.CriterionCapture, tw.CriterionBinding, error) {
	c, e := tw.DecodeCriterionCapture(captureRaw)
	if e != nil {
		return c, tw.CriterionBinding{}, e
	}
	value, e := tw.Parse(verificationRaw)
	if e != nil {
		return c, tw.CriterionBinding{}, e
	}
	// The native CLI returns a command envelope; callers may alternatively
	// retain its exact verification item. Both use the complete native wire reader.
	if value.Obj != nil {
		if profile, ok := value.Obj.Get("profile"); ok && profile.Str == "taskman-command-result/0" {
			outer, err := tw.DecodeResult(verificationRaw)
			if err != nil {
				return c, tw.CriterionBinding{}, err
			}
			if outer.Outcome != tw.OutcomeOK || len(outer.Command) != 2 || outer.Command[0] != "criterion-binding" || outer.Command[1] != "verify" || len(outer.Items) != 1 || len(outer.Codes) != 0 || outer.Mutation != nil || outer.Page != nil || outer.Snapshot != nil {
				return c, tw.CriterionBinding{}, fmt.Errorf("historical verification envelope required")
			}
			value = outer.Items[0]
		}
	}
	v, e := tw.ReadCriterionVerification(value)
	if e != nil {
		return c, tw.CriterionBinding{}, e
	}
	b := v.Binding
	if c.Producer != v.Producer || string(v.CaptureSHA256) != tr.Digest(captureRaw) || c.ClaimedTicket == "" || b.TicketID.Raw != r.TicketID || b.AttemptID != r.AttemptID || b.BaseCommit != r.ExpectedBase {
		return c, b, fmt.Errorf("declared native capture binding mismatch")
	}
	if b.CandidateTreeOID != nil {
		tree, e := repo.CommitTree(ctx, r.Target)
		if e != nil {
			return c, b, e
		}
		if tree != *b.CandidateTreeOID {
			return c, b, fmt.Errorf("declared candidate tree differs from target")
		}
	}
	// The capture's policy/ticket/queue/attempt strings remain opaque. Only the
	// native Tasks verifier can establish their semantic/historical consistency.
	return c, b, nil
}
func runnerBinding(planRaw, receiptRaw []byte) (tr.PlanDocument, error) {
	var p tr.PlanDocument
	var r tr.ReceiptDocument
	if e := Decode(planRaw, &p); e != nil {
		return p, e
	}
	// A runner-killed phase records exitCode -1 (TRE-V0-040); the receipt is
	// still closed-field checked and may never claim a complete observation.
	structural, e := tr.StructuralBytes(receiptRaw)
	if e != nil {
		return p, e
	}
	if e = decode(structural, receiptRaw, &r); e != nil {
		return p, e
	}
	if e = tr.CheckReceipt(r); e != nil {
		return p, e
	}
	if p.Profile != "corvint-test-runner-plan/0" || r.Profile != "corvint-test-runner-receipt/0" || r.Execution.Profile != "corvint-test-runner-execution/0" || p.Request.Runner == "" || r.Execution.Runner != p.Request.Runner || r.Observation.Runner != p.Request.Runner || r.PlanSha256 != tr.Identity(p) || r.Execution.InputSha256 != tr.Identity(p.Request.InputFiles) || r.Execution.InvocationSha256 != tr.Identity(p.Invocation) || r.Execution.ExecutionAuthority != "CALLER_OBSERVED" || r.Execution.DependencyClosure != "NOT_OBSERVED" {
		return p, fmt.Errorf("runner reference binding mismatch")
	}
	return p, nil
}
func candidateBytes(r Request, sourceRaw []byte, b tw.CriterionBinding, artifacts map[string][]byte) ([]byte, error) {
	var root map[string]json.RawMessage
	if e := json.Unmarshal(sourceRaw, &root); e != nil {
		return nil, e
	}
	digest := func(name string) string { return tr.Digest(artifacts["artifacts/"+name+".json"]) }
	hashes := make([]string, len(b.AcceptanceCriteria))
	for i, c := range b.AcceptanceCriteria {
		hashes[i] = tr.Digest([]byte(c))
	}
	acceptance, _ := json.Marshal(hashes)
	criteria := []map[string]any{}
	links := []map[string]any{}
	for _, l := range r.Links {
		if l.CriterionIndex >= len(hashes) {
			return nil, fmt.Errorf("criterion index exceeds captured criteria")
		}
		c := map[string]any{"ticketId": b.TicketID.Raw, "acceptanceRevision": string(b.AcceptanceRevision), "acceptanceSha256": tr.Digest(acceptance), "criterionIndex": l.CriterionIndex, "criterionSha256": hashes[l.CriterionIndex], "captureSha256": digest("tasks-capture"), "verificationSha256": digest("tasks-verification"), "claimTicketSha256": digest("tasks-claimed-ticket"), "snapshotHeadReceiptSha256": string(b.Snapshot.HeadReceiptSHA256)}
		// All identity strings here are validated ASCII ticket/revision/digest fields;
		// encoding/json's sorted keys therefore match the public canonical algorithm.
		canonical, e := json.Marshal(c)
		if e != nil {
			return nil, e
		}
		id := cw.CandidateCriterionPrefix + tr.Digest(canonical)
		c["id"] = id
		criteria = append(criteria, c)
		links = append(links, map[string]any{"criterionId": id, "hunkIds": l.HunkIDs, "evidenceIds": l.EvidenceIDs, "runnerReceiptSha256s": []string{digest("runner-receipt")}})
	}
	records := []map[string]string{}
	for _, kind := range []string{"tasks-capture", "tasks-verification", "tasks-claimed-ticket", "runner-plan", "runner-receipt"} {
		records = append(records, map[string]string{"kind": kind, "path": "artifacts/" + kind + ".json", "sha256": digest(kind)})
	}
	fields := map[string]any{"spec": cw.CandidateSpec, "criterionBindings": criteria, "criterionLinks": links, "artifacts": records, "runnerReceipts": []map[string]string{{"sha256": digest("runner-receipt"), "profile": "corvint-test-runner-receipt/0", "planSha256": digest("runner-plan"), "sourceGitBinding": "NOT_OBSERVED", "executionAuthority": "CALLER_OBSERVED", "dependencyClosure": "NOT_OBSERVED", "authentication": "NOT_OBSERVED"}}}
	for key, value := range fields {
		raw, e := json.Marshal(value)
		if e != nil {
			return nil, e
		}
		root[key] = raw
	}
	raw, e := json.MarshalIndent(root, "", "  ")
	if e != nil {
		return nil, e
	}
	raw = append(raw, '\n')
	if len(raw) > MaxDocument {
		return nil, fmt.Errorf("candidate exceeds document bound")
	}
	return raw, nil
}
func newBundle(repository, out string) (*os.Root, error) {
	if !filepath.IsAbs(out) || filepath.Clean(out) != out || !literal(strings.TrimPrefix(filepath.ToSlash(out), "/")) {
		return nil, fmt.Errorf("new absolute bundle directory required")
	}
	repo, e := filepath.EvalSymlinks(repository)
	if e != nil {
		return nil, e
	}
	parent, e := filepath.EvalSymlinks(filepath.Dir(out))
	if e != nil {
		return nil, e
	}
	if parent != filepath.Dir(out) {
		return nil, fmt.Errorf("bundle parent must have no symlink aliases")
	}
	relative, e := filepath.Rel(repo, out)
	if e != nil || relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("bundle must be outside repository")
	}
	parentRoot, e := os.OpenRoot(parent)
	if e != nil {
		return nil, e
	}
	defer parentRoot.Close()
	name := filepath.Base(out)
	if e := parentRoot.Mkdir(name, 0700); e != nil {
		return nil, e
	}
	return parentRoot.OpenRoot(name)
}
func writeNew(root *os.Root, name string, raw []byte) error {
	f, e := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	_, e = f.Write(raw)
	if e == nil {
		e = f.Sync()
	}
	closed := f.Close()
	if e != nil {
		return e
	}
	return closed
}
