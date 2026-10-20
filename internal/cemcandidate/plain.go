package cemcandidate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/verify"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// PlainAssemblyProfile is the provider-neutral stable assembly request
// (CEM-V1-240). It names no ticket, attempt, capture or snapshot, so no Corvint
// Tasks store is read or required.
const PlainAssemblyProfile = "cem-stable-plain-assembly/1"

type PlainRequest struct {
	Profile       string           `json:"profile"`
	Repository    string           `json:"repository"`
	ExpectedBase  string           `json:"expectedBase"`
	Target        string           `json:"target"`
	SourceMap     FileRef          `json:"sourceMap"`
	RunnerPlan    FileRef          `json:"runnerPlan"`
	RunnerReceipt FileRef          `json:"runnerReceipt"`
	Criteria      []PlainCriterion `json:"criteria"`
	// RunnerAdmission is absent for opaque runner files (CEM-V1-240) or
	// RunnerAdmissionProfile to admit them as runner documents (CEM-V1-280).
	RunnerAdmission string `json:"runnerAdmission,omitempty"`
}

// PlainCriterion names the source map evidence record that holds the
// acceptance text, and the hunks and basis evidence its link covers. Its
// criterionIndex is its position in the request.
type PlainCriterion struct {
	CriterionEvidenceID string   `json:"criterionEvidenceId"`
	HunkIDs             []string `json:"hunkIds"`
	EvidenceIDs         []string `json:"evidenceIds"`
}

type PlainAssemblyResult struct {
	Profile           string              `json:"profile"`
	StablePath        string              `json:"stablePath"`
	StableSha256      string              `json:"stableSha256"`
	Verification      verify.StableResult `json:"verification"`
	TargetRevision    string              `json:"targetRevision"`
	ReceiptAuthority  string              `json:"receiptAuthority"`
	ExecutionAtCommit string              `json:"executionAtCommit"`
}

func (r PlainRequest) inputs() []FileRef {
	return []FileRef{r.SourceMap, r.RunnerPlan, r.RunnerReceipt}
}

func (r PlainRequest) validate() error {
	if r.Profile != PlainAssemblyProfile || !filepath.IsAbs(r.Repository) || !cw.IsGitOid(r.ExpectedBase) || !cw.IsGitOid(r.Target) || len(r.ExpectedBase) != len(r.Target) || len(r.Criteria) == 0 || len(r.Criteria) > 256 || r.RunnerAdmission != "" && r.RunnerAdmission != RunnerAdmissionProfile {
		return fmt.Errorf("incomplete plain stable assembly request")
	}
	for _, pin := range r.inputs() {
		if !filepath.IsAbs(pin.Path) || filepath.Clean(pin.Path) != pin.Path || !cw.IsSha256(pin.Sha256) {
			return fmt.Errorf("absolute pinned input required")
		}
	}
	for _, c := range r.Criteria {
		if len(c.HunkIDs) == 0 || len(c.HunkIDs) > 32 || len(c.EvidenceIDs) == 0 || len(c.EvidenceIDs) > 32 {
			return fmt.Errorf("invalid plain criterion link")
		}
	}
	return nil
}

