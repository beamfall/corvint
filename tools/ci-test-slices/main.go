// Command ci-test-slices generates and replays the experimental test slices
// that split owner-allowed Go packages across CI shards (AFP-V0-041). Slices only
// move tests between shards; the catch-all slice keeps every test in exactly one.
//
// Inputs are the hosted `go test -json` streams of one complete, passing CI run,
// one file per shard job, raw or as printed by `gh run view RUN --job JOB --log`
// (text before the JSON object is ignored):
//
//	ci-test-slices generate --root DIR --revision SHA --run-url URL [--shards 6] [--target 0] LOG...
//	ci-test-slices replay [--root DIR] [--shards 6] [--costs FILE] [--slices FILE] LOG...
//
// generate enumerates each allowed package with `go test -race -list .` in DIR,
// whose clean HEAD must be SHA, and rewrites the slice file. A package splits only
// when its observed time exceeds the target (default: the ideal shard share). Its
// named slices are contiguous runs of its sorted test names that minimise the
// largest slice, weighted by the observed time of each top-level test, or by
// count when the run has none; names the run lacks take the package median.
// Whatever cannot be enumerated stays whole.
//
// replay places the observed run with and without the slice file and prints the
// predicted per-shard sums of package time. A slice's predicted time is its
// package's observed time scaled by its tests' share of the package's summed
// top-level test time; that share model is an inference, not a measurement.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/.github/cishards"
)

const (
	allowPath  = ".github/cishards/test-split-allow.json"
	slicesPath = ".github/cishards/test-slices.json"
	costsPath  = ".github/cishards/package-costs.json"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ci-test-slices generate|replay [flags] LOG...")
		os.Exit(2)
	}
	code, err := run(context.Background(), os.Args[1], os.Args[2:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ci-test-slices:", err)
	}
	os.Exit(code)
}

// enumerate lists a package's top-level tests; tests replace it.
var enumerate = goList

func run(ctx context.Context, mode string, args []string, out, notes io.Writer) (int, error) {
	fs := flag.NewFlagSet(mode, flag.ContinueOnError)
	fs.SetOutput(notes)
	root := fs.String("root", ".", "repository root")
	revision := fs.String("revision", "", "generate: full commit the root checks out and the slices were listed at")
	runURL := fs.String("run-url", "", "generate: hosted run that produced the logs")
	shards := fs.Int("shards", 6, "CI shard count")
	target := fs.Duration("target", 0, "generate: split packages slower than this (0: the ideal shard share)")
	costs := fs.String("costs", "", "replay: placement cost table (default: the observed run)")
	slices := fs.String("slices", "", "replay: slice file (default: ROOT/"+slicesPath+")")
	if err := fs.Parse(args); err != nil {
		return 2, nil
	}
	if (mode != "generate" && mode != "replay") || fs.NArg() == 0 || *shards < 2 || *shards > cishards.MaxShards || *target < 0 {
		return 2, errors.New("usage: ci-test-slices generate|replay [flags] LOG...")
	}
	obs, err := observe(fs.Args())
	if err != nil {
		return 2, err
	}
	if mode == "generate" {
		return generate(ctx, obs, *root, *revision, *runURL, *shards, *target, notes)
	}
	return replay(obs, *root, *shards, *costs, *slices, out)
}

type observation struct {
	packages map[string]int64            // terminal package outcome, ms
	tests    map[string]map[string]int64 // top-level test outcome, ms
	files    []int64                     // summed package time per log, ms
}

// observe reads one complete passing run. A failed or repeated outcome is refused:
// its time is not a cost of the suite.
func observe(paths []string) (*observation, error) {
	o := &observation{packages: map[string]int64{}, tests: map[string]map[string]int64{}}
	for _, path := range paths {
		sum, err := o.read(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		o.files = append(o.files, sum)
	}
	if len(o.packages) == 0 {
		return nil, errors.New("no terminal package outcome in the logs")
	}
	return o, nil
}

func (o *observation) read(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var sum int64
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 1<<16), 8<<20)
	for s.Scan() {
		line := s.Bytes()
		i := bytes.IndexByte(line, '{')
		if i < 0 {
			continue
		}
		var e struct {
			Action  string
			Package string
			Test    string
			Elapsed float64
		}
		if json.Unmarshal(line[i:], &e) != nil || e.Package == "" || strings.Contains(e.Test, "/") {
			continue
		}
		if e.Action == "fail" {
			return 0, fmt.Errorf("%s %s failed", e.Package, e.Test)
		}
		if e.Action != "pass" && e.Action != "skip" {
			continue
		}
		ms := int64(math.Round(e.Elapsed * 1000))
		if e.Test == "" {
			if _, seen := o.packages[e.Package]; seen {
				return 0, fmt.Errorf("package %s has two terminal outcomes", e.Package)
			}
			o.packages[e.Package] = max(1, ms)
			sum += max(1, ms)
			continue
		}
		if o.tests[e.Package] == nil {
			o.tests[e.Package] = map[string]int64{}
		}
		if _, seen := o.tests[e.Package][e.Test]; seen {
			return 0, fmt.Errorf("test %s %s has two terminal outcomes", e.Package, e.Test)
		}
		o.tests[e.Package][e.Test] = ms
	}
	return sum, s.Err()
}

