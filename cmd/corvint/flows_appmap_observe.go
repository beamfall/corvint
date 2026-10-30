package main

import (
	"errors"
	"flag"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

func runFlowsAppmapObserve(args []string, out io.Writer) error {
	if len(args) > 0 && args[0] == "--records" {
		return runFlowsAppmapUIRecords(args[1:], out)
	}
	f := flag.NewFlagSet("flows appmap observe", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application map")
	trace := f.String("trace", "", "navigation events file or directory")
	revision := f.String("revision", "", "application revision the trace ran at")
	testKey := f.String("test-key", "", "test key of a Playwright trace zip")
	budget := f.Int("budget", 0, "byte cap")
	full := f.Bool("full", false, "full byte cap")
	if f.Parse(args) != nil || f.NArg() != 0 || *mapFile == "" || *trace == "" {
		return errors.New("flows appmap observe requires --map FILE and --trace FILE|DIR|ZIP; optional --test-key (required with a zip), --revision, --budget or --full")
	}
	isZip := strings.HasSuffix(*trace, ".zip")
	if isZip != (*testKey != "") {
		return errors.New("flows appmap observe takes --test-key exactly when --trace names a Playwright trace zip")
	}
	m, err := appmap.LoadMap(*mapFile)
	if err != nil {
		return err
	}
	var docs []appmap.NavigationEvents
	var raws [][]byte
	if isZip {
		// One document per page of the zip; every page shares the archive digest (ANL-V0-068).
		var raw []byte
		docs, raw, err = appmap.LoadTraceZip(*trace, *testKey)
		for range docs {
			raws = append(raws, raw)
		}
	} else {
		docs, raws, err = appmap.LoadNavigationEvents(*trace)
	}
	if err != nil {
		return err
	}
	rev := *revision
	if rev == "" {
		rev = m.Revision
	}
	data, err := appmap.Observe(m, docs, raws, rev, appmap.Options{Budget: *budget, Full: *full})
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
