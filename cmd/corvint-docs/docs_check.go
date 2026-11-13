// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/doccitations"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/docmaintain"
	"github.com/Beamfall/corvint/internal/docworktree"
	"github.com/Beamfall/corvint/internal/flowdocs"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// Part states of `docs check` (DMF-V1-002).
const (
	docsCheckPass       = "PASS"
	docsCheckStale      = "STALE"
	docsCheckNotChecked = "NOT_CHECKED"
	docsCheckNoneFound  = "none-found"
	docsCheckProfile    = "corvint-docs-check/0"
)

// docsCheckItem is one stale or uncheckable item of a part: its path and line, an optional
// identity (a block selector, a flow binding or a citation token) and what is wrong.
type docsCheckItem struct {
	Path   string `json:"path"`
	Line   int    `json:"line,omitempty"`
	ID     string `json:"id,omitempty"`
	Detail string `json:"detail"`
}

// docsCheckPart is one composed check: PASS, STALE or NOT_CHECKED with a reason.
type docsCheckPart struct {
	Part       string              `json:"part"`
	State      string              `json:"state"`
	Reason     string              `json:"reason,omitempty"`
	Checked    int                 `json:"checked"`
	Stale      []docsCheckItem     `json:"stale"`
	NotChecked []docsCheckItem     `json:"not_checked"`
	Worktree   *docworktree.Status `json:"worktree,omitempty"`
}

// docsCheckReport is the corvint-docs-check/0 report: one entry per part, in a fixed order.
type docsCheckReport struct {
	Profile  string             `json:"profile"`
	Commit   string             `json:"commit"`
	Status   string             `json:"status"`
	Parts    []docsCheckPart    `json:"parts"`
	Worktree docworktree.Status `json:"worktree"`
}

func parseDocsCheckArgs(options docsOptions, arguments []string) (docsOptions, error) {
	if len(arguments) != 0 {
		return options, argumentError("docs check takes no arguments; it finds every marker block, flows manifest and line citation itself")
	}
	root, err := resolveExplicitRoot(options.root)
	options.root = root
	return options, err
}

// runDocsCheck composes the existing docs checks over the whole repository and prints one
// corvint-docs-check/0 report whose status is STALE when any part is STALE, else NOT_CHECKED when
// any part is NOT_CHECKED, else PASS: exit 1 when any part is STALE, 0 otherwise (a NOT_CHECKED
// part is reported and named by the status but does not fail), 2 when the repository cannot be
// read or a part could not run: a cancelled or expired context, or a Git or I/O failure, is never
// reported as NOT_CHECKED (DMF-V1-002).
func runDocsCheck(ctx context.Context, options docsOptions, stdout, stderr io.Writer) int {
	ctx = gitstatus.WithIsolation(ctx)
	pages, commit, err := docmaintain.MarkerPages(ctx, options.root)
	if err != nil {
		emitError(stderr, docsCheckError(err))
		return 2
	}
	report := docsCheckReport{Profile: docsCheckProfile, Commit: commit, Status: docsCheckPass}
	markers, err := docsCheckMarkers(ctx, options.root, pages)
	if err == nil {
		report.Parts = append(report.Parts, markers)
		var flows docsCheckPart
		if flows, err = docsCheckFlows(ctx, options.root, commit); err == nil {
			report.Parts = append(report.Parts, flows)
			var citations docsCheckPart
			if citations, err = docsCheckCitations(ctx, options.root); err == nil {
				report.Parts = append(report.Parts, citations)
			}
		}
	}
	if err != nil {
		emitError(stderr, docsCheckError(err))
		return 2
	}
	report.Worktree = docworktree.Observe(ctx, options.root, docworktree.Head, nil)
	for _, p := range report.Parts {
		switch {
		case p.State == docsCheckStale:
			report.Status = docsCheckStale
		case p.State == docsCheckNotChecked && report.Status == docsCheckPass:
			report.Status = docsCheckNotChecked
		}
	}
	encoded, err := gokernel.CanonicalJSON(report)
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot encode the docs check report"})
		return 2
	}
	if _, err = fmt.Fprintf(stdout, "%s\n", encoded); err != nil {
		return 2
	}
	if report.Status == docsCheckStale {
		return 1
	}
	return 0
}

