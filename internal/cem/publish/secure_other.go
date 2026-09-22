//go:build !darwin && !linux

package publish

import (
	"context"
	"errors"
	"os"
)

func openSecureDirectory(*os.Root, string) (*os.File, *os.Root, error) {
	return nil, nil, errors.New("secure publication is unsupported on this platform")
}

func openSecureMember(*os.Root, string, int, os.FileMode) (*os.File, error) {
	return nil, errors.New("secure publication is unsupported on this platform")
}

func lockSecureDirectory(context.Context, *os.File) error {
	return errors.New("secure publication is unsupported on this platform")
}

func unlockSecureDirectory(*os.File) {}
