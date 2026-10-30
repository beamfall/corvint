package main

import (
	"context"
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// runFlowsAppmapInfer prints the stage 5 candidate flows (site-knowledge-flows/0) or, with
// --emit request, the proposed corvint-behavior-adapter-request/1 that flows import reads
// (SKH-V0-050..054, SKH-V0-120..126). It writes nothing. Refusals exit 2.
func runFlowsAppmapInfer(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap infer", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application-map/0 file")
	corpus := f.String("corpus", "", "built documentation corpus artifact whose test-knowledge joins rank and block sources")
	emit := f.String("emit", "report", "report or request")
	budget := f.Int("budget", 0, "byte budget")
	full := f.Bool("full", false, "raise cap to 1 MiB")
	var sources []string
	f.Func("source", "[ALIAS:]DIR test or page-object directory (repeatable)", func(v string) error {
		sources = append(sources, v)
		return nil
	})
	bound := bindSourceRepos(f)
	usage := "flows appmap infer requires --map FILE, --source [ALIAS:]DIR (repeatable), optional --repo ALIAS=ROOT (repeatable) or --workspace FILE, --no-workspace, --corpus ARTIFACT, --emit report|request and --budget N or --full"
	if _, err := parseSourceVerb(f, args, 0, mapFile, usage); err != nil {
		return err
	}
	if *emit != "report" && *emit != "request" {
		return errors.New("--emit must be report or request: " + usage)
	}
	f.Visit(func(fl *flag.Flag) {
		if fl.Name == "budget" && *budget == 0 {
			*budget = -1
		}
	})
	repos, err := bound.resolve(root)
	if err != nil {
		return err
	}
	m, err := appmap.LoadMap(*mapFile)
	if err != nil {
		return err
	}
	trust := []appmap.TestTrust{}
	if *corpus != "" {
		if trust, err = corpusTrust(ctx, root, *corpus, m.Revision); err != nil {
			return err
		}
	}
	o := appmap.Options{Root: root, Budget: *budget, Full: *full, Repos: repos}
	var doc []byte
	if *emit == "request" {
		doc, err = appmap.InferFlowsRequest(ctx, m, sources, trust, o)
	} else {
		doc, err = appmap.InferFlows(ctx, m, sources, trust, o)
	}
	if err != nil {
		return err
	}
	_, err = out.Write(doc)
	return err
}

// corpusTrust opens a built corpus artifact, which re-derives it against --root, and returns its
// joined test-knowledge records on the corpus's own repository (DCP-V1-089, DCP-V1-090). A corpus
// built at another revision than the map is refused, so line spans always name the map's source.
func corpusTrust(ctx context.Context, root, artifact, revision string) ([]appmap.TestTrust, error) {
	raw, err := doccorpus.ReadCorpusFile(root, artifact)
	if err != nil {
		return nil, err
	}
	a, err := doccorpus.Open(ctx, root, raw)
	if err != nil {
		return nil, err
	}
	if a.Manifest.Repository.Revision != revision {
		return nil, &gokernel.Error{Code: "appmap-invalid-query", Message: "--corpus was built at another revision than the map"}
	}
	return trustFromJoins(a.TestKnowledge, a.Manifest.Repository.ID), nil
}

// trustFromJoins keeps the joined records whose test anchor is in repository; stale and
// unanchored records never join (DCP-V1-089) and are not read.
func trustFromJoins(joins []doccorpus.TestKnowledgeJoin, repository string) []appmap.TestTrust {
	out := []appmap.TestTrust{}
	for _, j := range joins {
		t := j.Record.Test
		if j.Status != "joined" || t.Repository != repository {
			continue
		}
		out = append(out, appmap.TestTrust{ID: j.ID, Path: t.Path, Start: t.Start, End: t.End, TrustLevel: j.TrustLevel,
			Verdict: j.Verdict, Selection: j.Selection, Gap: j.Record.Gap})
	}
	return out
}
