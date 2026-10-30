//go:build !darwin && !freebsd && !linux && !windows

package worktreeimpact

import (
	"fmt"
	"os"
)

// Working-tree impact reads are qualified only where a contained no-follow open
// and a link-count identity are implemented; elsewhere every target is refused
// (PRS-V1-004, V1-0393).

type platformFileIdentity struct{}

func openTarget(*os.Root, string) (*os.File, error) {
	return nil, fmt.Errorf("working-tree target reads are not qualified on this platform")
}

func identityFrom(os.FileInfo) (platformFileIdentity, error) {
	return platformFileIdentity{}, fmt.Errorf("working-tree target link identity is not qualified on this platform")
}

func (platformFileIdentity) preimage(os.FileMode, int64) string { return "unqualified" }
