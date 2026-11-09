// Package flows holds the `corvint flows appmap` command handlers; cmd/corvint keeps the
// preamble, dispatch and help. The other flows subcommands were retired by decision 0594.
package flows

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// Env carries the root-owned behaviour the flows handlers share with every other command: the
// JSON error envelope written to the diagnostic stream, and the affected-revision refusal for a
// --base that does not name a commit.
type Env struct {
	EmitError   func(diagnostic io.Writer, err error)
	BaseRefusal func(base string) error
}

// Run executes `corvint flows ARGS` against root and returns the process exit code. The root
// preamble and help stay with the caller.
func Run(ctx context.Context, root string, args []string, out, diagnostic io.Writer, env Env) int {
	if len(args) > 0 && flowSubcommands[args[0]] != nil {
		return runFlowSubcommand(ctx, flowSubcommands[args[0]], root, args[1:], out, diagnostic, env)
	}
	env.EmitError(diagnostic, argumentError("flows requires appmap"))
	return 2
}

func argumentError(message string) error {
	return &gokernel.Error{Code: "invalid-arguments", Message: message}
}

// unresolvedBase marks a --base that is not a commit; runFlowSubcommand turns it into
// Env.BaseRefusal so the refusal itself stays defined once, by the root.
type unresolvedBase struct{ base string }

func (err *unresolvedBase) Error() string {
	return "base " + err.base + " is not a commit in this repository"
}

type flowSubcommand func(ctx context.Context, root string, args []string, out, diag io.Writer) error

// flowSubcommands holds the registered `flows` subcommands; appmap registers itself.
var flowSubcommands = map[string]flowSubcommand{}

// Subcommands names the registered `flows` subcommands for the help catalog.
func Subcommands() []string { return slices.Sorted(maps.Keys(flowSubcommands)) }

func runFlowSubcommand(ctx context.Context, run flowSubcommand, root string, args []string, out, diagnostic io.Writer, env Env) int {
	root, err := filepath.Abs(root)
	if err == nil {
		err = run(ctx, root, args, out, diagnostic)
	}
	var unresolved *unresolvedBase
	if errors.As(err, &unresolved) {
		err = env.BaseRefusal(unresolved.base)
	}
	if errors.Is(err, errAppmapEdgesUnproven) {
		return 1
	}
	var coded *gokernel.Error
	var gate0 *gateFailure
	if err != nil && !errors.As(err, &coded) && !errors.As(err, &gate0) {
		err = argumentError(err.Error())
	}
	var gate *gateFailure
	if errors.As(err, &gate) {
		fmt.Fprintln(diagnostic, gate.Error())
		return 1
	}
	if err != nil {
		env.EmitError(diagnostic, err)
		return 2
	}
	return 0
}
