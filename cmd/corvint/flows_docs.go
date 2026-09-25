package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// flowDocsCheckFailed is the error code of a `flows docs --check` that reports a failure (AFU-V1-032).
const flowDocsCheckFailed = "flow-docs-check-failed"

func runFlowsDocs(ctx context.Context, root string, args []string, out io.Writer) error {
	f, dir, evidence := flowQueryFlags("flows docs")
	var o appflows.DocOptions
	f.StringVar(&o.Page, "page", "", "rendered page, repository-relative")
	f.StringVar(&o.Claims, "claims", "", "flow-doc-claims/0 sidecar, repository-relative")
	f.StringVar(&o.DocsRoot, "docs-root", "", "hand-written Markdown root, repository-relative")
	check := f.Bool("check", false, "compare the committed page and sidecar with regeneration")
	waivers := f.String("waivers", "", "committed flow-doc-waivers/0 file, repository-relative")
	if f.Parse(args) != nil || *dir == "" || o.Page == "" || o.Claims == "" || f.NArg() != 0 || (*waivers != "" && !*check) {
		return errors.New("flows docs requires --flows DIR --page FILE --claims FILE, optional --docs-root DIR and repeatable --evidence FILE, and --waivers FILE only with --check")
	}
	set, err := appflows.LoadIntentsAt(ctx, root, *dir, "HEAD")
	if err != nil {
		return err
	}
	if o.Evidence, err = appflows.ReadRunEvidence(*evidence); err != nil {
		return err
	}
	if *check {
		return flowDocsCheck(ctx, root, set, o, *waivers, out)
	}
	page, claims, err := appflows.RenderDocs(ctx, root, set, o)
	if err != nil {
		return err
	}
	if err = appflows.WriteDocs(root, o, page, claims); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "%s\n%s\n", o.Page, o.Claims)
	return err
}

// flowDocsCheck writes the flow-doc-check/0 report and exits nonzero when it fails; it writes nothing
// to the repository.
func flowDocsCheck(ctx context.Context, root string, set appflows.IntentSet, o appflows.DocOptions, waivers string, out io.Writer) error {
	report, err := appflows.CheckDocs(ctx, root, set, o, waivers, time.Now().UTC().Format(time.DateOnly))
	if err != nil {
		return err
	}
	data, err := doccorpus.Encode(report)
	if err != nil {
		return err
	}
	if _, err = out.Write(data); err != nil {
		return err
	}
	if report.Status != "pass" {
		return &gokernel.Error{Code: flowDocsCheckFailed, Message: fmt.Sprintf("flows docs --check found %d failure(s)", len(report.Failures))}
	}
	return nil
}
