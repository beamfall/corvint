// SPDX-License-Identifier: AGPL-3.0-or-later

// Command tasks-scale builds synthetic corvint-tasks stores through the
// product's own CLI writers and measures every read and write command over
// them (V1-1053). It is a local measurement harness, not a product command:
// it never touches a real queue, and every store it writes lives under the
// root the caller names.
//
//	tasks-scale gen --root DIR --tickets N
//	tasks-scale run --store DIR [--fds] [--cpuprofile F] [--memprofile F] [--out F] -- ARGS...
//	tasks-scale matrix --store DIR [--reps N] [--writes] [--profiles DIR] [--outputs DIR]
//
// `run` executes one command in-process and prints one JSON line of
// metrics. `matrix` re-executes this binary once per measurement, so every
// sample starts from a cold process exactly as the CLI does.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
)

const (
	queueID = "queue:scale:main"
	prefix  = "APP"
	// reservedWrites is how many untouched OPEN tickets gen leaves for each
	// measured write, so every write repetition targets a revision-1 ticket.
	reservedWrites = 24
)

func main() {
	if len(os.Args) < 2 {
		fail(errors.New("usage: tasks-scale gen|run|matrix ..."))
	}
	var err error
	switch os.Args[1] {
	case "gen":
		err = gen(os.Args[2:])
	case "run":
		err = runOne(os.Args[2:])
	case "matrix":
		err = matrix(os.Args[2:])
	default:
		err = fmt.Errorf("unknown subcommand %q", os.Args[1])
	}
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "tasks-scale:", err)
	os.Exit(2)
}

// ---------------------------------------------------------------- gen

// manifest records what gen built and which tickets the write
// measurements may consume.
type manifest struct {
	Tickets  int      `json:"tickets"`
	Receipts int      `json:"receipts"`
	Seconds  float64  `json:"seconds"`
	Show     string   `json:"show"`
	Label    string   `json:"label"`
	Claim    []string `json:"claim"`
	Complete []string `json:"complete"`
	Next     int      `json:"next"`
}

func manifestPath(store string) string { return filepath.Join(store, ".scale-manifest.json") }

var labels = []string{"bugs", "fixes", "tests", "optimizations", "ideas", "questions", "agent-memory", "docs"}
var owners = []string{"alice", "bob", "carol", "dave"}
var kinds = []string{"FEATURE", "BUG", "CHORE", "DOC", "SPIKE"}

func createPayload(i int, deps []string) string {
	ds := make([]map[string]any, 0, len(deps))
	for _, d := range deps {
		ds = append(ds, map[string]any{"gateId": nil, "obligation": "COMPLETED", "ticketId": d})
	}
	ls := []string{labels[i%len(labels)]}
	if i%3 == 0 {
		ls = append(ls, labels[(i/3)%len(labels)])
	}
	sort.Strings(ls)
	ls = dedupe(ls)
	var milestone any
	if i%5 == 0 {
		milestone = fmt.Sprintf("m%d", i%7)
	}
	var owner any
	if i%2 == 0 {
		owner = owners[i%len(owners)]
	}
	p := map[string]any{
		"acceptanceCriteria": []string{fmt.Sprintf("criterion %d holds", i), "tests pass"},
		"body":               strings.Repeat(fmt.Sprintf("Synthetic scale ticket %d body text. ", i), 8),
		"capabilities":       []string{},
		"dependencies":       ds,
		"dueDate":            nil,
		"effects":            map[string]any{"coverage": "QUALIFIED", "externalUnbounded": false, "resources": []any{}, "touchPaths": []string{fmt.Sprintf("pkg/p%d/f%d.go", i%50, i)}},
		"estimateMinutes":    nil,
		"executionClass":     "AUTONOMOUS",
		"kind":               kinds[i%len(kinds)],
		"labels":             ls,
		"milestone":          milestone,
		"order":              strconv.Itoa(i % 100),
		"owner":              owner,
		"priority":           fmt.Sprintf("P%d", i%4),
		"requiredGates":      []string{},
		"requirementRefs":    []string{},
		"source":             map[string]any{"kind": "NATIVE", "sourceItemId": nil, "sourceQueueId": queueID, "sourceRevisionSha256": nil},
		"supersededBy":       nil,
		"supersedes":         nil,
		"title":              fmt.Sprintf("Synthetic ticket %d", i),
	}
	b, _ := json.Marshal(p)
	return string(b)
}

