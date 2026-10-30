package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"regexp"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/rootalias"
	"github.com/Beamfall/corvint/internal/workspace"
)

// sourceRepos is the aliased-root binding of a source verb: repeatable --repo ALIAS=ABSOLUTE_ROOT
// (SKH-V0-189), or --workspace FILE, or else the workspace discovered under --root unless
// --no-workspace (WSD-V0-145).
type sourceRepos struct {
	repos       map[string]string
	workspace   string
	noWorkspace bool
}

// bindSourceRepos registers --repo ALIAS=ABSOLUTE_ROOT (repeatable), --workspace FILE and
// --no-workspace on a source verb: the checkout of a root the map pinned, read for --source
// ALIAS:DIR at the map's revision of it. Call resolve after parsing.
func bindSourceRepos(f *flag.FlagSet) *sourceRepos {
	s := &sourceRepos{repos: map[string]string{}}
	f.Func("repo", "ALIAS=ABSOLUTE_ROOT of a root the map read (repeatable)", func(v string) error {
		alias, path, aliased := rootalias.Split(v)
		if !aliased || path == "" || s.repos[alias] != "" || len(s.repos) == rootalias.MaxRoots {
			return errors.New("invalid --repo")
		}
		s.repos[alias] = path
		return nil
	})
	f.Func("workspace", "FILE: a workspace declaration (WSD-V0) of the aliased roots", func(v string) error {
		if v == "" || s.workspace != "" {
			return errors.New("invalid --workspace")
		}
		s.workspace = v
		return nil
	})
	f.BoolVar(&s.noWorkspace, "no-workspace", false, "do not discover "+workspace.DiscoveryPath+" under the root")
	return s
}

// resolve returns the alias roots: the --repo flags, or each alias root of the named or discovered
// workspace. A workspace never merges with --repo (WSD-V0-004).
func (s *sourceRepos) resolve(root string) (map[string]string, error) {
	var w *workspace.Workspace
	var err error
	switch {
	case s.workspace != "" && (len(s.repos) > 0 || s.noWorkspace):
		return nil, errors.New("--workspace does not combine with --repo or --no-workspace")
	case s.workspace != "":
		w, err = workspace.Load(s.workspace)
	case len(s.repos) > 0 || s.noWorkspace:
		return s.repos, nil
	default:
		w, err = workspace.Discover(root)
	}
	if err != nil || w == nil {
		return s.repos, err
	}
	for alias, repo := range w.Repos {
		s.repos[alias] = repo.Root
	}
	return s.repos, nil
}

// flagSpelling matches the flag package's single-dash flag name, never a quoted value.
var flagSpelling = regexp.MustCompile(`(flag |for |: )-([a-z])`)

// parseSourceVerb parses a source verb's flags in any order around at most positionals arguments
// and returns them; a refusal names the flag or argument that failed before the usage (SKH-V0-330).
func parseSourceVerb(f *flag.FlagSet, args []string, positionals int, mapFile *string, usage string) ([]string, error) {
	var rest []string
	err := f.Parse(args)
	for err == nil && f.NArg() > 0 && len(rest) < positionals {
		rest = append(rest, f.Arg(0))
		err = f.Parse(f.Args()[1:])
	}
	switch {
	case err != nil:
		return nil, fmt.Errorf("%s: %s", flagSpelling.ReplaceAllString(err.Error(), "$1--$2"), usage)
	case f.NArg() > 0:
		return nil, fmt.Errorf("unexpected argument %q: %s", f.Arg(0), usage)
	case len(rest) < positionals:
		return nil, errors.New("missing GOAL: " + usage)
	case *mapFile == "":
		return nil, errors.New("missing --map: " + usage)
	}
	return rest, nil
}

// runFlowsAppmapKnowledge prints the site-knowledge page-object model (pageobjects,
// SKH-V0-010..016), the test-body step IR (steps, SKH-V0-020..024) or the setup records (setup,
// SKH-V0-030..034, SKH-V0-100..105, with repeatable --condition) or the screen graph's edges by
// provenance (links, SKH-V0-040..043, SKH-V0-110..115). Refusals exit 2.
func runFlowsAppmapKnowledge(ctx context.Context, root, verb string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap "+verb, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application-map/0 file")
	budget := f.Int("budget", 0, "byte budget")
	full := f.Bool("full", false, "raise cap to 1 MiB")
	var sources []string
	f.Func("source", "[ALIAS:]DIR test or page-object directory (repeatable)", func(v string) error {
		sources = append(sources, v)
		return nil
	})
	bound := bindSourceRepos(f)
	var conditions []string
	if verb == "setup" {
		f.Func("condition", "condition text to find producing setup records for (repeatable)", func(v string) error {
			conditions = append(conditions, v)
			return nil
		})
	}
	usage := "flows appmap " + verb + " requires --map FILE, --source [ALIAS:]DIR (repeatable), optional --repo ALIAS=ROOT (repeatable) or --workspace FILE, --no-workspace, --budget N or --full"
	if verb == "setup" {
		usage += " and --condition TEXT (repeatable)"
	}
	if _, err := parseSourceVerb(f, args, 0, mapFile, usage); err != nil {
		return err
	}
	// An explicit --budget 0 is out of range, not the default (as for refs).
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
	report := appmap.PageObjects
	switch verb {
	case "steps":
		report = appmap.Steps
	case "links":
		report = appmap.Links
	case "setup":
		report = func(ctx context.Context, m *appmap.Map, sources []string, o appmap.Options) ([]byte, error) {
			return appmap.SetupQuery(ctx, m, sources, conditions, o)
		}
	}
	doc, err := report(ctx, m, sources, appmap.Options{Root: root, Budget: *budget, Full: *full, Repos: repos})
	if err != nil {
		return err
	}
	_, err = out.Write(doc)
	return err
}
