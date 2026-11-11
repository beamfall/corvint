// SPDX-License-Identifier: Apache-2.0
package testvaliditydoc

import (
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/pinnedread"
)

// The receipt reader is the shared pinned no-follow reader
// (internal/pinnedread); these names keep this package's frozen surface and
// its callers unchanged. The values are the same, so errors.Is matches either.
var (
	// ErrInputTooLarge identifies a receipt that exceeds the input byte bound.
	ErrInputTooLarge = pinnedread.ErrInputTooLarge
	// ErrPlatformUnsupported is the typed refusal of every receipt file and
	// directory open on a platform without the qualified no-follow open boundary
	// (darwin and linux only). windows/amd64 is UNSUPPORTED for 1.0 and must
	// fail closed with this reason rather than a partial or wrong read
	// (PRS-V1-021, PRS-V1-022, decision 0559, V1-2484).
	ErrPlatformUnsupported = pinnedread.ErrPlatformUnsupported
)

// PlatformUnsupportedCode is the typed code every receipt-read caller outside
// the docs corpus reports for ErrPlatformUnsupported, in place of its own
// invalid-receipt or unreadable code (PRS-V1-920, V1-2683).
const PlatformUnsupportedCode = pinnedread.PlatformUnsupportedCode

// PlatformRefusal returns ErrPlatformUnsupported where receipt reads are
// refused and nil where they are qualified, so a caller can refuse before it
// opens anything.
func PlatformRefusal() error { return pinnedread.PlatformRefusal() }

// ReadFile reads a resolved, root-relative receipt without following symlinks,
// bounded by MaxInputBytes.
func ReadFile(root *os.Root, name string) ([]byte, error) { return pinnedread.ReadFile(root, name) }

// ReadAbsoluteFileBounded is ReadFileBounded for a clean absolute path; / is
// opened for reading only for a file directly in it (V1-0883).
func ReadAbsoluteFileBounded(name string, limit int64) ([]byte, error) {
	return pinnedread.ReadAbsoluteFileBounded(name, limit)
}

// ReadFileBounded retains the no-follow receipt reader while allowing an explicit
// corpus-sized bound. Receipt callers keep ReadFile and its original 4 MiB limit.
func ReadFileBounded(root *os.Root, name string, limit int64) ([]byte, error) {
	return pinnedread.ReadFileBounded(root, name, limit)
}

// StreamFileBounded is ReadFileBounded without holding the file: read(r)
// consumes the opened file through r, which ends after limit+1 bytes.
func StreamFileBounded(root *os.Root, name string, limit int64, read func(r io.Reader) error) error {
	return pinnedread.StreamFileBounded(root, name, limit, read)
}

// StatFile is ReadFileBounded without the read: the lstat record of the leaf,
// which must be a regular file, after every directory component is pinned.
func StatFile(root *os.Root, name string) (os.FileInfo, error) {
	return pinnedread.StatFile(root, name)
}

func openReceiptDirectory(parent *os.Root, name string) (*os.Root, error) {
	return pinnedread.OpenDirectory(parent, name)
}

func readRegular(parent *os.Root, leaf string, before os.FileInfo) ([]byte, error) {
	return pinnedread.ReadRegular(parent, leaf, before)
}
