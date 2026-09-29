//go:build !darwin && !linux

package supervisor

import "fmt"

func DirectoryIdentity(string) (string, error) {
	return "", fmt.Errorf("directory identity unsupported")
}
