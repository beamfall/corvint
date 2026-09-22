package main_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const (
	candidateProfile = "corvint-analyzer-candidate/experimental"
	candidateFamily  = "shader"

	// These are frozen receipts from the built command, not values calculated
	// by the production analyzer during the test.
	frozenSimpleOutputSHA256   = "1fca6056c59d0f90627337a889be2cc08d5e39fa70c2e4962db6b4499da49e43"
	frozenRepeatedOutputSHA256 = "d20d5704a34788a6b1ce0897225c4d135c83385f87c050a8119be5709f01fbd6"
	frozenRepeatedOutputBytes  = 2660
	frozenBuiltCLIVectorSHA256 = "6da20261acfd533929a5023179826a4144ad11d18cdf09ee9fc250f26eb7d5e8"
	frozenPermutedFrame        = `{"profile":"corvint-analyzer-candidate/experimental","family":"shader","shader_profile":"beamfall.glsl-es-3.00.fragment.visual-shaders","shader_language":"glsl","shader_version":"3.00","shader_toolchain":"glslang-16.4.0.spirv-cross-2026-07-06T12-43-32","request_id":"request-0000","request_sha256":"sha256:b933f1e277e92ab744801452ab42939b95e2a84d77357f9a46872f58e44b3b76","status":"REJECTED","scope_id":"root","compilation_unit_id":"unit-0000","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-2","sha256":"sha256:ac4cd20465b2617208f65dd341b0442007f4d876e8bbb47737b11e85169b0927"},{"handle":"input-1","sha256":"sha256:ac4cd20465b2617208f65dd341b0442007f4d876e8bbb47737b11e85169b0927"}],"reason":"DUPLICATE_VALUE"}` + "\n"
)

