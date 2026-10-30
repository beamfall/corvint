// SPDX-License-Identifier: Apache-2.0

//go:build !darwin && !linux

package corpusbridge

import (
	"io/fs"
	"os"
)

// fileTimes reports no change time where the stamp does not read one, so the
// stamp is unavailable and every call observes and verifies through Git.
func fileTimes(fs.FileInfo) (uint64, int64, int64, bool) { return 0, 0, 0, false }

// openStampFile is never reached here, since no checkout can be stamped.
func openStampFile(string) (*os.File, error) { return nil, errStampUnavailable }
