//go:build darwin

package trace

import (
	"os"
	"syscall"
)

func sameMetadata(left, right os.FileInfo) bool {
	leftStat, leftOK := left.Sys().(*syscall.Stat_t)
	rightStat, rightOK := right.Sys().(*syscall.Stat_t)
	return leftOK && rightOK && os.SameFile(left, right) && left.Size() == right.Size() &&
		left.Mode() == right.Mode() && left.ModTime().Equal(right.ModTime()) && leftStat.Ctimespec == rightStat.Ctimespec
}
