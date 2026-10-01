package stepverify

import (
	"fmt"
	"os"
	"syscall"
)

func nativeInfo(info os.FileInfo) (string, string, uint64, bool) {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", "", 0, false
	}
	identity := hash([]byte(fmt.Sprintf("%d:%d:%d:%d", s.Dev, s.Ino, s.Uid, s.Gid)))
	stamp := fmt.Sprintf("%s:%d:%d:%d:%d:%d:%d", identity, info.Mode(), info.Size(), info.ModTime().UnixNano(), s.Ctimespec.Sec, s.Ctimespec.Nsec, s.Nlink)
	return identity, stamp, uint64(s.Nlink), true
}
