// SPDX-License-Identifier: Apache-2.0

// Package flowreports holds the flow reports that join application flows with test results:
// `corvint flows status`, `corvint flows impact` and `corvint flows gaps` (AFL-V1-001,
// AFL-V1-007, PROD-DOCS-011). It is in the tests layer: it reads flows, intents, links and results
// through the shared readers of internal/appflows and never imports the docs command group, so
// the reports run with `corvint` installed alone (G5). The handlers moved here from
// internal/command/flows, whose `corvint-docs flows status|impact|gaps` now name `corvint`.
package flowreports

import (
	"context"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"slices"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// Env carries the root-owned behaviour of the calling binary: its JSON error envelope and the
// affected-revision refusal for a --base that does not name a commit.
type Env struct {
	EmitError   func(diagnostic io.Writer, err error)
	BaseRefusal func(base string) error
}

// Verbs are the `flows` subcommands this package serves, in help order.
var Verbs = []string{"status", "impact", "gaps"}

// IsVerb reports whether verb is one of Verbs.
func IsVerb(verb string) bool { return slices.Contains(Verbs, verb) }

var handlers = map[string]func(ctx context.Context, root string, args []string, out io.Writer) error{
	"status": runStatus, "impact": runImpact, "gaps": runGaps,
}

// unresolvedBase marks a --base that is not a commit; Run turns it into Env.BaseRefusal.
type unresolvedBase struct{ base string }

func (err *unresolvedBase) Error() string {
	return "base " + err.base + " is not a commit in this repository"
}

// Run executes `flows VERB ARGS` against root and returns the process exit code: 0 with the report
// on out, or 2 with one typed refusal on diagnostic and nothing on out (AFL-V1-006). A refusal
// that carries no code of its own is invalid-arguments.
func Run(ctx context.Context, root, verb string, args []string, out, diagnostic io.Writer, env Env) int {
	handler := handlers[verb]
	if handler == nil {
		env.EmitError(diagnostic, argumentError("unknown flows report "+verb))
		return 2
	}
	root, err := filepath.Abs(root)
	if err == nil {
		err = handler(ctx, root, args, out)
	}
	var unresolved *unresolvedBase
	if errors.As(err, &unresolved) {
		err = env.BaseRefusal(unresolved.base)
	}
	var coded *gokernel.Error
	if err != nil && !errors.As(err, &coded) {
		err = argumentError(err.Error())
	}
	if err != nil {
		env.EmitError(diagnostic, err)
		return 2
	}
	return 0
}

func argumentError(message string) error {
	return &gokernel.Error{Code: "invalid-arguments", Message: message}
}

// refuseUnreadableFlows is the check each report makes before reading anything else
// (FSV-V1-005, AFL-V1-006): a flows declaration the shared reader refuses, of an unsupported
// schema version or otherwise invalid, ends the command with the reader's reason and a non-zero
// exit, and no partial report is written. No declaration, or one this build reads, changes
// nothing.
func refuseUnreadableFlows(root string) error {
	refusal := appflows.ReportRefusal(root)
	if refusal == nil {
		return nil
	}
	return &gokernel.Error{Code: refusal.Code(), Message: refusal.Reason + ": " + refusal.Error()}
}

// repeatable declares one repeatable file flag.
func repeatable(f *flag.FlagSet, name, usage string) *[]string {
	values := &[]string{}
	f.Func(name, usage, func(s string) error {
		*values = append(*values, s)
		return nil
	})
	return values
}

func newFlags(name string) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	return f
}

