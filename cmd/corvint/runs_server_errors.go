package main

import (
	"flag"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/playwrightruns"
)

// runRunsServerErrors attaches the server-side exception to each failed attempt that names an
// HTTP 5xx (docs/specs/observed-ui-evidence-v0.md, OUE-V0-050..057). An unreadable server log is
// a refusal, never a report that says no server error happened.
func runRunsServerErrors(root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("runs server-errors", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	files := []string{}
	logPath, requestsPath, window := "", "", ""
	support := []string{}
	f.Func("runs", "Playwright JSON reports", runsFiles(root, &files))
	f.StringVar(&logPath, "server-log", "", "server-error-log/0 JSON-lines file")
	f.StringVar(&requestsPath, "requests", "", "optional HAR or JSON-lines request log")
	f.StringVar(&window, "window-ms", "", "match tolerance around each attempt, in milliseconds")
	f.Func("support-path", "test-support path prefixes", func(v string) error {
		for _, p := range strings.Split(v, ",") {
			if p == "" || len(support) >= 64 {
				return errRunsArgument
			}
			support = append(support, p)
		}
		return nil
	})
	if !runsParse(f, args, diagnostic, func() []runsNeed {
		return []runsNeed{runsRequired("runs", len(files) == 0), runsRequired("server-log", logPath == "")}
	}) {
		return 2
	}
	opts := playwrightruns.ServerErrorOptions{SupportPaths: support}
	if window != "" {
		ms, err := strconv.ParseInt(window, 10, 64)
		if err != nil || ms <= 0 || ms > 10*60*1000 {
			return runsInvalid(diagnostic, "window-ms", runsArgValueInvalid)
		}
		opts.Window = time.Duration(ms) * time.Millisecond
	}
	resolve := func(p string) string {
		if root != "" && !filepath.IsAbs(p) {
			return filepath.Join(root, p)
		}
		return p
	}
	runs := []*playwrightruns.Run{}
	for i, name := range files {
		run, err := playwrightruns.ReadFile(name)
		if err != nil {
			return runsRefuse(diagnostic, "runs-report-unavailable", map[string]any{"reason": err.Error(), "run": i + 1})
		}
		runs = append(runs, run)
	}
	errs, err := playwrightruns.ReadServerErrorLogFile(resolve(logPath))
	if err != nil {
		return runsRefuse(diagnostic, "runs-server-log-unavailable", map[string]any{"reason": err.Error()})
	}
	var requests *playwrightruns.RequestLog
	if requestsPath != "" {
		if requests, err = playwrightruns.ReadRequestLogFile(resolve(requestsPath)); err != nil {
			return runsRefuse(diagnostic, "runs-request-log-unavailable", map[string]any{"reason": err.Error()})
		}
	}
	return runsEmit(playwrightruns.AttachServerErrors(runs, errs, requests, opts), out, diagnostic)
}
