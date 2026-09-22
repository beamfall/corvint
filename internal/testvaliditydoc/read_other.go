//go:build !darwin && !linux

package testvaliditydoc

import (
	"errors"
	"os"
)

func openReceiptFile(parent *os.Root, name string) (*os.File, error) {
	return nil, errors.New("unsupported receipt platform")
}

func openReceiptDirectory(parent *os.Root, name string) (*os.Root, error) {
	return nil, errors.New("unsupported receipt platform")
}
