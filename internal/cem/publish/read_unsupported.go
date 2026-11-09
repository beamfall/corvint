//go:build !darwin && !linux && !windows

package publish

import (
	"errors"
	"os"
)

// Refuse rather than weaken descriptor-safe acquisition on unqualified platforms.
func openBoundedInput(string) (*os.File, error) {
	return nil, errors.New("bounded input acquisition is unsupported on this platform")
}

func openBoundedMember(*os.Root, string) (*os.File, error) {
	return nil, errors.New("bounded input acquisition is unsupported on this platform")
}
