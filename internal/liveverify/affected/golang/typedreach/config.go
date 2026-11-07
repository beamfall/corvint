package typedreach

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Config is the build configuration of the test run a Program models
// (AFP-V0-2372): the go command's own build context under the executor's
// environment and flags, so build constraints select the source the test
// binary compiles. Load without a Config refuses constrained source.
type Config struct {
	GOROOT      string   `json:"goroot"`
	GOOS        string   `json:"goos"`
	GOARCH      string   `json:"goarch"`
	CgoEnabled  bool     `json:"cgoEnabled"`
	Compiler    string   `json:"compiler"`
	UseAllFiles bool     `json:"useAllFiles"`
	BuildTags   []string `json:"buildTags"`
	ToolTags    []string `json:"toolTags"`
	ReleaseTags []string `json:"releaseTags"`
}

// Identity names the configuration in the analysis identity a slice reason
// carries, so a run under any other configuration plans another manifest.
func (c Config) Identity() string {
	encoded, _ := json.Marshal(c)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:8])
}

// testOnlyFlags are go test flags that take a value and select no source;
// testOnlySwitches take none, or an inline value. selectingFlags change the
// build context: -tags its build tags, the instrumentation modes a tool tag.
var (
	testOnlyFlags = map[string]bool{
		"-exec": true, "-timeout": true, "-count": true, "-run": true, "-skip": true, "-parallel": true,
		"-cpu": true, "-shuffle": true, "-p": true, "-vet": true, "-bench": true, "-benchtime": true,
	}
	// moduleModes are the -mod values that keep the module's own source and
	// its module cache dependencies; -mod=vendor compiles other source.
	moduleModes      = map[string]bool{"readonly": true, "mod": true}
	testOnlySwitches = map[string]bool{
		"-json": true, "-failfast": true, "-short": true, "-v": true, "-benchmem": true,
		"-modcacherw": true, "-trimpath": true, "-buildvcs": true,
	}
	selectingFlags    = map[string]bool{"-tags": true}
	selectingSwitches = map[string]bool{"-race": true, "-msan": true, "-asan": true}
)

// BuildFlags returns the flags of a go test command line that the go
// command's build context depends on, or the first flag the model does not
// know, which no configuration can be proved for.
// As in the go command, the last occurrence of a flag wins.
func BuildFlags(flags []string) ([]string, string) {
	tags, tagged := "", false
	switches := map[string]bool{}
	for i := 0; i < len(flags); i++ {
		flag := flags[i]
		name, value, inline := strings.Cut(flag, "=")
		name = "-" + strings.TrimLeft(name, "-")
		switch {
		// An explicit value (-race=false) is not modelled (disabled for 1.0).
		case selectingSwitches[name] && !inline:
			switches[name] = true
		case testOnlySwitches[name]:
		case selectingFlags[name] || testOnlyFlags[name] || name == "-mod":
			if !inline {
				if i+1 >= len(flags) {
					return nil, flag
				}
				i++
				value = flags[i]
			}
			switch {
			case selectingFlags[name]:
				tags, tagged = value, true
			case name == "-mod" && !moduleModes[value]:
				return nil, flag
			}
		default:
			return nil, flag
		}
	}
	var out []string
	if tagged {
		out = append(out, "-tags="+tags)
	}
	for _, name := range []string{"-asan", "-msan", "-race"} {
		if switches[name] {
			out = append(out, name)
		}
	}
	return out, ""
}

// sharedBuildSwitch returns a build-selecting flag name that both a and b
// set, or "".
func sharedBuildSwitch(a, b []string) string {
	names := func(flags []string) map[string]bool {
		set := map[string]bool{}
		for _, flag := range flags {
			name, _, _ := strings.Cut(flag, "=")
			name = "-" + strings.TrimLeft(name, "-")
			if selectingSwitches[name] || selectingFlags[name] {
				set[name] = true
			}
		}
		return set
	}
	inB := names(b)
	for _, flag := range a {
		name, _, _ := strings.Cut(flag, "=")
		name = "-" + strings.TrimLeft(name, "-")
		if inB[name] {
			return name
		}
	}
	return ""
}

// ProbeConfig reads the build context the go command uses in dir under env
// (the executor's complete environment) and flags (its go test flags). Any
// flag, in flags or in the effective GOFLAGS, the model does not know is an
// error: the run could compile other source.
func ProbeConfig(ctx context.Context, dir string, env, flags []string) (Config, error) {
	run := func(args ...string) ([]byte, error) {
		command := exec.CommandContext(ctx, "go", args...)
		command.Dir, command.Env = dir, env
		var stderr bytes.Buffer
		command.Stderr = &stderr
		out, err := command.Output()
		if err != nil {
			return nil, fmt.Errorf("go %s: %v: %s", args[0], err, strings.TrimSpace(stderr.String()))
		}
		return out, nil
	}
	goflags, err := run("env", "GOFLAGS")
	if err != nil {
		return Config{}, err
	}
	fromEnv := strings.Fields(string(goflags))
	if _, unknown := BuildFlags(fromEnv); unknown != "" {
		return Config{}, fmt.Errorf("GOFLAGS holds %s, which the model does not know", unknown)
	}
	selecting, unknown := BuildFlags(flags)
	if unknown != "" {
		return Config{}, fmt.Errorf("the executor flag %s is not modelled", unknown)
	}
	// A build switch set in both places is not modelled (disabled for 1.0).
	if both := sharedBuildSwitch(fromEnv, flags); both != "" {
		return Config{}, fmt.Errorf("the build switch %s is set in both GOFLAGS and the executor flags", both)
	}
	// The go list template has no json function: the context is printed as
	// tab-separated fields, its tag lists joined by commas, which no tag holds.
	const format = "{{with context}}{{.GOROOT}}\t{{.GOOS}}\t{{.GOARCH}}\t{{.CgoEnabled}}\t{{.Compiler}}\t{{.UseAllFiles}}\t" +
		"{{join .BuildTags \",\"}}\t{{join .ToolTags \",\"}}\t{{join .ReleaseTags \",\"}}{{end}}"
	out, err := run(append(append([]string{"list", "-e"}, selecting...), "-f", format, "unsafe")...)
	if err != nil {
		return Config{}, err
	}
	fields := strings.Split(strings.TrimSuffix(string(out), "\n"), "\t")
	if len(fields) != 9 || (fields[3] != "true" && fields[3] != "false") || (fields[5] != "true" && fields[5] != "false") {
		return Config{}, fmt.Errorf("go list context %q is not readable", out)
	}
	tags := func(field string) []string {
		if field == "" {
			return []string{}
		}
		return strings.Split(field, ",")
	}
	config := Config{GOROOT: fields[0], GOOS: fields[1], GOARCH: fields[2], CgoEnabled: fields[3] == "true", Compiler: fields[4], UseAllFiles: fields[5] == "true",
		BuildTags: tags(fields[6]), ToolTags: tags(fields[7]), ReleaseTags: tags(fields[8])}
	if config.GOROOT == "" || config.GOOS == "" || config.GOARCH == "" || config.Compiler != "gc" {
		return Config{}, fmt.Errorf("go list context %s/%s compiler %q is not modelled", config.GOOS, config.GOARCH, config.Compiler)
	}
	return config, nil
}
