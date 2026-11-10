package main

import (
	"flag"
	"io"
	"time"

	"github.com/Beamfall/corvint/internal/playwrightruns"
)

// runsReadAll reads every named report or refuses, naming the option and 1-based position.
// ok is false after a refusal was written.
// ident, when not nil, binds identity sidecars (PWR-V0-065).
func runsReadAll(option string, names []string, ident *runsIdentityOpts, diagnostic io.Writer) (runs []*playwrightruns.Run, ok bool) {
	runs = []*playwrightruns.Run{}
	for i, name := range names {
		run, err := playwrightruns.ReadFile(name)
		if err != nil {
			runsRefuse(diagnostic, "runs-report-unavailable", map[string]any{"option": option, "reason": err.Error(), "run": i + 1})
			return nil, false
		}
		if !ident.apply(option, i+1, name, run, diagnostic) {
			return nil, false
		}
		runs = append(runs, run)
	}
	return runs, true
}

func runRunsReusable(root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("runs reusable", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	priors, selection := []string{}, []string{}
	f.Func("prior", "prior Playwright JSON reports", runsFiles(root, &priors))
	f.Func("selection", "the selected tests as one Playwright JSON report", runsFiles(root, &selection))
	summary := f.Bool("summary", false, "compact form")
	ident := runsIdentityFlags(f, root)
	if !runsParse(f, args, diagnostic, func() []runsNeed {
		if len(selection) > 1 {
			return []runsNeed{{"selection", runsArgValueInvalid}}
		}
		return []runsNeed{runsRequired("selection", len(selection) == 0), ident.need(selection, priors)}
	}) {
		return 2
	}
	selected, ok := runsReadAll("selection", selection, ident, diagnostic)
	if !ok {
		return 2
	}
	prior, ok := runsReadAll("prior", priors, ident, diagnostic)
	if !ok {
		return 2
	}
	report := playwrightruns.Reusable(selected[0], prior, nil)
	if *summary {
		report = report.Compact()
	}
	return runsEmit(report, out, diagnostic)
}

func runRunsRedundancy(root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("runs redundancy", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	files := []string{}
	var since *time.Time
	f.Func("runs", "Playwright JSON reports", runsFiles(root, &files))
	f.Func("since", "RFC 3339 window start", func(v string) error {
		at, err := time.Parse(time.RFC3339Nano, v)
		if err != nil || since != nil {
			return errRunsArgument
		}
		since = &at
		return nil
	})
	summary := f.Bool("summary", false, "compact form")
	ident := runsIdentityFlags(f, root)
	if !runsParse(f, args, diagnostic, func() []runsNeed {
		return []runsNeed{runsRequired("runs", len(files) == 0), ident.need(files)}
	}) {
		return 2
	}
	runs, ok := runsReadAll("runs", files, ident, diagnostic)
	if !ok {
		return 2
	}
	report := playwrightruns.Redundancy(runs, since)
	if *summary {
		report = report.Compact()
	}
	return runsEmit(report, out, diagnostic)
}
