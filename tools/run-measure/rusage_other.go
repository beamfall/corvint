//go:build !unix

package main

import "os"

// peakRSSKiB has no portable source outside Unix; the report says unknown.
func peakRSSKiB(*os.ProcessState) (int64, bool) { return 0, false }

func signalNumber(*os.ProcessState) int { return 0 }
