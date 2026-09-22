package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/appflows"
)

func flowInvocation(args []string) (string, []string, bool) {
	root := "."
	for len(args) > 0 && strings.HasPrefix(args[0], "--root") {
		if strings.HasPrefix(args[0], "--root=") {
			root, args = strings.TrimPrefix(args[0], "--root="), args[1:]
			continue
		}
		if args[0] != "--root" || !rootPreambleValue(args, 1) {
			return "", nil, false
		}
		root, args = args[1], args[2:]
	}
	if len(args) == 0 || args[0] != "flows" {
		return "", nil, false
	}
	return root, args[1:], true
}

func runFlows(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	record := false
	if len(args) > 0 && args[0] == "record" {
		record, args = true, args[1:]
	}
	f := flag.NewFlagSet("flows", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	manifest := f.String("manifest", "", "tracked relative manifest")
	evidence := f.String("evidence", "", "optional observation file")
	output := f.String("output", "", "explicit record destination")
	if err := f.Parse(args); err != nil {
		emitError(diagnostic, argumentError("invalid flows arguments"))
		return 2
	}
	if *manifest == "" || f.NArg() != 0 {
		emitError(diagnostic, argumentError("flows requires --manifest FILE"))
		return 2
	}
	if !record && *output != "" {
		emitError(diagnostic, argumentError("--output requires flows record"))
		return 2
	}
	if record && (*evidence == "" || *output == "") {
		emitError(diagnostic, argumentError("flows record requires --evidence and --output"))
		return 2
	}
	root, err := filepath.Abs(root)
	if err != nil {
		emitError(diagnostic, argumentError("invalid flow root"))
		return 2
	}
	in, err := appflows.Capture(ctx, root, *manifest)
	if err != nil {
		emitError(diagnostic, argumentError(err.Error()))
		return 2
	}
	var e *appflows.Evidence
	var raw []byte
	if *evidence != "" {
		raw, err = appflows.ReadFile(*evidence)
		if err != nil {
			emitError(diagnostic, argumentError(err.Error()))
			return 2
		}
		e = new(appflows.Evidence)
		if err = appflows.Decode(raw, e); err != nil {
			emitError(diagnostic, argumentError(err.Error()))
			return 2
		}
	}
	r, err := appflows.ReportFor(in, e)
	if err != nil {
		emitError(diagnostic, argumentError(err.Error()))
		return 2
	}
	if record {
		if err = appflows.Record(in, raw, *output); err != nil {
			emitError(diagnostic, argumentError(err.Error()))
			return 2
		}
	}
	if json.NewEncoder(out).Encode(r) != nil {
		return 2
	}
	return 0
}