func (o *observation) total() int64 {
	var n int64
	for _, ms := range o.packages {
		n += ms
	}
	return n
}

var (
	listed   = regexp.MustCompile(`^(Test|Example|Fuzz|Benchmark)[\p{L}\p{Nd}_]*$`)
	nameable = regexp.MustCompile(`^(Test|Example|Fuzz)[A-Za-z0-9_]*$`)
)

// goList enumerates with the race-enabled test binary CI runs, outside any
// workspace or GOFLAGS redirection. Unexpected output is an enumeration failure.
func goList(ctx context.Context, root, pkg string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-race", "-count=1", "-list", ".", pkg)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "GOTOOLCHAIN=local", "CGO_ENABLED=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go test -list %s: %v: %s", pkg, err, strings.TrimSpace(stderr.String()))
	}
	return parseList(raw, pkg)
}

func parseList(raw []byte, pkg string) ([]string, error) {
	var names []string
	done := false
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		switch {
		case done:
			return nil, fmt.Errorf("go test -list %s: output after the package result", pkg)
		case strings.HasPrefix(line, "ok") && strings.Contains(line, pkg):
			done = true
		case listed.MatchString(line):
			if !strings.HasPrefix(line, "Benchmark") {
				names = append(names, line)
			}
		default:
			return nil, fmt.Errorf("go test -list %s: unexpected output %q", pkg, line)
		}
	}
	if !done {
		return nil, fmt.Errorf("go test -list %s: no package result", pkg)
	}
	sort.Strings(names)
	return names, nil
}

type namedSlice struct {
	Milliseconds int64    `json:"milliseconds"`
	Tests        []string `json:"tests"`
}

type split struct {
	Named            []namedSlice `json:"named"`
	RestMilliseconds int64        `json:"restMilliseconds"`
}

type sliceFile struct {
	Profile string `json:"profile"`
	Source  struct {
		Revision  string `json:"revision"`
		RunURL    string `json:"runURL"`
		GoVersion string `json:"goVersion"`
	} `json:"source"`
	Packages map[string]split `json:"packages"`
}

func generate(ctx context.Context, obs *observation, root, revision, runURL string, shards int, target time.Duration, notes io.Writer) (int, error) {
	if err := cleanAt(ctx, root, revision); err != nil {
		return 2, err
	}
	allowRaw, err := os.ReadFile(filepath.Join(root, allowPath))
	if err != nil {
		return 2, err
	}
	allowed, ok := cishards.Allowed(allowRaw)
	if !ok {
		return 2, errors.New("refused: the partition would reject the allow-list")
	}
	limit := target.Milliseconds()
	if limit == 0 {
		limit = max(1, obs.total()/int64(shards))
	}
	var f sliceFile
	f.Profile = "corvint-ci-test-slices/0"
	f.Source.Revision, f.Source.RunURL, f.Source.GoVersion = revision, runURL, "go1.27.1"
	f.Packages = map[string]split{}
	pkgs := make([]string, 0, len(allowed))
	for p := range allowed {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	for _, p := range pkgs {
		elapsed, seen := obs.packages[p]
		if !seen {
			fmt.Fprintf(notes, "whole %s: no observed outcome\n", p)
			continue
		}
		k := int(min(int64(cishards.MaxSlices), int64(shards), (elapsed+limit-1)/limit))
		if k < 2 {
			fmt.Fprintf(notes, "whole %s: %dms within the %dms target\n", p, elapsed, limit)
			continue
		}
		names, err := enumerate(ctx, root, p)
		if err != nil {
			fmt.Fprintf(notes, "whole %s: %v\n", p, err)
			continue
		}
		s, ok := slice(names, obs.tests[p], elapsed, k)
		if !ok {
			fmt.Fprintf(notes, "whole %s: too few nameable tests for %d slices\n", p, k)
			continue
		}
		f.Packages[p] = s
		fmt.Fprintf(notes, "split %s: %dms into %d slices\n", p, elapsed, len(s.Named)+1)
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return 2, err
	}
	raw = append(raw, '\n')
	universe := make([]string, 0, len(obs.packages))
	for p := range obs.packages {
		universe = append(universe, p)
	}
	if got := cishards.SplitPackages(universe, shards, allowRaw, raw); len(got) != len(f.Packages) {
		return 2, errors.New("refused: the partition would not admit every generated split (check --revision, --run-url and the pattern bound)")
	}
	return 0, replace(filepath.Join(root, slicesPath), raw)
}

// cleanAt requires the enumerated tree to be exactly the recorded revision.
func cleanAt(ctx context.Context, root, revision string) error {
	head, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("revision unavailable: %v", err)
	}
	if strings.TrimSpace(string(head)) != revision {
		return fmt.Errorf("refused: %s checks out %s, not --revision %q", root, strings.TrimSpace(string(head)), revision)
	}
	dirty, err := exec.CommandContext(ctx, "git", "-C", root, "status", "--porcelain", "--untracked-files=no").Output()
	if err != nil || len(dirty) != 0 {
		return errors.New("refused: tracked files differ from --revision")
	}
	return nil
}

