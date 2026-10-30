package cli

import (
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// sourceContainsMaxCommits bounds one `source contains` call (SCC-V0-005).
const sourceContainsMaxCommits = 64

// sourceContainsCommand answers `source contains <rev> <commit>...`
// (SCC-V0-005): one item per commit with ANCESTOR, EQUIVALENT, ABSENT or
// UNKNOWN. It reads Git in the working directory only; it opens no store and
// writes nothing.
func sourceContainsCommand(env Env, args []string) *wire.Result {
	if len(args) == 0 || args[0] != "contains" {
		return usage([]string{"source"}, "expected contains")
	}
	cmd := []string{"source", "contains"}
	args = args[1:]
	if len(args) < 2 {
		return usage(cmd, "expected a source revision and at least one commit")
	}
	if len(args)-1 > sourceContainsMaxCommits {
		return &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Codes: []string{wire.CodeLimitExceeded},
			Warnings: []string{"source contains takes at most " + strconv.Itoa(sourceContainsMaxCommits) + " commits per call; nothing was read"}}
	}
	for _, a := range args {
		if a == "" || strings.HasPrefix(a, "-") || strings.ContainsAny(a, "\x00\n\r") {
			return usage(cmd, "source contains takes revisions only, no flags or empty arguments")
		}
	}
	res := &wire.Result{Command: cmd, Outcome: wire.OutcomeOK}
	for _, c := range args[1:] {
		sc := store.SourceContainmentOf(env.Cwd, args[0], c)
		o := wire.NewObject().Set("source", wire.String(args[0])).Set("commit", wire.String(c)).Set("state", wire.String(sc.State))
		if sc.Source != "" {
			o.Set("sourceResolved", wire.String(sc.Source))
		}
		if sc.Commit != "" {
			o.Set("commitResolved", wire.String(sc.Commit))
		}
		if sc.Equivalent != "" {
			o.Set("equivalent", wire.String(sc.Equivalent))
		}
		if sc.PatchDiffers != "" {
			o.Set("patchDiffers", wire.String(sc.PatchDiffers))
		}
		if sc.Reason != "" {
			o.Set("reason", wire.String(sc.Reason))
		}
		if sc.Detail != "" {
			o.Set("detail", wire.String(prose(sc.Detail)))
		}
		res.Items = append(res.Items, wire.ObjectValue(o))
	}
	return res
}
