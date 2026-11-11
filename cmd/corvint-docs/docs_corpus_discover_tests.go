package main

import (
	"context"

	"github.com/Beamfall/corvint/internal/testdiscovery"
)

// compileTestDiscovery prints the static corvint-test-discovery/1 record of the Playwright or
// RSpec test files under --path at --revision (DCX-V1-370..376). It reads Git objects only.
func compileTestDiscovery(ctx context.Context, o corpusOptions) ([]byte, error) {
	return testdiscovery.BuildTestDiscovery(ctx, testdiscovery.TestDiscoveryInput{Root: o.root, Runner: o.runner, Revision: o.revision, Path: o.path})
}
