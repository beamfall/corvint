// corvint-analyzer-structured-data is an unselected experimental analyzer.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/analyzerstructured"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout)) }

// run returns 1 when the descriptor fails or the frame cannot be written in full.
func run(args []string, stdin *os.File, stdout io.Writer) int {
	frame, err := selectFrame(args, stdin)
	if err != nil || writeAll(stdout, frame) != nil {
		return 1
	}
	return 0
}

func selectFrame(args []string, stdin *os.File) ([]byte, error) {
	if len(args) == 1 && args[0] == "--version" {
		return []byte(fmt.Sprintf("{\"profile\":%q,\"family\":%q,\"status\":\"UNSELECTED\"}\n", analyzerstructured.Profile, analyzerstructured.Family)), nil
	}
	if len(args) == 1 && args[0] == "--descriptor" {
		return analyzerstructured.CandidateDescriptor()
	}
	if len(args) != 0 {
		return noncanonical, nil
	}
	// ACP-012: a stdin stat, read, or descriptor-identity failure is the
	// NONCANONICAL_REQUEST sentinel; Analyze reports an oversize frame as
	// LIMIT_EXCEEDED.
	before, err := stdin.Stat()
	if err != nil {
		return noncanonical, nil
	}
	frame, err := io.ReadAll(io.LimitReader(stdin, 1_500_001))
	after, statErr := stdin.Stat()
	if err != nil || statErr != nil || !os.SameFile(before, after) {
		return noncanonical, nil
	}
	return analyzerstructured.Analyze(frame), nil
}

var noncanonical = []byte(`{"profile":"corvint-structured-data/experimental-v1","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n")

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if written < 0 || written > len(data) || written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
		if err != nil {
			return err
		}
	}
	return nil
}
