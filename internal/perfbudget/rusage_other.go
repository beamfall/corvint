//go:build !unix

package perfbudget

import "os"

const rssUnknownReason = "peak resident set NOT_OBSERVED: this host has no getrusage"

func peakRSS(*os.ProcessState) *int64 { return nil }
