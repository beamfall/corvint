package tcq

// TreeEntry is one resolved target-tree object. TCQ never opens a worktree
// path: TCQ-V0-001 confines every source read to the inherited bounded,
// sanitized, no-fetch Git boundary.
type TreeEntry struct {
	Mode  string
	Type  string
	Found bool
}

// Repository is that inherited boundary. Both methods read the verified target
// tree only; sibling repositories, alternates, implicit fetch, and
// additional-repository discovery are forbidden and must be refused by the
// implementation, not by TCQ.
type Repository interface {
	// Blob returns the bytes of one target-tree blob by OID.
	Blob(oid string) ([]byte, error)
	// TreeEntry resolves one normalized repository-relative path at revision.
	// A missing object is (TreeEntry{}, nil), never an error; an error is a
	// failed lookup, which TCQ returns as-is (TCQ-V0-028).
	TreeEntry(revision, path string) (TreeEntry, error)
}

// blobModes are the file modes TCQ-V0-028 accepts for a keyed report row. A
// symlink or gitlink is not a supported mode and leaves the row unkeyed.
var blobModes = map[string]bool{"100644": true, "100755": true}

const treeMode = "040000"
