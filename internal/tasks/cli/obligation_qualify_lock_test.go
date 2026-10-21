package cli_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/obligation"
)

// qualifyCode runs qualify-version without the testing.T helpers, so it is
// safe from a goroutine.
func qualifyCode(root, report string) int {
	return cli.Run(cli.Env{Cwd: root, Args: []string{"ticket", "obligations", "qualify-version", "--from-playwright-report", report},
		Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
}

// TestV11745_QualifyVersionHoldsTheStoreLock: qualify-version reads and
// rewrites the allow-list under the store mutation lock, so it waits for a
// holder and concurrent runs keep every qualified version (V1-1745).
func TestV11745_QualifyVersionHoldsTheStoreLock(t *testing.T) {
	r, _, _ := obligationRepo(t)
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	p := obligation.AllowListPath(r.CommonDir)
	report := fixtureRun(t, qualifiedVersion, nil)

	lock, err := authority.AcquireLock(context.Background(), repo, authority.LockOptions{})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)
	go func() { done <- qualifyCode(r.Root, report) }()
	select {
	case code := <-done:
		lock.Close()
		t.Fatalf("qualify-version finished (exit %d) while another process held the store lock", code)
	case <-time.After(500 * time.Millisecond):
	}
	if _, err := os.Lstat(p); !os.IsNotExist(err) {
		lock.Close()
		t.Fatalf("allow-list written under another holder's lock: %v", err)
	}
	lock.Close()
	if code := <-done; code != 0 {
		t.Fatalf("qualify-version after release: exit %d", code)
	}

	versions := []string{"1.65.0", "1.66.0", "1.67.0", "1.68.0"}
	reports := make([]string, len(versions))
	for i, v := range versions {
		reports[i] = fixtureRun(t, v, nil)
	}
	var wg sync.WaitGroup
	codes := make([]int, len(versions))
	for i := range versions {
		wg.Add(1)
		go func(i int) { defer wg.Done(); codes[i] = qualifyCode(r.Root, reports[i]) }(i)
	}
	wg.Wait()
	for i, c := range codes {
		if c != 0 {
			t.Fatalf("qualify %s: exit %d", versions[i], c)
		}
	}
	a, err := obligation.ReadAllowList(p)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(a.Versions(), ","), strings.Join(append([]string{qualifiedVersion}, versions...), ","); got != want {
		t.Fatalf("allow-list versions %s, want %s", got, want)
	}
}
