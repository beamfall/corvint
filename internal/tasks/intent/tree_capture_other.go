// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package intent

import (
	"os"
	"time"
)

func sameCaptureFile(a, b os.FileInfo, capturedAt time.Time) bool { return false }
