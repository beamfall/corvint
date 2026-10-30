//go:build !darwin && !linux

package delta

import "os"

// No platform capture is qualified here; never fall back to a potentially blocking open.
func openRegular(string) (*os.File, error) { return nil, errInput }
