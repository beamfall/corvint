// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// epochWorkflows are the workflows that name the forward epoch, and the variables each must set
// (CIN-V0-960). ci.yml's CORVINT_PR_FORWARD_EPOCH is the single source the tests read.
var epochWorkflows = []struct{ file, variable string }{
	{"ci.yml", "CORVINT_PR_FORWARD_EPOCH"},
	{"ci.yml", "FORWARD_EPOCH"},
	{"ci-forward-bar.yml", "FORWARD_EPOCH"},
	{"ci-control-plane.yml", "FORWARD_EPOCH"},
}

// forwardEpochSites returns every value each workflow sets for its epoch variable, keyed
// "file variable", reading the workflow text through read.
func forwardEpochSites(read func(file string) (string, error)) (map[string][]string, error) {
	sites := map[string][]string{}
	for _, w := range epochWorkflows {
		text, err := read(w.file)
		if err != nil {
			return nil, err
		}
		pattern := regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(w.variable) + `: "([0-9]+)"\s*$`)
		for _, m := range pattern.FindAllStringSubmatch(text, -1) {
			sites[w.file+" "+w.variable] = append(sites[w.file+" "+w.variable], m[1])
		}
	}
	return sites, nil
}

// epochDisagreement is nil when every site names one epoch, else it names the sites.
func epochDisagreement(sites map[string][]string) error {
	seen := map[string]bool{}
	for _, w := range epochWorkflows {
		values := sites[w.file+" "+w.variable]
		if len(values) == 0 {
			return fmt.Errorf("%s sets no %s", w.file, w.variable)
		}
		for _, v := range values {
			seen[v] = true
		}
	}
	if len(seen) != 1 {
		return fmt.Errorf("the workflows disagree on the forward epoch: %v", sites)
	}
	return nil
}

func readWorkflow(file string) (string, error) {
	b, err := os.ReadFile("../../.github/workflows/" + file)
	return string(b), err
}

