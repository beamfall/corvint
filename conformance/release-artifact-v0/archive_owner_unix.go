//go:build unix

package main

import (
	"fmt"
	"os"
	"syscall"
)

func invokingUserOwns(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) {
		return fmt.Errorf("directory is not owned by the invoking user")
	}
	return nil
}
