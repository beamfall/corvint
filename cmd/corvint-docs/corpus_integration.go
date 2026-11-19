package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/docsenvelope"
	"github.com/Beamfall/corvint/internal/gokernel"
)

type corpusCapture struct{ bytes bytes.Buffer }

func (b *corpusCapture) Write(p []byte) (int, error) {
	if len(p) > doccorpus.MaxBytes-b.bytes.Len() {
		return 0, argumentError("corpus native result exceeds bound")
	}
	return b.bytes.Write(p)
}

// The explicit wrapper is absent from all legacy invocations. It never widens
// the accepted command surface to mutations or forwards corpus text as intent.
func runCorpusIntegration(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, bool) {
	original := []string{}
	artifact := ""
	depth := 0
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			original = append(original, args[i:]...)
			break
		}
		name, value, inline := strings.Cut(args[i], "=")
		if name == "--corpus-depth" {
			// DCP-V1-097: downstream flow-relation depth for impact/affected only.
			n, err := strconv.Atoi(value)
			if !inline || depth != 0 || err != nil || n < 1 || n > doccorpus.MaxFlowDepth {
				emitError(stderr, argumentError("invalid or duplicate --corpus-depth=N (1-16)"))
				return 2, true
			}
			depth = n
			continue
		}
		if name != "--corpus" || !inline {
			original = append(original, args[i])
			// Native flags consume their next token; preserve it byte-for-byte.
			if strings.HasPrefix(name, "--") && !inline && corpusNativeValueFlag(name) && corpusNativeValueFollows(args, i) {
				i++
				original = append(original, args[i])
			}
			continue
		}
		if artifact != "" || value == "" {
			emitError(stderr, argumentError("invalid or duplicate --corpus=FILE"))
			return 2, true
		}
		artifact = value
	}
	if artifact == "" && depth != 0 {
		emitError(stderr, argumentError("--corpus-depth requires --corpus=FILE"))
		return 2, true
	}
	if artifact == "" {
		return 0, false
	}
	pos := commandPositionAfterRoots(original)
	if pos < 0 {
		emitError(stderr, argumentError("--corpus requires a native read command"))
		return 2, true
	}
	command := original[pos]
	if command != "impact" && command != "affected" {
		emitError(stderr, argumentError("--corpus is supported only on native evidence reads"))
		return 2, true
	}
	root := "."
	for i := 0; i < pos; i++ {
		if original[i] == "--root" {
			i++
			root = original[i]
		} else {
			root = strings.TrimPrefix(original[i], "--root=")
		}
	}
	root, err := resolveExplicitRoot(root)
	if err != nil {
		emitError(stderr, err)
		return 2, true
	}
	// The joined corpus is a read: it is admitted by the verified read, streamed from the file,
	// as `docs corpus` reads are (DCR-V1-410, DCR-V1-432).
	corpus, err := doccorpus.OpenReadFile(ctx, root, artifact, doccorpus.ReadOptions{})
	if err != nil {
		emitCorpusError(stderr, err)
		return 2, true
	}

	var captured corpusCapture
	exit := runCoreRead(ctx, original, stdin, &captured, stderr)
	if exit != 0 {
		// The native command already wrote its own error envelope to stderr; a
		// failed copy of its (usually empty) stdout here must not append a
		// second envelope on the same stream.
		_, _ = stdout.Write(captured.bytes.Bytes())
		return exit, true
	}
	var native map[string]json.RawMessage
	if err := json.Unmarshal(captured.bytes.Bytes(), &native); err != nil {
		emitError(stderr, argumentError("native output is not a structured corpus consumer"))
		return 2, true
	}
	overlay, err := doccorpus.LiveFreshness(ctx, root, corpus)
	if err != nil {
		emitCorpusError(stderr, err)
		return 2, true
	}
	request := doccorpus.Request{Operation: "info"}
	receipt, err := doccorpus.QueryLive(ctx, corpus, request, overlay)
	if err != nil {
		emitCorpusError(stderr, err)
		return 2, true
	}
	section := map[string]any{"receipt": receipt, "authority": "generated-documentation", "task_authority": false}
	paths := []string{}
	if command == "impact" {
		var impact struct {
			Request struct {
				Paths []string `json:"paths"`
			} `json:"request"`
			Coverage struct {
				Critical []string `json:"critical"`
			} `json:"coverage"`
			Omissions struct {
				Count   int `json:"count"`
				Samples []struct {
					Path string `json:"path"`
				} `json:"samples"`
			} `json:"omissions"`
		}
		if err := json.Unmarshal(native["context"], &impact); err != nil {
			emitError(stderr, argumentError("native impact receipt unavailable"))
			return 2, true
		}
		paths = append(paths, impact.Request.Paths...)
		for _, selector := range impact.Coverage.Critical {
			if p, ok := strings.CutPrefix(selector, "path:"); ok {
				paths = append(paths, p)
			}
		}
		for _, omitted := range impact.Omissions.Samples {
			paths = append(paths, omitted.Path)
		}
		section["native_omissions"] = impact.Omissions
	}
	if p := flagValue(original, "--subject"); p != "" {
		paths = append(paths, p)
	}
	if command == "affected" {
		var plan struct {
			Dirty []string `json:"dirty"`
		}
		_ = json.Unmarshal(native["plan"], &plan)
		paths = append(paths, plan.Dirty...)
	}
	section["impact"] = doccorpus.Impact(corpus, paths, overlay.State, depth)
	encoded, err := doccorpus.Encode(section)
	if err != nil {
		emitCorpusError(stderr, err)
		return 2, true
	}
	native["documentation"] = json.RawMessage(bytes.TrimSpace(encoded))
	output, err := doccorpus.Encode(native)
	if err != nil {
		emitCorpusError(stderr, err)
		return 2, true
	}
	if _, err := stdout.Write(output); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write corpus output"})
		return 2, true
	}
	return 0, true
}
func flagValue(args []string, name string) string {
	for i, arg := range args {
		if arg == name && i+1 < len(args) {
			return args[i+1]
		}
		if value, ok := strings.CutPrefix(arg, name+"="); ok {
			return value
		}
	}
	return ""
}

