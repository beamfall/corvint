// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package main

import "time"

// rusage reports zero CPU time where getrusage is unavailable; wall time and
// allocation counts remain measured.
func rusage() [2]time.Duration { return [2]time.Duration{} }
