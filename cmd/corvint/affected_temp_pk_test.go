// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitstatus"
)

// TSP-V0-7301: an actual scratch creation failure has a typed, actionable CLI error.
func TestAffectedTempFailureNamesPathAndRemedy(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	missing := filepath.Join(t.TempDir(), "missing")
	_, err := gitstatus.StatusIn(context.Background(), root, missing, 1<<20, nil, "status", "--porcelain=v1", "-z")
	if err == nil {
		t.Fatal("scratch creation unexpectedly succeeded")
	}
	var stderr bytes.Buffer
	emitError(&stderr, affectedStatusError(err))
	if !strings.Contains(stderr.String(), "unsupported-affected-temp") || !strings.Contains(stderr.String(), "TMPDIR") || !strings.Contains(stderr.String(), "scratch") {
		t.Fatalf("unhelpful error: %s", &stderr)
	}
}

// TSP-V0-7301: the operator-selected temporary path is retained, not just a generic remedy.
func TestAffectedTempFailureReportsSelectedTMPDIR(t *testing.T) {
	root := affectedFixtureRepository(t)
	missing := filepath.Join(t.TempDir(), "unavailable-scratch")
	t.Setenv("TMPDIR", missing)
	_, err := gitstatus.StatusIn(context.Background(), root, missing, 1<<20, nil, "status", "--porcelain=v1", "-z")
	if err == nil {
		t.Fatal("scratch creation unexpectedly succeeded")
	}
	var stderr bytes.Buffer
	emitError(&stderr, affectedStatusError(err))
	if !strings.Contains(stderr.String(), missing) || !strings.Contains(stderr.String(), "unsupported-affected-temp") || !strings.Contains(stderr.String(), "set TMPDIR") {
		t.Fatalf("path or recovery missing: %s", &stderr)
	}
}
