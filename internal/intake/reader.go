package intake

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/gitstatus"
)

// ReaderBoundary is a preflight observation only. The host must enforce read-only
// raw/repository mounts, no network/credentials, and one exclusively writable output.
// The candidate is still untrusted until BuildAuthorInput validates it.
type ReaderBoundary struct {
	AuthorRoot   string `json:"author_root"`
	RawInput     string `json:"raw_input"`
	IntakeOutput string `json:"intake_output"`
}

func PreflightReader(ctx context.Context, b ReaderBoundary) error {
	author, err := filepath.EvalSymlinks(b.AuthorRoot)
	if err != nil {
		return ErrBoundary
	}
	author, err = filepath.Abs(author)
	if err != nil {
		return ErrBoundary
	}
	gitdir, common, err := gitauth.WorktreeDirectories(author)
	if err != nil {
		return ErrBoundary
	}
	raw, err := filepath.Abs(b.RawInput)
	if err != nil {
		return ErrBoundary
	}
	rawInfo, err := os.Lstat(raw)
	if err != nil || !rawInfo.Mode().IsRegular() || !singleLink(rawInfo) {
		return ErrBoundary
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(raw))
	if err != nil {
		return ErrBoundary
	}
	if gitstatus.ScratchOutside(ctx, parent, author, gitdir, common) != nil {
		return ErrBoundary
	}
	out, err := filepath.Abs(b.IntakeOutput)
	if err != nil || filepath.Base(out) == "." {
		return ErrBoundary
	}
	outputParent, err := filepath.EvalSymlinks(filepath.Dir(out))
	if err != nil {
		return ErrBoundary
	}
	if gitstatus.ScratchOutside(ctx, outputParent, author, gitdir, common) != nil {
		return ErrBoundary
	}
	// A reader gets one fresh file, never overwrite authority, source or prior output.
	out = filepath.Join(outputParent, filepath.Base(out))
	if _, err := os.Lstat(out); !os.IsNotExist(err) {
		return ErrBoundary
	}
	return nil
}
