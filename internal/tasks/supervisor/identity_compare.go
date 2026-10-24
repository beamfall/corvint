package supervisor

import (
	"strconv"
	"strings"
)

// IdentityMatch is how a fresh start identity relates to a recorded one.
type IdentityMatch int

const (
	// IdentityDifferent names another process, or no process.
	IdentityDifferent IdentityMatch = iota
	// IdentitySame names the recorded process.
	IdentitySame
	// IdentityUnknown is a `linux:` pair whose start ticks match but whose
	// btimes are more than one second apart.
	IdentityUnknown
)

// CompareIdentity compares fresh, a ProcessIdentity read now for the pid a
// record names, with recorded, the identity that record holds, which another
// process may have written (PSR-V0-204, PSR-V0-227, V1-1091). Equal non-empty
// identities are the same process. On Linux the kernel recomputes btime on
// every read, so two `linux:<btime>:<starttime>` identities whose start ticks
// match exactly and whose btimes are at most one second apart are the same
// process too: a PID reused within one boot has another start tick. Equal
// start ticks with btimes further apart are unknown: a realtime clock step
// moves btime by any amount while the process lives, and the identity
// carries no stable boot identity to tell that from a reboot. Anything else,
// including an empty identity, is different. The caller must have read fresh
// for the recorded pid; the identity alone never names a pid.
func CompareIdentity(fresh, recorded string) IdentityMatch {
	if fresh == "" || recorded == "" {
		return IdentityDifferent
	}
	if fresh == recorded {
		return IdentitySame
	}
	fb, fs, ok1 := SplitLinuxIdentity(fresh)
	rb, rs, ok2 := SplitLinuxIdentity(recorded)
	if !ok1 || !ok2 || fs != rs {
		return IdentityDifferent
	}
	if fb == rb || fb == rb+1 || rb == fb+1 {
		return IdentitySame
	}
	return IdentityUnknown
}

// SameProcess reports whether fresh, read now for the pid a record names,
// names the process recorded as recorded (CompareIdentity's IdentitySame).
// An unknown pair is not the same process, exactly as before the btime
// tolerance (PSR-V0-227).
func SameProcess(fresh, recorded string) bool {
	return CompareIdentity(fresh, recorded) == IdentitySame
}

// SplitLinuxIdentity parses `linux:<btime>:<starttime>` into its btime and
// its exact start tick text.
func SplitLinuxIdentity(s string) (boot uint64, start string, ok bool) {
	rest, found := strings.CutPrefix(s, "linux:")
	if !found {
		return 0, "", false
	}
	b, start, found := strings.Cut(rest, ":")
	if !found || start == "" {
		return 0, "", false
	}
	boot, e := strconv.ParseUint(b, 10, 64)
	if e != nil || boot == 0 || boot == ^uint64(0) {
		return 0, "", false
	}
	if _, e := strconv.ParseUint(start, 10, 64); e != nil {
		return 0, "", false
	}
	return boot, start, true
}
