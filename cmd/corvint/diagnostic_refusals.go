package main

import (
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/diagnostic"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// The DRC-V0 refusals of the cmd/corvint families converted after the working-tree impact
// family. Each constructor keeps its site's code and message byte-identical and attaches one
// diagnostic.Refusal literal, so the coverage ratchet counts one literal per site. This file parses
// no message text (DRC-V0-012); the callers that do stay outside it.

func refused(code, message string, refusal diagnostic.Refusal) error {
	return &diagnostic.Error{Err: &gokernel.Error{Code: code, Message: message}, Refusal: refusal}
}

// notRepositoryRootRefusal: an explicit --root, normalized to root, holds no .git entry.
func notRepositoryRootRefusal(operand, root string) error {
	message, topLevel := notRepositoryRootMessage(root)
	return refused("invalid-arguments", message, diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: operand},
		Evidence:       append([]diagnostic.Evidence{{Name: "root", Value: root}}, topLevel...),
		SupportedFixes: []string{"cli.use-git-repository-root"},
	})
}

// notQueryRepositoryRootRefusal: the query adapter holds only the absolute --root it normalized
// before argument validation, so that form is the refused spelling.
func notQueryRepositoryRootRefusal(root string) error {
	message, topLevel := notRepositoryRootMessage(root)
	return refused("invalid-arguments", message, diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: root},
		Evidence:       topLevel,
		SupportedFixes: []string{"cli.use-git-repository-root"},
	})
}

// notRepositoryRootMessage names the enclosing repository's top level when root is a directory
// inside one, instead of claiming root is in no repository (CCF-V1-004). The top level is the
// nearest ancestor holding a .git entry; no Git process runs on this refusal path.
func notRepositoryRootMessage(root string) (string, []diagnostic.Evidence) {
	topLevel := enclosingRepositoryRoot(root)
	if topLevel == "" {
		return "not a Git repository: " + root, nil
	}
	return "not the repository root; top level is " + topLevel, []diagnostic.Evidence{{Name: "top_level", Value: topLevel}}
}

func enclosingRepositoryRoot(root string) string {
	for parent := filepath.Dir(root); parent != root; root, parent = parent, filepath.Dir(parent) {
		if _, err := os.Stat(filepath.Join(parent, ".git")); err == nil {
			return parent
		}
	}
	return ""
}

func queryPlatformRefusal(platform string) error {
	return refused("unsupported-query-platform", "native Go authority-start query is qualified only on Darwin and Linux", diagnostic.Refusal{
		Subject:  diagnostic.Subject{Kind: "host-capability", Value: "native-platform"},
		Evidence: []diagnostic.Evidence{{Name: "platform", Value: platform}},
		Terminal: "unsupported-platform",
	})
}

func featurePlatformRefusal(platform string) error {
	return refused("unsupported-feature-platform", "native Go feature is qualified only on Darwin and Linux", diagnostic.Refusal{
		Subject:  diagnostic.Subject{Kind: "host-capability", Value: "native-platform"},
		Evidence: []diagnostic.Evidence{{Name: "platform", Value: platform}},
		Terminal: "unsupported-platform",
	})
}

func genesisPlatformRefusal(platform string) error {
	return refused("unsupported-genesis-platform", "native Go Genesis inventory is qualified only on Darwin and Linux", diagnostic.Refusal{
		Subject:  diagnostic.Subject{Kind: "host-capability", Value: "native-platform"},
		Evidence: []diagnostic.Evidence{{Name: "platform", Value: platform}},
		Terminal: "unsupported-platform",
	})
}

func impactBudgetOptionRefusal() error {
	return refused("unsupported-impact-option", "--budget-bytes is not implemented for native Go impact", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "argument", Value: "--budget-bytes"},
		SupportedFixes: []string{"impact.omit-budget-bytes"},
	})
}

// batchSnapshotRefusal: no snapshot matches, so there is no snapshot identity to report.
func batchSnapshotRefusal() error {
	return refused("unsupported-batch-snapshot", "no index snapshot matches this tree and this binary: run corvint index", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "repository-state", Value: "index-snapshot"},
		SupportedFixes: []string{"index.write-snapshot"},
	})
}

func affectedGitExecutableRefusal() error {
	return refused("unsupported-affected-status", "git executable is unavailable", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "host-capability", Value: "git-executable"},
		SupportedFixes: []string{"host.put-git-on-path"},
	})
}

func affectedBaseRefusal(base string) error {
	return refused("unsupported-affected-revision", "base "+base+" is not a commit in this repository", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: base},
		SupportedFixes: []string{"affected.use-commit-base", "affected.omit-base"},
	})
}

func affectedHeadRefusal() error {
	return refused("unsupported-affected-revision", "repository has no resolvable HEAD commit", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "repository-state", Value: "head-commit"},
		SupportedFixes: []string{"git.create-head-commit"},
	})
}

func proveGitExecutableRefusal() error {
	return refused("unsupported-prove-revision", "git executable is not available", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "host-capability", Value: "git-executable"},
		SupportedFixes: []string{"host.put-git-on-path"},
	})
}

func checkpointHeadRefusal() error {
	return refused("unsupported-prove-revision", "repository has no resolvable HEAD commit and tree", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "repository-state", Value: "head-commit"},
		SupportedFixes: []string{"git.create-head-commit"},
	})
}