func dedupe(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

func local(i int) string { return fmt.Sprintf("%s-%04d", prefix, i) }
func ticketID(i int) string {
	return "ticket:scale:main:" + local(i)
}

// call runs one CLI command in-process and requires outcome OK.
func call(store string, args ...string) (map[string]any, error) {
	var out, errb bytes.Buffer
	code := cli.Run(cli.Env{Cwd: store, Args: args, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &errb})
	var env map[string]any
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		return nil, fmt.Errorf("%v: exit %d: undecodable envelope: %s %s", args, code, out.String(), errb.String())
	}
	if code != 0 || env["outcome"] != "OK" {
		return env, fmt.Errorf("%v: exit %d: %s", args, code, out.String())
	}
	return env, nil
}

func item(env map[string]any) map[string]any {
	items, _ := env["items"].([]any)
	if len(items) == 0 {
		return map[string]any{}
	}
	m, _ := items[0].(map[string]any)
	return m
}

func writeJSONFile(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func gen(args []string) error {
	fs := flag.NewFlagSet("gen", flag.ExitOnError)
	root := fs.String("root", "", "store root to create (must not exist)")
	n := fs.Int("tickets", 1000, "ticket count (the frozen queue limit is 10000)")
	_ = fs.Parse(args)
	if *root == "" || *n < 2*reservedWrites+10 {
		return errors.New("gen needs --root and --tickets >= 58")
	}
	if _, err := os.Lstat(*root); err == nil {
		return fmt.Errorf("%s exists; refusing to reuse it", *root)
	}
	start := time.Now()
	if err := os.MkdirAll(filepath.Join(*root, ".taskman"), 0o755); err != nil {
		return err
	}
	store, err := filepath.EvalSymlinks(*root)
	if err != nil {
		return err
	}
	if out, err := exec.Command("git", "-C", store, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		return fmt.Errorf("git init: %v %s", err, out)
	}
	queue := map[string]any{"canonicalWriter": "NATIVE", "executionCutover": nil, "fixture": true, "foreignAdapterId": nil,
		"importMapSha256": nil, "intentBranch": "main", "nextSerial": "1", "prefix": prefix, "profile": "taskman-queue/0",
		"queueId": queueID, "repositoryAuthorityId": "repo:scale", "schemaVersion": "0",
		"writeBarrier": map[string]any{"reason": "NONE", "since": nil}}
	policy := map[string]any{"allowEmptyObligationsKinds": []string{"CHORE"},
		"budgets": map[string]any{"lane": map[string]any{"cacheCreationTokens": "1000000", "cacheReadTokens": "100000000", "inputTokens": "2000000",
			"outputTokens": "400000", "turns": "400", "wallClockMinutes": "90"}, "requireEnforcedFields": []string{}, "ticketMultiplier": "4"},
		"capacity": map[string]any{"classes": []any{}, "maxActiveAttempts": "64", "maxWorkersTotal": "64"}, "cemRequired": false,
		"docsLane": map[string]any{"required": false}, "environment": map[string]any{"allowedEnvKeys": []string{"PATH"}},
		"gates": []any{map[string]any{"argv": []string{"make", "verify"}, "cwd": "WORKTREE", "env": []string{"PATH"}, "evidence": []any{},
			"expected": map[string]any{"exitCode": "0", "reducer": nil}, "gateId": "verify", "inputs": []any{}, "kind": "COMMAND",
			"required": true, "reusable": true, "sharedResource": nil, "timeoutSeconds": "1800"}},
		"integrationRequiredKinds": []any{}, "ocmRequired": false, "policyVersion": "1", "profile": "taskman-policy/0",
		"retention": map[string]any{"evidenceDays": "90"}, "retries": map[string]any{"admissionsPerRevision": "3", "gateRerunOnStale": "1",
			"malformedReviewRetry": "1", "reconcileAttempts": "3", "repairRounds": "2"}, "reviewLane": map[string]any{"required": false},
		"roles": map[string]any{}, "runtimes": []any{}, "serialFallback": "WHOLE_REPOSITORY"}
	// encoding/json sorts map keys and writes compact separators: canonical.
	if err := writeJSONFile(filepath.Join(store, ".taskman", "queue.json"), queue); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(store, ".taskman", "policy.json"), policy); err != nil {
		return err
	}
	if _, err := call(store, "init", "--request-id", "scale-init"); err != nil {
		return err
	}
	receipts := 1
	m := manifest{Tickets: *n, Label: "optimizations", Show: local(*n / 2)}
	reserved := map[int]bool{}
	for k := 0; k < reservedWrites; k++ {
		c, d := *n-2*reservedWrites+k+1, *n-reservedWrites+k+1
		reserved[c], reserved[d] = true, true
		m.Claim = append(m.Claim, local(c))
		m.Complete = append(m.Complete, local(d))
	}
	committed := false
	for i := 1; i <= *n; i++ {
		var deps []string
		if !reserved[i] && i > 10 && i%3 == 0 {
			for k := 1; k <= 1+i%3; k++ {
				deps = append(deps, ticketID(i-k*3-1))
			}
		}
		if _, err := call(store, "ticket", "create", "--request-id", fmt.Sprintf("c%d", i), "--payload", createPayload(i, deps)); err != nil {
			return err
		}
		receipts++
		if reserved[i] {
			continue
		}
		if i%2 == 0 {
			if _, err := call(store, "ticket", "prioritize", "--request-id", fmt.Sprintf("p%d", i), "--target", local(i),
				"--expected-revision", "1", "--payload", fmt.Sprintf(`{"order":"%d","priority":"P%d"}`, (i*7)%100, (i+1)%4)); err != nil {
				return err
			}
			receipts++
		}
		// A claim observes git: the intent tree is committed once, then
		// every fifth dependency-free ticket is claimed and released, then
		// (like the real store) roughly 60% complete manually.
		if i%5 == 0 && i%3 != 0 && i > 50 {
			if !committed {
				if out, err := exec.Command("git", "-C", store, "add", "-A").CombinedOutput(); err != nil {
					return fmt.Errorf("git add: %v %s", err, out)
				}
				if out, err := exec.Command("git", "-C", store, "-c", "user.name=scale", "-c", "user.email=scale@example.invalid", "commit", "-qm", "seed").CombinedOutput(); err != nil {
					return fmt.Errorf("git commit: %v %s", err, out)
				}
				committed = true
			}
			if err := claimRelease(store, local(i), fmt.Sprintf("g%d", i)); err != nil {
				return err
			}
			receipts += 2
		}
		if i%5 != 1 && i%5 != 3 && (i%3 != 0 || i%2 == 0) {
			rev := 1
			if i%2 == 0 {
				rev = 2
			}
			if _, err := call(store, "ticket", "complete-manual", "--request-id", fmt.Sprintf("m%d", i), "--target", local(i),
				"--expected-revision", strconv.Itoa(rev), "--payload", `{"evidence":[],"reason":"synthetic"}`); err != nil {
				return err
			}
			receipts++
		}
		if i%500 == 0 {
			fmt.Fprintf(os.Stderr, "gen: %d/%d tickets, %d receipts, %.0fs\n", i, *n, receipts, time.Since(start).Seconds())
		}
	}
	m.Receipts = receipts
	m.Seconds = time.Since(start).Seconds()
	return writeJSONFile(manifestPath(store), m)
}

