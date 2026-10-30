//go:build linux && !amd64 && !arm64

package main

import (
	"fmt"
	"os"
)

func renameRootNoReplace(_ *os.Root, _, _ string) error {
	return fmt.Errorf("atomic no-replace directory promotion is unsupported on this Linux architecture")
}
