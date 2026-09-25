package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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
	if len(args) > 0 && flowSubcommands[args[0]] != nil {
		return runFlowSubcommand(ctx, flowSubcommands[args[0]], root, args[1:], out, diagnostic)
	}
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

type flowSubcommand func(ctx context.Context, root string, args []string, out io.Writer) error

var flowSubcommands = map[string]flowSubcommand{"export": runFlowsExport, "import": runFlowsImport}

// flowExports maps each --emit value to the one document it writes to stdout.
var flowExports = map[string]func(ctx context.Context, root string, set appflows.IntentSet, envelope string) ([]byte, error){
	"inventory": func(_ context.Context, _ string, set appflows.IntentSet, _ string) ([]byte, error) {
		return appflows.CompileInventory(set)
	},
	"provider": func(ctx context.Context, root string, set appflows.IntentSet, _ string) ([]byte, error) {
		return appflows.ExportProvider(ctx, root, set, "HEAD")
	},
	"request": func(_ context.Context, _ string, set appflows.IntentSet, envelope string) ([]byte, error) {
		inventory, err := appflows.CompileInventory(set)
		if err != nil {
			return nil, err
		}
		raw, err := appflows.ReadFile(envelope)
		if err != nil {
			return nil, err
		}
		return appflows.ExportRequest(raw, inventory)
	},
}

func runFlowSubcommand(ctx context.Context, run flowSubcommand, root string, args []string, out, diagnostic io.Writer) int {
	root, err := filepath.Abs(root)
	if err == nil {
		err = run(ctx, root, args, out)
	}
	if err != nil {
		emitError(diagnostic, argumentError(err.Error()))
		return 2
	}
	return 0
}

func runFlowsExport(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows export", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	dir := f.String("flows", "", "intent directory inside the root")
	emit := f.String("emit", "", "request, provider or inventory")
	envelope := f.String("envelope", "", "behavior-adapter request anchoring the committed inventory")
	parseErr := f.Parse(args)
	export, known := flowExports[*emit]
	if parseErr != nil || *dir == "" || f.NArg() != 0 || !known || (*emit == "request") != (*envelope != "") {
		return errors.New("flows export requires --flows DIR --emit request|provider|inventory, and --envelope FILE exactly when --emit request")
	}
	set, err := appflows.LoadIntents(root, *dir)
	if err != nil {
		return err
	}
	data, err := export(ctx, root, set, *envelope)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

func runFlowsImport(_ context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows import", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	dir := f.String("flows", "", "intent directory inside the root")
	from := f.String("from", "", "source document")
	format := f.String("format", "", "behavior-adapter-request, openapi or playwright-list")
	if f.Parse(args) != nil || *dir == "" || *from == "" || *format == "" || f.NArg() != 0 {
		return errors.New("flows import requires --flows DIR --from FILE --format behavior-adapter-request|openapi|playwright-list")
	}
	raw, err := appflows.ReadFile(*from)
	if err != nil {
		return err
	}
	written, err := appflows.Import(root, *dir, raw, *format, *from)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, strings.Join(written, "\n"))
	return err
}

// flowsIntentHelp documents the AFU-V1 intent subcommands; help.go appends it to flowsHelp.
const flowsIntentHelp = `
Intent usage:
  corvint [--root PATH] flows export --flows DIR --emit inventory|provider|request [--envelope FILE]
  corvint [--root PATH] flows import --flows DIR --from FILE --format behavior-adapter-request|openapi|playwright-list

export reads application-flow-intent/1 files from DIR (one <flow_id>.json each, plus
retired.json) and writes one document to stdout: the application-flow-inventory/1
inventory, the EEP-V1 provider record evaluated at HEAD, or the
corvint-behavior-adapter-request/1 built from --envelope, whose application-flows
input must anchor the committed inventory bytes. Review anchors are self-attested:
review identity is not verified. import writes new proposed intents with inferred
links, never overwrites an intent, and prints the written paths.
`
