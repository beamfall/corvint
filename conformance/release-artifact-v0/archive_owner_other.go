//go:build !unix

package main

import (
	"errors"
	"os"
)

func invokingUserOwns(info os.FileInfo) error {
	return errors.New("owner verification is unsupported on this platform")
}
