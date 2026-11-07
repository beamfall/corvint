package dogfoodflow

import (
	"context"
	"io"
)

// CheckVerifySyntax runs finish-lane's verify input check alone, with the
// same refusal lines, so script/dogfood-finish.sh refuses a malformed VERIFY
// file before any build or tree work: the shared validator cannot drift from
// the one finish-lane and inputs-check run later (DWC-V0-753). options carry
// the wrapper's inputs and --entry, so each rerun hint names the command the
// later pre-flight names (DCW-V0-730). It reads only options.Change.VerifyFile,
// never Git, and returns 0 for an admitted file.
func CheckVerifySyntax(ctx context.Context, options FinishLaneOptions, prefix string, stderr io.Writer) (int, error) {
	options.Preflight = true
	run := &finishLane{
		flow:    flow{ctx: ctx, prefix: prefix, stderr: stderr},
		options: options,
	}
	return boundary(ctx, func() int {
		run.requireVerifyInput()
		return 0
	}, func() {})
}
