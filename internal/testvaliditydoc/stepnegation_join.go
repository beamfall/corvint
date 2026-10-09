package testvaliditydoc

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/stepnegation"
)

// stepNegationJoinQualified gates LPCV-V0-068. It stays false until the
// LPCV-V0-070 live matrix is retained on a PWP-V0-008 qualified tuple; while
// false JoinStepNegation reads nothing and every output byte is unchanged.
var stepNegationJoinQualified = false

// stepNegationAnchorPrefix names the retained step-negation document a joined
// strength axis came from.
const stepNegationAnchorPrefix = "strength-evidence:"

// JoinStepNegation joins retained corvint-step-negation/0 documents into the
// per-test strength axes of a Playwright /0 or /1 receipt (LPCV-V0-068). A
// document joins only when its test identity, runner tuple and application
// identity equal the receipt's, its tuple is a qualified candidate, and its
// bound config and spec digests equal both the receipt's and the worktree's
// current bytes. Without a joinable document the projection is unchanged.
func JoinStepNegation(document Document, input Input, root string) Document {
	if !stepNegationJoinQualified || input.js == nil {
		return document
	}
	if root == "" {
		root = "."
	}
	root, err := filepath.Abs(root)
	if err == nil {
		root, err = filepath.EvalSymlinks(root)
	}
	if err != nil {
		return document
	}
	receipt := *input.js
	if receipt.Profile != jstestprovider.ExternalProfile && receipt.Profile != jstestprovider.AttestedExternalProfile {
		return document
	}
	if jstestprovider.ReceiptRuntimeTuple(receipt) != jstestprovider.RuntimeTupleCandidate {
		return document
	}
	worktree, err := os.OpenRoot(root)
	if err != nil {
		return document
	}
	defer worktree.Close()
	tests := document.Tests
	for index, outcome := range receipt.Tests {
		if index >= len(tests) {
			break
		}
		key, ok := negationKey(root, outcome)
		if !ok {
			continue
		}
		retained, err := stepnegation.Load(root, key)
		if err != nil || retained == nil || !joinable(worktree, root, receipt, outcome, key, *retained) {
			continue
		}
		strength := stepnegation.Aggregate(*retained)
		strength.Anchors = append([]string{stepNegationAnchorPrefix + stepnegation.EvidenceDirectory + "/" + stepnegation.FileName(key)}, strength.Anchors...)
		joined := append([]Test(nil), tests...)
		joined[index].Projection.Strength = strength
		tests = joined
	}
	document.Tests = tests
	return document
}

// negationKey is the retained identity of one receipt test: its
// worktree-relative spec file, screened full title and project.
func negationKey(root string, outcome jstestprovider.TestOutcome) (stepnegation.TestKey, bool) {
	parts := strings.SplitN(outcome.FullName, " > ", 4)
	if len(parts) != 4 || outcome.Anchor == nil || outcome.Project == nil || outcome.Project.Name == "" {
		return stepnegation.TestKey{}, false
	}
	relative, err := filepath.Rel(root, outcome.Anchor.File)
	if err != nil || !filepath.IsLocal(relative) {
		return stepnegation.TestKey{}, false
	}
	return stepnegation.TestKey{File: filepath.ToSlash(relative), FullTitle: stepnegation.Screen(parts[3]), Project: outcome.Project.Name}, true
}

func joinable(worktree *os.Root, root string, receipt jstestprovider.Receipt, outcome jstestprovider.TestOutcome, key stepnegation.TestKey, retained stepnegation.Document) bool {
	binding := retained.Binding
	if binding.Test.Key() != key || binding.Test.Browser != outcome.Project.Browser || binding.Test.Device != outcome.Project.Device {
		return false
	}
	if binding.Runner.Name != "playwright" || binding.Runner.Version != receipt.Identity.RunnerVersion || binding.Runner.NodeVersion != receipt.Identity.NodeVersion || binding.Runner.Tuple != jstestprovider.RuntimeTupleCandidate {
		return false
	}
	if !sameApplication(receipt, binding.Application) || !sameTestRepository(receipt, binding.TestRepository) {
		return false
	}
	spec := filepath.Join(root, filepath.FromSlash(binding.SpecFile))
	config := filepath.Join(root, filepath.FromSlash(binding.ConfigFile))
	if binding.SpecFile != key.File || config != receipt.Identity.ConfigFile {
		return false
	}
	if receipt.Identity.TestFileDigests[spec] != binding.SpecDigest || receipt.Identity.ConfigDigest != binding.ConfigDigest {
		return false
	}
	return compareBound(worktree, root, spec, binding.SpecDigest) == "" && compareBound(worktree, root, config, binding.ConfigDigest) == ""
}

// sameApplication compares the /0 declared label or the /1 attested
// repository and instance with the receipt's.
func sameApplication(receipt jstestprovider.Receipt, application stepnegation.Application) bool {
	if application.Profile != receipt.Profile {
		return false
	}
	if receipt.Profile == jstestprovider.ExternalProfile {
		return receipt.External != nil && application.Label == stepnegation.Screen(receipt.External.DeclaredAppIdentity)
	}
	attestation := receipt.ApplicationAttestation
	if attestation == nil || attestation.Before == nil || len(attestation.Failures) != 0 {
		return false
	}
	observed := attestation.Before.Attestation
	return application.RootCommit == observed.Repository.RootCommit && application.Revision == observed.Repository.Revision && application.Tree == observed.Repository.Tree &&
		application.InstanceKind == observed.Instance.Kind && application.InstanceID == observed.Instance.ID && application.StartGeneration == observed.Instance.StartGeneration
}

// sameTestRepository requires a /1 receipt's test repository identity, at
// start and at publish, to be clean and equal to the bound one, so a witness never survives a
// new test revision (LPCV-V0-068). A /0 receipt carries no test repository
// identity; its test side is bound by the spec and config digests only.
func sameTestRepository(receipt jstestprovider.Receipt, bound stepnegation.Repository) bool {
	if receipt.Profile != jstestprovider.AttestedExternalProfile {
		return true
	}
	for _, observed := range []*jstestprovider.ApplicationRepositoryIdentity{receipt.TestRepositoryAtStart, receipt.TestRepositoryAtPublish} {
		if observed == nil || observed.DirtyState != "clean" || observed.RootCommit != bound.RootCommit || observed.Revision != bound.Revision || observed.Tree != bound.Tree {
			return false
		}
	}
	return true
}
