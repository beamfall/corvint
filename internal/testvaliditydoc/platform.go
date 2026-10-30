package testvaliditydoc

import "errors"

// ErrPlatformUnsupported is the typed refusal of every receipt file and
// directory open on a platform without the qualified no-follow open boundary
// (read_unix.go builds only on darwin and linux). windows/amd64 is UNSUPPORTED
// for 1.0 and must fail closed with this reason rather than a partial or wrong
// read (PRS-V1-021, PRS-V1-022, decision 0559, V1-2484).
var ErrPlatformUnsupported = errors.New("receipt reads are unsupported on this platform")

// PlatformRefusal returns ErrPlatformUnsupported where receipt reads are
// refused and nil where they are qualified, so a caller can refuse before it
// opens anything.
func PlatformRefusal() error { return platformRefusal }
