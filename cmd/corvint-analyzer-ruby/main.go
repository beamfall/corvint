// corvint-analyzer-ruby is an unregistered experimental static analyzer.
package main

import (
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/analyzerruby"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout)) }

// run deliberately owns only inherited standard streams. It opens no caller
// path and the analyzer itself has no ambient filesystem, process, or network
// dependency.
func run(args []string, in io.Reader, out io.Writer) int {
	if len(args) != 0 {
		return writeResponse(out, analyzerruby.AnalyzeCanonical(nil), 0)
	}
	request, err := io.ReadAll(io.LimitReader(in, analyzerruby.MaxRequestBytes+1))
	// ACP-012: a read failure is the NONCANONICAL_REQUEST sentinel and an
	// oversize request is the LIMIT_EXCEEDED sentinel; both frames exit 0.
	if err != nil {
		return writeResponse(out, analyzerruby.AnalyzeCanonical(nil), 0)
	}
	if len(request) > analyzerruby.MaxRequestBytes {
		return writeResponse(out, limitExceeded, 0)
	}
	return writeResponse(out, analyzerruby.AnalyzeCanonical(request), 0)
}

var limitExceeded = []byte(`{"profile":"` + analyzerruby.Profile + `","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"LIMIT_EXCEEDED"}` + "\n")

func writeResponse(out io.Writer, response []byte, code int) int {
	for len(response) != 0 {
		n, err := out.Write(response)
		if err != nil || n < 0 || n > len(response) || n == 0 {
			return 1
		}
		response = response[n:]
	}
	return code
}
