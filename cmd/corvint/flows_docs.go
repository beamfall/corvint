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
	maintenance := len(args) > 0 && args[0] == "maintain"
	if maintenance {
		args = args[1:]
	}
	f, dir, evidence, registry := flowQueryFlags("flows docs")
	var o appflows.DocOptions
	f.StringVar(&o.Page, "page", "", "rendered page, repository-relative")
	f.StringVar(&o.Claims, "claims", "", "flow-doc-claims/0 sidecar, repository-relative")
	f.StringVar(&o.DocsRoot, "docs-root", "", "hand-written Markdown root, repository-relative")
	preview := f.Bool("preview", false, "preview maintenance without writes")
	apply := f.Bool("apply", false, "apply the exact previewed maintenance proposal")
	expected := f.String("expected-proposal", "", "SHA-256 of the previewed proposal")
	receipt := f.String("receipt", "", "committed maintenance receipt, repository-relative")
	adopt := f.Bool("adopt", false, "explicitly adopt a committed legacy generated pair; historical provenance UNKNOWN")
	check := f.Bool("check", false, "compare the committed page and sidecar with regeneration")
	waivers := f.String("waivers", "", "committed flow-doc-waivers/0 file, repository-relative")
	if f.Parse(args) != nil || *dir == "" || o.Page == "" || o.Claims == "" || f.NArg() != 0 || (*waivers != "" && !*check) {
		return errors.New("flows docs requires --flows DIR --page FILE --claims FILE, optional --registry FILE, --docs-root DIR and repeatable --evidence FILE, and --waivers FILE only with --check")
	}
	if maintenance {
		if *preview == *apply || *check || *waivers != "" || (*preview && *expected != "") || (*apply && *expected == "") {
			return errors.New("flows docs maintain needs --preview or --apply --expected-proposal SHA256")
		}
		o.Registry = *registry
		options := appflows.DocMaintenanceOptions{Flows: *dir, Docs: o, EvidenceFiles: *evidence, Receipt: *receipt, Adopt: *adopt}
		var data []byte
		var err error
		if *preview {
			var proposal appflows.DocProposal
			proposal, err = appflows.PreviewDocMaintenance(ctx, root, options)
			if err == nil {
				data, err = doccorpus.Encode(proposal)
			}
		} else {
			data, err = appflows.ApplyDocMaintenance(ctx, root, options, *expected)
		}
		if err != nil {
			return err
		}
		_, err = out.Write(data)
		if err != nil && *apply {
			return fmt.Errorf("maintenance outputs may be published but receipt delivery failed; retain recovery paths and explicitly adopt the unreceipted pair: %w", err)
		}
		return err
	}
	if *preview || *apply || *expected != "" || *receipt != "" || *adopt {
		return errors.New("maintenance flags require flows docs maintain")
	}
	set, err := appflows.LoadIntentsAt(ctx, root, *dir, "HEAD")
	if err != nil {
		return err
	}
	if o.Evidence, err = appflows.ReadRunEvidence(*evidence); err != nil {
		return err
	}
	o.Registry = *registry
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
