//go:build windows

package worktreeimpact

import "os"

func openTarget(root *os.Root, value string) (*os.File, error) {
	return root.Open(value)
}