// slice splits one package into k slices: k-1 named runs of its sorted nameable
// tests and a catch-all holding the rest of them plus every test it cannot name.
func slice(names []string, observed map[string]int64, elapsed int64, k int) (split, bool) {
	var named []string
	for _, n := range names {
		if nameable.MatchString(n) {
			named = append(named, n)
		}
	}
	sort.Strings(named)
	if len(named) < k-1 {
		return split{}, false
	}
	weight := func(string) int64 { return 1 }
	var rest int64
	if len(observed) != 0 {
		values := make([]int64, 0, len(observed))
		for _, ms := range observed {
			values = append(values, ms)
		}
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		median := values[len(values)/2]
		weight = func(n string) int64 {
			if ms, ok := observed[n]; ok {
				return ms
			}
			return median
		}
		in := map[string]bool{}
		for _, n := range named {
			in[n] = true
		}
		for n, ms := range observed {
			if !in[n] {
				rest += ms
			}
		}
	}
	costs := make([]int64, len(named)+1)
	var sum int64
	for i, n := range named {
		costs[i] = weight(n)
		sum += costs[i]
	}
	costs[len(named)] = rest
	sum += rest
	cuts := minMaxCuts(costs, k)
	scale := func(from, to int) int64 {
		var c int64
		for _, x := range costs[from:to] {
			c += x
		}
		ms := elapsed
		if sum > 0 {
			ms = int64(math.Round(float64(elapsed) * float64(c) / float64(sum)))
		}
		return min(3000000, max(1, ms))
	}
	var s split
	for i := 0; i < k-1; i++ {
		s.Named = append(s.Named, namedSlice{Milliseconds: scale(cuts[i], cuts[i+1]), Tests: named[cuts[i]:cuts[i+1]]})
	}
	s.RestMilliseconds = scale(cuts[k-1], len(costs))
	return s, true
}

// minMaxCuts returns the k group starts of the contiguous partition of costs
// that minimises the largest group, every group non-empty, preferring earlier cuts.
func minMaxCuts(costs []int64, k int) []int {
	n := len(costs)
	prefix := make([]int64, n+1)
	for i, c := range costs {
		prefix[i+1] = prefix[i] + c
	}
	const inf = math.MaxInt64
	best := make([][]int64, k+1)
	from := make([][]int, k+1)
	for g := range best {
		best[g] = make([]int64, n+1)
		from[g] = make([]int, n+1)
		for i := range best[g] {
			best[g][i] = inf
		}
	}
	best[0][0] = 0
	for g := 1; g <= k; g++ {
		for i := g; i <= n; i++ {
			for j := g - 1; j < i; j++ {
				if best[g-1][j] == inf {
					continue
				}
				if v := max(best[g-1][j], prefix[i]-prefix[j]); v < best[g][i] {
					best[g][i], from[g][i] = v, j
				}
			}
		}
	}
	cuts := make([]int, k)
	for g, i := k, n; g > 0; g-- {
		cuts[g-1] = from[g][i]
		i = from[g][i]
	}
	return cuts
}

