// Command test-shards splits one Go test package into SHARDS local `go test`
// invocations that together run every top-level test exactly once (ASD-V0-026),
// so a package slower than the agent's 600 s foreground limit, such as
// internal/tasks/store or internal/tasks/cli, can run as bounded pieces:
//
//	test-shards -shards N -shard I PACKAGE
//
// It enumerates PACKAGE with `go test -list .` and prints one line, the `go test`
// selection flag and its pattern: shards 0..N-2 `-run` the anchored names whose
// FNV-1a hash falls in that shard, and shard N-1 is the catch-all, which `-skip`s
// the union of the others. A test added after enumeration, or one the listing
// missed, therefore still runs in the catch-all, and subtests always run with
// their parent. Placement is by name only, so adding a test never moves another;
// it is not balanced by cost. `make test-shard` runs one shard through
// script/heavy.sh.
package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// MaxShards bounds N; MaxPatternBytes keeps one argument far below Linux's
// 128 KiB per-argument limit (MAX_ARG_STRLEN).
const (
	MaxShards       = 64
	MaxPatternBytes = 64 << 10
)

// Only plain ASCII top-level names enter a pattern: they need no regexp quoting
// and contain neither `/` nor `|`, which `go test` splits patterns on.
// Benchmarks never run without -bench, so they are not listed.
var testName = regexp.MustCompile(`^(Test|Example|Fuzz)[A-Za-z0-9_]*$`)

// Names returns the sorted, de-duplicated top-level test names of a
// `go test -list` output, or an error for a line that names a test the
// patterns cannot carry.
func Names(listing []byte) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(listing))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "Test") && !strings.HasPrefix(line, "Example") && !strings.HasPrefix(line, "Fuzz") {
			continue // "ok  pkg 0.1s", "?  pkg [no test files]", Benchmark names
		}
		if !testName.MatchString(line) {
			return nil, fmt.Errorf("test name %q cannot be carried by a shard pattern", line)
		}
		if !seen[line] {
			seen[line] = true
			out = append(out, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// Of returns the shard of a test name among n.
func Of(name string, n int) int {
	h := fnv.New32a()
	h.Write([]byte(name))
	return int(h.Sum32() % uint32(n))
}

// Selection returns the `go test` flag and pattern of shard i of n over names.
// Shards 0..n-2 run their own names; shard n-1 skips every name of the others,
// so it runs its own names and anything not enumerated. A named shard with no
// names gets a pattern that matches nothing.
func Selection(names []string, n, i int) (flag, pattern string, err error) {
	if n < 2 || n > MaxShards {
		return "", "", fmt.Errorf("shards must be 2..%d", MaxShards)
	}
	if i < 0 || i >= n {
		return "", "", errors.New("shard index must be 0..shards-1")
	}
	var picked []string
	for _, name := range names {
		s := Of(name, n)
		if (i < n-1 && s == i) || (i == n-1 && s != n-1) {
			picked = append(picked, name)
		}
	}
	flag = "-run"
	if i == n-1 {
		flag = "-skip"
	}
	if len(picked) == 0 {
		// `^$` matches no top-level name: an empty named shard runs nothing and
		// an empty catch-all skips nothing.
		return flag, "^$", nil
	}
	pattern = "^(" + strings.Join(picked, "|") + ")$"
	if len(pattern) > MaxPatternBytes {
		return "", "", fmt.Errorf("shard pattern is %d bytes, over %d; use more shards", len(pattern), MaxPatternBytes)
	}
	return flag, pattern, nil
}

func main() {
	n := flag.Int("shards", 0, "shard count, 2..64")
	i := flag.Int("shard", -1, "zero-based shard index")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: test-shards -shards N -shard I PACKAGE")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if _, _, err := Selection(nil, *n, *i); err != nil {
		fmt.Fprintln(os.Stderr, "test-shards:", err)
		os.Exit(2)
	}
	cmd := exec.Command("go", "test", "-list", ".", flag.Arg(0))
	cmd.Stderr = os.Stderr
	listing, err := cmd.Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "test-shards: go test -list failed:", err)
		os.Exit(1)
	}
	if code := emit(os.Stdout, listing, *n, *i); code != 0 {
		os.Exit(code)
	}
}

func emit(w io.Writer, listing []byte, n, i int) int {
	names, err := Names(listing)
	if err == nil && len(names) == 0 {
		err = errors.New("the package lists no tests")
	}
	var f, p string
	if err == nil {
		f, p, err = Selection(names, n, i)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "test-shards:", err)
		return 1
	}
	fmt.Fprintln(w, f, p)
	return 0
}
