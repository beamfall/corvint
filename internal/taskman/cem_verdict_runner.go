package taskman

import (
	"strings"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/testrunner"
)

// The runner-bound satisfaction bases (CEM-V1-606, CEM-V1-607). Neither raises
// a recomputed tier: a governed hunk keeps its citation tier and is satisfied
// on one of these named bases only when the sealed map's own runnerReceipts,
// read back from the candidate commit, bind it.
const (
	runnerAnchorInput    = "runner-input"
	runnerAnchorArtifact = "runner-artifact"
)

// The closed reasons an unsatisfied governed hunk names (CEM-V1-608).
const (
	runnerReasonUncited          = "uncited"
	runnerReasonNoPostImage      = "no-post-image"
	runnerReasonNoBinding        = "no-runner-binding"
	runnerReasonNotPassed        = "runner-not-passed"
	runnerReasonInputMismatch    = "runner-input-mismatch"
	runnerReasonArtifactMismatch = "runner-artifact-mismatch"
)

// runnerAnchor answers, for one governed hunk below anchored, the runner-bound
// basis that satisfies it or the closed reason none does. A non-nil error is a
// failed Git read: the gate is then unverifiable, never satisfied.
type runnerAnchor func(hunk wire.Hunk, tier string) (anchor, reason string, err error)

// runnerBinding reads runner evidence from the candidate commit's objects
// only, so the answer is pinned to the commit the gate admitted, never to the
// working tree.
type runnerBinding struct {
	git       func(args ...string) ([]byte, error)
	commit    string
	doc       *wire.StableMap
	criteria  map[string]wire.StableCriterion
	receipts  map[string]wire.StableReceipt
	artifacts map[string]wire.StableArtifact // by kind + "\x00" + sha256
	byPath    map[string]wire.StableArtifact
	files     map[string]candidateFile
	runs      map[string]runnerRun
}

type candidateFile struct {
	present bool
	sha256  string
	bytes   []byte
}

// runnerRun is one runnerReceipts row read back and checked.
type runnerRun struct {
	reason string // "" when the run passed and its bytes are bound
	inputs map[string]string
}

func newRunnerBinding(git func(args ...string) ([]byte, error), commit string, doc *wire.StableMap) *runnerBinding {
	b := &runnerBinding{git: git, commit: commit, doc: doc, criteria: map[string]wire.StableCriterion{}, receipts: map[string]wire.StableReceipt{}, artifacts: map[string]wire.StableArtifact{}, byPath: map[string]wire.StableArtifact{}, files: map[string]candidateFile{}, runs: map[string]runnerRun{}}
	for _, c := range doc.Criteria {
		b.criteria[c.ID] = c
	}
	for _, r := range doc.Receipts {
		b.receipts[r.Sha256] = r
	}
	for _, a := range doc.Artifacts {
		if a.Kind == "runner-plan" || a.Kind == "runner-receipt" {
			b.artifacts[a.Kind+"\x00"+a.Sha256] = a
			b.byPath[a.Path] = a
		}
	}
	return b
}

// file reads one candidate path as a regular-file blob; an absent path or a
// non-regular entry has no post-image.
func (b *runnerBinding) file(path string) (candidateFile, error) {
	if f, ok := b.files[path]; ok {
		return f, nil
	}
	out, err := b.git("ls-tree", "-z", "--full-tree", "--end-of-options", b.commit, "--", path)
	if err != nil {
		return candidateFile{}, err
	}
	f := candidateFile{}
	entry := strings.TrimSuffix(string(out), "\x00")
	meta, name, cut := strings.Cut(entry, "\t")
	fields := strings.Fields(meta)
	if cut && name == path && len(fields) == 3 && fields[1] == "blob" && (fields[0] == "100644" || fields[0] == "100755") {
		raw, err := b.git("cat-file", "blob", fields[2])
		if err != nil {
			return candidateFile{}, err
		}
		f = candidateFile{present: true, sha256: sum(raw), bytes: raw}
	}
	b.files[path] = f
	return f, nil
}

// artifactBytes returns the bytes of the map artifact of kind bound to digest
// when the candidate carries exactly those bytes at its path.
func (b *runnerBinding) artifactBytes(kind, digest string) ([]byte, string, error) {
	a, ok := b.artifacts[kind+"\x00"+digest]
	if !ok {
		return nil, runnerReasonNoBinding, nil
	}
	f, err := b.file(a.Path)
	if err != nil {
		return nil, "", err
	}
	if !f.present || f.sha256 != digest {
		return nil, runnerReasonArtifactMismatch, nil
	}
	return f.bytes, "", nil
}

