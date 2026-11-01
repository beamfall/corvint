package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AFP-V0-871 (V1-4112): when stdout and stderr are one file or pipe, as after
// 2>&1, a stale binary's affected receipt still parses as one JSON document
// and carries the degradation; separate streams keep the AFP-V0-846 stderr
// line, and --format text names the degradation on its own line.
func TestAffectedStaleBinaryMergedStreamStillParses(t *testing.T) {
	root, built := selfBinaryRepository(t)
	stubSelfBuildInfo(t, selfBinaryInfo("example.test/self", built))
	commitSelfBinaryFiles(t, root, map[string]string{"self.go": "package self\n\nfunc Self() int { return 2 }\n"})

	for _, options := range [][]string{nil, {"--full"}, {"--by-path", "--path", "self.go"}} {
		merged, err := os.Create(filepath.Join(t.TempDir(), "merged"))
		if err != nil {
			t.Fatal(err)
		}
		code := runContext(context.Background(), append([]string{"--root", root, "affected"}, options...), strings.NewReader(""), merged, merged)
		_ = merged.Close()
		output, err := os.ReadFile(merged.Name())
		if err != nil {
			t.Fatal(err)
		}
		var receipt map[string]any
		if code != 0 || json.Unmarshal(output, &receipt) != nil {
			t.Fatalf("%v: merged output is not one JSON document (exit %d):\n%s", options, code, output)
		}
		if degradations, _ := receipt["degradations"].([]any); len(degradations) != 1 || degradations[0] != selfBinaryStaleCode {
			t.Fatalf("%v: merged receipt lost the degradation: %v", options, receipt["degradations"])
		}
	}

	// Separate streams: the stderr line stays (AFP-V0-846).
	_, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 || !strings.Contains(stderr, "corvint: DEGRADED self-binary-stale") {
		t.Fatalf("separate streams lost the stderr line: exit %d %s", code, stderr)
	}
	merged, err := os.Create(filepath.Join(t.TempDir(), "text"))
	if err != nil {
		t.Fatal(err)
	}
	code = runContext(context.Background(), []string{"--root", root, "affected", "--format", "text"}, strings.NewReader(""), merged, merged)
	_ = merged.Close()
	text, _ := os.ReadFile(merged.Name())
	if code != 0 || !strings.Contains(string(text), "\ndegraded self-binary-stale\n") || strings.Contains(string(text), "DEGRADED") {
		t.Fatalf("merged text output: exit %d\n%s", code, text)
	}
}

// AFP-V0-871: only one pipe or regular file behind both streams is shared; a
// terminal, two files or a non-file writer is not.
func TestSharedOutputStream(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	if !sharedOutputStream(writer, writer) {
		t.Fatal("one pipe on both streams is not shared")
	}
	other, otherWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	defer otherWriter.Close()
	if sharedOutputStream(writer, otherWriter) {
		t.Fatal("two pipes are shared")
	}
	if sharedOutputStream(&strings.Builder{}, writer) {
		t.Fatal("a non-file writer is shared")
	}
	if tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0); err == nil {
		defer tty.Close()
		if sharedOutputStream(tty, tty) {
			t.Fatal("a terminal is shared")
		}
	}
}
