package rubytestprovider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
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
	// Image unknown reasons of a Corvint formatter run (LPCV-V0-138).
	runtimeRunContainerUnknown = "run-container-unknown"
	runtimeHostInspectUnavail  = "host-inspect-unavailable"
	runtimeNotInspectable      = "container-not-inspectable"

	hostInspectTimeout = time.Minute

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

// observeRunImage asks the host docker named by the prefix for the image ID of
// the container the RSpec process reported, after the run (LPCV-V0-138). The
// container ID is a validated 64-hex value, never prefix text; any prefix
// argument before the docker subcommand (such as -H or --context) is kept so
// the inspection reaches the same daemon. A container removed by --rm, or any
// failed or malformed inspection, is an explicit unknown.
func observeRunImage(ctx context.Context, prefix []string, container *RuntimeFact, dir string) RuntimeFact {
	if container == nil || container.Value == "" {
		return RuntimeFact{Unknown: runtimeRunContainerUnknown}
	}
	if len(prefix) == 0 || filepath.Base(prefix[0]) != "docker" {
		return RuntimeFact{Unknown: runtimeHostInspectUnavail}
	}
	global := -1
	for index, argument := range prefix[1:] {
		if argument == "run" || argument == "compose" || argument == "exec" || argument == "container" {
			global = index + 1
			break
		}
	}
	if global < 0 {
		return RuntimeFact{Unknown: runtimeHostInspectUnavail}
	}
	argv := append(append([]string{}, prefix[:global]...), "container", "inspect", "--format", "{{.Image}}", container.Value)
	observation := procgroup.Run(ctx, procgroup.Spec{Argv: resolveArgv(argv), Dir: dir, Env: os.Environ(), Timeout: hostInspectTimeout, OutputLimit: runtimeProbeOutput, OverflowPolicy: procgroup.OverflowTruncate})
	if !observation.Started || observation.TimedOut || observation.Cancelled || observation.Signal != "" || !observation.ExitObserved || observation.ExitStatus != 0 || observation.StdoutOverflow {
		return RuntimeFact{Unknown: runtimeNotInspectable}
	}
	image := strings.TrimSpace(string(observation.Stdout))
	if !imageDigest(image) {
		return RuntimeFact{Unknown: runtimeNotInspectable}
	}
	return RuntimeFact{Value: image}
}

// imageDigest admits a docker image ID: sha256: and 64 lowercase hex digits.
func imageDigest(value string) bool {
	digest, ok := strings.CutPrefix(value, "sha256:")
	return ok && blobPattern("sha256", digest)
}

func validateRuntime(runtime *Runtime) error {
	if runtime == nil {
		return nil
	}
	if runtime.Basis != RuntimeBasis {
		return errors.New("Ruby provider runtime identity has an unknown basis")
	}
	containerValue := func(value string) bool { return blobPattern("sha256", value) }
	probeReasons := []string{runtimeProbeFailed, runtimeNotReported, runtimeNotInsidePfx}
	// Without a run container the image is never observed: any value is a
	// claim. With one, the image is the host inspection of it (LPCV-V0-138).
	imageValid, imageReasons := func(string) bool { return false }, []string{runtimeNotInsidePfx}
	checks := []struct {
		fact    RuntimeFact
		valid   func(string) bool
		reasons []string
	}{
		{runtime.Ruby, versionValue, probeReasons},
		{runtime.Bundler, versionValue, probeReasons},
		{runtime.ProbeContainer, containerValue, probeReasons},
	}
	if runtime.RunContainer != nil {
		checks = append(checks, struct {
			fact    RuntimeFact
			valid   func(string) bool
			reasons []string
		}{*runtime.RunContainer, containerValue, []string{runtimeNotReported}})
		imageReasons = []string{runtimeRunContainerUnknown, runtimeHostInspectUnavail, runtimeNotInspectable}
		if runtime.RunContainer.Value != "" {
			imageValid = imageDigest
		}
	}
	checks = append(checks, struct {
		fact    RuntimeFact
		valid   func(string) bool
		reasons []string
	}{runtime.Image, imageValid, imageReasons})
	for _, check := range checks {
		switch {
		case (check.fact.Value == "") == (check.fact.Unknown == ""):
			return errors.New("Ruby provider runtime fact must carry exactly one of value and unknown")
		case check.fact.Value != "" && !check.valid(check.fact.Value):
			return errors.New("Ruby provider runtime fact value is malformed")
		case check.fact.Unknown != "" && !slices.Contains(check.reasons, check.fact.Unknown):
			return errors.New("Ruby provider runtime fact reason is outside the closed set")
		}
	}
	return nil
}