var noncanonicalFrame = []byte(`{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n")

type shaderTarget struct {
	OS           string   `json:"os"`
	Architecture string   `json:"architecture"`
	ABI          string   `json:"abi"`
	Features     []string `json:"features"`
}

type shaderInput struct {
	Handle        string `json:"handle"`
	Family        string `json:"family"`
	Path          string `json:"path"`
	SHA256        string `json:"sha256"`
	ContentBase64 string `json:"content_base64"`
}

// shaderRequest deliberately repeats the published wire shape. This external
// test package never imports or invokes the candidate implementation.
type shaderRequest struct {
	Profile           string        `json:"profile"`
	Family            string        `json:"family"`
	ShaderProfile     string        `json:"shader_profile"`
	ShaderLanguage    string        `json:"shader_language"`
	ShaderVersion     string        `json:"shader_version"`
	ShaderToolchain   string        `json:"shader_toolchain"`
	RequestID         string        `json:"request_id"`
	ScopeID           string        `json:"scope_id"`
	CompilationUnitID string        `json:"compilation_unit_id"`
	Target            shaderTarget  `json:"target"`
	Inputs            []shaderInput `json:"inputs"`
}

func TestBuiltCLIFramesAndReplaysExactFrozenReceipts(t *testing.T) {
	binary := buildCLI(t)
	home := t.TempDir()
	valid := shaderFrame("#version 300 es\nvoid main(){}\n")
	if got := runCLI(t, binary, home, valid); receiptSHA256(got) != frozenSimpleOutputSHA256 {
		t.Fatalf("valid SHA-256=%s", receiptSHA256(got))
	}
	for name, raw := range map[string][]byte{
		"truncated":   valid[:len(valid)-1],
		"extra-frame": append(append([]byte(nil), valid...), '\n'),
		"malformed":   []byte("{\n"),
	} {
		t.Run(name, func(t *testing.T) {
			if got := runCLI(t, binary, home, raw); !bytes.Equal(got, noncanonicalFrame) {
				t.Fatalf("frozen error receipt\nwant=%s\ngot=%s", noncanonicalFrame, got)
			}
		})
	}
}

func TestBuiltCLIFrozenUniqueVectorsAndPermutation(t *testing.T) {
	binary := buildCLI(t)
	home := t.TempDir()
	digest := sha256.New()
	seen := map[[32]byte]struct{}{}
	for index := 0; index < 1024; index++ {
		raw := freshShaderFrame(index)
		identity := sha256.Sum256(raw)
		if _, duplicate := seen[identity]; duplicate {
			t.Fatalf("duplicate fresh vector %d", index)
		}
		seen[identity] = struct{}{}
		got := runCLI(t, binary, home, raw)
		_, _ = digest.Write([]byte(fmt.Sprintf("%d:", len(got))))
		_, _ = digest.Write(got)
	}
	if got := hex.EncodeToString(digest.Sum(nil)); got != frozenBuiltCLIVectorSHA256 {
		t.Fatalf("frozen unique built CLI SHA-256=%s", got)
	}
	if got := string(runCLI(t, binary, home, permutedShaderFrame(t))); got != frozenPermutedFrame {
		t.Fatalf("frozen permutation receipt\nwant=%s\ngot=%s", frozenPermutedFrame, got)
	}
}

// Each iteration starts a separate process: 1,000 identical frames exercise
// process start, stdin read, full output write, and frame determinism without
// using a production oracle.
func TestBuiltCLIRepeatedIdenticalFreshProcesses(t *testing.T) {
	binary := buildCLI(t)
	home := t.TempDir()
	raw := freshShaderFrame(0)
	for index := 0; index < 1000; index++ {
		got := runCLI(t, binary, home, raw)
		if len(got) != frozenRepeatedOutputBytes || receiptSHA256(got) != frozenRepeatedOutputSHA256 {
			t.Fatalf("iteration=%d bytes=%d SHA-256=%s", index, len(got), receiptSHA256(got))
		}
	}
}

func TestBuiltCLIRejectsFullWriteFailure(t *testing.T) {
	for _, vector := range []struct {
		name  string
		args  []string
		input []byte
	}{
		{"analysis", nil, freshShaderFrame(0)},
		{"version", []string{"--version"}, nil},
		{"invalid-arguments", []string{"unexpected"}, nil},
	} {
		t.Run(vector.name, func(t *testing.T) {
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Close()
			if err := reader.Close(); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(buildCLI(t), vector.args...)
			command.Stdin = bytes.NewReader(vector.input)
			command.Stdout = writer
			command.Env = []string{"PATH=/absent", "HOME=" + t.TempDir()}
			if err := command.Run(); err == nil {
				t.Fatal("closed stdout accepted")
			}
		})
	}
}

func TestBuiltCLIVersionKeepsCEMAndOCMUnsupported(t *testing.T) {
	command := exec.Command(buildCLI(t), "--version")
	command.Env = []string{"PATH=/absent", "HOME=" + t.TempDir()}
	got, err := command.CombinedOutput()
	want := []byte(`{"profile":"corvint-analyzer-candidate/experimental","family":"shader","status":"EXPERIMENTAL_UNSELECTABLE","cem":"UNSUPPORTED","ocm":"UNSUPPORTED"}` + "\n")
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("version=%s err=%v", got, err)
	}
}

// This tests observable process, descriptor, filesystem, and network traps.
// The source-level alias guard provides the read-side filesystem/descriptor
// proof; these execution traps catch any bypass that affects the command.
func TestBuiltCLIDeniesAmbientExecutionChannels(t *testing.T) {
	binary := buildCLI(t)
	input := freshShaderFrame(0)
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(root, "ambient-secret")
	if err := os.WriteFile(secret, []byte("candidate-must-not-read-this"), 0o600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	accepted := make(chan error, 2)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			_ = connection.Close()
		}
		accepted <- err
	}()
	descriptorRead, descriptorWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer descriptorRead.Close()
	defer descriptorWrite.Close()
	marker := filepath.Join(root, "process-observed")
	trap := filepath.Join(root, "command-trap")
	if err := os.WriteFile(trap, []byte("#!/bin/sh\nprintf process > "+marker+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	commandContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(commandContext, binary)
	command.Dir = root
	command.Stdin = bytes.NewReader(input)
	command.ExtraFiles = []*os.File{descriptorRead}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	command.Env = []string{"PATH=" + root, "HOME=" + home, "SHELL=" + trap, "CORVINT_SHADER_ENV_TRAP=" + secret, "HTTP_PROXY=http://" + listener.Addr().String(), "HTTPS_PROXY=http://" + listener.Addr().String(), "NO_PROXY="}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	got, readErr := io.ReadAll(stdout)
	if waitErr := command.Wait(); readErr != nil || waitErr != nil {
		t.Fatalf("read=%v wait=%v stderr=%s", readErr, waitErr, stderr.Bytes())
	}
	if len(got) != frozenRepeatedOutputBytes || receiptSHA256(got) != frozenRepeatedOutputSHA256 || stderr.Len() != 0 {
		t.Fatalf("ambient bytes=%d SHA-256=%s stderr=%s", len(got), receiptSHA256(got), stderr.Bytes())
	}
	if body, err := os.ReadFile(secret); err != nil || string(body) != "candidate-must-not-read-this" {
		t.Fatalf("filesystem trap changed: %q %v", body, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("process trap observed candidate execution: %v", err)
	}
	select {
	case err := <-accepted:
		t.Fatalf("network trap observed a connection: %v", err)
	case <-time.After(125 * time.Millisecond):
	}
	// Positive controls prove all three live external probes can observe their
	// designated channel; the command itself is the only process under test.
	positive := exec.Command(trap)
	positive.Env = []string{"PATH=" + root}
	if output, err := positive.CombinedOutput(); err != nil {
		t.Fatalf("process positive control: %v %s", err, output)
	}
	if body, err := os.ReadFile(marker); err != nil || string(body) != "process" {
		t.Fatalf("process positive control=%q err=%v", body, err)
	}
	connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
	select {
	case err := <-accepted:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("network positive control did not reach listener")
	}
	positiveRead, positiveWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := positiveWrite.Write([]byte("descriptor-positive")); err != nil {
		t.Fatal(err)
	}
	if err := positiveWrite.Close(); err != nil {
		t.Fatal(err)
	}
	descriptorPositive := exec.Command("/bin/sh", "-c", "cat <&3")
	descriptorPositive.ExtraFiles = []*os.File{positiveRead}
	output, err := descriptorPositive.Output()
	_ = positiveRead.Close()
	if err != nil || string(output) != "descriptor-positive" {
		t.Fatalf("descriptor positive control=%q err=%v", output, err)
	}
}

func buildCLI(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "corvint-analyzer-shader")
	build := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", binary, ".")
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	return binary
}

func runCLI(t *testing.T, binary, home string, raw []byte) []byte {
	t.Helper()
	command := exec.Command(binary)
	command.Stdin = bytes.NewReader(raw)
	command.Env = []string{"PATH=/absent", "HOME=" + home, "LANG=C"}
	got, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v %s", err, got)
	}
	return got
}

func receiptSHA256(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func freshShaderFrame(index int) []byte {
	source := fmt.Sprintf("#version 300 es\nprecision highp float;\nuniform float value%04d;\nvoid main(){}\n", index)
	sum := sha256.Sum256([]byte(source))
	request := shaderRequest{Profile: candidateProfile, Family: candidateFamily, ShaderProfile: "beamfall.glsl-es-3.00.fragment.visual-shaders", ShaderLanguage: "glsl", ShaderVersion: "3.00", ShaderToolchain: "glslang-16.4.0.spirv-cross-2026-07-06T12-43-32", RequestID: fmt.Sprintf("request-%04d", index), ScopeID: "root", CompilationUnitID: fmt.Sprintf("unit-%04d", index), Target: shaderTarget{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []shaderInput{{Handle: "input-1", Family: "shader.glsl", Path: fmt.Sprintf("visuals/vector-%04d.glsl", index), SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString([]byte(source))}}}
	frame, _ := json.Marshal(request)
	return append(frame, '\n')
}

func permutedShaderFrame(t *testing.T) []byte {
	t.Helper()
	var request shaderRequest
	if err := json.Unmarshal(freshShaderFrame(0), &request); err != nil {
		t.Fatal(err)
	}
	second := request.Inputs[0]
	second.Handle = "input-2"
	second.Path = "visuals/vector-0000-b.glsl"
	request.Inputs = []shaderInput{second, request.Inputs[0]}
	frame, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(frame, '\n')
}

func shaderFrame(source string) []byte {
	sum := sha256.Sum256([]byte(source))
	request := shaderRequest{Profile: candidateProfile, Family: candidateFamily, ShaderProfile: "beamfall.glsl-es-3.00.fragment.visual-shaders", ShaderLanguage: "glsl", ShaderVersion: "3.00", ShaderToolchain: "glslang-16.4.0.spirv-cross-2026-07-06T12-43-32", RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: shaderTarget{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []shaderInput{{Handle: "input-1", Family: "shader.glsl", Path: "visuals/probe.glsl", SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString([]byte(source))}}}
	frame, _ := json.Marshal(request)
	return append(frame, '\n')
}
