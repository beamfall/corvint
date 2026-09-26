package authority

import (
	"fmt"
	"slices"
	"strings"
)

// Linux `f_type` magics (include/uapi/linux/magic.h). ext2, ext3 and ext4
// share EXT4_SUPER_MAGIC; statfs cannot tell them apart.
const (
	magicExt4  = 0xEF53
	magicXFS   = 0x58465342
	magicBtrfs = 0x9123683E
	magicTmpfs = 0x01021994
)

var magicNames = map[uint32]string{
	magicExt4:  "ext2/ext3/ext4",
	magicXFS:   "xfs",
	magicBtrfs: "btrfs",
	magicTmpfs: "tmpfs",
}

// extTypes are the mountinfo fstypes that carry the shared ext magic.
var extTypes = map[string]bool{"ext2": true, "ext3": true, "ext4": true}

func filesystemFromMagic(magic uint32) Filesystem {
	if name, ok := magicNames[magic]; ok {
		return Filesystem{Platform: "linux", Type: name, Local: magic != magicExt4}
	}
	return Filesystem{Platform: "linux", Type: fmt.Sprintf("magic:0x%x", magic), Local: false}
}

// extFromMountinfo resolves the shared ext magic from the kernel's record of
// one mount: the single /proc/self/mountinfo line whose mount ID is id names
// the fstype after its "-" separator (proc(5)). A missing, repeated or
// malformed line, or an fstype that cannot carry the ext magic, stays
// ambiguous and is refused.
func extFromMountinfo(raw []byte, id string) Filesystem {
	ambiguous := filesystemFromMagic(magicExt4)
	fstype := ""
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != id {
			continue
		}
		sep := slices.Index(fields, "-")
		if fstype != "" || sep < 6 || sep+1 >= len(fields) {
			return ambiguous
		}
		fstype = fields[sep+1]
	}
	if !extTypes[fstype] {
		return ambiguous
	}
	return Filesystem{Platform: "linux", Type: fstype, Local: true}
}
