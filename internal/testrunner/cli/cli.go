// Package cli is the runner companion's command entry, shared by the `corvint-test-runner` shim
// and the `corvint-test runner` group (CTF-V0-004). Trusted test code runs locally; its receipts
// attest observed bytes, not complete dependencies.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// PlanProfile is the plan document profile ID; it is unchanged by the fold (CTF-V0-003).
const PlanProfile = tr.PlanProfile

type plan = tr.PlanDocument
type receipt = tr.ReceiptDocument

// Command runs the runner companion under the invocation name prog: `corvint-test-runner` for the
// old companion shim, `corvint-test runner` for the group (CTF-V0-004). Only diagnostics and help
// name prog; output documents do not depend on it (CTF-V0-002, CTF-V0-007).
func Command(ctx context.Context, prog string, args []string, out, errout io.Writer) int {
	usage := fmt.Sprintf(usageFormat, prog)
	if len(args) == 0 {
		fmt.Fprintln(errout, usage)
		return 2
	}
	switch args[0] {
	case "-h", "-help", "--help", "help":
		// TRE-V0-078: help needs no executable pin.
		fmt.Fprintln(out, usage)
		return 0
	case "runners", "profiles":
		// TRE-V0-079: read-only listings need no executable pin.
		if len(args) > 1 && slices.ContainsFunc(args[1:], isHelp) {
			fmt.Fprintf(out, "usage: %s %s\n\nLists the runner profile IDs as JSON. No executable pin is needed.\n", prog, args[0])
			return 0
		}
		if len(args) != 1 {
			fmt.Fprintln(errout, usage)
			return 2
		}
		json.NewEncoder(out).Encode(registry.Runners())
		return 0
	case "plan", "run":
	default:
		fmt.Fprintf(errout, "unknown command %q\n%s\n", args[0], usage)
		return 2
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	f.SetOutput(errout)
	f.Usage = func() {
		fmt.Fprintf(f.Output(), "usage: %s %s [flags]\n\nflags:\n", prog, args[0])
		f.PrintDefaults()
	}
	request := f.String("request", "", "bounded request JSON")
	planPath := f.String("plan", "", "previously reviewed plan JSON")
	executable := f.String("executable", "", "independently trusted absolute executable")
	sha := f.String("executable-sha256", "", "independently pinned executable SHA256")
	toolsPath := f.String("tools", "", "independently trusted auxiliary Tool map JSON")
	approve := f.String("approve", "", "exact canonical plan SHA256")
	output := f.String("out", "", "NEW output JSON file; default stdout")
	experimental := f.Bool("experimental", false, "admit experimental execution")
	trusted := f.Bool("trusted-local", false, "admit execution of trusted local tests")
	for _, a := range args[1:] {
		if isHelp(a) {
			// TRE-V0-078: per-command help prints to stdout and exits 0.
			f.SetOutput(out)
			f.Usage()
			return 0
		}
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
	if f.Parse(args[1:]) != nil || f.NArg() != 0 {
		return 2
	}
	fail := func(e error) int { fmt.Fprintln(errout, "runner refused:", e); return 1 }
	if e := missingIdentity(*executable, *sha); e != nil {
		return fail(e)
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
		p.Profile = PlanProfile
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
		if p.Profile != PlanProfile || p.Request.Executable != *executable || p.Request.ExecutableSha256 != *sha || *approve != tr.Identity(p) {
			return fail(fmt.Errorf("plan admission or independently trusted executable mismatch"))
		}
		if len(p.Request.Tools) > 0 && tr.Identity(p.Request.Tools) != tr.Identity(trustedTools) {
			return fail(fmt.Errorf("plan auxiliary tools disagree with independent pins"))
		}
		v, e := AdmittedInvocation(p)
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

// usageFormat names the invocation as %[1]s so profile IDs in the text are never rewritten.
const usageFormat = `usage: %[1]s <command> [flags] (experimental runner companion)

commands:
  runners    list runner profile IDs as JSON (no executable pin)
  profiles   alias of runners (no executable pin)
  plan       derive a plan; requires --executable and --executable-sha256
  run        execute an approved plan; requires --executable and --executable-sha256

Run "%[1]s <command> --help" for its flags.`

func isHelp(a string) bool { return a == "-h" || a == "-help" || a == "--help" }

// missingIdentity names every absent independent executable pin flag
// (TRE-V0-080), so the refusal says exactly what the operator must supply.
func missingIdentity(executable, sha string) error {
	var missing []string
	if executable == "" {
		missing = append(missing, "--executable")
	}
	if sha == "" {
		missing = append(missing, "--executable-sha256")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("independent executable identity required: missing %s", strings.Join(missing, " and "))
}

// AdmittedInvocation rebuilds the fixed profile and requires the plan to match
// it exactly. A plan from before TRE-V0-030 keeps its identity, so its
// receipts still bind, but it is refused for new execution: running it
// without retirement could hide a detached-process leak, and running it with
// retirement would break its receipt's invocation binding. Re-plan instead.
// A device-bound plan from before TRE-V0-087 is refused the same way.
func AdmittedInvocation(p plan) (tr.Invocation, error) {
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
	preCleanup := v
	preCleanup.DeviceCleanup = nil
	if len(v.DeviceCleanup) > 0 && tr.Identity(preCleanup) == tr.Identity(p.Invocation) {
		return v, fmt.Errorf("plan predates device cleanup; re-plan it")
	}
	if p.Request.Runner == "gradle-junit" && tr.Identity(preIsolationGradle(compare)) == tr.Identity(p.Invocation) {
		return v, fmt.Errorf("plan predates per-run Gradle user home; re-plan it")
	}
	return v, fmt.Errorf("plan does not match current fixed runner profile")
}

// preIsolationGradle reconstructs the exact historical gradle-junit invocation
// (TRE-V0-043): the current one without its single --gradle-user-home pair.
func preIsolationGradle(v tr.Invocation) tr.Invocation {
	argv := make([]string, 0, len(v.Argv))
	for i := 0; i < len(v.Argv); i++ {
		if v.Argv[i] == "--gradle-user-home" && i+1 < len(v.Argv) {
			i++
			continue
		}
		argv = append(argv, v.Argv[i])
	}
	v.Argv = argv
	return v
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
