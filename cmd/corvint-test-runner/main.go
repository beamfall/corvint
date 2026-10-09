// corvint-test-runner is an experimental optional companion. Trusted test code
// runs locally; its receipts attest observed bytes, not complete dependencies.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

const profile = "corvint-test-runner-plan/0"

type plan = tr.PlanDocument
type receipt = tr.ReceiptDocument

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(command(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
func command(ctx context.Context, args []string, out, errout io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errout, "experimental runner companion: runners | plan | run")
		return 2
	}
	if args[0] == "runners" {
		if len(args) != 1 {
			return 2
		}
		json.NewEncoder(out).Encode(registry.Runners())
		return 0
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	f.SetOutput(errout)
	request := f.String("request", "", "bounded request JSON")
	planPath := f.String("plan", "", "previously reviewed plan JSON")
	executable := f.String("executable", "", "independently trusted absolute executable")
	sha := f.String("executable-sha256", "", "independently pinned executable SHA256")
	toolsPath := f.String("tools", "", "independently trusted auxiliary Tool map JSON")
	approve := f.String("approve", "", "exact canonical plan SHA256")
	output := f.String("out", "", "NEW output JSON file; default stdout")
	experimental := f.Bool("experimental", false, "admit experimental execution")
	trusted := f.Bool("trusted-local", false, "admit execution of trusted local tests")
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
	if f.Parse(args[1:]) != nil || f.NArg() != 0 {
		return 2
	}
	fail := func(e error) int { fmt.Fprintln(errout, "runner refused:", e); return 1 }
	if *executable == "" || *sha == "" {
		return fail(fmt.Errorf("independent executable identity required"))
	}
	var trustedTools map[string]tr.Tool
	if *toolsPath != "" {
		if e := read(*toolsPath, &trustedTools); e != nil {
			return fail(e)
		}
	}
	var p plan
	switch args[0] {
	case "plan":
		if e := read(*request, &p.Request); e != nil {
			return fail(e)
		}
		if p.Request.Executable != "" && p.Request.Executable != *executable || p.Request.ExecutableSha256 != "" && p.Request.ExecutableSha256 != *sha {
			return fail(fmt.Errorf("request disagrees with independent executable identity"))
		}
		if len(p.Request.Tools) > 0 && tr.Identity(p.Request.Tools) != tr.Identity(trustedTools) {
			return fail(fmt.Errorf("request auxiliary tools disagree with independent pins"))
		}
		if len(trustedTools) > 0 {
			p.Request.Tools = trustedTools
		}
		p.Profile = profile
		p.Request.Executable = *executable
		p.Request.ExecutableSha256 = *sha
		v, e := registry.Build(p.Request)
		if e != nil {
			return fail(e)
		}
		p.Invocation = v
		if e = write(*output, p, out); e != nil {
			return fail(e)
		}
		fmt.Fprintln(errout, "planSha256:", tr.Identity(p))
		return 0
	case "run":
		if !*experimental || !*trusted {
			return fail(fmt.Errorf("experimental and trusted-local admissions required"))
		}
		if e := read(*planPath, &p); e != nil {
			return fail(e)
		}
		if p.Profile != profile || p.Request.Executable != *executable || p.Request.ExecutableSha256 != *sha || *approve != tr.Identity(p) {
			return fail(fmt.Errorf("plan admission or independently trusted executable mismatch"))
		}
		if len(p.Request.Tools) > 0 && tr.Identity(p.Request.Tools) != tr.Identity(trustedTools) {
			return fail(fmt.Errorf("plan auxiliary tools disagree with independent pins"))
		}
		v, e := admittedInvocation(p)
		if e != nil {
			return fail(e)
		}
		if *output == "" || !filepath.IsAbs(*output) {
			return fail(fmt.Errorf("run requires a new absolute receipt file"))
		}
		source, e := filepath.EvalSymlinks(p.Request.Root)
		if e != nil {
			return fail(e)
		}
		parent, e := filepath.EvalSymlinks(filepath.Dir(*output))
		if e != nil {
			return fail(e)
		}
		rel, e := filepath.Rel(source, filepath.Join(parent, filepath.Base(*output)))
		if e != nil || rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return fail(fmt.Errorf("receipt must be outside source root"))
		}
		reserved, e := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return fail(e)
		}
		defer reserved.Close()
		x, runErr := tr.Execute(ctx, p.Request, v)
		o, parseErr := registry.Parse(x.Input)
		msg := ""
		if runErr != nil {
			msg = runErr.Error()
		}
		if parseErr != nil {
			if msg != "" {
				msg += "; "
			}
			msg += parseErr.Error()
		}
		r := receipt{Profile: "corvint-test-runner-receipt/0", PlanSha256: tr.Identity(p), Execution: x, Observation: o, Error: msg}
		if e = json.NewEncoder(reserved).Encode(r); e != nil {
			return fail(e)
		}
		if e = reserved.Close(); e != nil {
			return fail(e)
		}
		if runErr != nil || parseErr != nil || !o.Complete {
			return 1
		}
		for _, t := range o.Tests {
			if t.State == tr.Failed || t.State == tr.Flaky {
				return 1
			}
		}
		return 0
	default:
		return fail(fmt.Errorf("unknown command"))
	}
}

// admittedInvocation rebuilds the fixed profile and requires the plan to match
// it exactly. A plan from before TRE-V0-030 keeps its identity, so its
// receipts still bind, but it is refused for new execution: running it
// without retirement could hide a detached-process leak, and running it with
// retirement would break its receipt's invocation binding. Re-plan instead.
func admittedInvocation(p plan) (tr.Invocation, error) {
	v, e := registry.Build(p.Request)
	if e != nil {
		return v, e
	}
	if tr.Identity(v) == tr.Identity(p.Invocation) {
		return v, nil
	}
	compare := v
	compare.RetireDetachedDescendants = false
	if v.RetireDetachedDescendants && tr.Identity(compare) == tr.Identity(p.Invocation) {
		return v, fmt.Errorf("plan predates detached descendant retirement; re-plan it")
	}
	return v, fmt.Errorf("plan does not match current fixed runner profile")
}

// read refuses a nonregular document before any blocking open (V1-0624): a
// FIFO without a writer would otherwise block os.Open, which a signal context
// cannot interrupt. The nonblocking open and same-file check close the swap race.
func read(path string, dst any) error {
	before, e := os.Stat(path)
	if e != nil {
		return e
	}
	if !before.Mode().IsRegular() {
		return fmt.Errorf("regular document required")
	}
	f, e := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if e != nil {
		return e
	}
	defer f.Close()
	s, e := f.Stat()
	if e != nil || !s.Mode().IsRegular() || !os.SameFile(before, s) {
		return fmt.Errorf("regular document required")
	}
	b, e := io.ReadAll(io.LimitReader(f, tr.MaxReportBytes+1))
	if e != nil {
		return e
	}
	return tr.DecodeDocument(b, dst)
}
func write(path string, v any, out io.Writer) error {
	if path == "" {
		return json.NewEncoder(out).Encode(v)
	}
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	encodeErr := json.NewEncoder(f).Encode(v)
	closeErr := f.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}