func docsCheckError(err error) error {
	var refusal *docmaintain.Refusal
	var typed *doccorpus.Error
	switch {
	case errors.Is(err, context.Canceled):
		return &gokernel.Error{Code: "cancelled", Message: "docs check was cancelled before every part ran"}
	case errors.Is(err, context.DeadlineExceeded):
		return &gokernel.Error{Code: "cancelled", Message: "docs check ran out of time before every part ran"}
	case errors.As(err, &refusal):
		message := refusal.Message
		if message == "" {
			message = "docs check cannot read the repository: " + refusal.Code
		}
		return &gokernel.Error{Code: refusal.Code, Message: message}
	case errors.As(err, &typed):
		return &gokernel.Error{Code: typed.Code, Message: typed.Message}
	}
	return &gokernel.Error{Code: "docs-check-failed", Message: err.Error()}
}

// settle sets a part's state from its items: STALE with any stale item, else NOT_CHECKED with any
// uncheckable item, else PASS; none checked and none uncheckable is NOT_CHECKED none-found.
func (p *docsCheckPart) settle(uncheckedReason string) {
	switch {
	case len(p.Stale) > 0:
		p.State, p.Reason = docsCheckStale, fmt.Sprintf("%d stale item(s)", len(p.Stale))
	case len(p.NotChecked) > 0:
		p.State, p.Reason = docsCheckNotChecked, uncheckedReason
	case p.Checked == 0:
		p.State, p.Reason = docsCheckNotChecked, docsCheckNoneFound
	default:
		p.State = docsCheckPass
	}
}

func newDocsCheckPart(name string) docsCheckPart {
	return docsCheckPart{Part: name, Stale: []docsCheckItem{}, NotChecked: []docsCheckItem{}}
}

// docsCheckMarkers checks every generated block of every committed page carrying a marker
// (docs maintain --check over the whole repository).
func docsCheckMarkers(ctx context.Context, root string, pages []string) (docsCheckPart, error) {
	part := newDocsCheckPart("markers")
	if len(pages) == 0 {
		part.settle("")
		return part, nil
	}
	selection := make([]docmaintain.CheckPage, 0, len(pages))
	for _, page := range pages {
		selection = append(selection, docmaintain.CheckPage{Page: page})
	}
	report, err := docmaintain.Check(ctx, root, selection)
	if err != nil {
		return part, err
	}
	part.Worktree = &report.Worktree
	for _, b := range report.Blocks {
		part.Checked++
		item := docsCheckItem{Path: b.Page, Line: b.Line, ID: "source=" + b.Source + " package=" + b.Package, Detail: b.State + ": " + b.Reason}
		if b.Source == "" && b.Package == "" {
			item.ID = ""
		}
		if b.State != docmaintain.BlockFresh {
			part.Stale = append(part.Stale, item)
		}
	}
	part.settle("")
	return part, nil
}

// docsCheckRevision is the flows check docsCheckFlows runs; tests replace it to inject failures.
var docsCheckRevision = flowdocs.CheckRevision

