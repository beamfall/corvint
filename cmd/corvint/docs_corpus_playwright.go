package main

import (
	"context"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// playwrightCorpusOptions holds the caller-run Playwright inputs of the discovery and witness
// producers (DCP-V1-046..050). Corvint reads these files and never runs Playwright.
type playwrightCorpusOptions struct {
	migration, config, listing, receipt, receiptInput, report string
}

func (p *playwrightCorpusOptions) addFlags(flags map[string]*string) {
	flags["--migration"] = &p.migration
	flags["--config"] = &p.config
	flags["--playwright-list"] = &p.listing
	flags["--receipt"] = &p.receipt
	flags["--receipt-input"] = &p.receiptInput
	flags["--report"] = &p.report
}

func compilePlaywrightCorpus(ctx context.Context, o corpusOptions) ([]byte, error) {
	if o.op == "discover-playwright" {
		migration, err := doccorpus.ReadFile(o.root, o.playwright.migration)
		if err != nil {
			return nil, err
		}
		listing, err := doccorpus.ReadPlaywrightFile(o.root, o.playwright.listing)
		if err != nil {
			return nil, err
		}
		var receipt []byte
		if o.playwright.receipt != "" {
			if receipt, err = doccorpus.ReadFile(o.root, o.playwright.receipt); err != nil {
				return nil, err
			}
		}
		return doccorpus.BuildPlaywrightDiscovery(ctx, doccorpus.PlaywrightDiscoveryInput{Root: o.root, Migration: migration, ConfigPath: o.playwright.config, Listing: listing, Receipt: receipt})
	}
	request, err := doccorpus.ReadFile(o.root, o.input)
	if err != nil {
		return nil, err
	}
	report, err := doccorpus.ReadPlaywrightFile(o.root, o.playwright.report)
	if err != nil {
		return nil, err
	}
	return doccorpus.ImportPlaywrightWitnesses(request, o.playwright.receiptInput, report)
}