// run reads one runnerReceipts row: its plan and receipt artifacts must be the
// committed bytes the row digests, decode as closed runner documents carrying
// the runner plan and receipt profiles, bind the receipt to that plan, and
// record a complete run in which every observed test passed.
func (b *runnerBinding) run(digest string) (runnerRun, error) {
	if r, ok := b.runs[digest]; ok {
		return r, nil
	}
	row, ok := b.receipts[digest]
	if !ok || row.Profile != wire.StableRunnerReceiptProfile {
		b.runs[digest] = runnerRun{reason: runnerReasonNoBinding}
		return b.runs[digest], nil
	}
	planBytes, reason, err := b.artifactBytes("runner-plan", row.PlanSha256)
	if err != nil {
		return runnerRun{}, err
	}
	var receiptBytes []byte
	if reason == "" {
		if receiptBytes, reason, err = b.artifactBytes("runner-receipt", row.Sha256); err != nil {
			return runnerRun{}, err
		}
	}
	out := runnerRun{reason: reason}
	if reason == "" {
		var plan testrunner.PlanDocument
		var receipt testrunner.ReceiptDocument
		if testrunner.DecodeDocument(planBytes, &plan) != nil || testrunner.DecodeDocument(receiptBytes, &receipt) != nil ||
			plan.Profile != testrunner.PlanProfile || receipt.Profile != wire.StableRunnerReceiptProfile || receipt.PlanSha256 != testrunner.Identity(plan) {
			out.reason = runnerReasonArtifactMismatch
		} else if !runnerPassed(receipt) {
			out.reason = runnerReasonNotPassed
		} else {
			out.inputs = plan.Request.InputFiles
		}
	}
	b.runs[digest] = out
	return out, nil
}

func runnerPassed(r testrunner.ReceiptDocument) bool {
	if testrunner.CheckReceipt(r) != nil || r.Error != "" || !r.Observation.Complete || len(r.Observation.Problems) != 0 || len(r.Observation.Tests) == 0 {
		return false
	}
	for _, t := range r.Observation.Tests {
		if t.State != testrunner.Passed {
			return false
		}
	}
	return true
}

// anchor implements runnerAnchor. Only a cited hunk (one carrying a basis) can
// be bound; the first applicable basis wins and otherwise the most specific
// reason is reported.
func (b *runnerBinding) anchor(hunk wire.Hunk, tier string) (string, string, error) {
	if tier != verify.TierCited {
		return "", runnerReasonUncited, nil
	}
	f, err := b.file(hunk.Path)
	if err != nil {
		return "", "", err
	}
	if !f.present {
		return "", runnerReasonNoPostImage, nil
	}
	reason := runnerReasonNoBinding
	worse := func(r string) {
		if runnerReasonRank(r) > runnerReasonRank(reason) {
			reason = r
		}
	}
	if a, ok := b.byPath[hunk.Path]; ok {
		if anchor, r, err := b.artifactAnchor(a, f); err != nil || anchor != "" {
			return anchor, "", err
		} else {
			worse(r)
		}
	}
	basis := map[string]bool{}
	for _, x := range hunk.Basis {
		basis[x.EvidenceID] = true
	}
	for _, link := range b.doc.Links {
		criterion, ok := b.criteria[link.CriterionID]
		if !ok || !criterion.Plain() || !basis[criterion.EvidenceID] || len(link.RunnerReceiptSha256s) == 0 || !containsString(link.HunkIDs, hunk.ID) {
			continue
		}
		bound, failed := false, ""
		for _, digest := range link.RunnerReceiptSha256s {
			run, err := b.run(digest)
			if err != nil {
				return "", "", err
			}
			if run.reason != "" {
				failed = run.reason
				break
			}
			if want, ok := run.inputs[hunk.Path]; ok {
				if want == f.sha256 {
					bound = true
				} else {
					worse(runnerReasonInputMismatch)
				}
			}
		}
		switch {
		case failed != "":
			worse(failed)
		case bound:
			return runnerAnchorInput, "", nil
		}
	}
	return "", reason, nil
}

// artifactAnchor binds a hunk that adds or edits a committed runner artifact:
// a runnerReceipts row must digest it and its run must pass. A plan that
// several rows name is bound by any one passing run; otherwise the most
// specific failure is reported.
func (b *runnerBinding) artifactAnchor(a wire.StableArtifact, f candidateFile) (string, string, error) {
	reason := runnerReasonNoBinding
	for _, row := range b.doc.Receipts {
		if (a.Kind == "runner-receipt" && row.Sha256 != a.Sha256) || (a.Kind == "runner-plan" && row.PlanSha256 != a.Sha256) {
			continue
		}
		if f.sha256 != a.Sha256 {
			return "", runnerReasonArtifactMismatch, nil
		}
		run, err := b.run(row.Sha256)
		if err != nil {
			return "", "", err
		}
		if run.reason == "" {
			return runnerAnchorArtifact, "", nil
		}
		if runnerReasonRank(run.reason) > runnerReasonRank(reason) {
			reason = run.reason
		}
	}
	return "", reason, nil
}

// runnerReasonRank orders reasons from least to most specific.
func runnerReasonRank(reason string) int {
	switch reason {
	case runnerReasonNoBinding:
		return 1
	case runnerReasonNotPassed:
		return 2
	case runnerReasonArtifactMismatch:
		return 3
	case runnerReasonInputMismatch:
		return 4
	}
	return 0
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
