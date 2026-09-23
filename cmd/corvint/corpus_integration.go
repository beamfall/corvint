package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/doccorpus"
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
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			original = append(original, args[i:]...)
			break
		}
		name, value, inline := strings.Cut(args[i], "=")
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
	if artifact == "" {
		return 0, false
	}
	pos := commandPositionAfterRoots(original)
	if pos < 0 {
		emitError(stderr, argumentError("--corpus requires a native read command"))
		return 2, true
	}
	command := original[pos]
	allowed := command == "query" || command == "context" || command == "impact" || command == "affected" || command == "test-validity" || command == "work" || command == "cem"
	if command == "work" {
		allowed = pos+1 < len(original) && (original[pos+1] == "observe" || original[pos+1] == "propose-wave")
	}
	if command == "cem" {
		allowed = pos+1 < len(original) && (original[pos+1] == "status" || original[pos+1] == "verify" || original[pos+1] == "report")
	}
	if command == "cem" && flagValue(original, "--output") != "" {
		allowed = false
	}
	if !allowed {
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
	lexicalRoot, err := filepath.Abs(root)
	if err != nil {
		emitCorpusError(stderr, err)
		return 2, true
	}
	root, err = resolveExplicitRoot(root)
	if err != nil {
		emitError(stderr, err)
		return 2, true
	}
	raw, err := doccorpus.ReadFile(root, artifact)
	if err != nil {
		emitCorpusError(stderr, err)
		return 2, true
	}
	corpus, err := doccorpus.Open(ctx, root, raw)
	if err != nil {
		emitCorpusError(stderr, err)
		return 2, true
	}

	var receiptBytes []byte
	receiptPath := flagValue(original, "--receipt")
	if command == "test-validity" && receiptPath != "" {
		absolute, pathErr := filepath.Abs(receiptPath)
		if pathErr != nil {
			emitCorpusError(stderr, pathErr)
			return 2, true
		}
		relative, pathErr := filepath.Rel(lexicalRoot, absolute)
		if pathErr != nil {
			emitCorpusError(stderr, pathErr)
			return 2, true
		}
		receiptBytes, err = doccorpus.ReadFile(root, relative)
		if err != nil {
			emitCorpusError(stderr, err)
			return 2, true
		}
	}
	var captured corpusCapture
	exit := runContext(ctx, original, stdin, &captured, stderr)
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
	freshness, limitations, err := doccorpus.Freshness(ctx, root, corpus)
	if err != nil {
		emitCorpusError(stderr, err)
		return 2, true
	}
	query := flagValue(original, "--task")
	request := doccorpus.Request{Operation: "info"}
	if command == "query" || command == "context" {
		request.Operation = "search"
		request.Query = query
	}
	receipt, err := doccorpus.Query(corpus, request, freshness, limitations)
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
	if command == "impact" || command == "affected" || len(paths) > 0 {
		section["impact"] = doccorpus.Impact(corpus, paths, freshness)
	}
	if command == "test-validity" {
		// The native document stays intact; exact retained input digests alone join
		// its observations to the corpus. Similar test names never establish a link.
		linked := []doccorpus.Observation{}
		if receiptPath != "" {
			absolute, _ := filepath.Abs(receiptPath)
			relative, _ := filepath.Rel(lexicalRoot, absolute)
			after, readErr := doccorpus.ReadFile(root, relative)
			if readErr != nil || !bytes.Equal(after, receiptBytes) {
				emitError(stderr, argumentError("retained receipt changed during corpus join"))
				return 2, true
			}
			digest := doccorpus.Digest(receiptBytes)
			for _, o := range corpus.Observations {
				if o.InputSHA256 == digest {
					linked = append(linked, o)
				}
			}
		}
		section["observations"] = linked
		section["observation_limitations"] = []string{"only an exact explicitly supplied receipt is joined; discovery and missing matches remain unlinked"}
	}
	if command == "cem" {
		mapPath := flagValue(original, "--map")
		data, readErr := doccorpus.ReadFile(root, mapPath)
		if readErr != nil {
			emitCorpusError(stderr, readErr)
			return 2, true
		}
		projection, projectionErr := doccorpus.CEMProjection(ctx, root, corpus, data, "")
		if projectionErr != nil {
			emitCorpusError(stderr, projectionErr)
			return 2, true
		}
		section["cem"] = projection
	}
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
	case "--root", "--task", "--subject", "--kind", "--language", "--limit", "--max-bytes", "--receipt", "--map", "--base", "--output", "--intent", "--manifest", "--harness", "--plan", "--scope", "--artifact", "--id", "--expected-base", "--target", "--envelope", "--source", "--package", "--format", "--output-dir", "--executor", "--observations", "--repository":
		return true
	}
	return false
}
