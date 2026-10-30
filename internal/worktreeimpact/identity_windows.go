//go:build windows

package worktreeimpact

import (
	"fmt"
	"os"
)

type platformFileIdentity struct{}

func identityFrom(os.FileInfo) (platformFileIdentity, error) {
	return platformFileIdentity{}, fmt.Errorf("working-tree target link identity is not qualified on Windows")
}

func (platformFileIdentity) preimage(os.FileMode, int64) string { return "unqualified" }
