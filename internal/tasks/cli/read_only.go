package cli

import (
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ReadOnlyEnv is the documented read-only session signal (ATR-V0-212). A
// review recipe sets it on the reviewer process, for example
// `CORVINT_TASKS_READ_ONLY=1 codex exec -s read-only ...`; every child
// inherits it, including a command the host escalates out of its sandbox,
// which a sandbox marker such as CODEX_SANDBOX does not reach (V1-1237,
// V1-1441).
const ReadOnlyEnv = "CORVINT_TASKS_READ_ONLY"

// readOnlyVerbs are the pure reads a read-only session may run: they take no
// store lock and write nothing under the store (TM-V0-008).
var readOnlyVerbs = map[string]bool{
	"version": true, "--version": true, "roadmap": true, "critical-path": true, "pending": true, "doctor": true,
	"queue status": true, "queue snapshot": true, "program show": true, "attempt show": true, "plan preview": true, "plan batches": true,
	"policy show": true, "pool status": true, "dispatch status": true, "defects list": true, "source contains": true,
	"receipt audit": true, "receipt get": true, "receipt path": true, "archive verify": true, "gate list": true, "gate show": true, "gate history": true,
	"release list": true, "release show": true, "release readiness": true,
	"ticket list": true, "ticket search": true, "ticket show": true, "ticket blockers": true, "ticket closeout-debt": true, "ticket effects-proposals": true, "ticket export": true, "ticket similar": true, "ticket digest": true,
	"ticket note show": true, "ticket note history": true, "ticket know-how list": true,
	"ticket escalation list": true, "ticket escalation show": true, "ticket escalation history": true,
	"work start": true, "work deliver": true, "work block": true,
	"ticket obligations show": true, "ticket obligations plan": true, "ticket obligations boundary": true, "ticket obligations worklist": true,
}

// workFlagSpec is a work verb's flag set, shared by the verb and the
// read-only dry-run check so both parse the same arguments the same way.
type workFlagSpec struct{ single, repeat, bools []string }

func (w workFlagSpec) parse(cmd, args []string) (map[string][]string, map[string]bool, *wire.Result) {
	return listFlags(cmd, args, w.single, w.repeat, w.bools)
}

// dryRunVerbs are the work verbs with a --dry-run that writes nothing
// (TAW-V0-016); work finding has none.
var dryRunVerbs = map[string]*workFlagSpec{"work close-merged": &closeMergedFlags, "work release-sync": &releaseSyncFlags}

// readOnlyRefusal refuses, before any store read or lock, every command
// that is not a listed pure read while ReadOnlyEnv is set to a nonempty
// value other than "0" (ATR-V0-212). doctor --refresh writes its cache and
// is refused too.
func readOnlyRefusal(args []string) *wire.Result {
	if v := os.Getenv(ReadOnlyEnv); v == "" || v == "0" {
		return nil
	}
	verb := verbPath(args)
	if readOnlyVerbs[verb] && !(verb == "doctor" && hasArg(args, "--refresh")) {
		return nil
	}
	if spec, ok := dryRunVerbs[verb]; ok && len(args) >= 2 {
		// Only a --dry-run the verb itself parses as its boolean flag writes
		// nothing; a flag value spelled --dry-run does not count (TAW-V0-016).
		if _, set, res := spec.parse(strings.Fields(verb), args[2:]); res == nil && set["dry-run"] {
			return nil
		}
	}
	return &wire.Result{Command: strings.Fields(verb), Outcome: wire.OutcomeError, Codes: []string{wire.CodeUnsupported}, NotRetryable: true, Warnings: []string{
		ReadOnlyEnv + " is set: this is a read-only session, and " + verb + " is not a pure read, so it was refused before any store access; nothing was written. Run it outside the read-only session (ATR-V0-212).",
	}}
}

// verbPath is the longest listed verb path args start with, else the
// leading words up to the first flag or the third word.
func verbPath(args []string) string {
	words := []string{}
	for _, a := range args {
		if strings.HasPrefix(a, "-") && len(words) > 0 || len(words) == 3 {
			break
		}
		words = append(words, a)
	}
	for n := len(words); n > 0; n-- {
		if p := strings.Join(words[:n], " "); readOnlyVerbs[p] {
			return p
		}
	}
	return strings.Join(words, " ")
}

func hasArg(args []string, flag string) bool {
	for _, a := range args {
		if a == flag || strings.HasPrefix(a, flag+"=") {
			return true
		}
	}
	return false
}
