// corvint-analyzer-rust is an unselected experimental Rust analyzer candidate.
// It is unregistered and unreachable from Core selection; the `rust` family is
// specified by docs/specs/rust-analyzer-candidate-v0.md, whose intent status is
// accepted by decision 0007, D6; Core support and qualification remain ungranted.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/analyzerrust"
)

func main() {
	if run(os.Args[1:], os.Stdin, os.Stdout) != 0 {
		os.Exit(1)
	}
}

func run(args []string, input io.Reader, output io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		if err := writeAll(output, versionFrame()); err != nil {
			return 1
		}
		return 0
	}
	if len(args) != 0 {
		if err := writeAll(output, rejectFrame("NONCANONICAL_REQUEST")); err != nil {
			return 1
		}
		return 0
	}
	data, err := io.ReadAll(io.LimitReader(input, analyzerrust.MaxRequestBytes+1))
	// ACP-012: a read failure is the NONCANONICAL_REQUEST sentinel and an
	// oversize request is the LIMIT_EXCEEDED sentinel; both frames exit 0.
	if err != nil {
		data = nil
	}
	response := analyzerrust.AnalyzeCanonical(data)
	if len(data) > analyzerrust.MaxRequestBytes {
		response = rejectFrame("LIMIT_EXCEEDED")
	}
	if err := writeAll(output, response); err != nil {
		return 1
	}
	return 0
}

func versionFrame() []byte {
	return []byte(fmt.Sprintf("{\"profile\":%q,\"family\":%q,\"status\":\"EXPERIMENTAL_UNSELECTABLE\",\"cem\":%q,\"ocm\":%q}\n",
		analyzerrust.Profile, analyzerrust.Family, analyzerrust.CEMOCMState, analyzerrust.CEMOCMState))
}

func rejectFrame(reason string) []byte {
	return []byte(`{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"` + reason + `"}` + "\n")
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if written < 0 || written > len(data) {
			return io.ErrShortWrite
		}
		data = data[written:]
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
