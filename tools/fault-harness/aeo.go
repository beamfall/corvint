// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
)

// The parity path of FIH-V1-011: an affected-edit-oracle/0 mutation case is
// run through the harness core and its Go adapter, and the miss verdict is
// reported in the oracle's terms so both tools can be compared on one input.

const aeoProfile = "affected-edit-oracle/0"

type aeoEdit struct {
	Path      string `json:"path"`
	Operation string `json:"operation"`
	Before    string `json:"before,omitempty"`
	Content   string `json:"content,omitempty"`
}
type aeoCommand struct {
	Argv   []string `json:"argv"`
	Format string   `json:"format"`
	UnitID string   `json:"unitId,omitempty"`
}
type aeoCase struct {
	ID         string   `json:"id"`
	Repo       string   `json:"repo"`
	BaseSHA    string   `json:"baseSha"`
	EditClass  string   `json:"editClass"`
	Edit       aeoEdit  `json:"edit"`
	MustSelect []string `json:"mustSelect"`
	MaySelect  []string `json:"maySelect"`
	Why        string   `json:"why"`
	Mutation   *struct {
		Edit     aeoEdit      `json:"edit"`
		Commands []aeoCommand `json:"commands"`
	} `json:"mutation,omitempty"`
}
type aeoCorpus struct {
	Profile string    `json:"profile"`
	Cases   []aeoCase `json:"cases"`
}

// parityRow states one case's harness result in the oracle's safety terms:
// PASS (no miss), SAFETY_FAIL (a miss), ERROR (failed measurement) or SKIPPED
// (a case the Go adapter cannot run, such as a node-assert command).
type parityRow struct {
	ID      string   `json:"id"`
	Safety  string   `json:"safety"`
	Missing []string `json:"missing,omitempty"`
	Detail  string   `json:"detail,omitempty"`
}

// aeoGoScope splits a case's go-json commands into one `go test` argv prefix
// and the package arguments, which become the adapter's whole-suite scope.
func aeoGoScope(cmds []aeoCommand) (prefix, pkgs []string, err error) {
	valued := map[string]bool{"-run": true, "-timeout": true, "-count": true, "-p": true, "-tags": true, "-skip": true}
	for _, c := range cmds {
		if c.Format != "go-json" {
			return nil, nil, fmt.Errorf("command format %q is not run by the Go adapter", c.Format)
		}
		at := -1
		for i := 1; i < len(c.Argv); i++ {
			if c.Argv[i] == "test" && path.Base(c.Argv[i-1]) == "go" {
				at = i
				break
			}
		}
		if at < 0 {
			return nil, nil, fmt.Errorf("command %q is not a go test command", strings.Join(c.Argv, " "))
		}
		p := c.Argv[:at+1]
		if prefix != nil && !slices.Equal(prefix, p) {
			return nil, nil, fmt.Errorf("commands use different go test prefixes")
		}
		prefix = slices.Clone(p)
		rest := c.Argv[at+1:]
		for i := 0; i < len(rest); i++ {
			a := rest[i]
			if strings.HasPrefix(a, "-") {
				if valued[a] {
					i++
				}
				continue
			}
			pkgs = append(pkgs, a)
		}
	}
	if len(pkgs) == 0 {
		pkgs = []string{"."}
	}
	slices.Sort(pkgs)
	return prefix, slices.Compact(pkgs), nil
}

// aeoFault converts the case's unique replace-literal mutation into a byte
// fault at its offset in the pinned file.
func aeoFault(ctx context.Context, source string, c aeoCase) (fault, error) {
	m := c.Mutation.Edit
	if m.Operation != "replace-literal" || m.Before == "" || m.Before == m.Content {
		return fault{}, fmt.Errorf("mutation is not a replace-literal edit")
	}
	body, err := gitRaw(ctx, source, "show", c.BaseSHA+":"+m.Path)
	if err != nil {
		return fault{}, err
	}
	if bytes.Count(body, []byte(m.Before)) != 1 {
		return fault{}, fmt.Errorf("%s: expected exactly one replacement site", m.Path)
	}
	at := bytes.Index(body, []byte(m.Before))
	return fault{ID: c.ID + "-mutation", Path: m.Path, Line: 1 + bytes.Count(body[:at], []byte("\n")), Offset: at, Before: m.Before, After: m.Content, Operator: "constant-replacement"}, nil
}

// runParity measures one oracle mutation case through the harness. The plan
// base is the pin itself, as the oracle plans the committed mutant against
// HEAD~1.
func runParity(ctx context.Context, cfg runConfig, opt adapterOptions, c aeoCase) parityRow {
	row := parityRow{ID: c.ID, Safety: "ERROR"}
	if c.Mutation == nil {
		row.Safety, row.Detail = "SKIPPED", "case has no mutation"
		return row
	}
	prefix, pkgs, err := aeoGoScope(c.Mutation.Commands)
	if err != nil {
		row.Safety, row.Detail = "SKIPPED", err.Error()
		return row
	}
	opt.GoTest, opt.Scope = prefix, pkgs
	adapter, err := adapterFor("go", opt)
	if err != nil {
		row.Detail = err.Error()
		return row
	}
	f, err := aeoFault(ctx, cfg.Source, c)
	if err != nil {
		row.Detail = err.Error()
		return row
	}
	cfg.Adapter = adapter
	r := newHarness(cfg).runCase(ctx, harnessCase{ID: c.ID, Kind: "mutants", BaseSHA: c.BaseSHA, HeadSHA: c.BaseSHA, Faults: []fault{f}})
	switch {
	case r.Status == caseError:
		row.Detail = r.Error
	case r.Status != caseCounted:
		row.Detail = "no confirmed kill: " + r.Faults[0].Detail
	default:
		// A NARROWED plan reports the harness's own misses (FIH-V1-006). The
		// oracle compares failing units with the plan whatever its scope, so a
		// FULL or UNKNOWN plan, which FIH-V1-014 does not count and the harness
		// does not run selectively, is held to unit inclusion instead.
		fr := r.Faults[0]
		row.Safety = "PASS"
		for _, k := range fr.Kills {
			if fr.PlanKind == "NARROWED" && slices.Contains(fr.Misses, k) || fr.PlanKind != "NARROWED" && !unitSelected(fr.Plan, testUnit(k)) {
				row.Missing = append(row.Missing, testUnit(k))
			}
		}
		slices.Sort(row.Missing)
		row.Missing = slices.Compact(row.Missing)
		if len(row.Missing) > 0 {
			row.Safety = "SAFETY_FAIL"
		}
	}
	return row
}

func unitSelected(entries []planEntry, unit string) bool {
	return slices.ContainsFunc(entries, func(e planEntry) bool { return e.Unit == unit })
}
