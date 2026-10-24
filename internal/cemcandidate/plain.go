package cemcandidate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/verify"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// PlainAssemblyProfile is the provider-neutral stable assembly request
// (CEM-V1-240). It names no ticket, attempt, capture or snapshot, so no Corvint
// Tasks store is read or required.
const PlainAssemblyProfile = "cem-stable-plain-assembly/1"

type PlainRequest struct {
	Profile       string  `json:"profile"`
	Repository    string  `json:"repository"`
	ExpectedBase  string  `json:"expectedBase"`
	Target        string  `json:"target"`
	SourceMap     FileRef `json:"sourceMap"`
	RunnerPlan    FileRef `json:"runnerPlan,omitzero"`
	RunnerReceipt FileRef `json:"runnerReceipt,omitzero"`
	// Runners is the multi-receipt form (CEM-V1-573): one plan/receipt pair
	// per runner receipt, in map order. A request names either the single
	// runnerPlan/runnerReceipt pair or runners, never both.
	Runners  []RunnerPair     `json:"runners,omitempty"`
	Criteria []PlainCriterion `json:"criteria"`
	// RunnerAdmission holds the runnerAdmission member's JSON bytes. It is nil
	// only when the member is absent, which keeps the runner files opaque
	// (CEM-V1-240); a present member must be the JSON string
	// RunnerAdmissionProfile (CEM-V1-280), so a present "" or null is refused
	// rather than read as absent.
	RunnerAdmission json.RawMessage `json:"runnerAdmission,omitempty"`
	// runnersNamed and singleNamed record that the request JSON named
	// runners, or runnerPlan or runnerReceipt, even as null or an empty pin,
	// so a named member of the other form is refused (CEM-V1-575).
	runnersNamed bool `json:"-"`
	singleNamed  bool `json:"-"`
}

// UnmarshalJSON decodes the request and records which runner form members
// its JSON names.
func (r *PlainRequest) UnmarshalJSON(raw []byte) error {
	type request PlainRequest
	var members map[string]json.RawMessage
	if e := json.Unmarshal(raw, &members); e != nil {
		return e
	}
	if e := json.Unmarshal(raw, (*request)(r)); e != nil {
		return e
	}
	_, r.runnersNamed = members["runners"]
	_, plan := members["runnerPlan"]
	_, receipt := members["runnerReceipt"]
	r.singleNamed = plan || receipt
	return nil
}

// PlainCriterion names the source map evidence record that holds the
// acceptance text, and the hunks and basis evidence its link covers. Its
// criterionIndex is its position in the request.
type PlainCriterion struct {
	CriterionEvidenceID string   `json:"criterionEvidenceId"`
	HunkIDs             []string `json:"hunkIds"`
	EvidenceIDs         []string `json:"evidenceIds"`
	// RunnerReceiptSha256s names the receipts of the runners pairs that cover
	// this criterion. It is required in the multi-receipt form and refused in
	// the single form, whose one receipt covers every criterion (CEM-V1-573).
	RunnerReceiptSha256s []string `json:"runnerReceiptSha256s,omitempty"`
	// receiptsNamed records that the criterion JSON named
	// runnerReceiptSha256s, even as null (CEM-V1-575).
	receiptsNamed bool `json:"-"`
}

// UnmarshalJSON decodes the criterion and records whether its JSON names
// runnerReceiptSha256s.
func (c *PlainCriterion) UnmarshalJSON(raw []byte) error {
	type criterion PlainCriterion
	var members map[string]json.RawMessage
	if e := json.Unmarshal(raw, &members); e != nil {
		return e
	}
	if e := json.Unmarshal(raw, (*criterion)(c)); e != nil {
		return e
	}
	_, c.receiptsNamed = members["runnerReceiptSha256s"]
	return nil
}

func (c PlainCriterion) namesReceipts() bool {
	return c.RunnerReceiptSha256s != nil || c.receiptsNamed
}

// RunnerPair is one runner plan and the receipt that binds it.
type RunnerPair struct {
	RunnerPlan    FileRef `json:"runnerPlan"`
	RunnerReceipt FileRef `json:"runnerReceipt"`
}

// MaxRunnerPairs is the reader's runnerReceipts bound, so the multi-receipt
// form can name as many receipts as a stable map may hold (CEM-V1-573).
const MaxRunnerPairs = 64

// MaxCriterionReceipts is the reader's bound on one link's
// runnerReceiptSha256s (CEM-V1-573).
const MaxCriterionReceipts = 32

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
	inputs := []FileRef{r.SourceMap}
	for _, pair := range r.pairs() {
		inputs = append(inputs, pair.RunnerPlan, pair.RunnerReceipt)
	}
	return inputs
}

// multi reports the multi-receipt form: any runners member, even an empty
// or null one, so a request is never read as both forms.
func (r PlainRequest) multi() bool { return r.Runners != nil || r.runnersNamed }

