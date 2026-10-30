package witness

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// PairedProfile identifies a report over one application range and one tests
// range compiled as a single logical change (AGW-V0-013).
const PairedProfile = "corvint-witness-paired/0"

// The two range names a paired report accepts, in report order.
const (
	AppRange   = "app"
	TestsRange = "tests"
)

// crossRangeSourceName names a tests-range document cited on application
// obligations (AGW-V0-014).
const crossRangeSourceName = "cross-range"

// PairedInput is one side of a paired report: the checkout root its commits
// were resolved in, the index built at its head, and its single-range options.
type PairedInput struct {
	Root    string
	Index   *contextindex.Index
	Options Options
}

// NamedReport is one range of a paired report.
type NamedReport struct {
	Name   string  `json:"name"`
	Root   string  `json:"root"`
	Report *Report `json:"report"`
}

// PairedReport carries both ranges' complete single-range reports, app first,
// and the headline count over the obligations of both. Pairing is present
// only when --paired-from read the ranges from a trailer (AGW-V0-036).
type PairedReport struct {
	Profile string        `json:"profile"`
	Pairing *Pairing      `json:"pairing,omitempty"`
	Ranges  []NamedReport `json:"ranges"`
	Summary Summary       `json:"summary"`
}

// receiptBinding is a Playwright receipt bound to its own range's head tree,
// with the paths resolved there, kept for paired-range citation.
type receiptBinding struct {
	path    string
	receipt *jstestprovider.Receipt
	paths   evidencePaths
}

// CompilePaired compiles the application range and the tests range as one
// logical change (AGW-V0-013). Caller-supplied test-run documents belong to
// the tests range; each Playwright receipt bound to the tests head is then
// checked against the application head and, only when its application
// attestation and file digests prove the run exercised exactly that tree,
// cites application obligations as non-closing CALLER_REPORTED witnesses
// (AGW-V0-014). It only reads.
func CompilePaired(ctx context.Context, app, tests PairedInput) (*PairedReport, error) {
	if len(app.Options.Evidence)+len(app.Options.RepositoryEvidence) != 0 {
		return nil, errors.New("paired ranges read test-run evidence only for the tests range")
	}
	if app.Options.CEMPath != "" || tests.Options.CEMPath != "" || app.Options.StableArtifacts != "" || tests.Options.StableArtifacts != "" {
		return nil, errors.New("paired ranges read each range's committed Change Evidence Map; --cem and --stable-artifact-root name one repository and are refused")
	}
	appReport, _, err := compile(ctx, app.Index, app.Options)
	if err != nil {
		return nil, fmt.Errorf("--range %s: %w", AppRange, err)
	}
	testsReport, receipts, err := compile(ctx, tests.Index, tests.Options)
	if err != nil {
		return nil, fmt.Errorf("--range %s: %w", TestsRange, err)
	}
	budget := gitrun.NewDefaultBudget()
	for _, bound := range receipts {
		source, cited := bindCrossRange(ctx, budget, app.Index, bound)
		source.Basis, source.Unattached = attachEvidenceAs(appReport.Obligations, cited, crossRangeSourceName)
		appReport.Sources = append(appReport.Sources, source)
	}
	if len(receipts) != 0 {
		decide(appReport.Obligations)
		sortObligations(appReport.Obligations)
		appReport.Summary = summarize(appReport.Obligations)
		appReport.Preconditions = preconditions(appReport)
	}
	all := append(append([]Obligation{}, appReport.Obligations...), testsReport.Obligations...)
	return &PairedReport{
		Profile: PairedProfile,
		Ranges: []NamedReport{
			{Name: AppRange, Root: app.Root, Report: appReport},
			{Name: TestsRange, Root: tests.Root, Report: testsReport},
		},
		Summary: summarize(all),
	}, nil
}