// corpusNativeValueFollows mirrors the native parsers: `--root` refuses an option-like
// value exactly as rootPreambleValue does, so `--root --corpus=FILE` is never swallowed as
// the root value; every other native value flag preserves its next token byte-for-byte.
func corpusNativeValueFollows(args []string, i int) bool {
	if args[i] == "--root" {
		return rootPreambleValue(args, i+1)
	}
	return i+1 < len(args)
}

func corpusNativeValueFlag(flag string) bool {
	switch flag {
	case "--root", "--task", "--subject", "--kind", "--language", "--limit", "--max-bytes", "--receipt", "--map", "--base", "--output", "--manifest", "--harness", "--plan", "--scope", "--artifact", "--id", "--source", "--package", "--format", "--output-dir", "--repository":
		return true
	}
	return false
}

// locateCore finds the corvint executable; tests point it at a freshly built one.
var locateCore = func() (string, error) { return docsenvelope.Locate(docsenvelope.CoreBinary) }

// runCoreRead runs the native read in the corvint executable beside this one or on PATH, with
// the original argument vector and this process's stdin, and captures its stdout (CMS-V0-003:
// the app is an untrusted caller of the core's command line and passes it no authority). The
// core's own stderr envelope passes through; its exit status is returned unchanged.
func runCoreRead(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	executable, err := locateCore()
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: docsenvelope.NotInstalledCode, Message: docsenvelope.NotInstalledMessage(docsenvelope.CoreBinary, "--corpus")})
		return 2
	}
	// CMS-V0-002: a corvint installed beside this executable must be the same release.
	if reason := docsenvelope.CheckBeside(ctx, executable, versionSpec().Line()); reason != "" {
		emitError(stderr, &gokernel.Error{Code: docsenvelope.VersionMismatchCode, Message: "--corpus refused: " + reason})
		return 2
	}
	command := exec.CommandContext(ctx, executable, args...)
	command.Stdin, command.Stdout, command.Stderr = stdin, stdout, stderr
	docsenvelope.StopOnCancel(command, 5*time.Second)
	err = command.Run()
	var exit *exec.ExitError
	var refused *gokernel.Error
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		return exit.ExitCode()
	case ctx.Err() != nil:
		emitError(stderr, &gokernel.Error{Code: "cancelled", Message: "the core read was interrupted"})
	case errors.As(err, &refused):
		// The bounded capture refused more output; report its own refusal.
		emitError(stderr, refused)
	default:
		emitError(stderr, &gokernel.Error{Code: "internal-error", Message: "cannot run " + docsenvelope.CoreBinary + ": " + err.Error()})
	}
	return 2
}
