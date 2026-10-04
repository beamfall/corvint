package main

import (
	"context"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/dogfoodflow"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
	"io"
	"path/filepath"
	"strings"
)

func runDogfoodOutcomeWorker(arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// This is deliberately before parsing input or resolving the repository.
	if err := gitstatus.EnableOwnedWorker(); err != nil {
		emitDogfoodRecordError(stderr, "aggregate-worker-invalid", err)
		return 2
	}
	if len(arguments) != 1 || !filepath.IsAbs(arguments[0]) {
		emitDogfoodRecordError(stderr, "aggregate-worker-invalid", argumentError("worker requires one absolute root"))
		return 2
	}
	raw, err := io.ReadAll(io.LimitReader(stdin, tracerecordrepo.AggregateReceiptLimit+4096+1))
	if err != nil {
		emitDogfoodRecordError(stderr, "aggregate-worker-invalid", err)
		return 2
	}
	request, err := dogfoodflow.ParseAggregateWorkerRequest(raw)
	if err != nil {
		emitDogfoodRecordError(stderr, "aggregate-worker-invalid", err)
		return 2
	}
	result, err := dogfoodflow.ExecuteAggregateWorker(context.Background(), arguments[0], request)
	if err != nil {
		emitDogfoodRecordError(stderr, dogfoodflow.AggregateWorkerFailureCode(err), err)
		return 2
	}
	if _, err = stdout.Write(result); err != nil {
		return 2
	}
	return 0
}

// The separate verifier entrypoint admits only the three read commands used by
// strict Check. It cannot produce an aggregate receipt or select a write path.
func runDogfoodVerifierWorker(arguments []string, stdout, stderr io.Writer) int {
	if err := gitstatus.EnableOwnedWorker(); err != nil {
		emitDogfoodRecordError(stderr, "aggregate-worker-invalid", err)
		return 2
	}
	if len(arguments) < 2 || !filepath.IsAbs(arguments[0]) || !dogfoodflow.ValidAggregateVerifierArguments(arguments[1:]) {
		emitDogfoodRecordError(stderr, "aggregate-worker-invalid", argumentError("invalid contained verifier invocation"))
		return 2
	}
	ctx := gitrun.WithInheritedWorkerGroup(context.Background())
	return runContext(ctx, append([]string{"--root", arguments[0]}, arguments[1:]...), strings.NewReader(""), stdout, stderr)
}
