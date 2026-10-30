// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package corpusbridge

import "time"

// cpuTime reports zero where getrusage is unavailable; wall time stays measured.
func cpuTime() time.Duration { return 0 }
