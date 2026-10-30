//go:build darwin || freebsd

package worktreeimpact

import (
	"fmt"
	"os"
	"syscall"
)

type platformFileIdentity struct {
	device, inode, links uint64
	changeSeconds        int64
	changeNanoseconds    int64
}

func identityFrom(info os.FileInfo) (platformFileIdentity, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 {
		return platformFileIdentity{}, fmt.Errorf("working-tree target must have exactly one hard link")
	}
	return platformFileIdentity{
		device: uint64(stat.Dev), inode: uint64(stat.Ino), links: uint64(stat.Nlink),
		changeSeconds: stat.Ctimespec.Sec, changeNanoseconds: stat.Ctimespec.Nsec,
	}, nil
}

func (identity platformFileIdentity) preimage(mode os.FileMode, size int64) string {
	return fmt.Sprintf("dev=%d\x00ino=%d\x00links=%d\x00ctime=%d.%09d\x00mode=%d\x00size=%d", identity.device, identity.inode, identity.links, identity.changeSeconds, identity.changeNanoseconds, uint32(mode), size)
}
