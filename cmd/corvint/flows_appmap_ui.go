package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/observedstale"
)

// runFlowsAppmapUIRecords writes observed-ui-records/0 from Playwright trace zips
// (docs/specs/observed-ui-evidence-v0.md, OUE-V0-001..011).
func runFlowsAppmapUIRecords(args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap observe --records", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application map")
	routesFile := f.String("routes", "", "observed-ui-routes/0 templates")
	revision := f.String("revision", "", "full commit the traces ran at")
	var inputs []appmap.UITraceInput
	f.Func("trace", "KEY=FILE.zip (repeatable)", func(v string) error {
		key, file, ok := strings.Cut(v, "=")
		if !ok || key == "" || !strings.HasSuffix(file, ".zip") {
			return errors.New("bad --trace")
		}
		inputs = append(inputs, appmap.UITraceInput{TestKey: key, File: file})
		return nil
	})
	if f.Parse(args) != nil || f.NArg() != 0 || len(inputs) == 0 || *revision == "" {
		return errors.New("flows appmap observe --records requires --revision FULL_SHA and --trace TEST_KEY=FILE.zip (repeatable); optional --map FILE and --routes FILE")
	}
	var m *appmap.Map
	if *mapFile != "" {
		loaded, err := appmap.LoadMap(*mapFile)
		if err != nil {
			return err
		}
		m = loaded
	}
	var routes *appmap.UIRoutes
	routesDigest := ""
	if *routesFile != "" {
		r, d, err := appmap.LoadUIRoutes(*routesFile)
		if err != nil {
			return err
		}
		routes, routesDigest = r, d
	}
	data, err := appmap.ObserveUIRecords(m, routes, routesDigest, inputs, *revision)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

// runFlowsAppmapStructureDiff compares two records documents' ARIA structure digests
// (OUE-V0-040..044): a candidate signal, never a verdict.
func runFlowsAppmapStructureDiff(args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap structure-diff", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	base := f.String("base", "", "base observed-ui-records/0")
	head := f.String("head", "", "head observed-ui-records/0")
	if f.Parse(args) != nil || f.NArg() != 0 || *base == "" || *head == "" {
		return errors.New("flows appmap structure-diff requires --base FILE and --head FILE (observed-ui-records/0)")
	}
	b, err := appmap.LoadUIRecords(*base)
	if err != nil {
		return err
	}
	h, err := appmap.LoadUIRecords(*head)
	if err != nil {
		return err
	}
	data, err := appmap.DiffUIStructures(b, h)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

// uiStaleRecords lists every observed record of one document with its linked source paths.
func uiStaleRecords(doc *appmap.UIRecords) []observedstale.Record {
	var recs []observedstale.Record
	add := func(id, kind string, paths []string) {
		recs = append(recs, observedstale.Record{ID: id, Kind: kind, Revision: doc.Revision, Paths: paths})
	}
	for _, s := range doc.Screens {
		add(s.ID, "screen", s.Paths)
	}
	for _, t := range doc.Transitions {
		add(t.ID, "transition", t.Paths)
	}
	for _, e := range doc.Endpoints {
		add(e.ID, "endpoint", e.Paths)
	}
	for _, b := range doc.Bindings {
		add(b.ID, "binding", b.Paths)
	}
	for _, s := range doc.Structures {
		add("structure:"+s.Screen, "structure", s.Paths)
	}
	return recs
}

// runFlowsAppmapStaleness states each observed record at --at through the shared evaluator
// (OUE-V0-030..036), reading Git at the root.
func runFlowsAppmapStaleness(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap staleness", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	records := f.String("records", "", "observed-ui-records/0")
	at := f.String("at", "HEAD", "revision to evaluate at")
	retire := f.String("retire-after", "0", "commit distance after which a stale record retires; 0 never")
	parseErr := f.Parse(args)
	n, nErr := strconv.Atoi(*retire)
	if parseErr != nil || f.NArg() != 0 || *records == "" || nErr != nil || n < 0 {
		return errors.New("flows appmap staleness requires --records FILE; optional --at REV (default HEAD) and --retire-after N (0 never retires)")
	}
	doc, err := appmap.LoadUIRecords(*records)
	if err != nil {
		return err
	}
	if root == "" {
		root = "."
	}
	h := observedstale.GitHistory{Ctx: ctx, Root: root}
	resolved, err := h.Resolve(*at)
	if err != nil {
		return errors.New("flows appmap staleness: --at does not resolve to a commit")
	}
	rep := observedstale.Evaluate(h, resolved, uiStaleRecords(doc), observedstale.Options{RetireAfter: n})
	rep.Source = doc.Digest
	data, err := json.Marshal(rep)
	if err != nil {
		return err
	}
	_, err = out.Write(append(data, '\n'))
	return err
}
