//go:build !unix

package main

import (
	"errors"
	"os"
)

func workOpenManifest(root *os.Root, name string) (*os.File, error) {
	return nil, errors.New("unsupported mutation platform")
}
