package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// runTestPlanPrefixes implements `test-plan prefixes` (ANL-V0-009..011): read-only; with --check
// it exits 1 when a flagged test is absent from --baseline (or any test flags without one).
func runTestPlanPrefixes(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("test-plan prefixes", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application map")
	ledger := f.String("ledger", "", "application-edge-ledger/0 file")
	facts := f.String("facts", "", "test-prefix-facts/0 file")
	baseline := f.String("baseline", "", "earlier test-plan-prefixes/0 output allowed to keep its flags")
	revision := f.String("revision", "", "revision to check freshness against")
	budget := f.Int("budget", 0, "byte cap")
	full := f.Bool("full", false, "full byte cap")
	check := f.Bool("check", false, "exit 1 when a new test introduces a flagged prefix")
	if f.Parse(arguments) != nil || f.NArg() != 0 || *mapFile == "" {
		emitError(stderr, errors.New("test-plan prefixes requires --map FILE; optional --ledger, --facts, --baseline, --check, --revision, --budget or --full"))
		return 2
	}
	if root == "" {
		root = "."
	}
	run := func() ([]byte, bool, error) {
		m, err := appmap.LoadMap(*mapFile)
		if err != nil {
			return nil, false, err
		}
		var l *appmap.EdgeLedger
		if *ledger != "" {
			if l, err = appmap.LoadEdgeLedger(*ledger); err != nil {
				return nil, false, err
			}
		}
		var pf *appmap.PrefixFacts
		if *facts != "" {
			if pf, err = appmap.LoadPrefixFacts(*facts); err != nil {
				return nil, false, err
			}
		}
		allowed := map[string]bool{}
		if *baseline != "" {
			old, err := appmap.LoadPrefixReport(*baseline)
			if err != nil {
				return nil, false, err
			}
			for _, t := range old.Flagged {
				allowed[t.Test] = true
			}
		}
		data, err := appmap.Prefixes(ctx, m, l, pf, appmap.Options{Root: root, Revision: *revision, Budget: *budget, Full: *full})
		if err != nil {
			return nil, false, err
		}
		var r appmap.PrefixReport
		if err = json.Unmarshal(data, &r); err != nil {
			return nil, false, err
		}
		fresh := false
		for _, t := range r.Flagged {
			fresh = fresh || !allowed[t.Test]
		}
		// Rows trimmed to the byte cap cannot be compared with the baseline: fail closed.
		fresh = fresh || r.Omitted["flagged"] > 0
		return data, fresh, nil
	}
	data, introduced, err := run()
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if _, err = stdout.Write(data); err != nil {
		emitError(stderr, err)
		return 2
	}
	if *check && introduced {
		emitError(stderr, &gokernel.Error{Code: "test-plan-mismatch", Message: "a test replays a proven navigation prefix"})
		return 1
	}
	return 0
}
