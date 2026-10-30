// corvint-analyzer-html-css is an unselected experimental analyzer candidate.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/analyzerhtmlcss"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout)) }

// run returns 1 only when the frame cannot be written in full (ACP-011).
func run(args []string, stdin *os.File, stdout io.Writer) int {
	frame := preEnvelopeSentinel()
	if len(args) == 1 && args[0] == "--version" {
		frame = []byte(fmt.Sprintf("{\"profile\":%q,\"family\":%q,\"html_input_profile\":%q,\"css_input_profile\":%q,\"source\":%q,\"web_source\":%q,\"toolchain\":%q}\n", analyzerhtmlcss.Profile, analyzerhtmlcss.Family, analyzerhtmlcss.HTMLInputProfile, analyzerhtmlcss.CSSInputProfile, analyzerhtmlcss.SourceCoordinate, analyzerhtmlcss.WebSourceCoordinate, analyzerhtmlcss.ToolchainCoordinate))
	}
	if len(args) == 0 {
		frame = analyzeStdin(stdin)
	}
	if writeAll(stdout, frame) != nil {
		return 1
	}
	return 0
}

func analyzeStdin(stdin *os.File) []byte {
	before, err := stdin.Stat()
	if err != nil {
		return preEnvelopeSentinel()
	}
	data, err := io.ReadAll(io.LimitReader(stdin, 1500001))
	after, statErr := stdin.Stat()
	if err != nil || statErr != nil || !os.SameFile(before, after) {
		return preEnvelopeSentinel()
	}
	return analyzerhtmlcss.Analyze(data)
}

func preEnvelopeSentinel() []byte {
	return []byte(`{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n")
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
