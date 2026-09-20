//go:build !darwin && !(linux && (amd64 || arm64))

package releasecandidate

import "fmt"

func promoteNoReplace(_, _, _ string) error {
	return fmt.Errorf("atomic no-replace promotion is unsupported on this platform")
}
