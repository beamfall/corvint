package contextindex

// PinnedIntentPointer is a caller-owned enrollment declaration, not authority.
// An empty BlobHash means the path was unavailable when enrollment was pinned.
type PinnedIntentPointer struct {
	Path     string `json:"path"`
	Revision string `json:"revision"`
	BlobHash string `json:"blob_hash"`
}
