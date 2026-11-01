// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
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
