//go:build !darwin && !linux

package main

import (
	"fmt"
	"os"
)

func renameRootNoReplace(_ *os.Root, _, _ string) error {
	return fmt.Errorf("atomic no-replace directory promotion is unsupported on this platform")
}
