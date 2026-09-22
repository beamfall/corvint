// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/procgroup"
)

const maxCapture = 8 << 20
const minClaimSamples = 30
const protocolProfile = "corvint-dogfood-measure-protocol/0"
const profile = "corvint-dogfood-measure/1"

type object = map[string]any
type workload struct {
	ID                   string   `json:"id"`
	Argv                 []string `json:"argv"`
	CacheMode            string   `json:"cacheMode"`
	ExpectedExit         int      `json:"expectedExit"`
	TimeoutSeconds       int      `json:"timeoutSeconds"`
	ExpectedStdoutSHA256 *string  `json:"expectedStdoutSha256"`
	MaxP95WallMs         *int64   `json:"maxP95WallMs"`
	MaxStdoutBytes       *int64   `json:"maxStdoutBytes"`
}
type protocol struct {
	Profile            string     `json:"profile"`
	ExpectedHead       string     `json:"expectedHead"`
	ExpectedDirtyPaths []string   `json:"expectedDirtyPaths"`
	Inputs             []string   `json:"inputs"`
	Workloads          []workload `json:"workloads"`
	SHA256             string     `json:"-"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	code, err := run(ctx, os.Args[1:])
	if err != nil {
		raw, _ := canonical(object{"ok": false, "error": err.Error()})
		os.Stderr.Write(raw)
		code = 2
	}
	if code != 0 {
		os.Exit(code)
	}
}
func run(ctx context.Context, args []string) (int, error) {
	flags := flag.NewFlagSet("dogfood-measure", flag.ContinueOnError)
	root := flags.String("root", "", "repository")
	protocolPath := flags.String("protocol", "", "measurement protocol")
	output := flags.String("output", "", "private receipt")
	samples := flags.Int("samples", 30, "samples per phase")
	if err := flags.Parse(args); err != nil {
		return 2, err
	}
	if *root == "" || *protocolPath == "" || *output == "" || flags.NArg() != 0 {
		return 2, errors.New("--root, --protocol and --output are required")
	}
	p, err := loadProtocol(*protocolPath)
	if err != nil {
		return 2, err
	}
	artifact, err := measure(ctx, *root, p, *samples)
	if err != nil {
		return 2, err
	}
	if err := writeOutput(*output, artifact); err != nil {
		return 2, err
	}
	ok := artifact["valid"] == true && artifact["checksState"] == "PASS"
	raw, _ := canonical(object{"ok": ok, "output": *output, "checksState": artifact["checksState"], "invalidReasons": artifact["invalidReasons"]})
	os.Stdout.Write(raw)
	if !ok {
		return 1, nil
	}
	return 0, nil
}
func canonical(value any) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(value); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func digest(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func readBounded(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("must be a regular non-symlink file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("file changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, errors.New("input byte bound exceeded")
	}
	return raw, nil
}
func loadProtocol(path string) (protocol, error) {
	var p protocol
	raw, err := readBounded(path, 1<<20)
	if err != nil {
		return p, err
	}
	value, err := wire.Parse(raw)
	if err != nil {
		return p, fmt.Errorf("measurement protocol is invalid JSON: %w", err)
	}
	if value.Kind != wire.KindObject {
		return p, errors.New("protocol must be object")
	}
	for _, key := range []string{"profile", "expectedHead", "expectedDirtyPaths", "inputs", "workloads"} {
		if _, ok := value.Obj.Get(key); !ok {
			return p, fmt.Errorf("protocol missing %s", key)
		}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return p, err
	}
	if p.Profile != protocolProfile || !regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`).MatchString(p.ExpectedHead) {
		return p, errors.New("unsupported protocol profile or full Git identity")
	}
	for _, list := range [][]string{p.ExpectedDirtyPaths, p.Inputs} {
		if list == nil {
			return p, errors.New("paths must be arrays")
		}
		prior := ""
		for _, path := range list {
			if path == "" || len(path) > 4096 || !utf8.ValidString(path) || strings.ContainsRune(path, 0) || filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || path == "." || path == ".." || strings.HasPrefix(path, "../") || path <= prior {
				return p, errors.New("paths must be normalized, sorted and unique")
			}
			prior = path
		}
	}
	if len(p.Workloads) < 1 || len(p.Workloads) > 16 {
		return p, errors.New("workloads must contain 1 to 16 entries")
	}
	rawWorkloads, _ := value.Obj.Get("workloads")
	seen := map[string]bool{}
	for i, w := range p.Workloads {
		rawItem := rawWorkloads.Arr[i]
		if rawItem.Kind != wire.KindObject {
			return p, errors.New("workload must be object")
		}
		for _, key := range []string{"id", "argv", "cacheMode", "expectedExit", "timeoutSeconds"} {
			if _, ok := rawItem.Obj.Get(key); !ok {
				return p, fmt.Errorf("workload missing %s", key)
			}
		}
		if !regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`).MatchString(w.ID) || seen[w.ID] {
			return p, errors.New("workload IDs must be unique normalized identifiers")
		}
		seen[w.ID] = true
		if len(w.Argv) < 1 || len(w.Argv) > 128 {
			return p, errors.New("argv must be a bounded array")
		}
		for _, arg := range w.Argv {
			if arg == "" || len(arg) > 4096 || !utf8.ValidString(arg) || strings.ContainsRune(arg, 0) {
				return p, errors.New("invalid argument")
			}
		}
		if w.CacheMode != "none" && w.CacheMode != "corvint-index" {
			return p, errors.New("unsupported cacheMode")
		}
		if w.ExpectedExit != 0 || w.TimeoutSeconds < 1 || w.TimeoutSeconds > 3600 {
			return p, errors.New("invalid expected exit or timeout")
		}
		if w.ExpectedStdoutSHA256 != nil && !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(*w.ExpectedStdoutSHA256) {
			return p, errors.New("invalid expected stdout digest")
		}
		if w.MaxP95WallMs != nil && (*w.MaxP95WallMs < 1 || *w.MaxP95WallMs > 3600000) {
			return p, errors.New("invalid p95 bound")
		}
		if w.MaxStdoutBytes != nil && (*w.MaxStdoutBytes < 1 || *w.MaxStdoutBytes > maxCapture) {
			return p, errors.New("invalid stdout bound")
		}
		if _, err := expandArgv(w.Argv, "/repository"); err != nil {
			return p, err
		}
	}
	p.SHA256 = digest(raw)
	return p, nil
}
func expandArgv(args []string, root string) ([]string, error) {
	out := make([]string, len(args))
	for i, arg := range args {
		out[i] = strings.ReplaceAll(arg, "{root}", root)
		if strings.ContainsAny(out[i], "{}") {
			return nil, errors.New("unsupported argv placeholder; the Python measurement runtime is retired")
		}
	}
	return out, nil
}
func environment(cache string) []string {
	values := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok && !strings.HasPrefix(key, "PYTHON") {
			values[key] = value
		}
	}
	values["CORVINT_CACHE_DIR"], values["LC_ALL"], values["TZ"] = cache, "C", "UTC"
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := []string{}
	for _, key := range keys {
		out = append(out, key+"="+values[key])
	}
	return out
}
func resolveExecutable(argv []string) ([]string, error) {
	out := append([]string(nil), argv...)
	p, err := exec.LookPath(out[0])
	if err != nil {
		return nil, err
	}
	out[0], err = filepath.Abs(p)
	return out, err
}
func inspectGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	argv, err := resolveExecutable(append([]string{"git", "-C", root}, args...))
	if err != nil {
		return nil, err
	}
	r := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: root, Env: os.Environ(), Timeout: 30 * time.Second, OutputLimit: 8 << 20})
	if r.Err != nil || r.ExitStatus != 0 || !r.OwnedProcessGroupCleanup {
		return nil, fmt.Errorf("cannot inspect measurement repository: %v", r.Err)
	}
	return r.Stdout, nil
}
func statusPaths(raw []byte) ([]string, error) {
	seen := map[string]bool{}
	parts := bytes.Split(raw, []byte{0})
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if len(part) == 0 {
			continue
		}
		if len(part) <= 3 || part[2] != ' ' {
			return nil, errors.New("cannot parse repository status")
		}
		seen[string(part[3:])] = true
		if bytes.ContainsAny(part[:2], "RC") {
			i++
			if i >= len(parts) || len(parts[i]) == 0 {
				return nil, errors.New("cannot parse repository rename status")
			}
			seen[string(parts[i])] = true
		}
	}
	out := []string{}
	for path := range seen {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}
func fileIdentity(root, path string) (object, error) {
	name := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(name)
	r := object{"path": path, "kind": "missing", "bytes": int64(0), "sha256": nil}
	if os.IsNotExist(err) {
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(name)
		if err != nil {
			return nil, err
		}
		r["kind"], r["bytes"], r["sha256"] = "symlink", int64(len(target)), digest([]byte(target))
		return r, nil
	}
	if !info.Mode().IsRegular() {
		r["kind"] = "other"
		return r, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	hash := sha256.New()
	n, err := io.Copy(hash, f)
	if err != nil {
		return nil, err
	}
	r["kind"], r["bytes"], r["sha256"] = "file", n, fmt.Sprintf("%x", hash.Sum(nil))
	return r, nil
}
func repositoryIdentity(ctx context.Context, root string, inputs []string) (object, error) {
	status, err := inspectGit(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	dirty, err := statusPaths(status)
	if err != nil {
		return nil, err
	}
	head, err := inspectGit(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	tree, err := inspectGit(ctx, root, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, path := range append(append([]string(nil), dirty...), inputs...) {
		set[path] = true
	}
	paths := []string{}
	for path := range set {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	files := []object{}
	for _, path := range paths {
		item, err := fileIdentity(root, path)
		if err != nil {
			return nil, err
		}
		files = append(files, item)
	}
	return object{"head": strings.TrimSpace(string(head)), "tree": strings.TrimSpace(string(tree)), "statusSha256": digest(status), "dirtyPaths": dirty, "files": files}, nil
}
func validateInputs(identity object, inputs []string) error {
	for _, path := range inputs {
		found := false
		for _, item := range identity["files"].([]object) {
			if item["path"] == path && item["kind"] == "file" {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("protocol input must be an existing regular non-symlink file: %s", path)
		}
	}
	return nil
}
func runProcess(ctx context.Context, args []string, root, cache string, timeout int) (object, error) {
	argv, err := resolveExecutable(args)
	if err != nil {
		return nil, errors.New("cannot start measurement workload")
	}
	start := time.Now()
	r := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: root, Env: environment(cache), Timeout: time.Duration(timeout) * time.Second, ShutdownTimeout: 2 * time.Second, OutputLimit: maxCapture})
	wall := time.Since(start).Nanoseconds()
	if r.Cancelled {
		return nil, context.Canceled
	}
	if !r.Started {
		return nil, errors.New("cannot start measurement workload")
	}
	if r.OutputOverflow {
		return nil, errors.New("workload output exceeds measurement capture bound")
	}
	if !r.WaitCompleted || !r.PipesDrained || !r.OwnedProcessGroupCleanup {
		return nil, errors.New("workload cleanup did not complete")
	}
	if r.Usage == nil {
		return nil, errors.New("process resource accounting is unsupported on this host")
	}
	return object{"exitCode": r.ExitStatus, "timedOut": r.TimedOut, "wallNs": wall, "userCpuNs": r.Usage.UserCPUNs, "systemCpuNs": r.Usage.SystemCPUNs, "maxRssBytes": r.Usage.MaxRSSBytes, "stdoutBytes": int64(len(r.Stdout)), "stdoutSha256": digest(r.Stdout), "stderrBytes": int64(len(r.Stderr)), "stderrSha256": digest(r.Stderr)}, nil
}
func distribution(values []int64) object {
	ordered := append([]int64(nil), values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	rank := func(p int) int64 { return ordered[(p*len(ordered)+99)/100-1] }
	p50 := rank(50)
	deviations := make([]int64, len(values))
	for i, v := range values {
		d := v - p50
		if d < 0 {
			d = -d
		}
		deviations[i] = d
	}
	sort.Slice(deviations, func(i, j int) bool { return deviations[i] < deviations[j] })
	return object{"n": len(values), "p50": p50, "p95": rank(95), "max": ordered[len(ordered)-1], "mad": deviations[(len(deviations)+1)/2-1]}
}
func validateObservation(observation object, w workload, expected string) []string {
	reasons := []string{}
	if observation["timedOut"] == true {
		reasons = append(reasons, w.ID+":timeout")
	}
	if observation["exitCode"] != 0 {
		reasons = append(reasons, w.ID+":unexpected-exit")
	}
	if observation["stderrBytes"] != int64(0) {
		reasons = append(reasons, w.ID+":unexpected-stderr")
	}
	if expected != "" && observation["stdoutSha256"] != expected {
		reasons = append(reasons, w.ID+":stdout-nondeterminism")
	}
	if w.ExpectedStdoutSHA256 != nil && observation["stdoutSha256"] != *w.ExpectedStdoutSHA256 {
		reasons = append(reasons, w.ID+":stdout-mismatch")
	}
	return reasons
}
func measure(ctx context.Context, root string, p protocol, samples int) (object, error) {
	if samples < 1 || samples > 1000 {
		return nil, errors.New("samples must be an integer from 1 to 1000")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return nil, errors.New("root must be a Git repository")
	}
	initial, err := repositoryIdentity(ctx, root, p.Inputs)
	if err != nil {
		return nil, err
	}
	if initial["head"] != p.ExpectedHead {
		return nil, errors.New("repository HEAD differs from expectedHead")
	}
	if !reflect.DeepEqual(initial["dirtyPaths"], p.ExpectedDirtyPaths) {
		return nil, errors.New("repository dirty paths differ from expectedDirtyPaths")
	}
	if err := validateInputs(initial, p.Inputs); err != nil {
		return nil, err
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	runner, err := os.ReadFile(executable)
	if err != nil {
		return nil, err
	}
	gitVersion, err := inspectGit(ctx, root, "--version")
	if err != nil {
		return nil, err
	}
	observability := object{}
	for _, key := range []string{"inputTokens", "outputTokens", "sourceOpenEvents", "distinctSourcePaths", "callerBroadSearches", "automaticWidenings", "reviewerCriticalMisses"} {
		observability[key] = "NOT_OBSERVED"
	}
	artifact := object{"profile": profile, "corvintCommit": initial["head"], "treeOid": initial["tree"], "repository": initial, "environment": object{"architecture": runtime.GOARCH, "logicalCpuCount": runtime.NumCPU(), "os": runtime.GOOS, "go": runtime.Version(), "git": strings.TrimSpace(string(gitVersion)), "runnerSha256": digest(runner)}, "protocol": object{"profile": p.Profile, "sha256": p.SHA256, "samplesPerPhase": samples, "order": "rotating-workload-blocks", "clock": "time.Since-monotonic", "percentile": "nearest-rank", "rssUnit": "bytes", "captureLimitBytesPerStream": maxCapture}, "workloads": p.Workloads, "sessionObservability": observability}
	cacheRoot, err := os.MkdirTemp("", "corvint-dogfood-cache-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(cacheRoot)
	shared := filepath.Join(cacheRoot, "primed")
	if err := os.Mkdir(shared, 0700); err != nil {
		return nil, err
	}
	reasons := []string{}
	expected := map[string]string{}
	observations := []object{}
	observe := func(w workload, cache, driftReason string) (object, error) {
		before, err := repositoryIdentity(ctx, root, p.Inputs)
		if err != nil {
			return nil, err
		}
		argv, err := expandArgv(w.Argv, root)
		if err != nil {
			return nil, err
		}
		observation, err := runProcess(ctx, argv, root, cache, w.TimeoutSeconds)
		if err != nil {
			return nil, err
		}
		after, err := repositoryIdentity(ctx, root, p.Inputs)
		if err != nil {
			return nil, err
		}
		if !reflect.DeepEqual(before, initial) || !reflect.DeepEqual(after, initial) {
			reasons = append(reasons, driftReason)
		}
		reasons = append(reasons, validateObservation(observation, w, expected[w.ID])...)
		if expected[w.ID] == "" {
			expected[w.ID] = observation["stdoutSha256"].(string)
		}
		return observation, nil
	}
	for _, w := range p.Workloads {
		if _, err := observe(w, shared, w.ID+":input-drift-during-prime"); err != nil {
			return nil, err
		}
	}
	for _, phase := range []string{"corvint-cache-cold", "primed-dirty"} {
		for block := range samples {
			for i := range len(p.Workloads) {
				w := p.Workloads[(block+i)%len(p.Workloads)]
				cache := shared
				disposition := "PRIMED_SHARED"
				if phase == "corvint-cache-cold" {
					cache = filepath.Join(cacheRoot, fmt.Sprintf("cold-%d-%s", block, w.ID))
					if err := os.Mkdir(cache, 0700); err != nil {
						return nil, err
					}
					disposition = "COLD_UNIQUE"
				}
				if w.CacheMode == "none" {
					disposition = "NOT_APPLICABLE"
				}
				observation, err := observe(w, cache, fmt.Sprintf("%s:%s:block-%d:input-drift", w.ID, phase, block))
				if err != nil {
					return nil, err
				}
				observation["phase"], observation["block"], observation["workloadId"], observation["cacheDisposition"] = phase, block, w.ID, disposition
				observations = append(observations, observation)
			}
		}
	}
	summaries, checks := []object{}, []object{}
	for _, phase := range []string{"corvint-cache-cold", "primed-dirty"} {
		for _, w := range p.Workloads {
			summary := object{"phase": phase, "workloadId": w.ID}
			for _, field := range []string{"wallNs", "userCpuNs", "systemCpuNs", "maxRssBytes", "stdoutBytes"} {
				values := []int64{}
				for _, o := range observations {
					if o["phase"] == phase && o["workloadId"] == w.ID {
						values = append(values, o[field].(int64))
					}
				}
				summary[field] = distribution(values)
			}
			summaries = append(summaries, summary)
			if w.MaxP95WallMs != nil && phase == "primed-dirty" {
				actual := summary["wallNs"].(object)["p95"].(int64)
				limit := *w.MaxP95WallMs * 1000000
				state := "PASS"
				if samples < minClaimSamples {
					state = "NOT_RUN"
				} else if actual >= limit {
					state = "FAIL"
				}
				checks = append(checks, object{"workloadId": w.ID, "phase": phase, "metric": "wallNs.p95", "limit": limit, "actual": actual, "samplesPerPhase": samples, "minimumSamplesPerPhase": minClaimSamples, "state": state})
			}
			if w.MaxStdoutBytes != nil {
				actual := summary["stdoutBytes"].(object)["max"].(int64)
				state := "PASS"
				if actual > *w.MaxStdoutBytes {
					state = "FAIL"
				}
				checks = append(checks, object{"workloadId": w.ID, "phase": phase, "metric": "stdoutBytes.max", "limit": *w.MaxStdoutBytes, "actual": actual, "state": state})
			}
		}
	}
	final, err := repositoryIdentity(ctx, root, p.Inputs)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(final, initial) {
		reasons = append(reasons, "repository-input-drift")
	}
	sort.Strings(reasons)
	unique := []string{}
	for _, reason := range reasons {
		if len(unique) == 0 || unique[len(unique)-1] != reason {
			unique = append(unique, reason)
		}
	}
	state := "PASS"
	for _, check := range checks {
		if check["state"] == "FAIL" {
			state = "FAIL"
			break
		}
		if check["state"] == "NOT_RUN" {
			state = "NOT_RUN"
		}
	}
	artifact["samples"], artifact["distributions"], artifact["checks"] = observations, summaries, checks
	artifact["invalidReasons"], artifact["valid"], artifact["checksState"] = unique, len(unique) == 0, state
	return artifact, nil
}
func writeOutput(path string, artifact object) error {
	raw, err := canonical(artifact)
	if err != nil {
		return err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".dogfood-measure-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
