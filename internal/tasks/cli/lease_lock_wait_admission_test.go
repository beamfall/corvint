package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0216_LockWaitAdmissionVerbs: claim, renew, submit, pool acquire,
// pool confirm-safe and pool release accept --lock-wait with the CAL-V0-111
// value rules and help, and a contended command waits for the caller's bound
// on preparation admission and on the writer lock, not the 30-second default,
// without a write. Under CAL-V0-263 a planned write (renew here) enters the
// preparation gate, while a refusal (the other five verbs in this fixture)
// is reported under the writer lock without entering it, so both are held.
func TestCALV0216_LockWaitAdmissionVerbs(t *testing.T) {
	t.Parallel() // idle-bound on fixed waits and independent of process state (V1-2366)
	root, repo, _, attempt, generation := lockWaitStore(t)
	digest := strings.Repeat("a", 64)
	verbs := [][]string{
		{"claim", "--next", "--holder", "h", "--request-id", "claim-wait"},
		{"renew", "--attempt", attempt, "--generation", generation, "--request-id", "renew-wait"},
		{"submit", "--attempt", attempt, "--generation", generation, "--request-id", "submit-wait", "--tree", strings.Repeat("0", 40)},
		{"pool", "acquire", "--attempt", attempt, "--generation", generation, "--pool", "db", "--request-id", "acquire-wait"},
		{"pool", "confirm-safe", "--member", "a", "--allocation", digest, "--evidence", "local:safe", "--reason", "checked", "--request-id", "safe-wait"},
		{"pool", "release", "--attempt", attempt, "--generation", generation, "--allocation", digest, "--request-id", "pool-release-wait"},
	}
	limit := int64(authority.MaxCallerLockWait / time.Second)
	before := fixture.TreeSnapshot(t, repo.StateDir)
	for _, base := range verbs {
		for _, value := range []string{"0", fmt.Sprint(limit + 1), "007", "5s", ""} {
			x := atm(t, root, nil, append(append([]string{}, base...), "--lock-wait", value)...)
			if x.code == 0 || !hasCode(x.res, wire.CodeMalformed) {
				t.Fatalf("%v --lock-wait %q: %s", base[:2], value, x.stdout)
			}
		}
		x := atm(t, root, nil, append(append([]string{}, base...), "--lock-wait", "5", "--lock-wait", "5")...)
		if !hasCode(x.res, wire.CodeMalformed) {
			t.Fatalf("%v repeated --lock-wait: %s", base[:2], x.stdout)
		}
		help := atm(t, root, nil, append(verbName(base), "--help", "--verbose")...)
		usage, note := field(help.res.Items[0], "usage").Str, field(help.res.Items[0], "lockWait").Str
		if help.code != 0 || !strings.Contains(usage, "[--lock-wait SECONDS]") || !strings.Contains(note, fmt.Sprintf("1..%d", limit)) {
			t.Fatalf("%v help: %s", base[:2], help.stdout)
		}
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, repo.StateDir)) {
		t.Fatal("a refused --lock-wait wrote state")
	}

	held := holdPreparation(t, repo)
	defer held.Close()
	writer, err := authority.AcquireLock(context.Background(), repo, authority.LockOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	// Every verb waits its own one-second bound against the same held locks,
	// so the six contended commands run concurrently instead of one after the
	// other; each is judged on its own elapsed time.
	type contended struct {
		x       run
		elapsed time.Duration
		err     error
	}
	results := make([]contended, len(verbs))
	var wg sync.WaitGroup
	for i, base := range verbs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var out, errb bytes.Buffer
			start := time.Now()
			code := cli.Run(cli.Env{Cwd: root, Args: append(append([]string{}, base...), "--lock-wait", "1"), Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &errb})
			results[i].elapsed = time.Since(start)
			results[i].x = run{code: code, stdout: out.Bytes(), stderr: errb.Bytes()}
			results[i].x.res, results[i].err = wire.DecodeResult(out.Bytes())
		}()
	}
	wg.Wait()
	for i, r := range results {
		if r.err != nil {
			t.Fatalf("%v: envelope does not decode: %v\n%s", verbs[i][:2], r.err, r.x.stdout)
		}
		lockTimedOut(t, r.x, r.elapsed, time.Second)
		// The planned renew waited on preparation admission itself.
		if i == 1 && !strings.Contains(string(r.x.stdout), "preparation acquisition exceeded") {
			t.Fatalf("renew timed out outside preparation admission: %s", r.x.stdout)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, repo.StateDir)) {
		t.Fatal("a timed-out --lock-wait command wrote state")
	}

	// A holder that leaves within the bound admits the waiting command.
	go func() {
		time.Sleep(1500 * time.Millisecond)
		_ = held.Close()
	}()
	start := time.Now()
	x := atm(t, root, nil, append(append([]string{}, verbs[1]...), "--lock-wait", "20")...)
	if x.code != 0 || field(x.res.Items[0], "replayed").Bool || time.Since(start) < time.Second {
		t.Fatalf("renew queued behind the holder after %v: %s", time.Since(start), x.stdout)
	}
}

// verbName is the command words of argv, up to its first flag.
func verbName(argv []string) []string {
	for i, a := range argv {
		if strings.HasPrefix(a, "--") {
			return append([]string{}, argv[:i]...)
		}
	}
	return append([]string{}, argv...)
}
