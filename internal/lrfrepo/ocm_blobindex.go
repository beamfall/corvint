package lrfrepo

// One derivation per blob, shared across an OCM operation's claims.
//
// Every claim rule -- Go, Python, JS -- asks its blob questions that only a
// whole-blob scan can answer: what encloses this offset, how many sites in the
// blob derive this selector. Asking per claim makes verification quadratic or
// cubic in claims per blob, so a LEGAL artifact (maxClaims claims in one
// maxOCMBlob blob) runs minutes to hours past gitrun.DefaultTotalBudget and
// returns git-timeout instead of a verdict. Each language index answers those
// questions from a single pass; this cache makes that pass happen once.
//
// Blobs are keyed by their Git OID, which is a content hash, so two claims
// sharing a key share a byte-identical blob. A cache is scoped to one
// operation and is never shared across them.
type blobIndexes struct {
	goBlobs     map[string]*goBlobIndex
	pythonBlobs map[string]*pythonBlobIndex
	jsBlobs     map[string]*jsBlobIndex
}

func newBlobIndexes() *blobIndexes {
	return &blobIndexes{
		goBlobs:     map[string]*goBlobIndex{},
		pythonBlobs: map[string]*pythonBlobIndex{},
		jsBlobs:     map[string]*jsBlobIndex{},
	}
}

func (cache *blobIndexes) goIndex(blobOID string, blob []byte) *goBlobIndex {
	if index, found := cache.goBlobs[blobOID]; found {
		return index
	}
	index := newGoBlobIndex(blob)
	cache.goBlobs[blobOID] = index
	return index
}

func (cache *blobIndexes) pythonIndex(blobOID string, blob []byte) *pythonBlobIndex {
	if index, found := cache.pythonBlobs[blobOID]; found {
		return index
	}
	index := newPythonBlobIndex(blob)
	cache.pythonBlobs[blobOID] = index
	return index
}

func (cache *blobIndexes) jsIndex(blobOID string, blob []byte) *jsBlobIndex {
	if index, found := cache.jsBlobs[blobOID]; found {
		return index
	}
	index := newJSBlobIndex(blob)
	cache.jsBlobs[blobOID] = index
	return index
}

// The byte classes the hand-walked site scans share with Go's regexp package.

func skipSpace(clean []byte, at int) int {
	for at < len(clean) && isSpaceByte(clean[at]) {
		at++
	}
	return at
}

// isSpaceByte is regexp's \s: [\t\n\f\r ].
func isSpaceByte(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\f' || value == '\r'
}

// isWordByte is regexp's \w, which is also the \b alphabet: [0-9A-Za-z_].
func isWordByte(value byte) bool {
	return value == '_' || value >= '0' && value <= '9' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}