// docsCheckFlows runs docs flows check at HEAD for every tracked generation manifest, against the
// output directory it records. A manifest whose output cannot be compared, or whose check refuses
// (a flow-docs refusal about the manifest or its source), is NOT_CHECKED with its reason, never
// PASS. Any other error (a cancelled or expired context, a Git or I/O failure) means the part did
// not run and fails the whole check (DMF-V1-002). The rendered pages are compared as the recorded
// output directory holds them in the worktree, so the part's worktree member observes every
// manifest's scope and output directory against HEAD: clean means the compared bytes are the
// committed ones (DMF-V1-005).
func docsCheckFlows(ctx context.Context, root, commit string) (docsCheckPart, error) {
	part := newDocsCheckPart("flows")
	manifests, err := flowdocs.TrackedManifests(ctx, root, commit)
	if err != nil {
		return part, err
	}
	read := []string{}
	for _, m := range manifests {
		if scope, ok := flowdocs.ManifestScope(m.Raw); ok {
			read = append(read, scope)
		}
		check, err := docsCheckRevision(ctx, root, m.Raw, commit, "")
		if err != nil {
			if ctx.Err() != nil || !flowdocs.Refused(err) {
				return part, err
			}
			part.NotChecked = append(part.NotChecked, docsCheckItem{Path: m.Path, Detail: err.Error()})
			continue
		}
		part.Checked++
		if check.OutputDir != "" && !filepath.IsAbs(check.OutputDir) {
			read = append(read, check.OutputDir)
		}
		for _, b := range check.Bindings {
			if b.State != "fresh" {
				part.Stale = append(part.Stale, docsCheckItem{Path: b.Before.Path, Line: b.Before.Start, ID: b.ID, Detail: b.State + ": " + b.Reason})
			}
		}
		for _, id := range check.Added {
			part.Stale = append(part.Stale, docsCheckItem{Path: m.Path, ID: id, Detail: "added: not in the generated pages"})
		}
		for _, page := range check.OutputDifferences {
			part.Stale = append(part.Stale, docsCheckItem{Path: strings.TrimSuffix(check.OutputDir, "/") + "/" + page, Detail: "rendered page differs from the generation"})
		}
		for _, c := range check.OpaqueChanges {
			part.Stale = append(part.Stale, docsCheckItem{Path: c.Path, Detail: "opaque gitlink changed from " + c.Before + " to " + c.After})
		}
		if check.Output == flowdocs.OutputNotChecked {
			detail := "output " + flowdocs.OutputNotChecked + ": " + check.OutputReason
			if check.OutputHint != "" {
				detail += "; " + check.OutputHint
			}
			part.NotChecked = append(part.NotChecked, docsCheckItem{Path: m.Path, Detail: detail})
		}
	}
	if len(manifests) > 0 {
		status := docworktree.Observe(ctx, root, docworktree.Head, docworktree.Under(read...))
		part.Worktree = &status
	}
	part.settle("output-not-compared")
	return part, nil
}

// docsCheckCitations runs the product citation check over every tracked Markdown file.
func docsCheckCitations(ctx context.Context, root string) (docsCheckPart, error) {
	part := newDocsCheckPart("citations")
	report, err := doccitations.Check(ctx, root, nil)
	if err != nil {
		return part, err
	}
	part.Worktree = &report.Worktree
	part.Checked = report.Citations
	for _, f := range report.Failures {
		part.Stale = append(part.Stale, docsCheckItem{Path: f.Document, Line: f.Line, ID: f.Token, Detail: f.Kind + ": " + f.Detail})
	}
	slices.SortStableFunc(part.Stale, func(a, b docsCheckItem) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return a.Line - b.Line
	})
	part.settle("")
	return part, nil
}

const docsCheckHelp = `Check every piece of the repository's documentation in one pass

Usage:
  corvint-docs [--root PATH] docs check

Example:
  corvint-docs --root /path/to/repo docs check

Composes the existing docs checks with no per-page arguments and prints one
corvint-docs-check/0 JSON report with one entry per part, each PASS, STALE or
NOT_CHECKED with a reason, and each stale item with its path and line:
  markers    every generated block of every committed Markdown page carrying a
             docs maintain marker (docs maintain --check)
  flows      every tracked generation.json flows manifest, checked at HEAD against
             the output directory it records (docs flows check); a manifest whose
             output cannot be compared is NOT_CHECKED, never PASS
  citations  every line citation in tracked Markdown (docs citations check)
A part with nothing to check is NOT_CHECKED with reason none-found. The report
status is STALE when any part is STALE, else NOT_CHECKED when any part is
NOT_CHECKED, else PASS. Every part and the report carry a worktree member;
answers stay about committed or indexed content. Exit 1 when any part is STALE,
0 otherwise (a NOT_CHECKED part does not fail the run), 2 when the repository
cannot be read or a part could not run (cancelled, timed out, or a Git or I/O
failure). It writes nothing.

Contract: docs/specs/docs-mcp-hardening-v1.md (DMF-V1-002)
`