// bindCrossRange admits a tests-range receipt on the application range only
// when its application attestation records the same clean application
// repository identity before and after the run, with no failure, and that
// identity is the application head commit and tree; and every imported-file
// digest that resolves under the application checkout equals the application
// head blob. It then returns each test with only its imported application
// files; the anchor stays the tests-range path the test declares itself in.
// Every other case is UNBOUND and cites nothing.
func bindCrossRange(ctx context.Context, budget *gitrun.Budget, app *contextindex.Index, bound receiptBinding) (Source, []evidenceTest) {
	source := Source{Name: crossRangeSourceName, Path: bound.path, Hunks: len(bound.receipt.Tests)}
	source.Status, source.Detail = bindAttestation(bound.receipt.ApplicationAttestation, app.CommitRevision, app.Revision)
	if source.Status != "BOUND" {
		return source, nil
	}
	paths := resolveApplicationPaths(app, bound.receipt)
	if !paths.hasRoot {
		source.Status, source.Detail = "UNBOUND", "no single application checkout root is named by the receipt's absolute imported files, so none resolves to an application head path"
		return source, nil
	}
	if detail := compareDigests(ctx, budget, app, bound.receipt, paths); detail != "" {
		source.Status, source.Detail = "UNBOUND", detail
		return source, nil
	}
	cited := make([]evidenceTest, 0, len(bound.receipt.Tests))
	for _, test := range bound.receipt.Tests {
		anchor := ""
		if test.Anchor != nil {
			anchor = bound.paths.resolve(test.Anchor.File)
		}
		files := map[string]bool{}
		for file := range test.ImportedFiles {
			if relative := paths.resolve(file); relative != "" {
				files[relative] = true
			}
		}
		cited = append(cited, evidenceTest{name: test.FullName, state: string(test.State), anchor: anchor, files: sortedKeys(files)})
	}
	return source, cited
}

// bindAttestation names why an application attestation does not prove the
// run exercised exactly the application head, or BOUND.
func bindAttestation(attestation *jstestprovider.ApplicationAttestationReceipt, head, headTree string) (string, string) {
	switch {
	case attestation == nil:
		return "UNBOUND", "the receipt carries no application attestation, so no test in it can be bound to the application head"
	case len(attestation.Failures) != 0:
		return "UNBOUND", "the application attestation records failures: " + strings.Join(attestation.Failures, "; ")
	case attestation.Before == nil || attestation.After == nil:
		return "UNBOUND", "the application attestation does not record the application repository before and after the run"
	}
	before, after := attestation.Before.Attestation.Repository, attestation.After.Attestation.Repository
	switch {
	case before != after:
		return "UNBOUND", "the application repository identity changed between before and after the run"
	case before.DirtyState != "clean":
		return "UNBOUND", "the application repository was not clean during the run (dirtyState=" + before.DirtyState + ")"
	case before.Revision != head:
		return "UNBOUND", "the application attestation names revision " + before.Revision + ", not the application head " + head
	case before.Tree != headTree:
		return "UNBOUND", "the application attestation names tree " + before.Tree + ", not the application head tree " + headTree
	}
	return "BOUND", "tests-range run attested against the application head tree; CALLER_REPORTED never closes an obligation (CF-V0-014)"
}

// resolveApplicationPaths infers the application checkout root from the
// receipt's absolute imported files: the directory under which the most of
// them name a tracked application head path, a tie broken only by the local
// application root. Relative names resolve to nothing here, because a
// relative name in a tests-range receipt is relative to the tests checkout.
func resolveApplicationPaths(app *contextindex.Index, receipt *jstestprovider.Receipt) evidencePaths {
	paths := evidencePaths{tracked: app.Tracked, absoluteOnly: true}
	roots := map[string]int{}
	for _, test := range receipt.Tests {
		for file := range test.ImportedFiles {
			if !evidenceAbsolute(file) {
				continue
			}
			slash := filepath.ToSlash(file)
			for position := 0; position < len(slash); position++ {
				if slash[position] == '/' && paths.tracks(slash[position+1:]) {
					roots[slash[:position]]++
				}
			}
		}
	}
	local := filepath.ToSlash(app.Root)
	if resolved, err := filepath.EvalSymlinks(app.Root); err == nil {
		local = filepath.ToSlash(resolved)
	}
	paths.root, paths.hasRoot = mostVoted(roots, local)
	return paths
}

// RenderPaired prints both ranges by name, the headline count over both, and
// then each range's single-range report.
func RenderPaired(report *PairedReport) string {
	builder := &strings.Builder{}
	fmt.Fprintf(builder, "WITNESS %s\n", report.Profile)
	if report.Pairing != nil {
		fmt.Fprintf(builder, "pairing %s commit=%s repository=%s\n", report.Pairing.Source, report.Pairing.Commit, report.Pairing.Repository)
	}
	for _, item := range report.Ranges {
		fmt.Fprintf(builder, "range %s base=%s head=%s root=%s\n", item.Name, item.Report.Range.Base, item.Report.Range.Head, item.Root)
	}
	renderSummary(builder, &Report{Summary: report.Summary})
	for _, item := range report.Ranges {
		fmt.Fprintf(builder, "\nRANGE %s\n", item.Name)
		builder.WriteString(Render(item.Report))
	}
	return builder.String()
}
