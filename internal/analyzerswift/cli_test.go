package analyzerswift

import (
	"bytes"
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestBuiltCLIExactLFResponses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows deliberately rejects descriptor acquisition")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "corvint-analyzer-swift")
	build := exec.Command("go", "build", "-o", binary, "./cmd/corvint-analyzer-swift")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	request := fixtureRequest(t, "real-cli")
	cases := []struct {
		name string
		wire []byte
		want int
	}{
		{"success", requestBytes(t, request), 0},
		{"digest-mismatch", requestBytes(t, mutateDigest(request)), 1},
		{"exact-binding", requestBytes(t, mutateSource(request)), 1},
		{"noncanonical", []byte("{\n"), 1},
	}
	assertFreshCLIReplays(t, binary)
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "request.json")
			if err := os.WriteFile(path, test.wire, 0o600); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(binary, "--request-file", path)
			got, err := command.Output()
			if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() != test.want {
				t.Fatalf("exit=%d want=%d stderr=%s", exit.ExitCode(), test.want, exit.Stderr)
			}
			if err == nil && test.want != 0 {
				t.Fatalf("exit=0 want=%d", test.want)
			}
			want := Analyze(bytes.NewReader(test.wire))
			if !bytes.Equal(got, want) || len(got) == 0 || got[len(got)-1] != '\n' {
				t.Fatalf("exact CLI bytes mismatch\nwant=%q\ngot=%q", want, got)
			}
		})
	}
	oversized := filepath.Join(t.TempDir(), "oversized.json")
	if err := os.WriteFile(oversized, bytes.Repeat([]byte{'x'}, maxWire+1), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "--request-file", oversized)
	got, err := command.Output()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatalf("oversized exit=%v", err)
	}
	if !bytes.Equal(got, sentinel) {
		t.Fatalf("oversized=%q", got)
	}
}

func mutateDigest(request Request) Request {
	request = cloneRequest(request)
	request.Inputs[2].SHA256 = "sha256:" + strings.Repeat("0", 64)
	return request
}

func mutateSource(request Request) Request {
	request = cloneRequest(request)
	request.Inputs[1] = fixtureInput("apple-source", "swift.source", "Sources/BeamfallA11y/A11yID.swift", []byte("import SwiftUI\n"))
	return request
}

func assertFreshCLIReplays(t *testing.T, binary string) {
	t.Helper()
	request := fixtureRequest(t, "fresh-identical")
	wire := requestBytes(t, request)
	want := Analyze(bytes.NewReader(wire))
	path := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(path, wire, 0o600); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 1_000; index++ {
		got, exit := runBuiltCLI(t, binary, path)
		if exit != 0 || !bytes.Equal(got, want) {
			t.Fatalf("identical replay=%d exit=%d got=%q want=%q", index, exit, got, want)
		}
	}
	seen := make(map[[32]byte]struct{}, 1_001)
	for index := 0; index < 1_001; index++ {
		request := cloneRequest(request)
		request.RequestID = "u" + strings.Repeat("0", 4-len(strconv.Itoa(index))) + strconv.Itoa(index)
		wire := requestBytes(t, request)
		path := filepath.Join(t.TempDir(), "request.json")
		if err := os.WriteFile(path, wire, 0o600); err != nil {
			t.Fatal(err)
		}
		want := Analyze(bytes.NewReader(wire))
		if !bytes.Contains(want, []byte(`"status":"CANDIDATE"`)) {
			_, _, err := parseCanonicalRequest(wire)
			t.Fatalf("unique replay=%d did not construct a successful request: response=%q parse=%v", index, want, err)
		}
		got, exit := runBuiltCLI(t, binary, path)
		if exit != 0 || !bytes.Equal(got, want) {
			t.Fatalf("unique replay=%d exit=%d got=%q want=%q", index, exit, got, want)
		}
		digest := sha256.Sum256(got)
		if _, duplicate := seen[digest]; duplicate {
			t.Fatalf("unique replay=%d duplicate output", index)
		}
		seen[digest] = struct{}{}
	}
}

func runBuiltCLI(t testing.TB, binary, path string) ([]byte, int) {
	t.Helper()
	command := exec.Command(binary, "--request-file", path)
	got, err := command.Output()
	if err == nil {
		return got, 0
	}
	exit, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatal(err)
	}
	return got, exit.ExitCode()
}
