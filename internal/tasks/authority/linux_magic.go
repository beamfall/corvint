package authority

import "fmt"

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

func filesystemFromMagic(magic uint32) Filesystem {
	if name, ok := magicNames[magic]; ok {
		return Filesystem{Platform: "linux", Type: name, Local: magic != magicExt4}
	}
	return Filesystem{Platform: "linux", Type: fmt.Sprintf("magic:0x%x", magic), Local: false}
}
