package companionrelease

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPUBV0024InstalledCoreDiscoveryWorkflows(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	binary := filepath.Join(root, "corvint")
	goPath, err := goBinary()
	if err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := runCaptured(context.Background(), repository, closedGoEnv(filepath.Join(root, "go-home"), ""), buildTimeout,
		goPath, "build", "-trimpath", "-buildvcs=false", "-ldflags=-X main.build=1", "-o", binary, "./cmd/corvint"); err != nil {
		t.Fatalf("build installed fixture: %v: %s", err, stderr)
	}
	steps, err := checkCoreDiscoveryWorkflows(context.Background(), binary, filepath.Join(root, "scratch"))
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 5 {
		t.Fatalf("got %d steps, want 5", len(steps))
	}
	for _, observed := range steps {
		if !observed.OK {
			t.Fatalf("%s failed: %s", observed.Name, observed.Detail)
		}
	}
	if _, err := os.Stat(binary); err != nil {
		t.Fatal(err)
	}
}
