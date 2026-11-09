// SPDX-License-Identifier: Apache-2.0
package closureguard

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// RSP-V0-020: the core executable includes every read command. Its entire
// dependency closure must exclude the isolated tracing package in this slice,
// and the tracing package must not link testevidence. Moved from
// internal/proctrace (V1-5071), which keeps the Apache-2.0 terms of its source.
func TestRSPV0020ImportDirection(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "list", "-f", `{{.ImportPath}} {{join .Deps " "}}`, "./cmd/corvint", "./internal/proctrace")
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(rows) != 2 {
		t.Fatalf("missing dependency closure: %s", output)
	}
	for _, p := range strings.Fields(rows[0]) {
		if p == "github.com/Beamfall/corvint/internal/proctrace" {
			t.Fatalf("forbidden dependency: %s", p)
		}
	}
	for _, p := range strings.Fields(rows[1]) {
		if p == "github.com/Beamfall/corvint/internal/testevidence" {
			t.Fatalf("forbidden dependency: %s", p)
		}
	}
}
