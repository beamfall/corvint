// SPDX-License-Identifier: Apache-2.0
package runnerprotocol

import (
	"context"
	"os"
	"testing"
)

// CMS-V0-010 / RSP-V0-063: per-run overhead of the out-of-process form. One
// iteration is one run: launch and hello, inventory, three commands, parse,
// staged evidence, shutdown, reap and both repository status snapshots.
func BenchmarkRSPV0063HostRun(b *testing.B) {
	pin, store, _ := hostPins(b)
	options := hostOptions(b, "reference", Limits{})
	c := corpus(b, options.RunDir)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h := OpenHost(context.Background(), HostInput{Required: []string{ReferenceRunner}, Plan: DiscoveryPlan{Pins: []Pin{pin}}, Store: store, Options: options})
		plugin, err := h.Plugin(ReferenceRunner)
		if err != nil {
			b.Fatal(err)
		}
		var lines []Line
		if _, err = Conform(context.Background(), plugin, c, func(ctx context.Context) error { lines = h.Close(ctx); return nil }); err != nil {
			b.Fatal(err)
		}
		if len(lines) != 1 || lines[0].Run != RunOK {
			b.Fatalf("lines=%+v", lines)
		}
	}
}

// The compiled-in twin over the same corpus: the in-process baseline.
func BenchmarkRSPV0063CompiledInRun(b *testing.B) {
	c := corpus(b, b.TempDir())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Conform(context.Background(), Reference{}, c, nil); err != nil {
			b.Fatal(err)
		}
	}
}

// One repository status snapshot of a real checkout (taken twice per session).
// Set CORVINT_RSP_OVERHEAD_REPO to the checkout; skipped otherwise.
func BenchmarkRSPV0029RepositorySnapshot(b *testing.B) {
	root := os.Getenv("CORVINT_RSP_OVERHEAD_REPO")
	if root == "" {
		b.Skip("CORVINT_RSP_OVERHEAD_REPO unset")
	}
	for i := 0; i < b.N; i++ {
		if _, err := repositorySnapshot(context.Background(), root); err != nil {
			b.Fatal(err)
		}
	}
}