// claimRelease claims a ticket and releases the attempt again; the claim is
// what a real store's attempts/ directory and lease receipts come from.
func claimRelease(store, target, req string) error {
	env, err := call(store, "claim", target, "--holder", "scale", "--request-id", req+"c")
	if err != nil {
		return err
	}
	it := item(env)
	attempt, _ := it["attemptId"].(string)
	generation, _ := it["generation"].(string)
	if attempt == "" || generation == "" {
		return fmt.Errorf("claim %s: no attempt in %v", target, it)
	}
	_, err = call(store, "release", "--attempt", attempt, "--generation", generation, "--request-id", req+"r")
	return err
}

// ---------------------------------------------------------------- run

type metrics struct {
	Args       []string `json:"args"`
	Exit       int      `json:"exit"`
	WallMs     float64  `json:"wallMs"`
	UserMs     float64  `json:"userMs"`
	SysMs      float64  `json:"sysMs"`
	Mallocs    uint64   `json:"mallocs"`
	AllocBytes uint64   `json:"allocBytes"`
	PeakFDs    int      `json:"peakFds,omitempty"`
	OutBytes   int      `json:"outBytes"`
	OutSha256  string   `json:"outSha256"`
}

func runOne(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	store := fs.String("store", "", "store root")
	fds := fs.Bool("fds", false, "sample the peak open descriptor count")
	cpu := fs.String("cpuprofile", "", "write a CPU profile")
	mem := fs.String("memprofile", "", "write an allocation profile")
	outFile := fs.String("out", "", "write the command's stdout here")
	_ = fs.Parse(args)
	cmd := fs.Args()
	if *store == "" || len(cmd) == 0 {
		return errors.New("run needs --store and -- ARGS")
	}
	if *mem != "" {
		runtime.MemProfileRate = 4096
	}
	var stop func() int
	if *fds {
		stop = sampleFDs()
	}
	if *cpu != "" {
		f, err := os.Create(*cpu)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
	}
	var out bytes.Buffer
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	ru0 := rusage()
	t0 := time.Now()
	code := cli.Run(cli.Env{Cwd: *store, Args: cmd, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: io.Discard})
	wall := time.Since(t0)
	ru1 := rusage()
	runtime.ReadMemStats(&after)
	if *cpu != "" {
		pprof.StopCPUProfile()
	}
	m := metrics{Args: cmd, Exit: code, WallMs: ms(wall), UserMs: ms(ru1[0] - ru0[0]), SysMs: ms(ru1[1] - ru0[1]),
		Mallocs: after.Mallocs - before.Mallocs, AllocBytes: after.TotalAlloc - before.TotalAlloc, OutBytes: out.Len()}
	sum := sha256.Sum256(out.Bytes())
	m.OutSha256 = hex.EncodeToString(sum[:])
	if stop != nil {
		m.PeakFDs = stop()
	}
	if *mem != "" {
		f, err := os.Create(*mem)
		if err != nil {
			return err
		}
		if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
			return err
		}
		f.Close()
	}
	if *outFile != "" {
		if err := os.WriteFile(*outFile, out.Bytes(), 0o644); err != nil {
			return err
		}
	}
	b, _ := json.Marshal(m)
	fmt.Println(string(b))
	return nil
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// sampleFDs polls the process's descriptor table until stopped and returns
// the largest count seen. It is a sampled peak: a descriptor opened and
// closed between two polls is not observed.
func sampleFDs() func() int {
	var peak atomic.Int64
	var done atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !done.Load() {
			if n := countFDs(); int64(n) > peak.Load() {
				peak.Store(int64(n))
			}
			runtime.Gosched()
		}
	}()
	return func() int {
		done.Store(true)
		wg.Wait()
		return int(peak.Load()) - 1 // the sampler's own directory handle
	}
}

