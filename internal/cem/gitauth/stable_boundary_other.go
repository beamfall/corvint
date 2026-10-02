//go:build !unix

package gitauth

import (
	"errors"
	"os"
)

// The no-follow directory descriptor is unavailable here, so OpenStable
// refuses every root on this platform.
func openDirNoFollow(string) (*os.File, error) { return nil, errors.ErrUnsupported }
