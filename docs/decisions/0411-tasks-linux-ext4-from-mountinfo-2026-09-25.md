# Decision 0411 — corvint-tasks qualifies Linux ext4 from the mount's own record

Date: 2026-09-25. Status: accepted. Authority: repository owner instruction "qualify ext4 via
mountinfo" (2026-09-25). Governs `internal/tasks/authority` until the ATCP-V0 specification is
recovered (V1-0310, decision 0397).

The §5.1 filesystem allowlist names Linux `ext4`. However, `fstatfs(2)` reports one magic (`0xEF53`)
for ext2, ext3 and ext4, and the store refused every mount carrying that magic as ambiguous.
`corvint-tasks init` therefore refused on stock Linux, where the root and home filesystems are
ext4. The Linux CI runner's temp directory is also ext4, so every store-backed test failed there
with `UNSUPPORTED_FILESYSTEM`.

1. **Observation.** For the shared magic only, the store reads the pinned descriptor's own mount:
   - `mnt_id` from `/proc/self/fdinfo/<fd>`, bounded to 4 KiB;
   - then the single `/proc/self/mountinfo` line with that mount ID, bounded to 4 MiB, and its
     fstype after the `-` separator (proc(5)).

   `ext4` is local and allowed. `ext2` and `ext3` are reported by name and refused by the unchanged
   allowlist. Any other outcome keeps the ambiguous `ext2/ext3/ext4` type, which is refused:
   - a missing, repeated or malformed line;
   - an fstype that cannot carry the magic;
   - an oversized or unreadable procfs file.

   No mount-point name or path guess is ever used. Both call sites use the same observation: the
   §5.1 qualification probe (`observeFilesystem`) and the fixture mount observation.
2. **What it proves.** The fstype is the kernel driver that mounted the filesystem. An ext2- or
   ext3-format volume mounted by the ext4 driver is reported as `ext4`. It is then served with
   ext4's fsync semantics, which are what §5.2 durability relies on.
3. **Tests.**
   - The `/dev/shm` temp-directory relocation added for CI in c4fa9a45 is removed, so the Linux
     suite runs on the runner's ext4.
   - `TestTMV0010_AS10_ExtFromMountinfo` covers the parser on every platform.
   - `TestTMV0010_AS10_ExtMagicResolvedFromOwnMount` observes a real ext mount on Linux, and skips
     when the temp dir is on another filesystem.

**Rollback.** Revert this change. The shared magic is then refused again, and Linux CI needs the
tmpfs relocation back.
