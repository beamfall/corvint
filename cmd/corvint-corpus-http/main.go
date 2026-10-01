// corvint-corpus-http is an explicitly started experimental companion transport.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Beamfall/corvint/internal/corpusindex"
	"github.com/Beamfall/corvint/internal/corpusserve"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Stderr))
}
func run(ctx context.Context, args []string, errout io.Writer) int {
	f := flag.NewFlagSet("corvint-corpus-http", flag.ContinueOnError)
	f.SetOutput(errout)
	artifact := f.String("index", "", "embedded index artifact")
	digest := f.String("sha256", "", "required operator-pinned SHA256")
	address := f.String("listen", "127.0.0.1:0", "explicit companion listener")
	remote := f.Bool("allow-remote", false, "operator declares external TLS/auth boundary; external qualification NOT_OBSERVED")
	if f.Parse(args) != nil || f.NArg() != 0 || *artifact == "" || *digest == "" {
		return 2
	}
	host, _, e := net.SplitHostPort(*address)
	ip := net.ParseIP(host)
	if e != nil || !*remote && (ip == nil || !ip.IsLoopback()) {
		fmt.Fprintln(errout, "corvint-corpus-http: remote address requires explicit allow-remote")
		return 2
	}
	raw, e := corpusindex.ReadFile(*artifact, corpusindex.MaxBytes)
	if e != nil {
		fmt.Fprintln(errout, "corvint-corpus-http: index unavailable")
		return 2
	}
	reader, e := corpusindex.Open(ctx, raw, *digest)
	if e != nil {
		fmt.Fprintln(errout, "corvint-corpus-http: index refused")
		return 2
	}
	listener, e := net.Listen("tcp", *address)
	if e != nil {
		fmt.Fprintln(errout, "corvint-corpus-http: listener unavailable")
		return 2
	}
	defer listener.Close()
	server := corpusserve.NewServer(*address, reader)
	fmt.Fprintln(errout, "corvint-corpus-http: experimental JSON query listener", listener.Addr(), "index", reader.Digest(), "external TLS/auth and deployment qualification NOT_OBSERVED")
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case e := <-done:
		if e != nil && e != http.ErrServerClosed {
			return 2
		}
		return 0
	case <-ctx.Done():
		stop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if server.Shutdown(stop) != nil {
			_ = server.Close()
		}
		<-done
		return 0
	}
}
