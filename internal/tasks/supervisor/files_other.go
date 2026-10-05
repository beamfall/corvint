//go:build !darwin && !linux

package supervisor

import (
	"fmt"
	"os"
)

func openExecutable(string) (*os.File, error) {
	return nil, fmt.Errorf("pinned runtime check unsupported")
}

func DirectoryIdentity(string) (string, error) {
	return "", fmt.Errorf("directory identity unsupported")
}
