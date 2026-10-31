//go:build !darwin && !linux

package main

import (
	"context"
	"time"
)

const snapshotRefreshWorkerEnv = "CORVINT_INTERNAL_INDEX_REFRESH"
const snapshotRefreshWallBound = time.Minute

func startSnapshotRefresh(ctx context.Context, root string) string {
	if adapterGetenv(ctx, "CORVINT_INDEX_AUTO_REFRESH") == "0" {
		return "disabled"
	}
	return "failed to start"
}

func startSnapshotRefreshReporting(ctx context.Context, root string) (string, string) {
	return startSnapshotRefresh(ctx, root), ""
}

func runSnapshotRefreshWorker(arguments []string, wallBound time.Duration) int { return 2 }
