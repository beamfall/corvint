package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/requestpath"
)

func init() { flowSubcommands["request-path"] = runFlowsRequestPath }

// flowsRequestPathHelp documents `flows request-path`; help.go appends it to flowsHelp.
const flowsRequestPathHelp = `
Request-path usage (AMAP-V0-130..139):
  corvint [--root PATH] flows request-path --file PATH (--route 'METHOD /path' | --handler NAME)
      [--revision REV] [--env-profile FILE [--warn]]

request-path reads one Express-style route registered in --file (or one named handler) from
committed JavaScript/TypeScript at --revision (default HEAD) and writes corvint-request-path/0:
each guard and the handler in run order, the environment and configuration keys each reads,
the outbound network calls each makes with the call chain, and guard-order findings where a guard
with an outbound call runs before a cheaper guard. A key, call, middleware or mount it cannot
resolve statically is listed as unknown with a reason, never omitted. --env-profile checks a
dotenv-format target profile before a batch is scheduled and names unset or empty keys only,
never values; BLOCKED or INCOMPLETE exits 1 (fail closed) unless --warn. It writes nothing.
`

func runFlowsRequestPath(ctx context.Context, root string, args []string, out, diag io.Writer) error {
	f := flag.NewFlagSet("flows request-path", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	file := f.String("file", "", "committed file registering the route or declaring the handler")
	route := f.String("route", "", "METHOD /path")
	handler := f.String("handler", "", "function name")
	revision := f.String("revision", "HEAD", "commit to read")
	profile := f.String("env-profile", "", "dotenv-format target environment profile")
	warn := f.Bool("warn", false, "schedule with warnings instead of failing closed")
	usage := errors.New("flows request-path requires --file PATH and exactly one of --route 'METHOD /path' or --handler NAME, with --warn only with --env-profile FILE")
	if f.Parse(args) != nil || f.NArg() != 0 || *file == "" || (*route == "") == (*handler == "") || (*warn && *profile == "") {
		return usage
	}
	clean, ok := requestpath.CleanPath(*file)
	if !ok {
		return usage
	}
	q := requestpath.Query{File: clean, Handler: *handler}
	if *route != "" {
		method, p, found := strings.Cut(strings.TrimSpace(*route), " ")
		p = strings.TrimSpace(p)
		if !found || !strings.HasPrefix(p, "/") {
			return usage
		}
		q.Method, q.Path = strings.ToUpper(method), p
	}
	var prof *requestpath.Profile
	if *profile != "" {
		text, err := readProfile(*profile)
		if err != nil {
			return err
		}
		if prof, err = requestpath.ParseProfile(*profile, text); err != nil {
			return codedRequestPath(err)
		}
	}
	src, err := requestpath.OpenGit(ctx, root, *revision)
	if err != nil {
		return codedRequestPath(err)
	}
	report, err := requestpath.Analyze(src, src.Commit(), q)
	if err != nil {
		return codedRequestPath(err)
	}
	if prof != nil {
		report.PreRun = requestpath.Check(report, prof, *warn)
	}
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}
	if _, err = out.Write(append(data, '\n')); err != nil {
		return err
	}
	if report.PreRun != nil && report.PreRun.Batch == requestpath.BatchDoNotSchedule {
		return &gateFailure{"flows request-path: " + report.PreRun.Verdict + ": " + report.PreRun.Message + "; the batch is not scheduled"}
	}
	return nil
}

// readProfile reads at most MaxProfileBytes of the operator's profile file.
func readProfile(name string) (string, error) {
	fh, err := os.Open(name)
	if err != nil {
		return "", &gokernel.Error{Code: "request-path-profile-invalid", Message: "the profile is unreadable"}
	}
	defer fh.Close()
	data, err := io.ReadAll(io.LimitReader(fh, requestpath.MaxProfileBytes+1))
	if err != nil {
		return "", &gokernel.Error{Code: "request-path-profile-invalid", Message: "the profile is unreadable"}
	}
	return string(data), nil
}

func codedRequestPath(err error) error {
	var e *requestpath.Error
	if errors.As(err, &e) {
		return &gokernel.Error{Code: e.Code, Message: e.Message}
	}
	return err
}
