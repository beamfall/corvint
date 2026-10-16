package gokernel

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// TestUnstableRepositoryNamesWhatChanged drives each repository-state-unstable cause through both
// probe entry points: a HEAD that moves on every observation, and a dirty-path set that churns
// under a fixed HEAD. The code stays repository-state-unstable; the message names the cause, so
// worktree churn no longer reads as a HEAD change (V1-0880).
func TestUnstableRepositoryNamesWhatChanged(t *testing.T) {
	const otherCommit = "cccccccccccccccccccccccccccccccccccccccc"
	causes := map[string]struct {
		moveHead bool
		want     string
	}{
		"head":  {true, "repository HEAD changed while probing repository state"},
		"dirty": {false, "repository dirty-path set changed while probing repository state"},
	}
	probes := map[string]func(gitRunner) error{
		"context": func(run gitRunner) error {
			_, err := probeRepositoryContext(context.Background(), t.TempDir(), run)
			return err
		},
		"sharing": func(run gitRunner) error {
			_, err := probeRepositorySharing(context.Background(), t.TempDir(), run, func(context.Context, Observation) (func() error, error) { return nil, nil }, false)
			return err
		},
	}
	for causeName, cause := range causes {
		for probeName, probe := range probes {
			var mutex sync.Mutex
			identityCalls, statusCalls := 0, 0
			run := func(_ context.Context, _ string, _ int, arguments ...string) ([]byte, error) {
				mutex.Lock()
				defer mutex.Unlock()
				switch arguments[0] {
				case "rev-parse":
					identityCalls++
					if cause.moveHead && identityCalls%2 == 0 {
						return []byte("sha1\n" + otherCommit + "\n" + testTreeRevision + "\n"), nil
					}
					return testIdentityOutput(), nil
				case "ls-tree":
					return []byte{}, nil
				case "status":
					statusCalls++
					if !cause.moveHead && statusCalls%2 == 0 {
						return []byte("?? changed\x00"), nil
					}
					return []byte{}, nil
				default:
					return nil, newError("test-command", "unexpected Git command "+strings.Join(arguments, " "))
				}
			}
			err := probe(run)
			var kernelFailure *Error
			if !errors.As(err, &kernelFailure) || kernelFailure.Code != "repository-state-unstable" || kernelFailure.Message != cause.want {
				t.Errorf("%s/%s: error = %#v, want repository-state-unstable %q", probeName, causeName, err, cause.want)
			}
		}
	}
}
