package cemcandidate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/verify"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
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

// AssembleStable constructed a typed stable document from a canonically verified
// cem/0.2 or cem/0.3 source. cem/0.x is retired (decision 0525 §1), so the source
// map is admitted only to be refused, as in Assemble (CEM-V1-210). No document is
// written.
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
	source, e := readSource(ctx, r.inputs())
	if e != nil {
		return result, e
	}
	return result, refuseSource(source)
}

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
