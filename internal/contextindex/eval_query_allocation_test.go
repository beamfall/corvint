//go:build !race

package contextindex

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// evalQueryAllocationRepository is shaped for the ranking half rather than the
// build: many small declarations spread over many files, so every ranked symbol
// carries a real context window and the per-symbol tokenisation dominates. The
// build fixtures do the opposite -- a few huge blobs, almost no symbols -- which
// is why TestBuildQuerySelectiveAllocationRatchet cannot see this cost at all.
func evalQueryAllocationRepository(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	benchmarkGit(t, root, "init", "-q")
	benchmarkGit(t, root, "config", "user.email", "corvint@example.test")
	benchmarkGit(t, root, "config", "user.name", "Corvint Test")
	benchmarkWriteFile(t, root, "go.mod", "module example.test/ratchet\n\ngo 1.27.0\n")
	for file := 0; file < 40; file++ {
		var source strings.Builder
		source.WriteString("package corpus\n\n")
		for declaration := 0; declaration < 25; declaration++ {
			fmt.Fprintf(&source, "// EnforceSessionRevocation%02d revokes an expired device session and\n", declaration)
			fmt.Fprintf(&source, "// enforces future denial for the heartbeat the session wrote.\n")
			fmt.Fprintf(&source, "func EnforceSessionRevocation%02d(deviceSession, heartbeatWriter string) bool {\n", declaration)
			for line := 0; line < 18; line++ {
				fmt.Fprintf(&source, "\tsessionRevocationStep%02d := deviceSession + heartbeatWriter // expiry enforcement %02d\n", line, line)
				fmt.Fprintf(&source, "\t_ = sessionRevocationStep%02d\n", line)
			}
			source.WriteString("\treturn true\n}\n\n")
		}
		benchmarkWriteFile(t, root, fmt.Sprintf("internal/corpus/file%02d.go", file), source.String())
	}
	benchmarkGit(t, root, "add", ".")
	benchmarkGit(t, root, "commit", "-qm", "eval ranking corpus")
	return root
}

// TestEvalQueryAllocationRatchet pins the ranking half of the user-prompt event,
// which is where the keep-set term construction removed its cost. The index is
// built once outside the timed loop, so the bound measures EvalQuery alone.
//
// Measured 2026-08-29 on go1.27.0 darwin/arm64: 12,461,000 bytes/op and 131,772
// allocs/op, stable to ten bytes across runs, against 43,171,000 and 236,780 at
// the frozen base. The bounds below sit ~15% above the measured candidate --
// loose enough for host variance, tight enough that restoring either unfiltered
// construction breaches the byte bound by 3x.
func TestEvalQueryAllocationRatchet(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("clean-file streaming verification is Unix-only")
	}
	root := evalQueryAllocationRepository(t)
	index, err := BuildEval(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	result := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for iteration := 0; iteration < b.N; iteration++ {
			budget := 7_500
			ranked, queryErr := EvalQuery(context.Background(), index, "session expiry device revocation enforcement heartbeat", 10, &budget)
			if queryErr != nil {
				b.Fatal(queryErr)
			}
			runtime.KeepAlive(ranked)
		}
	})
	if bytes := result.AllocedBytesPerOp(); bytes > 14_500_000 {
		t.Fatalf("EvalQuery allocation bytes = %d, want <= 14500000", bytes)
	}
	if allocations := result.AllocsPerOp(); allocations > 152_000 {
		t.Fatalf("EvalQuery allocations/op = %d, want <= 152000", allocations)
	}
	t.Logf("EvalQuery: %d bytes/op, %d allocs/op", result.AllocedBytesPerOp(), result.AllocsPerOp())
}