// rangeChangedPathsBoundRefusal: the bounded read stops at the bound, so the full size of the
// change list was never measured and no byte count is evidence.
func rangeChangedPathsBoundRefusal(base string) error {
	return refused("unsupported-prove-history", "the range's changed paths exceed the byte bound", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: base},
		SupportedFixes: []string{"prove.use-bounded-base"},
	})
}

func checkpointObjectFormatRefusal(checkpoint, repositoryFormat, checkpointFormat string) error {
	return refused("object-format-mismatch", "checkpoint object format differs from the current repository", diagnostic.Refusal{
		Subject: diagnostic.Subject{Kind: "artifact", Value: checkpoint},
		Evidence: []diagnostic.Evidence{
			{Name: "repository-object-format", Value: repositoryFormat},
			{Name: "checkpoint-object-format", Value: checkpointFormat},
		},
		SupportedFixes: []string{"prove.use-repository-object-format-checkpoint"},
	})
}

func impactPlatformRefusal(platform string) error {
	return refused("unsupported-impact-platform", "native Go impact is qualified only on Darwin and Linux", diagnostic.Refusal{
		Subject:  diagnostic.Subject{Kind: "host-capability", Value: "native-platform"},
		Evidence: []diagnostic.Evidence{{Name: "platform", Value: platform}},
		Terminal: "unsupported-platform",
	})
}

// impactPathSuffixRefusal is the one admission refusal the standalone impact verb, a batch impact
// operation and a harness file-change event share; normalized is the path the admission checked.
// It carries no suffix evidence: the subject already spells the suffix, and an extensionless path
// has none to report.
func impactPathSuffixRefusal(normalized string) error {
	return refused("unsupported-impact-path-suffix", "native Go impact requires a suffix admitted by the repository index", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: normalized},
		SupportedFixes: []string{"impact.omit-unadmitted-path"},
	})
}

// rangeDiffBoundRefusal: like the changed-paths bound, the bounded read stops at the bound, so no
// diff size is evidence.
func rangeDiffBoundRefusal(base string) error {
	return refused("unsupported-prove-history", "the range's diff exceeds the byte bound", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: base},
		SupportedFixes: []string{"prove.use-diff-bounded-base"},
	})
}

func attestKeyRefusal(keyPath string) error {
	return refused("attest-key-unavailable", "cannot read an Ed25519 PKCS#8 private key from "+keyPath, diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: keyPath},
		SupportedFixes: []string{"attest.use-ed25519-private-key"},
	})
}

func attestPublicKeyRefusal(publicKeyPath string) error {
	return refused("attest-public-key-unavailable", "cannot read an Ed25519 PKIX public key from "+publicKeyPath, diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: publicKeyPath},
		SupportedFixes: []string{"attest.use-ed25519-public-key"},
	})
}

func attestEnvelopeRefusal(envelopePath string) error {
	return refused("attest-envelope-unavailable", "cannot read the DSSE envelope from "+envelopePath, diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: envelopePath},
		SupportedFixes: []string{"attest.use-readable-envelope"},
	})
}

// cemMapPathRefusal and cemMapReadRefusal are the one CEM map read prove --cem and
// prove --verify-cem-attestation share.
func cemMapPathRefusal(mapPath string) error {
	return refused(cemcode.MapUnavailable, "map path must be repository-relative", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: mapPath},
		SupportedFixes: []string{"cem.use-repository-relative-map-path"},
	})
}

func cemMapReadRefusal(mapPath string) error {
	return refused(cemcode.MapUnavailable, "cannot read CEM map", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: mapPath},
		SupportedFixes: []string{"cem.use-readable-map"},
	})
}

// The prove-observe document refusals: stdin is the whole request, and every one of them is
// repaired by piping the document prove wrote.
func proofDocumentSizeRefusal() error {
	return refused("invalid-proof-document", "proof document exceeds 8 MiB", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "request", Value: "stdin"},
		SupportedFixes: []string{"prove-observe.pipe-prove-packet"},
	})
}

func proofDocumentJSONRefusal() error {
	return refused("invalid-proof-document", "stdin is not a JSON object", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "request", Value: "stdin"},
		SupportedFixes: []string{"prove-observe.pipe-prove-packet"},
	})
}

func proofDocumentProfileRefusal() error {
	return refused("invalid-proof-document", "stdin is not a "+proveProfile+" document", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "request", Value: "stdin"},
		SupportedFixes: []string{"prove-observe.pipe-prove-packet"},
	})
}

func proofDocumentMembersRefusal() error {
	return refused("invalid-proof-document", "proof.rows or proof.counts is missing", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "request", Value: "stdin"},
		SupportedFixes: []string{"prove-observe.pipe-prove-packet"},
	})
}

func proofDocumentRowRefusal() error {
	return refused("invalid-proof-document", "proof.rows carries an unknown falsifier or verdict", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "request", Value: "stdin"},
		SupportedFixes: []string{"prove-observe.pipe-prove-packet"},
	})
}

func proofDocumentCountsRefusal() error {
	return refused("invalid-proof-document", "proof.counts does not match proof.rows", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "request", Value: "stdin"},
		SupportedFixes: []string{"prove-observe.pipe-prove-packet"},
	})
}
