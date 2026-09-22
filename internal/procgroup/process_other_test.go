//go:build !darwin && !linux

package procgroup

import "errors"

// killTestProcessGroup has no equivalent on this platform: process_other.go
// never creates an OS process group to signal (see processExists and
// TestRunProcessNonPOSIXContract).
func killTestProcessGroup(int) error {
	return errors.New("process-group signal unsupported on this platform")
}
