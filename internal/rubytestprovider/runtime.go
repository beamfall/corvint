package rubytestprovider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/rubytestreceipt"
)

// RuntimeBasis is rubytestreceipt.RuntimeBasis, kept under its old name.
const RuntimeBasis = rubytestreceipt.RuntimeBasis

// runtimeProbeScript runs as `ruby -e` inside the prefix. It prints one line:
// the Ruby version, the version of the Bundler that Ruby loads, and the ID of
// the container whose /etc/hostname bind mount names it, when there is one.
// It reads no project file and writes nothing.
const runtimeProbeScript = `o=["ruby="+RUBY_VERSION]
begin;require "bundler";o<<"bundler="+Bundler::VERSION;rescue LoadError;end
begin;File.foreach("/proc/self/mountinfo"){|l|f=l.split(" ");m=f[4]=="/etc/hostname"&&f[3].to_s[%r{containers/([0-9a-f]{64})/},1];o<<"container="+m if m};rescue SystemCallError;end
puts "` + rubytestreceipt.RuntimeProbeMarker + ` "+o.join(" ")
`

// probeRuntime observes the runtime identity inside the exec prefix with one
// separate invocation of the prefix running `ruby -e`. Nothing observed on the
// host is recorded: a fact the probe did not report is an explicit unknown.
// The image digest is not observable from inside a container.
func probeRuntime(ctx context.Context, prefix []string, dir string, timeout time.Duration) *Runtime {
	runtime := &Runtime{Basis: RuntimeBasis, Image: RuntimeFact{Unknown: rubytestreceipt.RuntimeNotInsidePfx}}
	argv := append(append([]string{}, prefix...), "ruby", "-e", runtimeProbeScript)
	observation := procgroup.Run(ctx, procgroup.Spec{Argv: resolveArgv(argv), Dir: dir, Env: os.Environ(), Timeout: min(timeout, rubytestreceipt.RuntimeProbeTimeout), OutputLimit: rubytestreceipt.RuntimeProbeOutput, OverflowPolicy: procgroup.OverflowTruncate})
	facts, err := runtimeFacts(observation)
	if err != nil {
		runtime.Ruby, runtime.Bundler, runtime.ProbeContainer = RuntimeFact{Unknown: rubytestreceipt.RuntimeProbeFailed}, RuntimeFact{Unknown: rubytestreceipt.RuntimeProbeFailed}, RuntimeFact{Unknown: rubytestreceipt.RuntimeProbeFailed}
		return runtime
	}
	runtime.Ruby = fact(facts, "ruby", rubytestreceipt.VersionValue)
	runtime.Bundler = fact(facts, "bundler", rubytestreceipt.VersionValue)
	runtime.ProbeContainer = fact(facts, "container", func(value string) bool { return rubytestreceipt.BlobPattern("sha256", value) })
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
		if rest, ok := strings.CutPrefix(strings.TrimSuffix(candidate, "\r"), rubytestreceipt.RuntimeProbeMarker+" "); ok {
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
	return RuntimeFact{Unknown: rubytestreceipt.RuntimeNotReported}
}

// observeRunImage asks the host docker named by the prefix for the image ID of
// the container the RSpec process reported, after the run (LPCV-V0-138). The
// container ID is a validated 64-hex value, never prefix text; any prefix
// argument before the docker subcommand (such as -H or --context) is kept so
// the inspection reaches the same daemon. A container removed by --rm, or any
// failed or malformed inspection, is an explicit unknown.
func observeRunImage(ctx context.Context, prefix []string, container *RuntimeFact, dir string) RuntimeFact {
	if container == nil || container.Value == "" {
		return RuntimeFact{Unknown: rubytestreceipt.RuntimeRunContainerUnknown}
	}
	if len(prefix) == 0 || filepath.Base(prefix[0]) != "docker" {
		return RuntimeFact{Unknown: rubytestreceipt.RuntimeHostInspectUnavail}
	}
	global := -1
	for index, argument := range prefix[1:] {
		if argument == "run" || argument == "compose" || argument == "exec" || argument == "container" {
			global = index + 1
			break
		}
	}
	if global < 0 {
		return RuntimeFact{Unknown: rubytestreceipt.RuntimeHostInspectUnavail}
	}
	argv := append(append([]string{}, prefix[:global]...), "container", "inspect", "--format", "{{.Image}}", container.Value)
	observation := procgroup.Run(ctx, procgroup.Spec{Argv: resolveArgv(argv), Dir: dir, Env: os.Environ(), Timeout: rubytestreceipt.HostInspectTimeout, OutputLimit: rubytestreceipt.RuntimeProbeOutput, OverflowPolicy: procgroup.OverflowTruncate})
	if !observation.Started || observation.TimedOut || observation.Cancelled || observation.Signal != "" || !observation.ExitObserved || observation.ExitStatus != 0 || observation.StdoutOverflow {
		return RuntimeFact{Unknown: rubytestreceipt.RuntimeNotInspectable}
	}
	image := strings.TrimSpace(string(observation.Stdout))
	if !rubytestreceipt.ImageDigest(image) {
		return RuntimeFact{Unknown: rubytestreceipt.RuntimeNotInspectable}
	}
	return RuntimeFact{Value: image}
}
