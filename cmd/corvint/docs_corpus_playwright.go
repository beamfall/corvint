package main

import (
	"context"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// playwrightCorpusOptions holds the caller-run Playwright inputs of the discovery producer
// (DCP-V1-046..048). Corvint reads these files and never runs Playwright.
type playwrightCorpusOptions struct {
	migration, config, listing, receipt string
}

func (p *playwrightCorpusOptions) addFlags(flags map[string]*string) {
	flags["--migration"] = &p.migration
	flags["--config"] = &p.config
	flags["--playwright-list"] = &p.listing
	flags["--receipt"] = &p.receipt
}

func compilePlaywrightCorpus(ctx context.Context, o corpusOptions) ([]byte, error) {
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
