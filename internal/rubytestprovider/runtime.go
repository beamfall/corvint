package rubytestprovider

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// Runtime identity basis and closed unknown reasons (LPCV-V0-133).
const (
	RuntimeBasis = "exec-prefix-probe"

	runtimeProbeFailed  = "probe-failed"
	runtimeNotReported  = "not-reported"
	runtimeNotInsidePfx = "not-observable-inside-prefix"

	runtimeProbeTimeout = 5 * time.Minute
	runtimeProbeOutput  = 64 << 10
	runtimeProbeMarker  = "corvint-runtime-identity"
)

// runtimeProbeScript runs as `ruby -e` inside the prefix. It prints one line:
// the Ruby version, the version of the Bundler that Ruby loads, and the ID of
// the container whose /etc/hostname bind mount names it, when there is one.
// It reads no project file and writes nothing.
const runtimeProbeScript = `o=["ruby="+RUBY_VERSION]
begin;require "bundler";o<<"bundler="+Bundler::VERSION;rescue LoadError;end
begin;File.foreach("/proc/self/mountinfo"){|l|f=l.split(" ");m=f[4]=="/etc/hostname"&&f[3].to_s[%r{containers/([0-9a-f]{64})/},1];o<<"container="+m if m};rescue SystemCallError;end
puts "` + runtimeProbeMarker + ` "+o.join(" ")
`

// probeRuntime observes the runtime identity inside the exec prefix with one
// separate invocation of the prefix running `ruby -e`. Nothing observed on the
// host is recorded: a fact the probe did not report is an explicit unknown.
// The image digest is not observable from inside a container.
func probeRuntime(ctx context.Context, prefix []string, dir string, timeout time.Duration) *Runtime {
	runtime := &Runtime{Basis: RuntimeBasis, Image: RuntimeFact{Unknown: runtimeNotInsidePfx}}
	argv := append(append([]string{}, prefix...), "ruby", "-e", runtimeProbeScript)
	observation := procgroup.Run(ctx, procgroup.Spec{Argv: resolveArgv(argv), Dir: dir, Env: os.Environ(), Timeout: min(timeout, runtimeProbeTimeout), OutputLimit: runtimeProbeOutput, OverflowPolicy: procgroup.OverflowTruncate})
	facts, err := runtimeFacts(observation)
	if err != nil {
		runtime.Ruby, runtime.Bundler, runtime.ProbeContainer = RuntimeFact{Unknown: runtimeProbeFailed}, RuntimeFact{Unknown: runtimeProbeFailed}, RuntimeFact{Unknown: runtimeProbeFailed}
		return runtime
	}
	runtime.Ruby = fact(facts, "ruby", versionValue)
	runtime.Bundler = fact(facts, "bundler", versionValue)
	runtime.ProbeContainer = fact(facts, "container", func(value string) bool { return blobPattern("sha256", value) })
	return runtime
}

// runtimeFacts reads the probe's single identity line from a completed,
// zero-exit probe; a repeated key is dropped so it reads as not reported.
func runtimeFacts(observation procgroup.Observation) (map[string]string, error) {
	if !observation.Started || observation.TimedOut || observation.Cancelled || observation.Signal != "" || !observation.ExitObserved || observation.ExitStatus != 0 || observation.StdoutOverflow || observation.StderrOverflow {
		return nil, errors.New("runtime probe did not complete")
	}
	var line string
	found := 0
	for _, candidate := range strings.Split(string(observation.Stdout), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSuffix(candidate, "\r"), runtimeProbeMarker+" "); ok {
			line, found = rest, found+1
		}
	}
	if found != 1 {
		return nil, errors.New("runtime probe printed no single identity line")
	}
	facts, repeated := map[string]string{}, map[string]bool{}
	for _, token := range strings.Fields(line) {
		key, value, ok := strings.Cut(token, "=")
		if !ok {
			continue
		}
		if _, seen := facts[key]; seen {
			repeated[key] = true
		}
		facts[key] = value
	}
	for key := range repeated {
		delete(facts, key)
	}
	return facts, nil
}

func fact(facts map[string]string, key string, valid func(string) bool) RuntimeFact {
	if value, ok := facts[key]; ok && valid(value) {
		return RuntimeFact{Value: value}
	}
	return RuntimeFact{Unknown: runtimeNotReported}
}

// versionValue admits a bounded version string, the runnerVersion charset.
func versionValue(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '.' || c == '+' || c == '-') {
			return false
		}
	}
	return true
}

func validateRuntime(runtime *Runtime) error {
	if runtime == nil {
		return nil
	}
	if runtime.Basis != RuntimeBasis {
		return errors.New("Ruby provider runtime identity has an unknown basis")
	}
	for _, check := range []struct {
		fact  RuntimeFact
		valid func(string) bool
	}{
		{runtime.Ruby, versionValue},
		{runtime.Bundler, versionValue},
		{runtime.ProbeContainer, func(value string) bool { return blobPattern("sha256", value) }},
		// The probe basis never observes the image: any value is a claim.
		{runtime.Image, func(string) bool { return false }},
	} {
		switch {
		case (check.fact.Value == "") == (check.fact.Unknown == ""):
			return errors.New("Ruby provider runtime fact must carry exactly one of value and unknown")
		case check.fact.Value != "" && !check.valid(check.fact.Value):
			return errors.New("Ruby provider runtime fact value is malformed")
		case check.fact.Unknown != "" && check.fact.Unknown != runtimeProbeFailed && check.fact.Unknown != runtimeNotReported && check.fact.Unknown != runtimeNotInsidePfx:
			return errors.New("Ruby provider runtime fact reason is outside the closed set")
		}
	}
	return nil
}
