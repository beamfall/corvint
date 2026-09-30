//go:build !(darwin || linux)

package postmergeconnector

import (
	"fmt"
	"os"
)

func openRegular(string, os.FileInfo) (*os.File, error) {
	return nil, fmt.Errorf("file-transport-platform-unsupported")
}
func singleLink(os.FileInfo) bool { return false }