// pairs returns the request's plan/receipt pairs in map order; the single
// form is one pair.
func (r PlainRequest) pairs() []RunnerPair {
	if r.multi() {
		return r.Runners
	}
	return []RunnerPair{{RunnerPlan: r.RunnerPlan, RunnerReceipt: r.RunnerReceipt}}
}

func (r PlainRequest) validate() error {
	if r.Profile != PlainAssemblyProfile || !filepath.IsAbs(r.Repository) || !cw.IsGitOid(r.ExpectedBase) || !cw.IsGitOid(r.Target) || len(r.ExpectedBase) != len(r.Target) || len(r.Criteria) == 0 || len(r.Criteria) > 256 {
		return fmt.Errorf("incomplete plain stable assembly request")
	}
	if r.RunnerAdmission != nil {
		var profile string
		if json.Unmarshal(r.RunnerAdmission, &profile) != nil || profile != RunnerAdmissionProfile {
			return fmt.Errorf("incomplete plain stable assembly request")
		}
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
	return r.validateRunnerSet()
}

// validateRunnerSet applies the CEM-V1-575 request checks, before any input is
// read: the two forms never mix, the multi form is admitted and bounded, every
// pin is distinct, and every criterion names receipts of the set that, between
// them, use every receipt.
func (r PlainRequest) validateRunnerSet() error {
	if !r.multi() {
		for _, c := range r.Criteria {
			if c.namesReceipts() {
				return cemcode.New("runner-set-form", "a single-form request names no per-criterion runner receipts")
			}
		}
		return nil
	}
	if r.singleNamed || r.RunnerPlan != (FileRef{}) || r.RunnerReceipt != (FileRef{}) {
		return cemcode.New("runner-set-form", "a request names either runnerPlan/runnerReceipt or runners, not both")
	}
	if r.RunnerAdmission == nil {
		return cemcode.New("runner-admission-required", "the runners form requires runnerAdmission")
	}
	if len(r.Runners) == 0 || len(r.Runners) > MaxRunnerPairs {
		return cemcode.New("runner-set-bound", "runners must hold 1 to %d plan/receipt pairs", MaxRunnerPairs)
	}
	digests, paths := map[string]bool{}, map[string]bool{}
	receipts := map[string]bool{}
	for _, pair := range r.Runners {
		for _, pin := range []FileRef{pair.RunnerPlan, pair.RunnerReceipt} {
			if digests[pin.Sha256] || paths[pin.Path] {
				return cemcode.New("runner-set-duplicate", "runner pin %s is named twice", pin.Path)
			}
			digests[pin.Sha256], paths[pin.Path] = true, true
		}
		receipts[pair.RunnerReceipt.Sha256] = true
	}
	used := map[string]bool{}
	for _, c := range r.Criteria {
		if c.RunnerReceiptSha256s == nil {
			return cemcode.New("runner-set-form", "each criterion of the runners form names its runner receipts")
		}
		if len(c.RunnerReceiptSha256s) == 0 || len(c.RunnerReceiptSha256s) > MaxCriterionReceipts {
			return cemcode.New("runner-set-bound", "a criterion names 1 to %d runner receipts", MaxCriterionReceipts)
		}
		seen := map[string]bool{}
		for _, digest := range c.RunnerReceiptSha256s {
			if seen[digest] {
				return cemcode.New("runner-set-duplicate", "a criterion names runner receipt %s twice", digest)
			}
			seen[digest] = true
			if !receipts[digest] {
				return cemcode.New("runner-set-unresolved", "criterion runner receipt %s is not a runners receipt", digest)
			}
			used[digest] = true
		}
	}
	if len(used) != len(receipts) {
		return cemcode.New("runner-set-unused", "every runners receipt must cover a criterion")
	}
	return nil
}

// checkRunnerProfiles refuses a criterion covered by two receipts of the same
// runner profile (CEM-V1-576): each criterion has at most one receipt per
// runner, so two profiles never disagree about one criterion silently.
func (r PlainRequest) checkRunnerProfiles(runners map[string]string) error {
	for i, c := range r.Criteria {
		seen := map[string]bool{}
		for _, digest := range c.RunnerReceiptSha256s {
			if seen[runners[digest]] {
				return cemcode.New("runner-profile-conflict", "criterion %d is covered by two %s receipts", i, runners[digest])
			}
			seen[runners[digest]] = true
		}
	}
	return nil
}

// AssemblePlain completes a cem/1.0 producer map (the output of cem prepare and
// cem cite) with plain criterion bindings, one runner receipt record per
// runner pair (one in the single form, up to MaxRunnerPairs in the runners
// form), their links and two runner artifacts per pair. The map is written
// only after the stable verifier ACCEPTs it against the bundle; without runner
// admission the runner files are checked only by SHA-256, as the verifier
// checks them, and the receipt record is labelled opaque/0 (CEM-V1-310); no
// producer field raises a result axis.
func AssemblePlain(ctx context.Context, r PlainRequest, out string) (PlainAssemblyResult, error) {
	opts := verify.StableOptions{Repository: r.Repository, ExpectedBase: r.ExpectedBase, Target: r.Target, ArtifactRoot: out}
	result := PlainAssemblyResult{Profile: "cem-stable-plain-assembly-result/1", TargetRevision: r.Target, Verification: verify.NewStableResult(opts), ReceiptAuthority: "NOT_OBSERVED", ExecutionAtCommit: "NOT_OBSERVED"}
	if e := r.validate(); e != nil {
		return result, e
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	raws := make([][]byte, 0, 1+2*len(r.pairs()))
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
	runners := raws[1:]
	if r.RunnerAdmission != nil {
		profiles := map[string]string{}
		for i := 0; i < len(runners); i += 2 {
			runner, e := admitRunnerReceipt(ctx, r.Repository, r.Target, runners[i], runners[i+1])
			if e != nil {
				return result, e
			}
			profiles[tr.Digest(runners[i+1])] = runner
		}
		if e := r.checkRunnerProfiles(profiles); e != nil {
			return result, e
		}
	}
	stable, e := plainStableBytes(r, source, runners)
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
	for i := 0; i < len(runners); i += 2 {
		plan, receipt := r.artifactPaths(i / 2)
		if e := writeNew(root, plan, runners[i]); e != nil {
			return result, e
		}
		if e := writeNew(root, receipt, runners[i+1]); e != nil {
			return result, e
		}
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

// artifactPaths returns the bundle paths of runner pair i. The single form
// keeps its fixed paths, so its map bytes are unchanged (CEM-V1-574); the
// runners form gives each pair its own 0-based paths (CEM-V1-573).
func (r PlainRequest) artifactPaths(i int) (string, string) {
	if !r.multi() {
		return "artifacts/runner-plan.json", "artifacts/runner-receipt.json"
	}
	return fmt.Sprintf("artifacts/runner-plan-%d.json", i), fmt.Sprintf("artifacts/runner-receipt-%d.json", i)
}

// plainStableBytes builds the typed stable document from the source map and
// the runner bytes, plan then receipt for each pair in order. Each plain
// criterion is the cited evidence span itself, so its digest is that record's
// spanSha256 and its ID is computed over its three non-id members
// (CEM-V1-083).
func plainStableBytes(r PlainRequest, source *cw.StableMap, runners [][]byte) ([]byte, error) {
	spans := map[string]string{}
	for _, ev := range source.Change.Evidence {
		spans[ev.ID] = ev.SpanSha256
	}
	d := &cw.StableMap{Change: source.Change, Role: source.Role, Criteria: []cw.StableCriterion{}, Receipts: []cw.StableReceipt{}, Links: []cw.StableLink{}, Artifacts: []cw.StableArtifact{}}
	// Only an admitted receipt is labelled a runner receipt; without
	// admission the record says the bytes are opaque (CEM-V1-310).
	receiptProfile := cw.StableOpaqueReceiptProfile
	if r.RunnerAdmission != nil {
		receiptProfile = cw.StableRunnerReceiptProfile
	}
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
		covered := c.RunnerReceiptSha256s
		if !r.multi() {
			covered = []string{tr.Digest(runners[1])}
		}
		d.Links = append(d.Links, cw.StableLink{CriterionID: id, HunkIDs: append([]string(nil), c.HunkIDs...), EvidenceIDs: append([]string(nil), c.EvidenceIDs...), RunnerReceiptSha256s: append([]string(nil), covered...)})
	}
	for i := 0; i < len(runners); i += 2 {
		planDigest, receiptDigest := tr.Digest(runners[i]), tr.Digest(runners[i+1])
		planPath, receiptPath := r.artifactPaths(i / 2)
		d.Receipts = append(d.Receipts, cw.StableReceipt{Sha256: receiptDigest, Profile: receiptProfile, PlanSha256: planDigest, SourceGitBinding: "NOT_OBSERVED", ExecutionAuthority: "CALLER_OBSERVED", DependencyClosure: "NOT_OBSERVED"})
		d.Artifacts = append(d.Artifacts, cw.StableArtifact{Kind: "runner-plan", Path: planPath, Sha256: planDigest}, cw.StableArtifact{Kind: "runner-receipt", Path: receiptPath, Sha256: receiptDigest})
	}
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

// syncNew is (*os.File).Sync in production. Tests replace it to inject a
// sync failure into writeNew (V1-1471).
var syncNew = (*os.File).Sync

// writeNew creates name exclusively and removes it again if the write, sync or
// close fails, so a failed write leaves no partial map behind.
func writeNew(root *os.Root, name string, raw []byte) error {
	f, e := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	_, e = f.Write(raw)
	if e == nil {
		e = syncNew(f)
	}
	if closed := f.Close(); e == nil {
		e = closed
	}
	if e != nil {
		_ = root.Remove(name)
	}
	return e
}
