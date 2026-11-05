package main

import (
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/analyzerjs"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

// An unexpected argv rejects like a malformed request instead of silently
// analyzing stdin, matching the sibling candidates in the same
// corvint-analyzer-candidate/experimental family (ACP-011).
func run(args []string, stdin io.Reader, stdout, _ io.Writer) int {
	if len(args) != 0 {
		return writeCandidate(stdout, analyzerjs.Rejection("unknown", "NONCANONICAL_REQUEST"))
	}
	raw, e := io.ReadAll(io.LimitReader(stdin, 1500001))
	if e != nil {
		return writeCandidate(stdout, analyzerjs.Rejection("unknown", "NONCANONICAL_REQUEST"))
	}
	if len(raw) > 1500000 {
		return writeCandidate(stdout, analyzerjs.Rejection("unknown", "LIMIT_EXCEEDED"))
	}
	request, e := analyzerjs.DecodeRequest(raw)
	if e != nil {
		return writeCandidate(stdout, analyzerjs.RejectionForRequest(request, analyzerjs.FailureReason(e)))
	}
	candidate, e := analyzerjs.Analyze(request)
	if e != nil {
		candidate = analyzerjs.RejectionForRequest(request, analyzerjs.FailureReason(e))
	}
	return writeCandidate(stdout, candidate)
}
func writeCandidate(stdout io.Writer, candidate analyzerjs.Candidate) int {
	encoded, e := analyzerjs.EncodeCandidate(candidate)
	if e != nil {
		return 2
	}
	encoded = append(encoded, '\n')
	for len(encoded) > 0 {
		written, err := stdout.Write(encoded)
		if err != nil || written <= 0 {
			return 2
		}
		encoded = encoded[written:]
	}
	return 0
}
