// Package authority supplies the explicit, mutation-only filesystem
// primitives of SPEC §3.4, §5.1 and §5.2 for the repository authority that
// internal/intent resolves: the bounded exclusive flock on
// `<git-common-dir>/taskman.lock`, filesystem qualification by actual OS
// observation, descriptor-based durable sync, and the exclusive
// link-in creation primitive used for `receipts/<seq>.json`, `.boot` and
// `.ack`.
//
// Nothing here is for reading. Every entry point opens for writing,
// creates, locks or syncs; a read verb (TM-V0-008) never calls this
// package. Repository identity comes only from intent.Resolve; this package
// invents no second notion of the common dir, the state dir or the lock.
//
// What a success here does NOT prove (see Prerequisites): a qualified local
// filesystem is not a caller authentication, a held lock is not an actor
// binding or ownership proof, and no capacity or escrow check exists. The
// §5.2 transaction writer is a separate slice; this package publishes single
// files exclusively and never sequences a transaction.
package authority

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"os"
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Frozen values (SPEC §1 lock wait; §3.4 lock location).
const (
	// LockFileName is the flock file name under the Git common dir (§3.4).
	LockFileName = "taskman.lock"
	// DefaultLockWait is the §1 lock wait: 30 s.
	DefaultLockWait = 30 * time.Second
	// MaxLockWait bounds every caller-supplied wait to the §1 value; a
	// larger request is clamped, never honoured.
	MaxLockWait = 30 * time.Second
	// DefaultLockPoll is the interval between non-blocking flock attempts.
	DefaultLockPoll = 10 * time.Millisecond
	// MaxLinkInBytes bounds one link-in publication: the largest §1 file the
	// primitive can be asked to publish is an evidence blob (64 MiB).
	MaxLinkInBytes = 64 * wire.MiB
	// probePrefix names the unique temporary file a qualification probe
	// creates under the caller's root. It is removed on every exit.
	probePrefix = ".taskman-qualify-"
)

// ErrExists is returned by Dir.LinkIn when the destination already exists.
// The link-in wrote nothing and removed its temp file. The caller maps it:
// a receipt destination is JOURNAL_FORKED (§5.2); a `.boot`/`.ack` means
// another party won (§6.4). It is deliberately not a §11 code.
var ErrExists = errors.New("authority: link-in destination already exists")

// Filesystem is what the OS reported for a directory (§5.1).
type Filesystem struct {
	// Platform is the GOOS the observation was made on.
	Platform string

	// Type is the filesystem type name: Darwin `f_fstypename`; Linux the
	// name of a recognised `f_type` magic, or `magic:0x...` when the magic
	// is not one this package knows. On Linux ext2, ext3 and ext4 share
	// one magic (0xEF53) and are reported as `ext4`.
	Type string

	// Local is true when the OS reported the mount as local (Darwin
	// MNT_LOCAL; Linux: a recognised local magic). False means not
	// observed as local, never "remote for sure".
	Local bool
}

// allowed is the closed §5.1 allowlist keyed by platform.
var allowed = map[string]map[string]bool{
	"darwin": {"apfs": true, "hfs": true},
	"linux":  {"ext4": true, "xfs": true, "btrfs": true, "tmpfs": true},
}

// Classify reports whether a (platform, filesystem type) pair is in the
// §5.1 allowlist: Darwin `apfs`, `hfs`; Linux `ext4`, `xfs`, `btrfs`,
// `tmpfs`. Anything else, on any other platform, is refused. It is a pure
// function over strings so the table can be tested without a mount.
func Classify(platform, fstype string) bool {
	return allowed[platform][fstype]
}

// Prerequisite is an integration requirement this package does not satisfy
// and does not stub. A caller that needs the guarantee must obtain it from
// the named owner; nothing here returns a placeholder success for it.
type Prerequisite struct {
	ID        string
	Owner     string
	Statement string
}

// Prerequisites lists what a successful lock, qualification or link-in does
// not establish.
var Prerequisites = []Prerequisite{
	{
		ID:    "AUTH-ACTOR-BINDING",
		Owner: "TCP-02 journal writer / TCP-04 runtime (independently qualified invocation)",
		Statement: "Trusted actor binding (OWNER, WORKER, SYSTEM) comes from an independently qualified " +
			"operator or runtime invocation, never from a mutation envelope, a claimed string, a " +
			"registration record or a same-UID check. A held taskman.lock proves mutual exclusion " +
			"only; this package mints no actor.",
	},
	{
		ID:    "AUTH-CAPACITY-ESCROW",
		Owner: "TCP-02 writer prerequisite frozen separately",
		Statement: "Whole-store archive capacity plus terminal/recovery/admin escrow is checked by the " +
			"transaction writer before any receipt is published; nothing here observes or enforces it.",
	},
	{
		ID:    "AUTH-TRANSACTION-WRITER",
		Owner: "internal/journal (TCP-02)",
		Statement: "The §5.2 sequence (barrier, chain and bounds checks, redo, temp cleanup, evidence " +
			"blobs, receipt link-in, post-file rename, head rename, release) is not implemented here. " +
			"Dir.LinkIn publishes exactly one file exclusively; Fsync/Dir.Sync are single-descriptor " +
			"operations. No rename-over helper exists in this package.",
	},
	{
		ID:    "AUTH-DARWIN-FULLSYNC-SOURCE",
		Owner: "verification run (make verify) and reviewer",
		Statement: "Darwin fullSync issues explicit F_FULLFSYNC under RawConn.Control, retries EINTR only, " +
			"and never calls File.Sync or downgrades ENOTSUP. Directory sync is separate fsync. " +
			"Go 1.27.0 internal/poll/fd_fsync_darwin.go falls back to fsync and is deliberately bypassed. " +
			"Syscall failure tests do not qualify physical-media durability.",
	},
}

// uniqueName returns prefix + pid + "-" + 16 hex chars + suffix. The random
// part makes the name unique across threads and processes sharing a pid
// namespace; O_EXCL creation is still the only proof of uniqueness.
func uniqueName(prefix, suffix string) (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", wire.Errorf(wire.CodeUnsupportedFilesystem, "", "cannot draw a random temp name: %v", err)
	}
	return prefix + strconv.Itoa(os.Getpid()) + "-" + hex.EncodeToString(b[:]) + suffix, nil
}

// withCleanup attaches a cleanup failure to a primary error so neither is
// lost: a *wire.Error keeps its code and gains the cleanup text; any other
// error is joined.
func withCleanup(primary, cleanup error) error {
	if cleanup == nil {
		return primary
	}
	if primary == nil {
		return cleanup
	}
	if e, ok := primary.(*wire.Error); ok {
		return &wire.Error{Code: e.Code, Where: e.Where, Msg: e.Msg + "; cleanup also failed: " + cleanup.Error()}
	}
	return errors.Join(primary, cleanup)
}

// fsErr is the storage failure code of this repository: §11 has no
// STORAGE_FAILED, and every existing package reports I/O failures under
// UNSUPPORTED_FILESYSTEM.
func fsErr(where, format string, args ...interface{}) *wire.Error {
	return wire.Errorf(wire.CodeUnsupportedFilesystem, where, format, args...)
}

// Private seam for pre-effect refusal tests; production follows the build target.
var supportedPlatform = safeopen.Supported

// Internal OS seams for controlled pre-open replacement witnesses.
var openRoot = safeopen.Root
var openSubRoot = safeopen.SubRoot
