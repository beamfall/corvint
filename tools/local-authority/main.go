// Prototype execution is unsigned. Explicit protected operator phases may sign
// completed observations; independent root admission is never performed here.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const maxSource = 16 << 20
const maxModule = 32 << 20
const maxOutput = 1 << 20
const moduleText = "module github.com/corvint-context/corvint\n\ngo 1.27.0\n"
const recipe = "go1.27.0|GOOS=wasip1|GOARCH=wasm|CGO_ENABLED=0|GOTOOLCHAIN=local|GOPROXY=off|GOSUMDB=off|GOENV=off|GOWORK=off|build -trimpath -buildvcs=false -ldflags=-buildid= -o candidate.wasm ./cmd/check"

type work struct {
	Nonce, Mode, Go     string
	Source, Wasm, Input []byte
}
type terminal struct {
	Nonce, Phase, ModuleSHA256 string
	Output, Wasm               []byte
	Error                      string
}
type receipt struct {
	Profile      string `json:"profile"`
	Authority    string `json:"authority"`
	CheckID      string `json:"checkId"`
	InvocationID string `json:"invocationId"`
	Nonce        string `json:"nonce"`
	SourceSHA256 string `json:"sourceSHA256"`
	RecipeSHA256 string `json:"recipeSHA256"`
	WasmSHA256   string `json:"wasmSHA256"`
	WorkerSHA256 string `json:"workerSHA256"`
	DriverSHA256 string `json:"driverSHA256"`
	Status       string `json:"status"`
	Reason       string `json:"reason,omitempty"`
	Cleanup      string `json:"cleanup"`
}

