// Copyright 2026 Corvint contributors.
// SPDX-License-Identifier: AGPL-3.0-or-later
package postmergehost

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/postmergeproof"
	"github.com/Beamfall/corvint/internal/postmergeproof/procfs"
)

func expectProcessCode(t *testing.T, err error, outcome, code string) {
	t.Helper()
	var refusal *postmergeproof.ProcessErrorV2
	if errors.As(err, &refusal) && refusal.Outcome == outcome && refusal.Code == code {
		return
	}
	var unsupported procfs.UnsupportedError
	if errors.As(err, &unsupported) && unsupported.Outcome() == outcome && unsupported.Code() == code {
		return
	}
	t.Fatalf("got %v, want %s %s", err, outcome, code)
}

func TestProcessCollectorV2UnsupportedHostIsNotObserved(t *testing.T) {
	if procfs.Supported {
		t.Skip("this host has the Linux procfs profile")
	}
	_, err := NewProcessCollectorV2(context.Background(), "x", "g", "p", ObserverCommandV2{Path: "/observer"}, nil)
	expectProcessCode(t, err, "NOT_OBSERVED", "process-observation-unsupported")
	err = RunProcessObserverV2(context.Background(), []string{"trusted-start"}, strings.NewReader(""), &strings.Builder{})
	expectProcessCode(t, err, "NOT_OBSERVED", "process-observation-unsupported")
}

func TestInvocationEnvironmentV2Allowlist(t *testing.T) {
	got, err := invocationEnvironmentV2([]string{"TZ=UTC", "LANG=C", "PATH=/bin"})
	if err != nil || !slices.Equal(got, []string{"LANG=C", "PATH=/bin", "TZ=UTC"}) {
		t.Fatalf("allowlisted environment: %v %v", got, err)
	}
	for _, env := range [][]string{{"HOME=/root"}, {"PATH=/bin", "PATH=/usr/bin"}, {"PATH"}, {"GITHUB_TOKEN=x"}, {"TZ=a\x00b"}} {
		_, err := invocationEnvironmentV2(env)
		expectProcessCode(t, err, "BLOCKED", "process-collection-refused")
	}
}
