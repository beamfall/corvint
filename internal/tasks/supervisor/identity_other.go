//go:build !linux && !(darwin && (arm64 || amd64))

package supervisor

import "fmt"

func ProcessIdentity(pid int) (string, error) {
	return "", fmt.Errorf("supervised profile unsupported on this platform")
}
