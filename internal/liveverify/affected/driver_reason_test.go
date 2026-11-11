package affected

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

// memorySource serves files from memory.
func memorySource(t *testing.T, files map[string]string) *Source {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body), Mode: 0o644}
	}
	return FSSource(fsys)
}

// driverComputedReasons returns, for each computed driver site of source,
// the sorted reasons it is one (AFP-V0-9620).
func driverComputedReasons(t *testing.T, source *Source) map[string][]string {
	t.Helper()
	reasons := map[string][]string{}
	result, err := driverUnitsSource(source, reasons)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range result.Units {
		_, recorded := reasons[unit.ID]
		if computed := slices.Contains(unit.Frontier, FrontierDriverComputedInvocation); computed != recorded {
			t.Fatalf("%s: computed=%v but reasons recorded=%v", unit.ID, computed, recorded)
		}
	}
	return reasons
}

// AFP-V0-9620: every computed site names at least one reason, and the
// recorder changes no unit.
func TestDriverComputedReasons_AFP_V0_9620(t *testing.T) {
	files := map[string]string{
		"go.mod":   "module example.test/m\n",
		"Makefile": "check:\n\tgo test ./...\nrun:\n\teval \"$$X\"\n",
		".github/workflows/ci.yml": "jobs:\n  plain:\n    runs-on: x\n    steps:\n      - run: go test ./...\n" +
			"  dynamic:\n    runs-on: x\n    steps:\n      - run: $TOOL check\n",
	}
	source := memorySource(t, files)
	reasons := driverComputedReasons(t, source)
	want := map[string][]string{
		"driver:make:Makefile#run":                   {"shell:eval"},
		"driver:ci:.github/workflows/ci.yml#dynamic": {"shell:computed-command-word"},
	}
	if fmt.Sprint(reasons) != fmt.Sprint(want) {
		t.Fatalf("reasons = %v, want %v", reasons, want)
	}
	plain, err := Driver{}.UnitsSource(memorySource(t, files))
	if err != nil {
		t.Fatal(err)
	}
	recorded, err := driverUnitsSource(memorySource(t, files), map[string][]string{})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%+v", plain) != fmt.Sprintf("%+v", recorded) {
		t.Fatal("recording reasons changed the driver units")
	}
}

// TestDriverComputedReasonCounts prints the per-reason count of computed
// driver sites of the checkout CORVINT_DRIVER_REASONS_ROOT names: the count
// of sites with each reason, and of sites for which it is the sole reason.
// It is a measurement aid (AFP-V0-9620) and skips without that variable.
func TestDriverComputedReasonCounts(t *testing.T) {
	root := os.Getenv("CORVINT_DRIVER_REASONS_ROOT")
	if root == "" {
		t.Skip("CORVINT_DRIVER_REASONS_ROOT is unset")
	}
	reasons := driverComputedReasons(t, DiskSource(root))
	type count struct{ make, ci, action, sole int }
	counts := map[string]*count{}
	sites := map[string]int{}
	for id, list := range reasons {
		kind := strings.SplitN(id, ":", 3)[1]
		sites[kind]++
		for _, reason := range list {
			entry := counts[reason]
			if entry == nil {
				entry = &count{}
				counts[reason] = entry
			}
			switch kind {
			case "make":
				entry.make++
			case "ci":
				entry.ci++
			default:
				entry.action++
			}
			if len(list) == 1 {
				entry.sole++
			}
		}
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	var out strings.Builder
	fmt.Fprintf(&out, "computed sites: make=%d ci=%d action=%d\n", sites["make"], sites["ci"], sites["action"])
	fmt.Fprintf(&out, "| reason | make | ci | action | sole |\n|---|---|---|---|---|\n")
	for _, name := range names {
		entry := counts[name]
		fmt.Fprintf(&out, "| %s | %d | %d | %d | %d |\n", name, entry.make, entry.ci, entry.action, entry.sole)
	}
	if os.Getenv("CORVINT_DRIVER_REASONS_SITES") != "" {
		ids := make([]string, 0, len(reasons))
		for id := range reasons {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			fmt.Fprintf(&out, "%s %s\n", id, strings.Join(reasons[id], ","))
		}
	}
	t.Log("\n" + out.String())
}

// AZU-V0-611: an evaluated, uncertified make site names the variable or
// function whose evaluation made it one and the command word, operand or
// form the scan could not bound, beside the AFP-V0-9620 category; a site
// computed for another reason, and a certified target, record nothing new.
func TestDriverEvaluatedUncertifiedReasons_AZU_V0_611(t *testing.T) {
	files := map[string]string{
		"go.mod": "module example.test/m\n",
		"Makefile": "MODE = ci\nMODE = local\n" +
			"check:\n\tscript/audit.sh tools/$(MODE)\n" +
			"redirect:\n\t$(strip wc -l) < $(MODE).txt\n" +
			"literal:\n\tscript/audit.sh tools\n" +
			"plain:\n\t$(if $(CI),go test,go vet) ./core/...\n",
		"script/audit.sh":     "#!/bin/sh\nls \"$1\"\n",
		"tools/local/main.go": "package main\n\nfunc main() {}\n",
		"core/core.go":        "package core\n",
	}
	reasons := driverComputedReasons(t, memorySource(t, files))
	want := map[string][]string{
		// A reassigned recursive variable leaves only its last value, so
		// each site names the one command or operand that value runs.
		"driver:make:Makefile#check":    {"make:evaluated-uncertified", "make:evaluated:MODE", "make:uncertified:shell:uncertified-command:script/audit.sh"},
		"driver:make:Makefile#redirect": {"make:evaluated-uncertified", "make:evaluated:$(strip)", "make:evaluated:MODE", "make:uncertified:shell:redirect-input:local.txt"},
	}
	if fmt.Sprint(reasons) != fmt.Sprint(want) {
		t.Fatalf("reasons = %v, want %v", reasons, want)
	}
}
