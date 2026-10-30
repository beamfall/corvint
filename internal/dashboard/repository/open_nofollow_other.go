//go:build !darwin && !linux && !windows

package repository

import "os"

const repositoryExecutionSupported = false

func openNoFollowFile(*os.Root, string) (*os.File, bool) { return nil, false }
func openNoFollowDirectory(*os.Root, string) (*os.File, *os.Root, bool) {
	return nil, nil, false
}
