package main

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunExactLFAndRejectsFileArguments(t *testing.T) {
	raw := []byte(`{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"r","scope_id":"s","compilation_unit_id":"u","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[]}` + "\n")
	var out bytes.Buffer
	if code := run(nil, bytes.NewReader(raw), &out); code != 0 || !strings.HasSuffix(out.String(), "\n") || strings.Count(out.String(), "\n") != 1 {
		t.Fatalf("code=%d out=%q", code, out.String())
	}
	out.Reset()
	if code := run([]string{"--input-file", "request.json"}, nil, &out); code != 0 || !strings.Contains(out.String(), "NONCANONICAL_REQUEST") {
		t.Fatalf("code=%d out=%q", code, out.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }
func TestRunShortWriteFails(t *testing.T) {
	if got := run(nil, bytes.NewReader(nil), failingWriter{}); got != 1 {
		t.Fatal(got)
	}
}

type partialErrorWriter struct{ writes int }

func (w *partialErrorWriter) Write(p []byte) (int, error) { w.writes++; return 1, os.ErrClosed }
func TestRunDoesNotRetryAfterPartialWriteError(t *testing.T) {
	w := &partialErrorWriter{}
	if got := run(nil, bytes.NewReader(nil), w); got != 1 || w.writes != 1 {
		t.Fatalf("code=%d writes=%d", got, w.writes)
	}
}

type partialWriter struct{ bytes.Buffer }

func (w *partialWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return w.Buffer.Write(p[:1])
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read denied") }
func TestRunWritesCompleteReadFailureResponse(t *testing.T) {
	var out partialWriter
	if code := run(nil, failingReader{}, &out); code != 0 || !strings.Contains(out.String(), "NONCANONICAL_REQUEST") || !strings.HasSuffix(out.String(), "\n") {
		t.Fatalf("code=%d out=%q", code, out.String())
	}
}

// TestRunReportsOversizeAsLimitExceeded pins ACP-012 for the shell CLI.
func TestRunReportsOversizeAsLimitExceeded(t *testing.T) {
	var out bytes.Buffer
	if code := run(nil, strings.NewReader(strings.Repeat("x", 1_500_001)), &out); code != 0 || out.String() != string(limitExceeded) || !strings.Contains(out.String(), `"reason":"LIMIT_EXCEEDED"`) {
		t.Fatalf("code=%d out=%q", code, out.String())
	}
}

const factGoldenRequest = `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"fact-golden","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input","family":"shell.posix-bash","path":"script/golden.sh","sha256":"sha256:67948dd9afd6afe5043b0029d5aa7cf0f8b2824baf16f4f097d40d830edb686d","content_base64":"dG9vbAo="}]}` + "\n"
const factGoldenResponse = `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"fact-golden","status":"CANDIDATE","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input","sha256":"sha256:67948dd9afd6afe5043b0029d5aa7cf0f8b2824baf16f4f097d40d830edb686d"}],"facts":[{"kind":"shell.command.static","input_handle":"input","related_handle":"-","subject":"script/golden.sh","predicate":"invokes","value":"tool","instance_id":"script/golden.sh","evidence_sha256":"sha256:8a64b2d7b60643a3676fde67e22effeff0ada46120a53113df2a3f21a1c38ad5"}]}` + "\n"
const errorGoldenRequest = `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"error-golden","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input","family":"shell.posix-bash","path":"script/golden.sh","sha256":"sha256:3e6453a90e9074f00391a26631483479ff2fd9e908a0c8c266a5a63a5cab555a","content_base64":"ZXZhbCB4Cg=="}]}` + "\n"
const errorGoldenResponse = `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"error-golden","status":"REJECTED","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input","sha256":"sha256:3e6453a90e9074f00391a26631483479ff2fd9e908a0c8c266a5a63a5cab555a"}],"reason":"DYNAMIC_INPUT"}` + "\n"
const heredocGoldenRequest = `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"heredoc-golden","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input","family":"shell.posix-bash","path":"script/golden.sh","sha256":"sha256:aaec69722390010169add6d360cd5ebb88b3853a944366628a616e66ce454c48","content_base64":"Y2F0IDw8RU5ECmJvZHkKRU5ECg=="}]}` + "\n"
const heredocGoldenResponse = `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"heredoc-golden","status":"REJECTED","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input","sha256":"sha256:aaec69722390010169add6d360cd5ebb88b3853a944366628a616e66ce454c48"}],"reason":"UNSUPPORTED_SCHEMA"}` + "\n"
const reservedGoldenRequest = `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"reserved-golden","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"linux","architecture":"amd64","abi":"none","features":["bash-5.2"]},"inputs":[{"handle":"input","family":"shell.posix-bash","path":"script/golden.sh","sha256":"sha256:203ccc9bf40fed73176bc2e5aa130382fb0e639f274f63e5d155d82fd0f2ab75","content_base64":"dGltZSBlY2hvIGhpCg=="}]}` + "\n"
const reservedGoldenResponse = `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"reserved-golden","status":"REJECTED","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"linux","architecture":"amd64","abi":"none","features":["bash-5.2"]},"input_echoes":[{"handle":"input","sha256":"sha256:203ccc9bf40fed73176bc2e5aa130382fb0e639f274f63e5d155d82fd0f2ab75"}],"reason":"UNSUPPORTED_SCHEMA"}` + "\n"
const quotedControlGoldenRequest = `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"quoted-control-golden","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"linux","architecture":"amd64","abi":"none","features":["bash-5.2"]},"inputs":[{"handle":"input","family":"shell.posix-bash","path":"script/golden.sh","sha256":"sha256:b9b8cc5d5b73e7109e30d53d80c84f65ed0c3308ebe3e3b68f94ec52422cec46","content_base64":"InRpbWUiCg=="}]}` + "\n"
const quotedControlGoldenResponse = `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"quoted-control-golden","status":"CANDIDATE","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"linux","architecture":"amd64","abi":"none","features":["bash-5.2"]},"input_echoes":[{"handle":"input","sha256":"sha256:b9b8cc5d5b73e7109e30d53d80c84f65ed0c3308ebe3e3b68f94ec52422cec46"}],"facts":[{"kind":"shell.command.static","input_handle":"input","related_handle":"-","subject":"script/golden.sh","predicate":"invokes","value":"time","instance_id":"script/golden.sh","evidence_sha256":"sha256:e8ea573c2c5fa31ca41fbad28882cc1a60f88482beba05679ff2d3071912ba4f"}]}` + "\n"
const unsetGoldenRequest = `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"unset-golden","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input","family":"shell.posix-bash","path":"script/golden.sh","sha256":"sha256:d1ce95ca084ab5cd5d4c362069eebd27e79412efc6d91a85f64dd095085b695e","content_base64":"dW5zZXQgSE9NRT12YWx1ZQo="}]}` + "\n"
const unsetGoldenResponse = `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"unset-golden","status":"REJECTED","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input","sha256":"sha256:d1ce95ca084ab5cd5d4c362069eebd27e79412efc6d91a85f64dd095085b695e"}],"reason":"UNSUPPORTED_SCHEMA"}` + "\n"
const duplicateGoldenRequest = `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"duplicate-golden","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input","family":"shell.posix-bash","path":"script/golden.sh","sha256":"sha256:5517b4b2e12792acf80f3db50ba00c92465bbbaa6f32c275b501fc6126408161","content_base64":"dG9vbAp0b29sCg=="}]}` + "\n"
const duplicateGoldenResponse = `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"duplicate-golden","status":"REJECTED","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input","sha256":"sha256:5517b4b2e12792acf80f3db50ba00c92465bbbaa6f32c275b501fc6126408161"}],"reason":"DUPLICATE_VALUE"}` + "\n"

func buildCLI(t *testing.T, root string) string {
	t.Helper()
	command := filepath.Join(root, "corvint-analyzer-shell")
	if runtime.GOOS == "windows" {
		command += ".exe"
	}
	build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-o", command, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	return command
}

func invokeCLI(t *testing.T, command string, raw []byte, env []string) []byte {
	t.Helper()
	child := exec.Command(command)
	child.Stdin = bytes.NewReader(raw)
	child.Env = env
	got, err := child.Output()
	if err != nil {
		t.Fatalf("command=%s err=%v", command, err)
	}
	return got
}

func TestBuiltCLILiteralGoldensAndFreshProcessPermutations(t *testing.T) {
	command := buildCLI(t, t.TempDir())
	goldens := []struct {
		name, request, response string
	}{
		{"fact", factGoldenRequest, factGoldenResponse},
		{"error", errorGoldenRequest, errorGoldenResponse},
		{"heredoc", heredocGoldenRequest, heredocGoldenResponse},
		{"reserved", reservedGoldenRequest, reservedGoldenResponse},
		{"quoted_control", quotedControlGoldenRequest, quotedControlGoldenResponse},
		{"unset", unsetGoldenRequest, unsetGoldenResponse},
		{"duplicate", duplicateGoldenRequest, duplicateGoldenResponse},
	}
	for _, golden := range goldens {
		t.Run(golden.name, func(t *testing.T) {
			if got := invokeCLI(t, command, []byte(golden.request), nil); !bytes.Equal(got, []byte(golden.response)) {
				t.Fatalf("literal golden mismatch\ngot:  %q\nwant: %q", got, golden.response)
			}
		})
	}
	envs := [][]string{
		append(append([]string(nil), os.Environ()...), "CORVINT_PERMUTATION_A=one", "CORVINT_PERMUTATION_B=two", "CORVINT_PERMUTATION_C=three"),
		append(append([]string(nil), os.Environ()...), "CORVINT_PERMUTATION_C=three", "CORVINT_PERMUTATION_A=one", "CORVINT_PERMUTATION_B=two"),
		append(append([]string(nil), os.Environ()...), "CORVINT_PERMUTATION_B=two", "CORVINT_PERMUTATION_C=three", "CORVINT_PERMUTATION_A=one"),
	}
	for _, golden := range goldens {
		t.Run(golden.name+"-fresh-process-permutations", func(t *testing.T) {
			t.Parallel()
			for run := 0; run < 1024; run++ {
				if got := invokeCLI(t, command, []byte(golden.request), envs[run%len(envs)]); !bytes.Equal(got, []byte(golden.response)) {
					t.Fatalf("fresh run=%d golden=%s got=%q want=%q", run, golden.name, got, golden.response)
				}
			}
		})
	}
}

func TestBuiltCLIAmbientEffectSpiesWithPositiveControls(t *testing.T) {
	root := t.TempDir()
	binary := buildCLI(t, root)
	pathDir := filepath.Join(root, "path")
	home := filepath.Join(root, "home")
	cwd := filepath.Join(root, "cwd")
	for _, dir := range []string{pathDir, home, cwd} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(root, "process-or-shell")
	shim := filepath.Join(pathDir, "sh")
	if err := os.WriteFile(shim, []byte("#!/bin/sh\nprintf used >"+marker+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command(shim).Run(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("process positive control: %v", err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	networkSpy := err == nil
	if networkSpy {
		defer listener.Close()
	}
	connected := make(chan struct{}, 2)
	if networkSpy {
		go func() {
			for {
				connection, err := listener.Accept()
				if err != nil {
					return
				}
				connection.Close()
				connected <- struct{}{}
			}
		}()
	}
	if networkSpy {
		positive, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		positive.Close()
		select {
		case <-connected:
		case <-time.After(time.Second):
			t.Fatal("network positive control")
		}
	}
	if err := os.WriteFile(filepath.Join(home, "positive"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(filepath.Join(home, "positive")); err != nil || string(body) != "x" {
		t.Fatalf("HOME read/write positive control body=%q err=%v", body, err)
	}
	if err := os.Remove(filepath.Join(home, "positive")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "positive"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(filepath.Join(cwd, "positive")); err != nil || string(body) != "x" {
		t.Fatalf("CWD read/write positive control body=%q err=%v", body, err)
	}
	if err := os.Remove(filepath.Join(cwd, "positive")); err != nil {
		t.Fatal(err)
	}
	readDescriptor, err := os.CreateTemp(root, "read-descriptor")
	if err != nil {
		t.Fatal(err)
	}
	defer readDescriptor.Close()
	if _, err := readDescriptor.WriteString("read-spy"); err != nil {
		t.Fatal(err)
	}
	if _, err := readDescriptor.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if probe := make([]byte, len("read-spy")); func() error {
		_, err := io.ReadFull(readDescriptor, probe)
		return err
	}() != nil || string(probe) != "read-spy" {
		t.Fatalf("read descriptor positive control=%q", probe)
	}
	if _, err := readDescriptor.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	writeDescriptor, err := os.CreateTemp(root, "write-descriptor")
	if err != nil {
		t.Fatal(err)
	}
	defer writeDescriptor.Close()
	if _, err := writeDescriptor.WriteString("positive"); err != nil {
		t.Fatal(err)
	}
	if err := writeDescriptor.Truncate(0); err != nil {
		t.Fatal(err)
	}
	if _, err := writeDescriptor.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if _, err := writeDescriptor.WriteString("write-spy"); err != nil {
		t.Fatal(err)
	}
	if err := writeDescriptor.Truncate(0); err != nil {
		t.Fatal(err)
	}
	if _, err := writeDescriptor.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	child := exec.Command(binary)
	child.Stdin = strings.NewReader(factGoldenRequest)
	child.Dir = cwd
	child.Env = []string{"PATH=" + pathDir, "HOME=" + home, "CORVINT_AMBIENT_SENTINEL=forbidden"}
	if runtime.GOOS != "windows" {
		child.ExtraFiles = []*os.File{readDescriptor, writeDescriptor}
	}
	output, err := child.Output()
	if err != nil || !bytes.Equal(output, []byte(factGoldenResponse)) {
		t.Fatalf("candidate err=%v output=%q", err, output)
	}
	if runtime.GOOS != "windows" {
		if offset, err := readDescriptor.Seek(0, io.SeekCurrent); err != nil || offset != 0 {
			t.Fatalf("read descriptor identity offset=%d err=%v", offset, err)
		}
		if offset, err := writeDescriptor.Seek(0, io.SeekCurrent); err != nil || offset != 0 {
			t.Fatalf("write descriptor identity offset=%d err=%v", offset, err)
		}
		if body, err := os.ReadFile(writeDescriptor.Name()); err != nil || len(body) != 0 {
			t.Fatalf("write descriptor spy=%q err=%v", body, err)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("process/shell spy=%v", err)
	}
	if entries, _ := os.ReadDir(home); len(entries) != 0 {
		t.Fatalf("HOME write spy=%v", entries)
	}
	if entries, _ := os.ReadDir(cwd); len(entries) != 0 {
		t.Fatalf("CWD write spy=%v", entries)
	}
	if networkSpy {
		select {
		case <-connected:
			t.Fatal("network spy")
		case <-time.After(50 * time.Millisecond):
		}
	}
}