// runStatus is `flows status`: one report of each flow, its variations, the tests attached to each
// and the last recorded result of each test, optionally limited to the flows and variations a
// change reaches (AFL-V1-001 to AFL-V1-006). It reads intents at HEAD and the results of the two
// named writers only: the run evidence store and the default `corvint test` receipt unless
// --no-store, each --receipt FILE and each --evidence FILE (AFL-V1-002). It writes nothing
// (AFL-V1-005, RVD-V1-012).
func runStatus(ctx context.Context, root string, args []string, out io.Writer) error {
	f := newFlags("flows status")
	dir := f.String("flows", "", "intent directory inside the root")
	evidence := repeatable(f, "evidence", "test-run-evidence/0 JSONL file written by flows ingest")
	receipts := repeatable(f, "receipt", "corvint-test-run/0 receipt written by corvint test")
	provider := f.String("provider", "", "repository-relative application-flow-selection-provider/1 file whose E2E inventory gives the unassigned count")
	base := f.String("base", "", "base commit that limits the report to the flows and variations the change reaches")
	noStore := f.Bool("no-store", false, "read only the named --receipt and --evidence files, not the run evidence store or the default receipt")
	if f.Parse(args) != nil || *dir == "" || f.NArg() != 0 {
		return errors.New("flows status requires --flows DIR, optional repeatable --receipt FILE and --evidence FILE, optional --provider FILE, --no-store and --base SHA")
	}
	if err := refuseUnreadableFlows(root); err != nil {
		return err
	}
	set, err := appflows.LoadIntentsAt(ctx, root, *dir, "HEAD")
	if err != nil {
		return err
	}
	results, err := appflows.ReadResults(ctx, root, appflows.ResultInputs{Evidence: *evidence, Receipts: *receipts, NoStore: *noStore})
	if err != nil {
		return err
	}
	options := appflows.StatusOptions{Results: results}
	if *base != "" {
		if options.Base, err = appflows.ResolveRevision(ctx, root, *base); err != nil {
			return &unresolvedBase{*base}
		}
	}
	if *provider != "" {
		if options.Inventory, err = appflows.ReadInventoryProvider(root, *provider, *dir); err != nil {
			return err
		}
	}
	data, err := appflows.FlowStatus(ctx, root, set, options)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

// runImpact is `flows impact` (AFL-V1-007, AFU-V1-017): the flows, variations and test keys the
// base..HEAD change reaches, with each reached flow's last stored result per test key unless
// --no-store (RVD-V1-007).
func runImpact(ctx context.Context, root string, args []string, out io.Writer) error {
	f := newFlags("flows impact")
	dir := f.String("flows", "", "intent directory inside the root")
	base := f.String("base", "", "base commit of the changed range")
	noStore := f.Bool("no-store", false, "do not read the run evidence store")
	if f.Parse(args) != nil || *dir == "" || *base == "" || f.NArg() != 0 {
		return errors.New("flows impact requires --flows DIR --base SHA and optional --no-store")
	}
	if err := refuseUnreadableFlows(root); err != nil {
		return err
	}
	set, err := appflows.LoadIntentsAt(ctx, root, *dir, "HEAD")
	if err != nil {
		return err
	}
	resolved, err := appflows.ResolveRevision(ctx, root, *base)
	if err != nil {
		return &unresolvedBase{*base}
	}
	records, _, err := appflows.RunEvidenceFor(ctx, root, nil, *noStore)
	if err != nil {
		return err
	}
	data, err := appflows.FlowImpactResults(ctx, root, set, resolved, records)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

// runGaps is `flows gaps` (AFU-V1-016, AFU-V1-150): the flows at HEAD without passing test
// evidence and, with a provider, the E2E inventory tests no flow variation links.
func runGaps(ctx context.Context, root string, args []string, out io.Writer) error {
	f := newFlags("flows gaps")
	dir := f.String("flows", "", "intent directory inside the root")
	evidence := repeatable(f, "evidence", "test-run-evidence/0 JSONL file")
	registry := f.String("registry", "", "committed flows-run-registry/0 stability policy")
	provider := f.String("provider", "", "repository-relative application-flow-selection-provider/1 file whose inventory is compared")
	noStore := f.Bool("no-store", false, "read only the named --evidence files, not the run evidence store")
	if f.Parse(args) != nil || *dir == "" || f.NArg() != 0 {
		return errors.New("flows gaps requires --flows DIR and optional --registry FILE, --provider FILE, --no-store and repeatable --evidence FILE")
	}
	if err := refuseUnreadableFlows(root); err != nil {
		return err
	}
	set, err := appflows.LoadIntentsAt(ctx, root, *dir, "HEAD")
	if err != nil {
		return err
	}
	records, _, err := appflows.RunEvidenceFor(ctx, root, *evidence, *noStore)
	if err != nil {
		return err
	}
	// AFU-V1-150: the E2E inventory is the provider's; without one the report says it is absent.
	var inventory *appflows.InventoryProvider
	if *provider != "" {
		if inventory, err = appflows.ReadInventoryProvider(root, *provider, *dir); err != nil {
			return err
		}
	}
	data, err := appflows.FlowGaps(ctx, root, set, records, *registry, inventory)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