func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func readBound(r io.Reader, n int) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, int64(n)+1))
	if len(b) > n {
		return nil, errors.New("size bound")
	}
	return b, e
}
func main() {
	if err := entry(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func entry() error {
	if len(os.Args) == 5 && os.Args[1] == "prepare-qualified-direct-hooks" {
		return prepareQualifiedHooksProfile(os.Args[2], os.Args[3], os.Args[4], true)
	}
	if len(os.Args) == 5 && os.Args[1] == "prepare-qualified-hooks" {
		return prepareQualifiedHooks(os.Args[2], os.Args[3], os.Args[4])
	}
	if len(os.Args) == 3 && os.Args[1] == "admit-reader" {
		return admitReader(os.Args[2])
	}
	if len(os.Args) == 2 && os.Args[1] == "withdraw-reader" {
		return withdrawReader()
	}
	if len(os.Args) == 4 && os.Args[1] == "stage-enrollment" {
		return stageEnrollment(os.Args[2], os.Args[3])
	}
	if len(os.Args) == 2 && os.Args[1] == "retire-accounts" {
		return retireAccounts()
	}
	if len(os.Args) == 8 && os.Args[1] == "prepare-release" {
		return prepareRelease(os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6], os.Args[7])
	}
	if len(os.Args) == 4 && os.Args[1] == "install-release" {
		return installRelease(os.Args[2], os.Args[3])
	}
	if len(os.Args) == 3 && os.Args[1] == "remove-release" {
		return removeRelease(os.Args[2])
	}
	if len(os.Args) == 2 && os.Args[1] == "setup-accounts" {
		return setupAccounts()
	}
	if len(os.Args) == 2 && os.Args[1] == "operator-keygen" {
		return operatorKeygen()
	}
	if len(os.Args) == 2 && os.Args[1] == "authority-keygen" {
		if e := watchLiveness(); e != nil {
			return e
		}
		return authorityKeygen()
	}

	if len(os.Args) == 3 && os.Args[1] == "operator-run" {
		return operatorRun(os.Args[2])
	}
	if len(os.Args) == 3 && (os.Args[1] == "authority-prepare" || os.Args[1] == "authority-built" || os.Args[1] == "authority-finish") {
		if e := watchLiveness(); e != nil {
			return e
		}
		return authorityPhase(os.Args[1], os.Args[2])
	}
	if len(os.Args) == 2 && os.Args[1] == "worker" {
		return worker()
	}
	if len(os.Args) == 2 && os.Args[1] == "guardian" {
		return guardian()
	}
	if len(os.Args) != 4 || os.Args[1] != "prototype" {
		return errors.New("usage: local-authority prototype SOURCE_CODEC_GO PINNED_GO_BINARY")
	}
	source, e := os.ReadFile(os.Args[2])
	if e != nil {
		return e
	}
	if len(source) > maxSource {
		return errors.New("source bound")
	}
	r := run(source, os.Args[3])
	if e := json.NewEncoder(os.Stdout).Encode(r); e != nil {
		return e
	}
	if r.Status != "PASS" {
		return errors.New(r.Reason)
	}
	return nil
}
func run(source []byte, goBinary string) receipt {
	nonce := make([]byte, 32)
	_, e := rand.Read(nonce)
	r := receipt{Profile: "corvint-protected-execution/0", Authority: "NONE", CheckID: CheckID, InvocationID: InvocationID, Nonce: hex.EncodeToString(nonce), Status: "FAIL", Cleanup: "UNCONFIRMED", RecipeSHA256: digest([]byte(recipe)), DriverSHA256: digest(driverSource)}
	if e != nil {
		r.Reason = e.Error()
		return r
	}
	manifest := struct{ Codec, Wrapper, Module string }{digest(source), digest(wrapper), digest([]byte(moduleText))}
	r.SourceSHA256 = digest(mustJSONLine(manifest))
	exe, e := os.Executable()
	if e != nil {
		r.Reason = e.Error()
		return r
	}
	binary, e := os.ReadFile(exe)
	if e != nil {
		r.Reason = e.Error()
		return r
	}
	r.WorkerSHA256 = digest(binary)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	built, e := supervise(ctx, work{Nonce: r.Nonce, Mode: "build", Go: goBinary, Source: source})
	if e != nil {
		r.Reason = e.Error()
		return r
	}
	r.WasmSHA256 = digest(built.Wasm)
	result, e := supervise(ctx, work{Nonce: r.Nonce, Mode: "execute", Wasm: built.Wasm, Input: driverInput()})
	if e != nil {
		r.Reason = e.Error()
		return r
	}
	if result.ModuleSHA256 != r.WasmSHA256 {
		r.Reason = "module identity mismatch"
		return r
	}
	r.Cleanup = "OWNED_GROUP_REAPED"
	e = TestCanonicalOutput(result.Output)
	if e != nil {
		r.Reason = e.Error()
		return r
	}
	r.Status = "PASS"
	return r
}
func strictDecode(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("trailing frame")
	}
	return nil
}
func supervise(ctx context.Context, w work) (terminal, error) {
	var out terminal
	exe, e := os.Executable()
	if e != nil {
		return out, e
	}
	lifeR, lifeW, e := os.Pipe()
	if e != nil {
		return out, e
	}
	defer lifeR.Close()
	defer lifeW.Close()
	cmd := exec.CommandContext(ctx, exe, "guardian")
	cmd.Env = []string{}
	cmd.ExtraFiles = []*os.File{lifeR}
	cmd.Stdin = bytes.NewReader(mustJSONLine(w))
	var output limitedBuffer
	output.max = 48 << 20
	cmd.Stdout = &output
	var diagnostic limitedBuffer
	diagnostic.max = maxOutput
	cmd.Stderr = &diagnostic
	// Cancel closes the liveness pipe; guardian owns worker cleanup even if this parent dies.
	cmd.Cancel = func() error { return lifeW.Close() }
	cmd.WaitDelay = 3 * time.Second
	if e = cmd.Start(); e != nil {
		return out, e
	}
	lifeR.Close()
	e = cmd.Wait()
	if e != nil {
		return out, fmt.Errorf("guardian failed: %v %s", e, diagnostic.Bytes())
	}
	if output.overflow || diagnostic.overflow {
		return out, errors.New("supervisor output overflow")
	}
	if e = strictDecode(output.Bytes(), &out); e != nil {
		return out, e
	}
	if out.Nonce != w.Nonce || out.Phase != "reaped" {
		return out, errors.New("terminal identity")
	}
	if out.Error != "" {
		return out, errors.New(out.Error)
	}
	return out, nil
}
func worker() error {
	// A lost guardian closes fd3. Kill the complete owned group, including compiler children.
	life := os.NewFile(3, "guardian-liveness")
	if life == nil {
		return errors.New("missing guardian")
	}
	go func() { var b [1]byte; _, _ = life.Read(b[:]); _ = syscall.Kill(-syscall.Getpgrp(), syscall.SIGKILL) }()
	if e := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &syscall.Rlimit{Cur: 32, Max: 32}); e != nil {
		return e
	}
	b, e := readBound(os.Stdin, 48<<20)
	if e != nil {
		return e
	}
	var w work
	if e = strictDecode(b, &w); e != nil {
		return e
	}
	t := terminal{Nonce: w.Nonce, Phase: "completed"}
	switch w.Mode {
	case "build":
		t.Wasm, e = build(w.Source, w.Go)
	case "execute":
		t.ModuleSHA256 = digest(w.Wasm)
		t.Output, e = execute(w.Wasm, w.Input)
	default:
		e = errors.New("unknown worker mode")
	}
	if e != nil {
		t.Error = e.Error()
	}
	return json.NewEncoder(os.Stdout).Encode(t)
}

type limitedBuffer struct {
	bytes.Buffer
	max      int
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.max-b.Len() {
		b.overflow = true
		return 0, errors.New("output bound")
	}
	return b.Buffer.Write(p)
}
func executableGo(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errors.New("absolute toolchain required")
	}
	p, e := filepath.EvalSymlinks(path)
	return p, e
}
