package sessiontest

import (
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ciWorkflow is the workflow whose full branch the ci profile reproduces.
const ciWorkflow = "../../.github/workflows/ci.yml"

// ciJob returns the lines of one top-level job of ci.yml.
func ciJob(t *testing.T, name string) []string {
	t.Helper()
	raw, err := os.ReadFile(ciWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	var job []string
	in := false
	for _, line := range strings.Split(string(raw), "\n") {
		header := strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && strings.HasSuffix(line, ":")
		switch {
		case header && line == "  "+name+":":
			in = true
		case header && in:
			return job
		case in:
			job = append(job, line)
		}
	}
	if !in {
		t.Fatalf("%s has no %s job", ciWorkflow, name)
	}
	return job
}

// STR-V0-012, STR-V0-015: the ci profile's Go environment, flags and shard
// budget are compared with the full branch of the go-product-shard job, the
// environment its go test actually sees (the job env over the runner's own),
// not with the narrowed driver's closed environment. A drift on either side
// fails here, so the constants cannot silently diverge from the branch that
// decides CI.
func TestSTRV0012CIConstantsMatchFullBranch(t *testing.T) {
	job := ciJob(t, "go-product-shard")
	var env []string
	inEnv, timeout := false, 0
	for _, line := range job {
		switch {
		case line == "    env:":
			inEnv = true
			continue
		case strings.HasPrefix(line, "    timeout-minutes: "):
			timeout, _ = strconv.Atoi(strings.TrimPrefix(line, "    timeout-minutes: "))
		}
		if inEnv {
			if !strings.HasPrefix(line, "      ") {
				inEnv = false
				continue
			}
			name, value, _ := strings.Cut(strings.TrimSpace(line), ": ")
			if strings.HasPrefix(name, "GO") || strings.HasPrefix(name, "CGO_") {
				env = append(env, name+"="+strings.Trim(value, `"`))
			}
		}
	}
	if want := slices.Sorted(slices.Values(ciGoEnv)); !slices.Equal(slices.Sorted(slices.Values(env)), want) {
		t.Fatalf("go-product-shard job Go environment %v, ciGoEnv %v", env, want)
	}
	// AFP-V0-540: the job takes -p from SHARD_TEST_P, which is above 1 only on a self-hosted
	// pull-request shard; the merge queue's deciding full suite, like this profile, runs -p 1.
	if ciGoFlags[0] != "-p" || ciGoFlags[1] != "1" {
		t.Fatalf("ciGoFlags %v must start with -p 1", ciGoFlags)
	}
	const shardP = `          SHARD_TEST_P: ${{ github.event_name == 'pull_request' && runner.environment == 'self-hosted' && '4' || '1' }}`
	if !slices.Contains(job, shardP) {
		t.Fatalf("go-product-shard does not set %q, so the merge-queue suite's -p is not this profile's -p 1", strings.TrimSpace(shardP))
	}
	prefix := `CORVINT_CONFINE_ROOT="$PWD" go test -exec "$RUNNER_TEMP/test-confine" -json -p "$SHARD_TEST_P" ` + strings.Join(ciGoFlags[2:], " ") + " "
	goTest := regexp.MustCompile(`\bgo test\b`)
	invocations := 0
	for _, line := range job {
		if !goTest.MatchString(line) || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		invocations++
		if !strings.HasPrefix(strings.TrimSpace(line), prefix) {
			t.Fatalf("full-branch go test %q does not start with %q", strings.TrimSpace(line), prefix)
		}
	}
	if invocations != 3 {
		t.Fatalf("go-product-shard runs %d go test lines, want the whole-package, slice and AFP-V0-047 rerun invocations", invocations)
	}
	// The shard budget ends before the job timeout, with room for checkout,
	// toolchain and tool builds, so the runner, not the job, kills a slow shard.
	if headroom := time.Duration(timeout)*time.Minute - CIBudget; headroom < 10*time.Minute {
		t.Fatalf("go-product-shard timeout-minutes %d leaves %s after the %s ci shard budget", timeout, headroom, CIBudget)
	}
}