// replace writes through a sibling temporary file so an interrupted run cannot
// leave bytes the partition would reject.
func replace(path string, raw []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".test-slices-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(raw); err == nil {
		err = tmp.Chmod(0o644)
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func replay(obs *observation, root string, shards int, costsFile, slicesFile string, out io.Writer) (int, error) {
	if slicesFile == "" {
		slicesFile = filepath.Join(root, slicesPath)
	}
	slicesRaw, err := os.ReadFile(slicesFile)
	if err != nil {
		return 2, err
	}
	allowRaw, err := os.ReadFile(filepath.Join(root, allowPath))
	if err != nil {
		return 2, err
	}
	var source sliceFile
	if json.Unmarshal(slicesRaw, &source) != nil {
		return 2, errors.New("slice file unreadable")
	}
	var costsRaw []byte
	if costsFile != "" {
		if costsRaw, err = os.ReadFile(costsFile); err != nil {
			return 2, err
		}
	} else if costsRaw, err = json.Marshal(map[string]any{
		"profile":      "corvint-ci-package-costs/0",
		"source":       source.Source,
		"milliseconds": obs.packages,
	}); err != nil {
		return 2, err
	}
	if _, ok := cishards.Costs(costsRaw); !ok {
		return 2, errors.New("placement cost table is invalid")
	}
	universe := make([]string, 0, len(obs.packages))
	for p := range obs.packages {
		universe = append(universe, p)
	}
	whole, err := cishards.PlanFrom(universe, shards, costsRaw, allowRaw, nil)
	if err != nil {
		return 2, err
	}
	sliced, err := cishards.PlanFrom(universe, shards, costsRaw, allowRaw, slicesRaw)
	if err != nil {
		return 2, err
	}
	fmt.Fprintf(out, "universe %d packages %.1fs ideal %.1fs over %d shards\n", len(universe), seconds(obs.total()), seconds(obs.total()/int64(shards)), shards)
	if len(obs.files) == shards {
		fmt.Fprintf(out, "observed %s\n", sums(obs.files))
	}
	fmt.Fprintf(out, "whole    %s\n", sums(predict(obs, whole)))
	fmt.Fprintf(out, "sliced   %s\n", sums(predict(obs, sliced)))
	for i, s := range sliced {
		for _, sl := range s.Slices {
			fmt.Fprintf(out, "slice    shard=%d %s#%d/%d %s %d tests predicted=%.1fs planned=%.1fs\n", i, sl.Package, sl.Index, sl.Count, sl.Flag, members(obs, sl), seconds(sliceTime(obs, sl)), seconds(sl.Milliseconds))
		}
	}
	return 0, nil
}

func predict(obs *observation, plan []cishards.Shard) []int64 {
	out := make([]int64, len(plan))
	for i, s := range plan {
		for _, p := range s.Packages {
			out[i] += obs.packages[p]
		}
		for _, sl := range s.Slices {
			out[i] += sliceTime(obs, sl)
		}
	}
	return out
}

// sliceTime scales the package's observed time by the observed top-level test
// time of the slice's members, or by 1/count when the run has no test records.
func sliceTime(obs *observation, sl cishards.Slice) int64 {
	elapsed := obs.packages[sl.Package]
	in := member(sl)
	var part, all int64
	for name, ms := range obs.tests[sl.Package] {
		all += ms
		if in(name) {
			part += ms
		}
	}
	if all == 0 {
		return elapsed / int64(sl.Count)
	}
	return int64(math.Round(float64(elapsed) * float64(part) / float64(all)))
}

func members(obs *observation, sl cishards.Slice) int {
	n, in := 0, member(sl)
	for name := range obs.tests[sl.Package] {
		if in(name) {
			n++
		}
	}
	return n
}

// member applies the slice's pattern to a top-level name the way `go test` does.
func member(sl cishards.Slice) func(string) bool {
	re := regexp.MustCompile(sl.Pattern)
	return func(name string) bool { return re.MatchString(name) == (sl.Flag == "-run") }
}

func sums(ms []int64) string {
	var b strings.Builder
	var top int64
	for i, n := range ms {
		fmt.Fprintf(&b, "shard%d=%.1fs ", i, seconds(n))
		top = max(top, n)
	}
	fmt.Fprintf(&b, "max=%.1fs", seconds(top))
	return b.String()
}

func seconds(ms int64) float64 { return float64(ms) / 1000 }
