//go:build darwin || linux

// corvint-postmerge-metrics is an optional, local, recommendation-only companion.
package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/postmergemetrics"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	cancel()
	os.Exit(code)
}
func read(path string, limit int) ([]byte, error) {
	f, e := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, postmergemetrics.ErrInput
	}
	defer f.Close()
	s, e := f.Stat()
	if e != nil || !s.Mode().IsRegular() || s.Size() > int64(limit) {
		return nil, postmergemetrics.ErrInput
	}
	b, e := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if e != nil || len(b) > limit {
		return nil, postmergemetrics.ErrInput
	}
	return b, nil
}
func run(ctx context.Context, args []string, out, errOut io.Writer) int {
	fail := func(e error) int {
		code, ok := e.(postmergemetrics.Error)
		if !ok {
			code = postmergemetrics.ErrInput
		}
		_ = json.NewEncoder(errOut).Encode(struct {
			Error string `json:"error"`
		}{string(code)})
		return 1
	}
	if len(args) != 9 || args[0] != "report" {
		return fail(postmergemetrics.ErrInput)
	}
	flags := map[string]string{}
	for i := 1; i < len(args); i += 2 {
		switch args[i] {
		case "--policy", "--records", "--from", "--until":
		default:
			return fail(postmergemetrics.ErrInput)
		}
		if _, ok := flags[args[i]]; ok || args[i+1] == "" {
			return fail(postmergemetrics.ErrInput)
		}
		flags[args[i]] = args[i+1]
	}
	pb, e := read(flags["--policy"], postmergemetrics.PolicyLimit)
	if e != nil {
		return fail(e)
	}
	hb, e := read(flags["--records"], postmergemetrics.HistoryLimit)
	if e != nil {
		return fail(e)
	}
	p, e := postmergemetrics.ParsePolicy(pb)
	if e != nil {
		return fail(e)
	}
	h, e := postmergemetrics.ParseHistory(hb)
	if e != nil {
		return fail(e)
	}
	m, e := postmergemetrics.NewGitMeasurer()
	if e != nil {
		return fail(e)
	}
	report, e := postmergemetrics.Build(ctx, p, h, flags["--from"], flags["--until"], m)
	if e != nil {
		return fail(e)
	}
	if ctx.Err() != nil {
		return fail(postmergemetrics.ErrCancelled)
	}
	if json.NewEncoder(out).Encode(report) != nil {
		return fail(postmergemetrics.ErrInput)
	}
	return 0
}
