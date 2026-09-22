package analyzerkotlinandroid

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func buildCandidateCLI(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "corvint-analyzer-kotlin-android")
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	command := exec.Command(goBinary, "build", "-o", binary, "../../cmd/corvint-analyzer-kotlin-android")
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	return binary
}

func runCandidateCLI(t *testing.T, binary string, raw []byte, directory string, environment ...string) []byte {
	t.Helper()
	command := exec.Command(binary)
	command.Stdin = bytes.NewReader(raw)
	command.Dir = directory
	command.Env = append(os.Environ(), environment...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("run CLI: %v stderr=%s", err, stderr.String())
	}
	return stdout.Bytes()
}

func TestBuiltCLIFreshSuccessesAndOrderPermutations(t *testing.T) {
	binary := buildCandidateCLI(t)
	raw := canonical(t, pinned(t))
	want := runCandidateCLI(t, binary, raw, "")
	if got := reason(t, want); got != "NONE" {
		t.Fatalf("built CLI did not succeed: %q", got)
	}
	wantDigest := sha256.Sum256(want)
	for index := 0; index < 1_001; index++ {
		got := runCandidateCLI(t, binary, raw, "")
		if digest := sha256.Sum256(got); digest != wantDigest || !bytes.Equal(got, want) {
			t.Fatalf("fresh execution %d drifted", index)
		}
	}
	request := pinned(t)
	for shift := 1; shift < len(request.Inputs); shift++ {
		permuted := request
		permuted.Inputs = append(append([]Input(nil), request.Inputs[shift:]...), request.Inputs[:shift]...)
		output := runCandidateCLI(t, binary, canonical(t, permuted), "")
		if got := reason(t, output); got != "DUPLICATE_VALUE" {
			t.Fatalf("permutation %d reason=%q", shift, got)
		}
	}
}

func TestEveryClosedReasonAndCountBounds(t *testing.T) {
	unknown := string(canonical(t, pinned(t)))
	unknown = strings.TrimSuffix(unknown, "}\n") + `,"unexpected":true}` + "\n"
	ambiguous := pinned(t)
	ambiguous.Inputs = append(ambiguous.Inputs, input("m-extra", "android.gradle.build", "extra.gradle.kts", []byte("extra\n")))
	duplicate := pinned(t)
	duplicate.Inputs[0], duplicate.Inputs[1] = duplicate.Inputs[1], duplicate.Inputs[0]
	invalidID := pinned(t)
	invalidID.Target.OS = "android!"
	malformed := pinned(t)
	malformed.Inputs[0].ContentBase64 = "="
	digestMismatch := pinned(t)
	digestMismatch.Inputs[0].SHA256 = "sha256:" + strings.Repeat("0", 64)
	unsupported := pinned(t)
	unsupported.Inputs[0].Family = "android.unknown"
	unknownFamily := pinned(t)
	unknownFamily.Family = "unknown"
	nullFeatures := pinned(t)
	nullFeatures.Target.Features = nil
	oversized := pinned(t)
	oversized.Inputs[0].Handle = strings.Repeat("a", 1_100_308)
	cases := []struct {
		name   string
		raw    []byte
		reason string
	}{
		{"noncanonical", []byte("{}\n"), "NONCANONICAL_REQUEST"},
		{"unknown-field", []byte(unknown), "UNKNOWN_FIELD"},
		{"unknown-family", canonical(t, unknownFamily), "UNKNOWN_FAMILY"},
		{"invalid-identifier", canonical(t, invalidID), "INVALID_IDENTIFIER"},
		{"duplicate", canonical(t, duplicate), "DUPLICATE_VALUE"},
		{"malformed-input", canonical(t, malformed), "MALFORMED_INPUT"},
		{"unsupported-schema", canonical(t, unsupported), "UNSUPPORTED_SCHEMA"},
		{"digest-mismatch", canonical(t, digestMismatch), "DIGEST_MISMATCH"},
		{"ambiguous-binding", canonical(t, ambiguous), "AMBIGUOUS_BINDING"},
		{"exact-binding", canonical(t, func() Request { request := pinned(t); request.Target.OS = "linux"; return request }()), "EXACT_BINDING_UNAVAILABLE"},
		{"null-features", canonical(t, nullFeatures), "NONCANONICAL_REQUEST"},
		{"oversized-handle", canonical(t, oversized), "INVALID_IDENTIFIER"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			output := mustProcess(t, testCase.raw)
			if got := reason(t, output); got != testCase.reason || len(output) > MaxOutputBytes {
				t.Fatalf("reason=%q size=%d", got, len(output))
			}
		})
	}
	if got := reason(t, mustReader(t, &countingReader{Reader: strings.NewReader(strings.Repeat("x", MaxRequestBytes+1))})); got != "LIMIT_EXCEEDED" {
		t.Fatalf("reader reason=%q", got)
	}
	for _, count := range []int{127, 128, 129} {
		request := pinned(t)
		for index := len(request.Inputs); index < count; index++ {
			request.Inputs = append(request.Inputs, input(fmt.Sprintf("z-extra-%03d", index), "android.gradle.build", fmt.Sprintf("extra-%03d.gradle.kts", index), []byte("x\n")))
		}
		want := "AMBIGUOUS_BINDING"
		if count > 128 {
			want = "LIMIT_EXCEEDED"
		}
		if got := reason(t, mustProcess(t, canonical(t, request))); got != want {
			t.Fatalf("inputs=%d reason=%q", count, got)
		}
	}
	for _, count := range []int{63, 64, 65} {
		request := pinned(t)
		request.Target.Features = make([]string, count)
		for index := range request.Target.Features {
			request.Target.Features[index] = fmt.Sprintf("feature-%03d", index)
		}
		want := "EXACT_BINDING_UNAVAILABLE"
		if count > 64 {
			want = "LIMIT_EXCEEDED"
		}
		if got := reason(t, mustProcess(t, canonical(t, request))); got != want {
			t.Fatalf("features=%d reason=%q", count, got)
		}
	}
	base := pinned(t)
	total := 0
	for _, item := range base.Inputs {
		var content []byte
		if err := json.Unmarshal([]byte(`"`+item.ContentBase64+`"`), &content); err != nil {
			t.Fatal(err)
		}
		total += len(content)
	}
	for _, size := range []int{MaxInputBytes - total - 1, MaxInputBytes - total, MaxInputBytes - total + 1} {
		request := pinned(t)
		request.Inputs = append(request.Inputs, input("m-content", "android.gradle.build", "content.gradle.kts", bytes.Repeat([]byte{'x'}, size)))
		want := "AMBIGUOUS_BINDING"
		if size > MaxInputBytes-total {
			want = "LIMIT_EXCEEDED"
		}
		if got := reason(t, mustProcess(t, canonical(t, request))); got != want {
			t.Fatalf("content=%d reason=%q", size, got)
		}
	}
}