// forwardEpoch is the live forward-evidence epoch, read from ci.yml's CORVINT_PR_FORWARD_EPOCH
// after every epoch site has been checked to agree (AFP-V0-030, CIN-V0-960).
func forwardEpoch(t *testing.T) string {
	t.Helper()
	sites, err := forwardEpochSites(readWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	if err := epochDisagreement(sites); err != nil {
		t.Fatal(err)
	}
	return sites["ci.yml CORVINT_PR_FORWARD_EPOCH"][0]
}

// shiftEpoch is the epoch delta away from epoch.
func shiftEpoch(t *testing.T, epoch string, delta int) string {
	t.Helper()
	n, err := strconv.Atoi(epoch)
	if err != nil {
		t.Fatal(err)
	}
	return strconv.Itoa(n + delta)
}

// TestCINV0960WorkflowsAgreeOnTheForwardEpoch pins CIN-V0-960: the committed workflows name one
// forward epoch, and a workflow that names another, or none, fails.
func TestCINV0960WorkflowsAgreeOnTheForwardEpoch(t *testing.T) {
	live := forwardEpoch(t)
	next := shiftEpoch(t, live, 1)
	for _, file := range []string{"ci.yml", "ci-forward-bar.yml", "ci-control-plane.yml"} {
		for _, mutation := range []struct {
			name string
			edit func(string) string
		}{
			{"advanced alone", func(s string) string {
				return strings.Replace(s, `FORWARD_EPOCH: "`+live+`"`, `FORWARD_EPOCH: "`+next+`"`, 1)
			}},
			{"dropped", func(s string) string {
				return regexp.MustCompile(`(?m)^(\s+)((?:CORVINT_PR_)?FORWARD_EPOCH): "[0-9]+"`).ReplaceAllString(s, `${1}${2}_REMOVED: "0"`)
			}},
		} {
			t.Run(file+"/"+mutation.name, func(t *testing.T) {
				sites, err := forwardEpochSites(func(f string) (string, error) {
					text, err := readWorkflow(f)
					if f == file {
						text = mutation.edit(text)
					}
					return text, err
				})
				if err != nil {
					t.Fatal(err)
				}
				if epochDisagreement(sites) == nil {
					t.Fatalf("%s %s: no disagreement reported: %v", file, mutation.name, sites)
				}
			})
		}
	}
}

// epochSinces records, per advanced epoch, the FORWARD_EPOCH_SINCE that ci-forward-bar.yml set
// with it: history, kept explicit. An epoch beyond the table needs a later time than the newest
// recorded one, so a synchronized advance passes and an advance that forgets the time fails.
var epochSinces = map[string]string{"2": "2026-10-30T00:00:00Z"}

// sinceLine is ci-forward-bar.yml's FORWARD_EPOCH_SINCE.
var sinceLine = regexp.MustCompile(`(?m)^\s+FORWARD_EPOCH_SINCE: "([^"]*)"\s*$`)

// epochSince reads FORWARD_EPOCH_SINCE from ci-forward-bar.yml through read.
func epochSince(read func(file string) (string, error)) (string, error) {
	text, err := read("ci-forward-bar.yml")
	if err != nil {
		return "", err
	}
	m := sinceLine.FindAllStringSubmatch(text, -1)
	if len(m) != 1 {
		return "", fmt.Errorf("ci-forward-bar.yml sets FORWARD_EPOCH_SINCE %d times, want once", len(m))
	}
	return m[0][1], nil
}

func liveEpochSince(t *testing.T) string {
	t.Helper()
	since, err := epochSince(readWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	return since
}

// forwardEpochSinceProblem checks the live FORWARD_EPOCH_SINCE against the live epoch: a UTC
// RFC 3339 time, equal to the recorded time of a recorded epoch, and later than the newest
// recorded time for an epoch past the table (CIN-V0-003, CIN-V0-960).
func forwardEpochSinceProblem(epoch, since string) error {
	at, err := time.Parse(time.RFC3339, since)
	if err != nil || !strings.HasSuffix(since, "Z") {
		return fmt.Errorf("FORWARD_EPOCH_SINCE %q is not a UTC RFC 3339 time", since)
	}
	if recorded, ok := epochSinces[epoch]; ok {
		if since != recorded {
			return fmt.Errorf("epoch %s began at %s, but ci-forward-bar.yml names FORWARD_EPOCH_SINCE %s; move both together", epoch, recorded, since)
		}
		return nil
	}
	n, err := strconv.Atoi(epoch)
	keys := slices.Collect(maps.Keys(epochSinces))
	slices.SortFunc(keys, func(a, b string) int { x, _ := strconv.Atoi(a); y, _ := strconv.Atoi(b); return x - y })
	newest := keys[len(keys)-1]
	if newestN, _ := strconv.Atoi(newest); err != nil || n < newestN {
		return fmt.Errorf("live epoch %q is neither recorded nor after epoch %s", epoch, newest)
	}
	if last, _ := time.Parse(time.RFC3339, epochSinces[newest]); !at.After(last) {
		return fmt.Errorf("epoch %s advanced past %s but FORWARD_EPOCH_SINCE %s is not later than epoch %s's %s; move both together", epoch, newest, since, newest, epochSinces[newest])
	}
	return nil
}

// TestCINV0960SynchronizedAdvancePasses pins the review finding: advancing every workflow to the
// next epoch with a later FORWARD_EPOCH_SINCE passes the live checks, and advancing the epoch
// while keeping the old time, or moving one workflow alone, fails.
func TestCINV0960SynchronizedAdvancePasses(t *testing.T) {
	live := forwardEpoch(t)
	next := shiftEpoch(t, live, 1)
	old := liveEpochSince(t)
	later := "2099-01-01T00:00:00Z"
	advanced := func(since string) func(string) (string, error) {
		return func(file string) (string, error) {
			text, err := readWorkflow(file)
			text = strings.ReplaceAll(text, `FORWARD_EPOCH: "`+live+`"`, `FORWARD_EPOCH: "`+next+`"`)
			text = strings.ReplaceAll(text, `FORWARD_EPOCH_SINCE: "`+old+`"`, `FORWARD_EPOCH_SINCE: "`+since+`"`)
			return text, err
		}
	}
	for _, c := range []struct {
		name  string
		since string
		ok    bool
	}{{"with a later time", later, true}, {"keeping the old time", old, false}} {
		t.Run(c.name, func(t *testing.T) {
			sites, err := forwardEpochSites(advanced(c.since))
			if err != nil {
				t.Fatal(err)
			}
			if err := epochDisagreement(sites); err != nil {
				t.Fatalf("a synchronized advance disagrees: %v", err)
			}
			if sites["ci.yml CORVINT_PR_FORWARD_EPOCH"][0] != next {
				t.Fatalf("advance not applied: %v", sites)
			}
			since, err := epochSince(advanced(c.since))
			if err != nil {
				t.Fatal(err)
			}
			if got := forwardEpochSinceProblem(next, since); (got == nil) != c.ok {
				t.Fatalf("epoch %s since %s: %v, want ok=%t", next, since, got, c.ok)
			}
		})
	}
	if forwardEpochSinceProblem(live, "2026-10-30") == nil {
		t.Fatal("a date without a time passed")
	}
}
