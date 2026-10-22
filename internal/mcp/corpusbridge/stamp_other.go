// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !darwin && !linux

package corpusbridge

import "io/fs"

// fileTimes reports no change time where the stamp does not read one, so the
// stamp is unavailable and every call observes and verifies through Git.
func fileTimes(fs.FileInfo) (uint64, int64, int64, bool) { return 0, 0, 0, false }
