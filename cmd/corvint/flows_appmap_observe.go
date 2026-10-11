package main

import (
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
)

func runFlowsAppmapObserve(args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap observe", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application map")
	trace := f.String("trace", "", "navigation events file or directory")
	revision := f.String("revision", "", "application revision the trace ran at")
	budget := f.Int("budget", 0, "byte cap")
	full := f.Bool("full", false, "full byte cap")
	if f.Parse(args) != nil || f.NArg() != 0 || *mapFile == "" || *trace == "" {
		return errors.New("flows appmap observe requires --map FILE and --trace FILE|DIR; optional --revision, --budget or --full")
	}
	m, err := appmap.LoadMap(*mapFile)
	if err != nil {
		return err
	}
	docs, raws, err := appmap.LoadNavigationEvents(*trace)
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
