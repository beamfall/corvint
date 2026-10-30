package testplatform

import (
	"errors"
	"runtime"
	"strings"
	"testing"
)

// PlatformRefusal is one deliberate production refusal of an unsupported
// platform: the exact text the production error carries and the file that
// produces it (V1-2486).
type PlatformRefusal struct {
	Message string
	Source  string
}

// PlatformRefusals lists the production refusals a native Windows test run may
// report. Each message is copied verbatim from its source, and
// TestPlatformRefusalsMatchTheirSources keeps the two in step, so a reworded
// refusal stops matching instead of silently skipping something else.
var PlatformRefusals = []PlatformRefusal{
	{"process-platform-unsupported", "internal/procgroup/process.go"},
	{"process execution with descendant containment is unsupported on this platform", "internal/procgroup/process_other.go"},
	{"unreaped process observation is unsupported on this platform", "internal/procgroup/process_other.go"},
	{"owned process-group cleanup is unsupported on this platform", "internal/procgroup/process_other.go"},
	{"groupreap: owned process groups are unavailable on this platform", "internal/groupreap/owner.go"},
	{"cannot be read without no-follow support on this platform", "internal/gitstatus/read_other.go"},
	{"worker platform unsupported", "internal/gitstatus/worker_other.go"},
	{"platform has no qualified safe opening boundary", "internal/tasks/safeopen/open_other.go"},
	{"Corvint Tasks is unsupported on this platform", "internal/tasks/wire/platform_other.go"},
	{"secure publication is unsupported on this platform", "internal/cem/publish/secure_other.go"},
	{"bounded input acquisition is unsupported on this platform", "internal/cem/publish/read_unsupported.go"},
	{"serialized CEM map updates are unsupported on this platform", "internal/cem/publish/lock_other.go"},
	{"stable platform unsupported", "internal/cem/cli/stable_other.go"},
	{"nonblocking confined shard reads unsupported on this platform", "internal/contextindex/blob_shards_open_unsupported.go"},
	{"confined shard publication unsupported on this platform", "internal/contextindex/blob_shards_open_unsupported.go"},
	{"git index stat routing is unsupported on this platform", "internal/contextindex/index_stat_other.go"},
	{"file-transport-platform-unsupported", "internal/postmergeconnector/files_other.go"},
	{"unsupported executable platform", "internal/analyzerexec/platform_other.go"},
	{"receipt reads are unsupported on this platform", "internal/testvaliditydoc/platform.go"},
	{"docs corpus reads are unsupported on this platform", "internal/doccorpus/encoding.go"},
}

// SkipWindowsRefusal skips t when, and only when, the host is Windows and err
// is one of the PlatformRefusals: some error in its chain reads exactly as the
// refusal, or ends with it after a "context: " separator. Refusal text
// anywhere else, such as inside a path or in front of an unrelated cause,
// does not qualify. The skip reason quotes the production
// refusal. On darwin, linux and every other platform it does nothing, so a
// supported platform never loses the test; the caller still fails on err.
func SkipWindowsRefusal(t testing.TB, err error) {
	t.Helper()
	if reason, skip := windowsRefusalSkip(runtime.GOOS, err); skip {
		t.Skip(reason)
	}
}

func windowsRefusalSkip(goos string, err error) (string, bool) {
	if goos != "windows" || err == nil {
		return "", false
	}
	for _, refusal := range PlatformRefusals {
		if isRefusal(err, refusal.Message) {
			return "production refuses this platform (" + refusal.Source + "): " + refusal.Message, true
		}
	}
	return "", false
}

// isRefusal reports whether err, or an error it wraps, is exactly message or
// ends with ": "+message.
func isRefusal(err error, message string) bool {
	if err == nil {
		return false
	}
	if text := err.Error(); text == message || strings.HasSuffix(text, ": "+message) {
		return true
	}
	switch wrapped := err.(type) {
	case interface{ Unwrap() []error }:
		for _, inner := range wrapped.Unwrap() {
			if isRefusal(inner, message) {
				return true
			}
		}
		return false
	default:
		return isRefusal(errors.Unwrap(err), message)
	}
}
