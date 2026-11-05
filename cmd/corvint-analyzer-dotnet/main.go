// Command corvint-analyzer-dotnet is the isolated .NET analyzer candidate. It
// reads one canonical request envelope from stdin and writes one canonical
// response plus LF to stdout. It opens no path and starts no subprocess.
package main

import (
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/analyzerdotnet"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout)) }

// An unexpected argv rejects like a malformed request instead of silently
// analyzing stdin, matching the sibling candidates in the same
// corvint-analyzer-candidate/experimental family (ACP-011).
func run(args []string, stdin io.Reader, stdout io.Writer) int {
	if len(args) != 0 {
		return emit(stdout, analyzerdotnet.Sentinel("NONCANONICAL_REQUEST"))
	}
	// The profile reads at most 1,500,000 bytes and rejects byte 1,500,001
	// without decoding it, so the reader is bounded one byte past the limit.
	raw, err := io.ReadAll(io.LimitReader(stdin, analyzerdotnet.MaxRequestBytes+1))
	if err != nil {
		return emit(stdout, analyzerdotnet.Sentinel("NONCANONICAL_REQUEST"))
	}
	if len(raw) > analyzerdotnet.MaxRequestBytes {
		return emit(stdout, analyzerdotnet.Sentinel("LIMIT_EXCEEDED"))
	}
	return emit(stdout, analyzerdotnet.AnalyzeCanonical(raw))
}

func emit(stdout io.Writer, response []byte) int {
	for len(response) > 0 {
		written, err := stdout.Write(response)
		if err != nil || written <= 0 {
			return 2
		}
		response = response[written:]
	}
	return 0
}
