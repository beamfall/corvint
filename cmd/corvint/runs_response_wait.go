package main

import (
	"errors"
	"flag"
	"io"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/playwrightruns"
)

// runRunsResponseWait diagnoses one timed-out response wait against a run's request log
// (TSA-V0-205). A log that cannot be read is an UNKNOWN report, not a refusal.
func runRunsResponseWait(root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("runs response-wait", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	logPath, start, timeout := "", "", ""
	wait := playwrightruns.ResponseWait{}
	f.StringVar(&logPath, "log", "", "HAR or JSON-lines request log")
	f.StringVar(&wait.URLContains, "url-contains", "", "URL substring the awaited response matches")
	f.StringVar(&wait.Method, "method", "", "request method the awaited response matches")
	f.StringVar(&start, "wait-start", "", "RFC 3339 time the wait began")
	f.StringVar(&timeout, "timeout-ms", "", "wait timeout in milliseconds")
	if !runsParse(f, args, diagnostic, func() []runsNeed {
		return []runsNeed{runsRequired("log", logPath == ""), runsRequired("url-contains", wait.URLContains == ""),
			runsRequired("wait-start", start == ""), runsRequired("timeout-ms", timeout == "")}
	}) {
		return 2
	}
	at, err := time.Parse(time.RFC3339Nano, start)
	if err != nil {
		return runsInvalid(diagnostic, "wait-start", runsArgValueInvalid)
	}
	ms, err := strconv.ParseInt(timeout, 10, 64)
	if err != nil || ms <= 0 || ms > 24*60*60*1000 {
		return runsInvalid(diagnostic, "timeout-ms", runsArgValueInvalid)
	}
	wait.Start, wait.Timeout = at, time.Duration(ms)*time.Millisecond
	if root != "" && !filepath.IsAbs(logPath) {
		logPath = filepath.Join(root, logPath)
	}
	log, err := playwrightruns.ReadRequestLogFile(logPath)
	reason := ""
	switch {
	case errors.Is(err, playwrightruns.ErrUnreadable):
		reason = playwrightruns.WaitLogUnavailable
	case err != nil:
		reason = playwrightruns.WaitLogUnreadable
	}
	return runsEmit(playwrightruns.DiagnoseResponseWait(log, wait, reason), out, diagnostic)
}
