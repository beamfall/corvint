package companionrelease

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// companionGenSource builds a throwaway generator inside a materialized,
// exact copy of the corvint-taskman source tree. It calls the module's own
// fixture.QueueBytes/PolicyBytes — the same encoder corvint-taskman's CLI
// test suite trusts — rather than hand-transcribing the wire schema here,
// so a schema change in corvint-taskman cannot silently desync this smoke
// fixture from the real product.
const companionGenSource = `package main

import (
	"os"

	"github.com/Beamfall/corvint-tasks/internal/fixture"
)

func main() {
	if len(os.Args) != 3 {
		os.Exit(2)
	}
	if err := os.WriteFile(os.Args[1], fixture.QueueBytes(), 0o600); err != nil {
		panic(err)
	}
	if err := os.WriteFile(os.Args[2], fixture.PolicyBytes(), 0o600); err != nil {
		panic(err)
	}
}
`

// generateQueuePolicy materializes taskmanExport's exact, hash-verified
// source into scratch, adds companionGenSource as a new package (never
// written back into any checkout), builds it, and runs it to obtain a
// smoke-test queue.json/policy.json pair. It touches no caller-supplied
// checkout: everything happens under scratch.
func generateQueuePolicy(ctx context.Context, taskmanExport Export, scratch string) (queuePolicyFiles, error) {
	srcDir := filepath.Join(scratch, "taskman-smoke-src")
	if err := materializeExport(taskmanExport, srcDir); err != nil {
		return queuePolicyFiles{}, fmt.Errorf("materialize taskman source for smoke fixtures: %w", err)
	}
	genDir := filepath.Join(srcDir, "cmd", "companiongen")
	if err := os.MkdirAll(genDir, 0o700); err != nil {
		return queuePolicyFiles{}, err
	}
	if err := os.WriteFile(filepath.Join(genDir, "main.go"), []byte(companionGenSource), 0o600); err != nil {
		return queuePolicyFiles{}, err
	}

	goPath, err := goBinary()
	if err != nil {
		return queuePolicyFiles{}, err
	}
	home := filepath.Join(scratch, "companiongen-home")
	if err := mkdirScratchDir(home); err != nil {
		return queuePolicyFiles{}, err
	}
	genBin := filepath.Join(scratch, "companiongen")
	if _, stderr, err := runCaptured(ctx, srcDir, closedGoEnv(home, ""), buildTimeout,
		goPath, "build", "-o", genBin, "./cmd/companiongen"); err != nil {
		return queuePolicyFiles{}, fmt.Errorf("build smoke fixture generator: %w (stderr=%s)", err, trimForError(stderr))
	}

	queueOut := filepath.Join(scratch, "smoke-queue.json")
	policyOut := filepath.Join(scratch, "smoke-policy.json")
	if _, stderr, err := runCaptured(ctx, scratch, minimalRunEnv(home), subprocessTimeout,
		genBin, queueOut, policyOut); err != nil {
		return queuePolicyFiles{}, fmt.Errorf("run smoke fixture generator: %w (stderr=%s)", err, trimForError(stderr))
	}

	queue, err := os.ReadFile(queueOut)
	if err != nil {
		return queuePolicyFiles{}, err
	}
	policy, err := os.ReadFile(policyOut)
	if err != nil {
		return queuePolicyFiles{}, err
	}
	return queuePolicyFiles{Queue: queue, Policy: policy}, nil
}

// materializeExport writes an Export's hash-verified files to disk under
// dir, preserving their recorded mode bits.
func materializeExport(export Export, dir string) error {
	for _, f := range export.Files {
		full := filepath.Join(dir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if f.Mode == "100755" {
			mode = 0o755
		}
		if err := os.WriteFile(full, f.Data, mode); err != nil {
			return err
		}
	}
	return nil
}
