// corvint-cem-candidate is an optional experimental reference-integrity companion.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/verify"
	cc "github.com/Beamfall/corvint/internal/cemcandidate"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(command(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
func command(ctx context.Context, args []string, out, errout io.Writer) int {
	if len(args) > 0 && args[0] == "assemble-stable" {
		return stableCommand(ctx, args[1:], out, errout)
	}
	if len(args) == 0 {
		fmt.Fprintln(errout, "experimental candidate companion: verify | assemble")
		return 2
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	f.SetOutput(errout)
	experimental := f.Bool("experimental", false, "admit experimental candidate interface")
	var repository, mapFile, base, target, artifacts, request, output *string
	switch args[0] {
	case "verify":
		repository = f.String("repository", "", "immutable repository")
		mapFile = f.String("map", "", "candidate map")
		base = f.String("expected-base", "", "independent full base OID")
		target = f.String("target", "", "independent full target OID")
		artifacts = f.String("artifacts", "", "absolute artifact root")
	case "assemble":
		request = f.String("request", "", "closed assembly request")
		output = f.String("out-dir", "", "new absolute bundle directory")
	default:
		return 2
	}
	seen := map[string]bool{}
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") {
			n := strings.SplitN(strings.TrimLeft(a, "-"), "=", 2)[0]
			if seen[n] {
				fmt.Fprintln(errout, "repeated flag", n)
				return 2
			}
			seen[n] = true
		}
	}
	if f.Parse(args[1:]) != nil || f.NArg() != 0 || !*experimental {
		return 2
	}
	if args[0] == "assemble" {
		if *request == "" || *output == "" {
			return 2
		}
	} else if *repository == "" || *mapFile == "" || *base == "" || *target == "" || *artifacts == "" {
		return 2
	}
	fail := func(e error) int { fmt.Fprintln(errout, "candidate refused:", e); return 1 }
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	if args[0] == "assemble" {
		raw, e := cc.Read(*request)
		if e != nil {
			return fail(e)
		}
		var r cc.Request
		if e := cc.Decode(raw, &r); e != nil {
			return fail(e)
		}
		result, e := cc.Assemble(ctx, r, *output)
		if e != nil {
			return fail(e)
		}
		if e := json.NewEncoder(out).Encode(result); e != nil {
			return fail(e)
		}
		return 0
	}
	raw, e := cc.Read(*mapFile)
	if e != nil {
		return fail(e)
	}
	repo, e := gitauth.Open(*repository, gitrun.NewDefaultBudget())
	if e != nil {
		return fail(e)
	}
	result, e := verify.ExperimentalCandidate(ctx, repo, raw, verify.CandidateOptions{ExpectedBase: *base, Target: *target, ArtifactRoot: *artifacts})
	if encodeErr := json.NewEncoder(out).Encode(result); encodeErr != nil {
		return fail(encodeErr)
	}
	if e != nil {
		return fail(e)
	}
	return 0
}
