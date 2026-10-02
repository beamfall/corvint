package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	cc "github.com/Beamfall/corvint/internal/cemcandidate"
)

func stableCommand(ctx context.Context, args []string, out, errout io.Writer) int {
	f := flag.NewFlagSet("assemble-stable", flag.ContinueOnError)
	f.SetOutput(errout)
	experimental := f.Bool("experimental", false, "admit experimental stable assembly")
	request := f.String("request", "", "closed stable assembly request")
	output := f.String("out-dir", "", "new absolute bundle directory")
	seen := map[string]bool{}
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			n := strings.SplitN(strings.TrimLeft(a, "-"), "=", 2)[0]
			if seen[n] {
				fmt.Fprintln(errout, "repeated flag", n)
				return 2
			}
			seen[n] = true
		}
	}
	if f.Parse(args) != nil || f.NArg() != 0 || !*experimental || *request == "" || *output == "" {
		return 2
	}
	fail := func(e error) int { fmt.Fprintln(errout, "stable assembly refused:", e); return 1 }
	raw, e := cc.Read(*request)
	if e != nil {
		return fail(e)
	}
	var r cc.StableRequest
	if e := cc.Decode(raw, &r); e != nil {
		return fail(e)
	}
	result, e := cc.AssembleStable(ctx, r, *output)
	if e != nil {
		return fail(e)
	}
	if e := json.NewEncoder(out).Encode(result); e != nil {
		return fail(e)
	}
	return 0
}