func TestJSONDepthTokenAndStringBoundsUnderAtAndOver(t *testing.T) {
	for _, depth := range []int{7, 8, 9} {
		raw := strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth)
		want := ""
		if depth > 8 {
			want = "LIMIT_EXCEEDED"
		}
		if got := scanCanonicalJSON([]byte(raw)); got != want {
			t.Fatalf("depth=%d got=%q want=%q", depth, got, want)
		}
	}
	for _, tokens := range []int{4095, 4096, 4097} {
		raw := "[" + strings.Repeat("0,", tokens-1) + "0]"
		want := ""
		if tokens > 4096 {
			want = "LIMIT_EXCEEDED"
		}
		if got := scanCanonicalJSON([]byte(raw)); got != want {
			t.Fatalf("tokens=%d got=%q want=%q", tokens, got, want)
		}
	}
	for _, size := range []int{1_398_103, 1_398_104, 1_398_105} {
		raw := `"` + strings.Repeat("x", size) + `"`
		want := ""
		if size > 1_398_104 {
			want = "LIMIT_EXCEEDED"
		}
		if got := scanCanonicalJSON([]byte(raw)); got != want {
			t.Fatalf("string=%d got=%q want=%q", size, got, want)
		}
	}
	if got := scanCanonicalJSON([]byte("{")); got != "NONCANONICAL_REQUEST" {
		t.Fatalf("invalid syntax got=%q want NONCANONICAL_REQUEST", got)
	}
}

func TestProcessNetworkAndAmbientSpiesWithPositiveControls(t *testing.T) {
	binary := buildCandidateCLI(t)
	raw := canonical(t, pinned(t))
	root := t.TempDir()
	marker := filepath.Join(root, "process-marker")
	trap := filepath.Join(root, "trap")
	if err := os.WriteFile(trap, []byte("#!/bin/sh\nprintf trap > \"$1\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(trap, marker).CombinedOutput(); err != nil || !bytes.Equal(bytes.TrimSpace(output), []byte("")) {
		t.Fatalf("process-spy positive control: %v %q", err, output)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	positive := make(chan struct{}, 1)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			_ = connection.Close()
			positive <- struct{}{}
		}
	}()
	connection, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
	select {
	case <-positive:
	case <-time.After(time.Second):
		t.Fatal("network-spy positive control did not connect")
	}
	before := runCandidateCLI(t, binary, raw, "")
	after := runCandidateCLI(t, binary, raw, root, "PATH="+root, "HOME="+root, "HTTP_PROXY=http://"+listener.Addr().String(), "HTTPS_PROXY=http://"+listener.Addr().String(), "ALL_PROXY=http://"+listener.Addr().String())
	if !bytes.Equal(before, after) {
		t.Fatal("ambient state changed CLI bytes")
	}
	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		t.Fatal("network spy is not TCP")
	}
	_ = tcpListener.SetDeadline(time.Now().Add(150 * time.Millisecond))
	if connection, err := listener.Accept(); err == nil {
		_ = connection.Close()
		t.Fatal("candidate contacted network spy")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("candidate launched process spy")
	}
}
