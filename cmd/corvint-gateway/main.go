// Command corvint-gateway is the experimental loopback model gateway described
// by docs/specs/local-model-gateway-v0.md.
//
// It is a separate, opt-in executable and is never linked into `corvint`. It
// binds loopback only, runs for the lifetime of this foreground `serve`
// invocation, installs no service, and sends nothing anywhere except each
// request the client was already making, forwarded to the upstream it names.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/clitree"
	"github.com/Beamfall/corvint/internal/companioncli"
	"github.com/Beamfall/corvint/internal/modelgateway"
)

// usageText is the synopsis text the command tree derives from (CAP-V0-972).
const usageText = `corvint-gateway serve [--addr HOST:PORT] [--upstream URL] [--state DIR] [--fold-threshold BYTES] [--no-fold]
corvint-gateway report [--state DIR]
`

var commandSummaries = map[string]string{
	"serve":  "Proxy Anthropic Messages API traffic on a loopback address, recording token usage and folding large Bash output",
	"report": "Summarise the gateway's usage ledger without writing anything",
}

var commandExamples = map[string]string{
	"serve":  "corvint-gateway serve --addr 127.0.0.1:8787",
	"report": "corvint-gateway report",
}

var companionHelp = sync.OnceValue(func() clitree.Help {
	return clitree.Help{Tree: clitree.CompanionTree("corvint-gateway", usageText, commandSummaries, commandExamples, nil, nil),
		Title: "the experimental loopback model gateway"}
})

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(runContext(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runContext(context.Background(), args, stdout, stderr)
}

func runContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if code, handled := companionHelp().Serve(args, stdout, stderr); handled {
		return code
	}
	if code, handled := companioncli.Answer(companioncli.Spec{Binary: "corvint-gateway", Version: companioncli.ExperimentalVersion, Usage: usageText}, args, stdout); handled {
		return code
	}
	if len(args) == 0 {
		fmt.Fprint(stderr, companionHelp().Root())
		return 2
	}
	switch args[0] {
	case "serve":
		return serve(ctx, args[1:], stderr)
	case "report":
		return report(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "corvint-gateway: unknown command %q\n", args[0])
	return 2
}

func defaultState() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".corvint-gateway"
	}
	return filepath.Join(home, ".corvint-gateway")
}

func serve(ctx context.Context, args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	addr := flags.String("addr", "127.0.0.1:8787", "loopback address to bind; a non-loopback address is refused")
	upstream := flags.String("upstream", "https://api.anthropic.com", "Messages API base URL to forward to")
	state := flags.String("state", defaultState(), "directory for usage.jsonl and the fold archive")
	threshold := flags.Int("fold-threshold", 8192, "fold Bash results longer than this many bytes")
	noFold := flags.Bool("no-fold", false, "forward every request body unchanged; still record usage")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "corvint-gateway serve: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if err := modelgateway.CheckLoopback(*addr); err != nil {
		fmt.Fprintf(stderr, "corvint-gateway: %v\n", err)
		return 2
	}
	target, err := modelgateway.ParseUpstream(*upstream)
	if err != nil {
		fmt.Fprintf(stderr, "corvint-gateway: %v\n", err)
		return 2
	}
	if err := os.MkdirAll(*state, 0o700); err != nil {
		fmt.Fprintf(stderr, "corvint-gateway: %v\n", err)
		return 1
	}
	ledger, err := os.OpenFile(filepath.Join(*state, "usage.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintf(stderr, "corvint-gateway: %v\n", err)
		return 1
	}
	defer ledger.Close()
	var folder *modelgateway.Folder
	if !*noFold {
		folder = modelgateway.NewFolder(modelgateway.FoldOptions{Threshold: *threshold, Archive: filepath.Join(*state, "archive")})
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintf(stderr, "corvint-gateway: %v\n", err)
		return 1
	}
	proxy := modelgateway.NewProxy(target, folder, ledger)
	proxy.Log = stderr
	server := &http.Server{Handler: proxy, ReadHeaderTimeout: 30 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	fmt.Fprintf(stderr, "corvint-gateway: listening on http://%s, forwarding to %s, state %s, folding %v\n", listener.Addr(), target, *state, folder != nil)
	fmt.Fprintf(stderr, "corvint-gateway: point the client at it with ANTHROPIC_BASE_URL=http://%s; ^C to stop\n", listener.Addr())
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(stderr, "corvint-gateway: %v\n", err)
		return 1
	}
	return 0
}

func report(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("report", flag.ContinueOnError)
	flags.SetOutput(stderr)
	state := flags.String("state", defaultState(), "directory holding usage.jsonl")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "corvint-gateway report: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	file, err := os.Open(filepath.Join(*state, "usage.jsonl"))
	if err != nil {
		fmt.Fprintf(stderr, "corvint-gateway: %v\n", err)
		return 1
	}
	defer file.Close()
	t, err := modelgateway.Summarise(file)
	if err != nil {
		fmt.Fprintf(stderr, "corvint-gateway: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "requests %d (errors or unreadable lines %d)\n", t.Requests, t.Errors)
	fmt.Fprintf(stdout, "input tokens %d uncached, %d cache-read, %d cache-creation; output tokens %d\n", t.Input, t.CacheRead, t.CacheCreation, t.Output)
	if total := t.Input + t.CacheRead + t.CacheCreation; total > 0 {
		fmt.Fprintf(stdout, "cache-read share of input %.1f%%\n", 100*float64(t.CacheRead)/float64(total))
	}
	fmt.Fprintf(stdout, "request bytes %d received, %d forwarded", t.RequestBytes, t.ForwardedBytes)
	if t.RequestBytes > 0 {
		fmt.Fprintf(stdout, " (%.1f%% smaller)", 100*(1-float64(t.ForwardedBytes)/float64(t.RequestBytes)))
	}
	fmt.Fprintf(stdout, "; %d results folded\n", t.Folds)
	if t.ToolsRequestBytes > 0 {
		fmt.Fprintf(stdout, "tool definitions %d bytes, %.1f%% of the requests that carried them\n", t.ToolsBytes, 100*float64(t.ToolsBytes)/float64(t.ToolsRequestBytes))
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		fmt.Fprintf(stderr, "corvint-gateway: %v\n", err)
		return 1
	}
	breaks, err := modelgateway.CacheBreaks(file)
	if err != nil {
		fmt.Fprintf(stderr, "corvint-gateway: %v\n", err)
		return 1
	}
	created := int64(0)
	for _, b := range breaks {
		created += b.Created
	}
	fmt.Fprintf(stdout, "cache breaks %d (%d cache-creation tokens on those requests)\n", len(breaks), created)
	for i, b := range breaks {
		if i == maxListedBreaks {
			fmt.Fprintf(stdout, "  ... %d more\n", len(breaks)-i)
			break
		}
		fmt.Fprintf(stdout, "  %s %s session %s: read %d of %d cached, wrote %d, %s after the previous start; %s\n",
			b.At, b.Model, b.SessionKey, b.Read, b.Expected, b.Created, b.Gap.Round(time.Second), b.Cause)
	}
	return 0
}

// maxListedBreaks bounds the per-break lines report prints.
const maxListedBreaks = 20
