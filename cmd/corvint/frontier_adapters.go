// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"context"

	"github.com/Beamfall/corvint/internal/frontier"
	"github.com/Beamfall/corvint/internal/frontierrepo"
)

// This file is the ONLY place the command names a repository-backed adapter.
// It exists so cmd/corvint/frontier.go stays free of Git and of internal
// verifier types: CF-V0-026 keeps repository access and path acquisition
// outside the Frontier wire and verifier, and the command surface honours that
// by holding a seam rather than an implementation.
//
// One `frontierrepo.Adapter` satisfies both seams because CF-V0-002 and
// CF-V0-021 step 5 require ONE shared OCM-consuming verification call per
// invocation: the adapter memoizes that single accepted verification, so the
// call Frontier makes at cascade stage 5 is the same one TCQ consumes at
// stage 7. Handing back two independent objects would silently verify twice.
func init() {
	frontierAdapters = func(ctx context.Context, root string) (frontier.Verifier, frontier.TCQRecomputer) {
		adapter := frontierrepo.New(ctx, root)
		return adapter, adapter
	}
}
