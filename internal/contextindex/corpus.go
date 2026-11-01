package contextindex

import (
	"context"
	"strings"
)

// BuildRevisionContext uses the native immutable evidence reader at an explicit
// commit. It creates no snapshot and does not change HEAD or the working tree.
func BuildRevisionContext(ctx context.Context, root, revision string) (*Index, error) {
	ctx, cancel := withGitDeadline(ctx)
	defer cancel()
	identity, err := revisionIdentity(ctx, root, revision)
	if err != nil {
		return nil, err
	}
	index, err := buildEvidence(withStatRoutedPins(ctx), root, identity, nil, "")
	if err != nil {
		return nil, err
	}
	index.compile()
	return index, nil
}

// revisionIdentity is the repository identity at an explicit full commit, with its tree.
func revisionIdentity(ctx context.Context, root, revision string) (repositoryIdentity, error) {
	identity, err := readIdentity(ctx, root)
	if err != nil {
		return identity, err
	}
	if !validObjectID(revision, identity.objectFormat) {
		return identity, &Error{Code: "corpus-invalid-revision", Message: "corpus requires a full immutable commit"}
	}
	kind, err := git(ctx, root, maxIdentityBytes, nil, "cat-file", "-t", revision)
	if err != nil {
		return identity, err
	}
	if string(kind) != "commit\n" {
		return identity, &Error{Code: "corpus-invalid-revision", Message: "corpus revision is not a commit"}
	}
	tree, err := git(ctx, root, maxIdentityBytes, nil, "rev-parse", revision+"^{tree}")
	if err != nil {
		return identity, err
	}
	identity.commitRevision, identity.treeRevision = revision, strings.TrimSpace(string(tree))
	if !validObjectID(identity.treeRevision, identity.objectFormat) {
		return identity, &Error{Code: "corpus-invalid-revision", Message: "invalid corpus tree"}
	}
	return identity, nil
}

// CorpusRepositoryID uses the same root-commit identity as external evidence.
// Shallow or grafted histories cannot establish that identity.
func CorpusRepositoryID(ctx context.Context, root, revision string) (string, error) {
	identity, err := readIdentity(ctx, root)
	if err != nil {
		return "", err
	}
	if identity.historyCut {
		return "", &Error{Code: "corpus-invalid-revision", Message: "corpus identity requires complete ungrafted history"}
	}
	if !validObjectID(revision, identity.objectFormat) {
		return "", &Error{Code: "corpus-invalid-revision", Message: "invalid corpus revision"}
	}
	raw, err := git(ctx, root, maxIdentityBytes, nil, "rev-list", "--max-parents=0", revision)
	if err != nil {
		return "", err
	}
	roots := strings.Fields(string(raw))
	if len(roots) != 1 {
		return "", &Error{Code: "corpus-invalid-revision", Message: "corpus requires one unambiguous root commit"}
	}
	return roots[0], nil
}

// EvidenceTerms exposes the native lexical vocabulary without a second index.
func EvidenceTerms(text string) map[string]struct{} { return terms(text) }

// CollectEvidenceTerms adds EvidenceTerms(text) to a caller-owned set, so a
// caller deriving many term sets can reuse one map instead of allocating one
// per text.
func CollectEvidenceTerms(text string, into map[string]struct{}) { collectTerms(text, into) }

// ImportAnchorLine returns the original native extractor's statement anchor.
func ImportAnchorLine(source Source, imported string) (int, bool) {
	return importEvidenceLine(source, imported)
}

// CorpusGeneratedSource reuses the native generated-path/header rules even for
// inventoried files whose suffix excluded them before the native content read.
func CorpusGeneratedSource(path string, data []byte) bool {
	prefix := data
	if len(prefix) > 4096 {
		prefix = prefix[:4096]
	}
	return generatedPath.MatchString(path) || isGeneratedHeader(prefix)
}