// AssemblePlain completes a cem/1.0 producer map (the output of cem prepare and
// cem cite) with plain criterion bindings, one runner receipt record, their
// links and the two runner artifacts. The map is written only after the stable
// verifier ACCEPTs it against the bundle; the runner files are checked only by
// SHA-256, as the verifier checks them, and no producer field raises a result
// axis.
func AssemblePlain(ctx context.Context, r PlainRequest, out string) (PlainAssemblyResult, error) {
	opts := verify.StableOptions{Repository: r.Repository, ExpectedBase: r.ExpectedBase, Target: r.Target, ArtifactRoot: out}
	result := PlainAssemblyResult{Profile: "cem-stable-plain-assembly-result/1", TargetRevision: r.Target, Verification: verify.NewStableResult(opts), ReceiptAuthority: "NOT_OBSERVED", ExecutionAtCommit: "NOT_OBSERVED"}
	if e := r.validate(); e != nil {
		return result, e
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	raws := make([][]byte, 0, 3)
	for _, p := range r.inputs() {
		b, e := readPinned(ctx, p)
		if e != nil {
			return result, e
		}
		raws = append(raws, b)
	}
	source, e := cw.ParseStable(raws[0])
	if e != nil {
		return result, e
	}
	if len(source.Criteria) != 0 || len(source.Receipts) != 0 || len(source.Links) != 0 || len(source.Artifacts) != 0 || source.Provenance != nil {
		return result, fmt.Errorf("plain assembly requires a cem/1.0 producer map source")
	}
	if r.RunnerAdmission == RunnerAdmissionProfile {
		if e := admitRunnerReceipt(ctx, r.Repository, r.Target, raws[1], raws[2]); e != nil {
			return result, e
		}
	}
	stable, e := plainStableBytes(r, source, raws[1], raws[2])
	if e != nil {
		return result, e
	}
	root, e := newBundle(r.Repository, out)
	if e != nil {
		return result, e
	}
	defer root.Close()
	if e := root.Mkdir("artifacts", 0700); e != nil {
		return result, e
	}
	if e := writeNew(root, "artifacts/runner-plan.json", raws[1]); e != nil {
		return result, e
	}
	if e := writeNew(root, "artifacts/runner-receipt.json", raws[2]); e != nil {
		return result, e
	}
	checked, status := verify.Stable(ctx, stable, opts)
	result.Verification = checked
	if afterPlainVerify != nil {
		afterPlainVerify()
	}
	if status != 0 {
		code := ""
		if checked.Code != nil {
			code = *checked.Code
		}
		return result, fmt.Errorf("stable verification refused at %s: %s", checked.Stage, code)
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
	return result, nil
}

// plainStableBytes builds the typed stable document. Each plain criterion is
// the cited evidence span itself, so its digest is that record's spanSha256
// and its ID is computed over its three non-id members (CEM-V1-083).
func plainStableBytes(r PlainRequest, source *cw.StableMap, plan, receipt []byte) ([]byte, error) {
	spans := map[string]string{}
	for _, ev := range source.Change.Evidence {
		spans[ev.ID] = ev.SpanSha256
	}
	d := &cw.StableMap{Change: source.Change, Role: source.Role, Criteria: []cw.StableCriterion{}, Receipts: []cw.StableReceipt{}, Links: []cw.StableLink{}, Artifacts: []cw.StableArtifact{}}
	planDigest, receiptDigest := tr.Digest(plan), tr.Digest(receipt)
	for i, c := range r.Criteria {
		span, ok := spans[c.CriterionEvidenceID]
		if !ok {
			return nil, fmt.Errorf("criterion evidence is not an evidence record of the source map")
		}
		criterion := cw.StableCriterion{CriterionIndex: int64(i), CriterionSha256: span, EvidenceID: c.CriterionEvidenceID}
		id, e := plainCriterionID(criterion)
		if e != nil {
			return nil, e
		}
		criterion.ID = id
		d.Criteria = append(d.Criteria, criterion)
		d.Links = append(d.Links, cw.StableLink{CriterionID: id, HunkIDs: append([]string(nil), c.HunkIDs...), EvidenceIDs: append([]string(nil), c.EvidenceIDs...), RunnerReceiptSha256s: []string{receiptDigest}})
	}
	d.Receipts = append(d.Receipts, cw.StableReceipt{Sha256: receiptDigest, Profile: "corvint-test-runner-receipt/0", PlanSha256: planDigest, SourceGitBinding: "NOT_OBSERVED", ExecutionAuthority: "CALLER_OBSERVED", DependencyClosure: "NOT_OBSERVED"})
	d.Artifacts = append(d.Artifacts, cw.StableArtifact{Kind: "runner-plan", Path: "artifacts/runner-plan.json", Sha256: planDigest}, cw.StableArtifact{Kind: "runner-receipt", Path: "artifacts/runner-receipt.json", Sha256: receiptDigest})
	return cw.EncodeStable(d)
}

func plainCriterionID(c cw.StableCriterion) (string, error) {
	value, e := cw.Parse([]byte(fmt.Sprintf(`{"criterionIndex":%d,"criterionSha256":%q,"evidenceId":%q}`, c.CriterionIndex, c.CriterionSha256, c.EvidenceID)))
	if e != nil {
		return "", e
	}
	return cw.CandidateCriterionPrefix + tr.Digest(cw.CanonicalValue(value)), nil
}

// newBundle creates the new bundle directory outside the repository, under a
// parent with no symlink aliases.
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

// afterPlainVerify is nil in production. Tests set it to change a pinned
// input between verification and the final pin check.
var afterPlainVerify func()

// writeNew creates name exclusively and removes it again if the write, sync or
// close fails, so a failed write leaves no partial map behind.
func writeNew(root *os.Root, name string, raw []byte) error {
	f, e := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	_, e = f.Write(raw)
	if e == nil {
		e = f.Sync()
	}
	if closed := f.Close(); e == nil {
		e = closed
	}
	if e != nil {
		_ = root.Remove(name)
	}
	return e
}
