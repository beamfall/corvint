// corvint-analyzer-shader is an unselected experimental shader analyzer candidate.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/analyzershader"
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
		if err := writeAll(output, []byte(`{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}`+"\n")); err != nil {
			return 1
		}
		return 0
	}
	data, err := io.ReadAll(io.LimitReader(input, 1_500_001))
	// ACP-012: a read failure is the NONCANONICAL_REQUEST sentinel, exit 0;
	// Analyze already reports an oversize request as LIMIT_EXCEEDED.
	if err != nil {
		data = nil
	}
	if err := writeAll(output, analyzershader.Analyze(data)); err != nil {
		return 1
	}
	return 0
}

func versionFrame() []byte {
	return []byte(fmt.Sprintf("{\"profile\":%q,\"family\":%q,\"status\":\"EXPERIMENTAL_UNSELECTABLE\",\"cem\":%q,\"ocm\":%q}\n", analyzershader.Profile, analyzershader.Family, analyzershader.CEMOCMState, analyzershader.CEMOCMState))
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
