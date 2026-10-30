//go:build darwin || linux

package intake

import (
	"os"
	"syscall"
)

// A multiply linked raw leaf can alias authoring or Git authority bytes even
// when its named parent lies outside the checkout.
func singleLink(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Nlink == 1
}