func countFDs() int {
	d, err := os.Open("/dev/fd")
	if err != nil {
		return 0
	}
	defer d.Close()
	names, _ := d.Readdirnames(-1)
	return len(names)
}

// ---------------------------------------------------------------- matrix

type spec struct {
	name  string
	args  []string
	write bool
}

func readSpecs(m manifest) []spec {
	return []spec{
		{"version", []string{"version"}, false},
		{"queue status", []string{"queue", "status"}, false},
		{"roadmap", []string{"roadmap"}, false},
		{"ticket list", []string{"ticket", "list"}, false},
		{"ticket search --facets", []string{"ticket", "search", "--label", m.Label, "--facets"}, false},
		{"ticket search --count", []string{"ticket", "search", "--label", m.Label, "--count"}, false},
		{"ticket show", []string{"ticket", "show", m.Show}, false},
		{"receipt audit", []string{"receipt", "audit"}, false},
	}
}

func matrix(args []string) error {
	fs := flag.NewFlagSet("matrix", flag.ExitOnError)
	store := fs.String("store", "", "store root built by gen")
	reps := fs.Int("reps", 3, "repetitions per command (median reported)")
	writes := fs.Bool("writes", false, "also measure create, claim and complete-manual (mutates the store)")
	profiles := fs.String("profiles", "", "write CPU and allocation profiles of each read here")
	outputs := fs.String("outputs", "", "write each read's stdout here (for old/new byte equivalence)")
	only := fs.String("only", "", "comma-separated command names to run")
	_ = fs.Parse(args)
	raw, err := os.ReadFile(manifestPath(*store))
	if err != nil {
		return err
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	want := map[string]bool{}
	for _, s := range strings.Split(*only, ",") {
		if s != "" {
			want[s] = true
		}
	}
	specs := readSpecs(m)
	fmt.Printf("| command | wall ms (median of %d) | user ms | sys ms | mallocs | alloc MiB | peak fds | out bytes |\n|---|---|---|---|---|---|---|---|\n", *reps)
	for _, s := range specs {
		if len(want) > 0 && !want[s.name] {
			continue
		}
		var samples []metrics
		for r := 0; r < *reps; r++ {
			x, err := exec1(self, *store, s.args, false, "", "")
			if err != nil {
				return err
			}
			if x.Exit != 0 {
				return fmt.Errorf("%v: exit %d", s.args, x.Exit)
			}
			samples = append(samples, x)
		}
		fd, err := exec1(self, *store, s.args, true, "", "")
		if err != nil {
			return err
		}
		var prof, out string
		if *profiles != "" {
			prof = filepath.Join(*profiles, strings.ReplaceAll(s.name, " ", "_"))
		}
		if *outputs != "" {
			out = filepath.Join(*outputs, strings.ReplaceAll(s.name, " ", "_")+".json")
		}
		if prof != "" || out != "" {
			if _, err := exec1(self, *store, s.args, false, prof, out); err != nil {
				return err
			}
		}
		row(s.name, samples, fd.PeakFDs)
	}
	if !*writes {
		return nil
	}
	if *reps > reservedWrites {
		return fmt.Errorf("--writes allows at most %d reps", reservedWrites)
	}
	type w struct {
		name string
		args func() []string
	}
	ws := []w{
		{"ticket create", func() []string {
			m.Next++
			return []string{"ticket", "create", "--request-id", fmt.Sprintf("bench-c%d", m.Next), "--payload", createPayload(m.Tickets+m.Next, nil)}
		}},
		{"claim", func() []string {
			m.Next++
			t := m.Claim[0]
			m.Claim = m.Claim[1:]
			return []string{"claim", t, "--holder", "bench", "--request-id", fmt.Sprintf("bench-l%d", m.Next)}
		}},
		{"complete-manual", func() []string {
			m.Next++
			t := m.Complete[0]
			m.Complete = m.Complete[1:]
			return []string{"ticket", "complete-manual", "--request-id", fmt.Sprintf("bench-m%d", m.Next), "--target", t, "--expected-revision", "1", "--payload", `{"evidence":[],"reason":"bench"}`}
		}},
	}
	tmp, err := os.MkdirTemp("", "tasks-scale-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for _, x := range ws {
		if len(want) > 0 && !want[x.name] {
			continue
		}
		var samples []metrics
		peak := 0
		for r := 0; r < *reps; r++ {
			a := x.args()
			out := filepath.Join(tmp, "out.json")
			s, err := exec1(self, *store, a, r == 0, "", out)
			if err != nil {
				return err
			}
			if s.Exit != 0 {
				b, _ := os.ReadFile(out)
				return fmt.Errorf("%v: exit %d: %s", a, s.Exit, b)
			}
			if r == 0 {
				peak = s.PeakFDs
			}
			if x.name == "claim" {
				// Release the attempt, unmeasured, so the next claim is not
				// serialized behind it.
				if err := releaseFrom(*store, out, a[1]); err != nil {
					return err
				}
			}
			samples = append(samples, s)
		}
		row(x.name+" (fds sampled on rep 1)", samples, peak)
	}
	return writeJSONFile(manifestPath(*store), m)
}

// releaseFrom releases the attempt a measured claim's envelope names.
func releaseFrom(store, envFile, target string) error {
	b, err := os.ReadFile(envFile)
	if err != nil {
		return err
	}
	var env map[string]any
	if err := json.Unmarshal(b, &env); err != nil {
		return err
	}
	it := item(env)
	id, _ := it["attemptId"].(string)
	gen, _ := it["generation"].(string)
	if id == "" {
		return fmt.Errorf("claim of %s named no attempt: %s", target, b)
	}
	_, err = call(store, "release", "--attempt", id, "--generation", gen, "--request-id", "bench-r-"+target)
	return err
}

func exec1(self, store string, args []string, fds bool, prof, out string) (metrics, error) {
	a := []string{"run", "--store", store}
	if fds {
		a = append(a, "--fds")
	}
	if prof != "" {
		a = append(a, "--cpuprofile", prof+".cpu.pprof", "--memprofile", prof+".alloc.pprof")
	}
	if out != "" {
		a = append(a, "--out", out)
	}
	a = append(a, "--")
	a = append(a, args...)
	c := exec.Command(self, a...)
	c.Stderr = os.Stderr
	b, err := c.Output()
	if err != nil {
		return metrics{}, fmt.Errorf("%v: %v", args, err)
	}
	var m metrics
	if err := json.Unmarshal(b, &m); err != nil {
		return metrics{}, fmt.Errorf("%v: %v: %s", args, err, b)
	}
	return m, nil
}

func row(name string, s []metrics, peak int) {
	sort.Slice(s, func(i, j int) bool { return s[i].WallMs < s[j].WallMs })
	x := s[len(s)/2]
	fmt.Printf("| %s | %.1f | %.1f | %.1f | %d | %.1f | %d | %d |\n", name, x.WallMs, x.UserMs, x.SysMs, x.Mallocs, float64(x.AllocBytes)/(1<<20), peak, x.OutBytes)
}
