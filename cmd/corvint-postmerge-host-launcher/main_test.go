// Copyright 2026 Corvint contributors.
// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestClosedLauncherInvocation(t *testing.T) {
	valid := []string{"--profile", "/profile", "--profile-sha256", strings.Repeat("a", 64), "--request", "/request", "--request-sha256", strings.Repeat("b", 64), "--out", "/fresh"}
	if _, err := parseOptions(valid); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, valid[:8], append(append([]string{}, valid...), "--engine", "tcp://remote"), {"--profile", "/p", "--profile", "/q", "--request", "/r", "--request-sha256", "x", "--out", "/o"}, {"--internal-envelope", "x", "--engine", "remote"}} {
		if _, err := parseOptions(args); err == nil {
			t.Fatal("accepted open invocation", args)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"--internal-envelope", "not-a-digest"}, io.NopCloser(strings.NewReader("execute\n")), &stdout, &stderr); code != 2 || stdout.Len() != 0 {
		t.Fatal("invalid internal mode ran")
	}
}
