// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin

package intent

import "os"

// changeTime reports no change time where the index parse is not reused, so
// every read parses the file (TCX-V0-849).
func changeTime(os.FileInfo) (int64, bool) { return 0, false }
