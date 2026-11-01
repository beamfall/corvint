package main

import (
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/frictionmetric"
)

// runFrictionInvocation is `eval --friction FILE` (URE-V0-996 to URE-V0-998):
// deterministic friction metrics over an explicit session record. It reads
// only FILE, writes nothing, and is the package's only importer (URE-V0-997).
func runFrictionInvocation(root string, arguments []string, stdout, stderr io.Writer) (int, bool) {
	if len(arguments) == 0 || (arguments[0] != "--friction" && !strings.HasPrefix(arguments[0], "--friction=")) {
		return 0, false
	}
	name, value, inline := strings.Cut(arguments[0], "=")
	rest := arguments[1:]
	if !inline {
		if len(rest) == 0 || argparseOptionLike(rest[0]) {
			return emitSlotWeightsFlagError(stderr, argumentError("argument "+name+": expected one argument")), true
		}
		value, rest = rest[0], rest[1:]
	}
	if len(rest) != 0 {
		return emitSlotWeightsFlagError(stderr, argumentError("argument --friction: takes no other arguments")), true
	}
	file, err := os.Open(resolveGoldenPath(root, value))
	if err != nil {
		emitEvalError(stderr, err)
		return 2, true
	}
	defer file.Close()
	report, err := frictionmetric.Evaluate(file)
	if err != nil {
		emitEvalError(stderr, err)
		return 2, true
	}
	return writeSlotWeightsPayload(stdout, stderr, map[string]any{"ok": true, "mutates": false, "tool": "eval", "friction": report}), true
}
