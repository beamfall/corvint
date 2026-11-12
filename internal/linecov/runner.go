package linecov

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// RunnerID names everything that must match between collection and
// selection: the toolchain, platform, build configuration, runtime settings,
// test runtime and coverage mode (LPCV-V0-951). Build settings bind through GOFLAGS (-race,
// -tags), CGO_ENABLED, GOEXPERIMENT and every ArchFeatureVars setting as `go
// env` reports them under env (nil means the current environment). A -p flag
// in GOFLAGS only bounds build parallelism and changes no executed code, so it
// is left out of the identity (GLS-V0-009). The test runtime is
// TestRuntime(timeout), timeout being the go test -timeout the tests run
// under (GLS-V0-020). A GOFLAGS entry that is not a build flag is refused:
// go test passes it to the test binary, where collection does not apply it.
// So is a setting the identity does not carry (UncarriedEnvVars changed from
// its default, or a go.work workspace in use). It also returns the effective
// GOFLAGS, which collection builds under. exec is the `go test -exec`
// wrapper the tests run under, "" for none: the identity names its content
// hash (ExecIdentity), so an index collected under a wrapper that confines or
// rewrites what a test can observe decides nothing for a run without it.
func RunnerID(ctx context.Context, root string, env []string, timeout time.Duration, execWrapper string) (id, goflags string, err error) {
	wrapper, err := ExecIdentity(execWrapper)
	if err != nil {
		return "", "", err
	}
	args := append(append([]string{"env", "-json", "GOVERSION", "GOOS", "GOARCH", "GOFLAGS", "CGO_ENABLED", "GOEXPERIMENT", "GOWORK"}, ArchFeatureVars...), UnreportedGoEnvVars...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir, cmd.Env = root, env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("go env: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	var e map[string]string
	if err := json.Unmarshal(out, &e); err != nil {
		return "", "", fmt.Errorf("go env: %w", err)
	}
	if err := checkGoFlags(e["GOFLAGS"]); err != nil {
		return "", "", err
	}
	if err := checkUnreportedEnv(e, env); err != nil {
		return "", "", err
	}
	if w := e["GOWORK"]; w != "" && w != "off" {
		return "", "", fmt.Errorf("GOWORK %q: a go.work workspace can change the module graph a test builds, and the runner identity does not carry it", w)
	}
	cmd = exec.CommandContext(ctx, "go", append([]string{"env", "-json", "-changed"}, UncarriedEnvVars...)...)
	cmd.Dir, cmd.Env = root, env
	stderr.Reset()
	cmd.Stderr = &stderr
	if out, err = cmd.Output(); err != nil {
		return "", "", fmt.Errorf("go env -changed: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	var changed map[string]string
	if err := json.Unmarshal(out, &changed); err != nil {
		return "", "", fmt.Errorf("go env -changed: %w", err)
	}
	for _, name := range UncarriedEnvVars {
		if v, ok := changed[name]; ok {
			return "", "", fmt.Errorf("%s=%q changes what a test executes, and the runner identity does not carry it", name, v)
		}
	}
	runtimeEnv, err := withoutDefaultGOROOT(ctx, root, env)
	if err != nil {
		return "", "", err
	}
	test := strings.Join(TestRuntime(timeout), " ")
	return RunnerIdentity(e["GOVERSION"], e["GOOS"], e["GOARCH"], e["CGO_ENABLED"], e["GOFLAGS"], e["GOEXPERIMENT"], ArchFeatures(e), RuntimeSettings(runtimeEnv), wrapper, test), e["GOFLAGS"], nil
}

// withoutDefaultGOROOT returns env (nil is this process's environment)
// without its GOROOT when that spells exactly the go command's own GOROOT
// (`go env GOROOT` with GOROOT unset). runtime.GOROOT returns the spelling the
// environment sets, so a symbolic-link or trailing-separator alias of the same
// directory can take a different branch in a test; any other spelling is bound
// (GLS-V0-052).
func withoutDefaultGOROOT(ctx context.Context, root string, env []string) ([]string, error) {
	if env == nil {
		env = os.Environ()
	}
	if _, set := EnvValues(env)["GOROOT"]; !set {
		return env, nil
	}
	unset := slices.DeleteFunc(slices.Clone(env), func(kv string) bool { return strings.HasPrefix(kv, "GOROOT=") })
	cmd := exec.CommandContext(ctx, "go", "env", "GOROOT")
	cmd.Dir, cmd.Env = root, unset
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go env GOROOT: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return dropDefaultGOROOT(env, strings.TrimSpace(string(out))), nil
}

// dropDefaultGOROOT returns env without its GOROOT entries when the GOROOT it
// sets is exactly defaultRoot; otherwise env unchanged (GLS-V0-052).
func dropDefaultGOROOT(env []string, defaultRoot string) []string {
	set := EnvValues(env)["GOROOT"]
	if set == "" || defaultRoot == "" || set != defaultRoot {
		return env
	}
	return slices.DeleteFunc(slices.Clone(env), func(kv string) bool { return strings.HasPrefix(kv, "GOROOT=") })
}

// checkUnreportedEnv refuses a non-empty UnreportedGoEnvVars setting in one
// go env reading, or UnreportedProcessEnvVars variable in env (nil is this
// process's environment): each changes what a test executes, `go env
// -changed` does not report it, and the identity does not carry it
// (GLS-V0-051).
func checkUnreportedEnv(goenv map[string]string, env []string) error {
	for _, name := range UnreportedGoEnvVars {
		if v := goenv[name]; v != "" {
			return fmt.Errorf("%s=%q changes what a test executes, go env -changed does not report it, and the runner identity does not carry it", name, v)
		}
	}
	if env == nil {
		env = os.Environ()
	}
	values := EnvValues(env)
	for _, name := range UnreportedProcessEnvVars {
		if v := values[name]; v != "" {
			return fmt.Errorf("%s=%q changes what the go command builds, go env does not report it, and the runner identity does not carry it", name, v)
		}
	}
	return nil
}

// ExecIdentity is the identity form of a `go test -exec` wrapper: "" for
// none, else `sha256:` of the wrapper file's content, so two runs share it
// only under the same wrapper build (GLS-V0-009). An unreadable wrapper is an
// error, so selection is runner-unavailable.
func ExecIdentity(wrapper string) (string, error) {
	if wrapper == "" {
		return "", nil
	}
	data, err := os.ReadFile(wrapper)
	if err != nil {
		return "", fmt.Errorf("exec wrapper %s: %w", wrapper, err)
	}
	return EnvHash(string(data)), nil
}

// RuntimeEnvVars are the environment variables the Go runtime reads itself,
// not through os.Getenv, that can change what a program executes: scheduler
// width, collector pacing and race detector behaviour (GLS-V0-009), the
// traceback level and the GOROOT runtime.GOROOT reports when it names another
// directory than the go command's own (withoutDefaultGOROOT, GLS-V0-052).
// Collection's env audit cannot see them, so the identity names each set one.
var RuntimeEnvVars = []string{"GOMAXPROCS", "GOGC", "GOMEMLIMIT", "GORACE", "GOTRACEBACK", "GOROOT"}

// RuntimeSettings is the identity form of the RuntimeEnvVars in env (nil is
// this process's environment): NAME=value for each set one, in order, then
// each set loader variable (LoaderEnvVars, or a name starting with one of
// LoaderEnvPrefixes) sorted by name (GLS-V0-052).
func RuntimeSettings(env []string) string {
	if env == nil {
		env = os.Environ()
	}
	values := EnvValues(env)
	var out, loader []string
	for _, name := range RuntimeEnvVars {
		if v, ok := values[name]; ok {
			out = append(out, name+"="+v)
		}
	}
	for name, v := range values {
		if loaderEnv(name) {
			loader = append(loader, name+"="+v)
		}
	}
	sort.Strings(loader)
	return strings.Join(append(out, loader...), " ")
}

// loaderEnv reports whether the dynamic loader or the C library reads name.
func loaderEnv(name string) bool {
	if slices.Contains(LoaderEnvVars, name) {
		return true
	}
	for _, p := range LoaderEnvPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// BuildFlags are the go build flags that can change the built test binary,
// which GOFLAGS may carry into the runner identity (GLS-V0-020). The values
// say whether the flag takes a value.
var BuildFlags = map[string]bool{
	"race": false, "msan": false, "asan": false, "trimpath": false, "linkshared": false, "buildvcs": false,
	"tags": true, "ldflags": true, "gcflags": true, "asmflags": true, "mod": true, "pgo": true,
	"installsuffix": true, "compiler": true, "gccgoflags": true, "buildmode": true,
	"modfile": true, "toolexec": true,
}

// checkGoFlags refuses a GOFLAGS entry other than a BuildFlags flag, -p or
// -count=1: go test passes every other one, a test flag such as -v, -short
// or -timeout, to the test binary, where collection does not apply it
// (GLS-V0-020).
func checkGoFlags(goflags string) error {
	for _, f := range strings.Fields(goflags) {
		name, value, _ := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(f, "-"), "-"), "=")
		_, build := BuildFlags[name]
		if !strings.HasPrefix(f, "-") || !build && name != "p" && (name != "count" || value != "1") {
			return fmt.Errorf("GOFLAGS entry %q is not a build flag: go test passes it to the test binary, and line coverage collection does not apply it", f)
		}
	}
	return nil
}

// DefaultTestTimeout is the -timeout go test runs a test binary under when
// none is given.
const DefaultTestTimeout = 10 * time.Minute

// TestRuntime is the test-binary arguments `go test -json -timeout timeout`
// passes besides the tests' own selection, each of which a test can observe:
// -test.v=test2json makes testing.Verbose report true, and -test.timeout
// gives every test a deadline. Collection passes them too, so a test executes
// the code it executes in the run being selected for (GLS-V0-020). A zero
// timeout is go test -timeout 0, which passes no -test.timeout.
func TestRuntime(timeout time.Duration) []string {
	args := []string{"-test.paniconexit0", "-test.v=test2json"}
	if timeout > 0 {
		args = append(args, "-test.timeout="+timeout.String())
	}
	return args
}

// RunnerTestTimeout is the go test -timeout a RunnerID names, so a check of
// an index or record made under it can read the current identity under the
// same test runtime; ok is false when id names no test runtime.
func RunnerTestTimeout(id string) (timeout time.Duration, ok bool) {
	i := strings.LastIndex(id, ` test="`)
	if i < 0 {
		return 0, false
	}
	quoted, err := strconv.QuotedPrefix(id[i+len(" test="):])
	if err != nil {
		return 0, false
	}
	test, _ := strconv.Unquote(quoted)
	for _, f := range strings.Fields(test) {
		if v, found := strings.CutPrefix(f, "-test.timeout="); found {
			d, err := time.ParseDuration(v)
			return d, err == nil
		}
	}
	return 0, true
}

// UncarriedEnvVars are the go env settings that can change what a test
// executes but that the runner identity does not name: GODEBUG, the C
// toolchain and flags a cgo build compiles with, and the archiver and gccgo
// a gccgo build uses (GLS-V0-051). RunnerID refuses each one go env
// -changed reports changed from its default rather than modelling it
// (GLS-V0-009); FC is never reported, so UnreportedGoEnvVars refuses it.
var UncarriedEnvVars = []string{"GODEBUG", "CC", "CXX", "FC", "CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_FFLAGS", "CGO_LDFLAGS", "PKG_CONFIG", "AR", "GCCGO"}

// UnreportedGoEnvVars are the go env settings the go command reads to build
// a test that `go env -changed` does not report even when set (go1.27.3
// prints {} for FC=x go env -json -changed FC): the Fortran compiler of a cgo
// build and the external-linking switch. RunnerID reads them through go env,
// which sees the environment and the go env -w file, and refuses a non-empty
// one (GLS-V0-051).
var UnreportedGoEnvVars = []string{"FC", "GO_EXTLINK_ENABLED"}

// UnreportedProcessEnvVars are the variables the go command, its compiler
// or cgo read with os.Getenv, which go env does not print at all: cgo's C
// compiler fallback and the compiler's debug settings the go command folds
// into its build action ID. RunnerID refuses a non-empty one in the
// environment the tests run under (GLS-V0-051).
var UnreportedProcessEnvVars = []string{"GCC", "GOCOMPILEDEBUG", "GOCLOBBERDEADHASH", "GOSSAFUNC", "GOSSADIR"}

// LoaderEnvVars and LoaderEnvPrefixes name the variables the dynamic loader
// and the C library read outside Go code in a dynamically linked test binary
// (every -race or cgo build): the environment audit cannot see those reads,
// so RuntimeSettings names each set one (GLS-V0-052).
var (
	LoaderEnvVars     = []string{"GLIBC_TUNABLES"}
	LoaderEnvPrefixes = []string{"LD_", "DYLD_"}
)

// ArchFeatureVars are the go env settings that choose the instructions or
// the FIPS module a build of a GOARCH compiles to, so two builds differing in
// one can execute different code (GLS-V0-009).
var ArchFeatureVars = []string{"GO386", "GOAMD64", "GOARM", "GOARM64", "GOFIPS140", "GOMIPS", "GOMIPS64", "GOPPC64", "GORISCV64", "GOWASM"}

// ArchFeatures is the identity form of the ArchFeatureVars settings in one go
// env reading: NAME=value for each non-empty one, in ArchFeatureVars order.
func ArchFeatures(env map[string]string) string {
	var out []string
	for _, name := range ArchFeatureVars {
		if v := env[name]; v != "" {
			out = append(out, name+"="+v)
		}
	}
	return strings.Join(out, " ")
}

// RunnerIdentity is the RunnerID string of one go env reading; arch is its
// ArchFeatures, runtime its RuntimeSettings, exec its ExecIdentity and test
// its TestRuntime arguments, space-separated.
func RunnerIdentity(version, goos, goarch, cgo, goflags, experiment, arch, runtime, wrapper, test string) string {
	return fmt.Sprintf("%s %s %s arch=%q cgo=%s goflags=%q goexperiment=%q runtime=%q exec=%q test=%q atomic", version, goos, goarch, arch, cgo, identityFlags(goflags), experiment, runtime, wrapper, test)
}

// identityFlags drops -p and -p=N (with one or two dashes) from GOFLAGS,
// keeping every other flag in order.
func identityFlags(goflags string) string {
	var kept []string
	fields := strings.Fields(goflags)
	for i := 0; i < len(fields); i++ {
		f := strings.TrimPrefix(strings.TrimPrefix(fields[i], "-"), "-")
		switch {
		case f == "p" && fields[i] != f:
			i++ // "-p N"
			continue
		case strings.HasPrefix(f, "p=") && fields[i] != f:
			continue
		}
		kept = append(kept, fields[i])
	}
	return strings.Join(kept, " ")
}

// ModulePath reads the module directive of root's go.mod.
func ModulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[0] == "module" {
			return strings.Trim(f[1], `"`), nil
		}
	}
	return "", errors.New("go.mod has no module directive")
}
