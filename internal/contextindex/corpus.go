package contextindex

import (
	"context"
	"strings"
)

// BuildRevisionContext uses the native immutable evidence reader at an explicit
// commit. It creates no snapshot and does not change HEAD or the working tree.
func BuildRevisionContext(ctx context.Context, root, revision string) (*Index, error) {
	ctx, cancel := context.WithTimeout(ctx, gitDeadline)
	defer cancel()
	identity, err := readIdentity(ctx, root)
	if err != nil {
		return nil, err
	}
	if !validObjectID(revision, identity.objectFormat) {
		return nil, &Error{Code: "corpus-invalid-revision", Message: "corpus requires a full immutable commit"}
	}
	kind, err := git(ctx, root, maxIdentityBytes, nil, "cat-file", "-t", revision)
	if err != nil {
		return nil, err
	}
	if string(kind) != "commit\n" {
		return nil, &Error{Code: "corpus-invalid-revision", Message: "corpus revision is not a commit"}
	}
	tree, err := git(ctx, root, maxIdentityBytes, nil, "rev-parse", revision+"^{tree}")
	if err != nil {
		return nil, err
	}
	identity.commitRevision, identity.treeRevision = revision, strings.TrimSpace(string(tree))
	if !validObjectID(identity.treeRevision, identity.objectFormat) {
		return nil, &Error{Code: "corpus-invalid-revision", Message: "invalid corpus tree"}
	}
	index, err := buildEvidence(ctx, root, identity, nil, "")
	if err != nil {
		return nil, err
	}
	index.compile()
	return index, nil
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
