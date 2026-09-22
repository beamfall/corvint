//go:build darwin || linux

package gitstatus

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestProbeReuseAnswersRepeatedProbesFromUnchangedMetadata pins the opt-in
// probe reuse (proposed GPK-V0-065): without WithProbeReuse every status read
// probes its private metadata again; with it, a read over the same captured
// bytes runs only `status`, and a read over changed config or index bytes
// probes again.
func TestProbeReuseAnswersRepeatedProbesFromUnchangedMetadata(t *testing.T) {
	root := fixture(t)
	config := filepath.Join(root, ".git", "config")
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	writeTest(t, config, string(data)+"# keep the real parser for comments\n")
	var calls []string
	run := func(ctx context.Context, directory string, limit int, args ...string) ([]byte, error) {
		calls = append(calls, args[len(args)-1])
		return testRun(ctx, directory, limit, args...)
	}
	args := []string{"status", "--porcelain=v1", "-z", "--untracked-files=all"}
	want := gitTest(t, root, args...)
	status := func(ctx context.Context) []string {
		calls = nil
		got, err := Status(ctx, root, metadataLimit, run, args...)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("private status=%q want=%q error=%v", got, want, err)
		}
		return calls
	}
	probed := []string{"--list", "-z", "--shared-index-path", "--ignore-submodules=all"}
	if got := status(context.Background()); !slices.Equal(got, probed) {
		t.Fatalf("first read ran %q, want %q", got, probed)
	}
	if got := status(context.Background()); !slices.Equal(got, probed) {
		t.Fatalf("read without reuse ran %q, want %q", got, probed)
	}
	reuse := WithProbeReuse(context.Background())
	if got := status(reuse); !slices.Equal(got, probed) {
		t.Fatalf("first reusing read ran %q, want %q", got, probed)
	}
	if got := status(reuse); !slices.Equal(got, probed[3:]) {
		t.Fatalf("reusing read over unchanged metadata ran %q, want %q", got, probed[3:])
	}
	if got := status(context.Background()); !slices.Equal(got, probed) {
		t.Fatalf("read without reuse after a remembered probe ran %q, want %q", got, probed)
	}
	writeTest(t, config, string(data)+"# a second comment changes the bytes\n")
	if got := status(reuse); !slices.Equal(got, probed) {
		t.Fatalf("reusing read over changed config bytes ran %q, want %q", got, probed)
	}
	writeTest(t, filepath.Join(root, "added.txt"), "staged\n")
	gitTest(t, root, "add", "added.txt")
	want = gitTest(t, root, args...)
	if got := status(reuse); !slices.Equal(got, probed[1:]) {
		t.Fatalf("reusing read over changed index bytes ran %q, want %q", got, probed[1:])
	}
}
