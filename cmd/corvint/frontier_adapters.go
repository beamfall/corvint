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
// The adapter is the stable viewer: it runs frontierrepo.RecomputeStable once
// per invocation over the repository root (V1-1304).
func init() {
	frontierAdapters = func(ctx context.Context, root string) frontier.StableViewer {
		return frontierrepo.New(ctx, root)
	}
}
