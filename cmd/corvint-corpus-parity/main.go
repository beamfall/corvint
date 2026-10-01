// corvint-corpus-parity is an experimental local immutable-corpus companion.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Beamfall/corvint/internal/corpusindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
func run(ctx context.Context, args []string, out, errout io.Writer) int {
	f := flag.NewFlagSet("corvint-corpus-parity", flag.ContinueOnError)
	f.SetOutput(errout)
	mode := f.String("mode", "parity", "build, query, or parity (experimental)")
	root := f.String("root", "", "source-revalidating producer only")
	artifact := f.String("artifact", "", "producer /2 corpus")
	path := f.String("index", "", "embedded indexed artifact")
	digest := f.String("sha256", "", "required operator-pinned index digest")
	request := f.String("request", "", "query or question recording JSON")
	if f.Parse(args) != nil || f.NArg() != 0 {
		return 2
	}
	fail := func(e error) int { fmt.Fprintln(errout, "corvint-corpus-parity:", e); return 2 }
	var data []byte
	var e error
	if *mode == "build" {
		if *root == "" || *artifact == "" || *path != "" || *digest != "" || *request != "" {
			return fail(fmt.Errorf("build requires only root and artifact"))
		}
		// Assign the outer error: a shadowed producer error must never exit zero.
		var raw []byte
		if raw, e = doccorpus.ReadCorpusFile(*root, *artifact); e != nil {
			return fail(e)
		}
		data, e = corpusindex.Build(ctx, *root, raw)
	} else {
		if *root != "" || *artifact != "" || *path == "" || *digest == "" || *request == "" {
			return fail(fmt.Errorf("read requires only index, sha256 and request"))
		}
		raw, err := corpusindex.ReadFile(*path, corpusindex.MaxBytes)
		if err != nil {
			return fail(err)
		}
		r, err := corpusindex.Open(ctx, raw, *digest)
		if err != nil {
			return fail(err)
		}
		limit := int64(doccorpus.MaxCorpusBytes)
		if *mode == "query" {
			limit = 16 << 10
		}
		input, err := corpusindex.ReadFile(*request, limit)
		if err != nil {
			return fail(err)
		}
		switch *mode {
		case "query":
			q, err := corpusindex.ParseRequest(input)
			if err != nil {
				return fail(err)
			}
			receipt, err := r.Query(ctx, q)
			if err != nil {
				return fail(err)
			}
			data, e = doccorpus.Encode(receipt)
		case "parity":
			report, err := corpusindex.Compare(ctx, r, input)
			if err != nil {
				return fail(err)
			}
			data, e = corpusindex.Encode(report)
		default:
			return fail(fmt.Errorf("unknown mode"))
		}
	}
	if e != nil {
		return fail(e)
	}
	if _, e = out.Write(data); e != nil {
		return fail(e)
	}
	return 0
}
