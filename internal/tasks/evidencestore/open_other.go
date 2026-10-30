//go:build !unix

package evidencestore

// openNonblock is zero where a named pipe cannot replace a stored file.
const openNonblock = 0
