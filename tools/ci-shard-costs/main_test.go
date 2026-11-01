package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/.github/cishards"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

const revision = "0123456789abcdef0123456789abcdef01234567"
const runURL = "https://github.com/beamfall/corvint/actions/runs/1"

func write(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// AFP-V0-022: costs come from retained hosted package outcomes, with the source recorded.
// partitionGoVersion is the source toolchain the fixture tables record; the
// partition admits it, go1.27.2 (.github/cishards, decision 0522) and the
// go1.27.3 pin (decision 0557). Tests
// refresh as that toolchain unless they exercise another one.
const partitionGoVersion = "go1.27.1"

func TestMain(m *testing.M) {
	sourceGoVersion = partitionGoVersion
	os.Exit(m.Run())
}

// TestAFPV0022RefreshRecordsTheSourceToolchain proves refresh records the
// toolchain it runs under, succeeds under each admitted one (go1.27.1, go1.27.2
// and the pinned go1.27.3), and refuses, without writing, one the partition does not
// admit.
func TestAFPV0022RefreshRecordsTheSourceToolchain(t *testing.T) {
	logs := write(t, "raw.log", passed("example.org/a", "0.5"))
	t.Cleanup(func() { sourceGoVersion = partitionGoVersion })
	refresh := func(version string) (string, int, error) {
		table := filepath.Join(t.TempDir(), "costs.json")
		sourceGoVersion = version
		code, err := run("refresh", []string{"--table", table, "--shards", "1", "--revision", revision, "--run-url", runURL, logs}, nil)
		return table, code, err
	}
	table, code, err := refresh("go1.27.99")
	if code != 2 || err == nil || !strings.Contains(err.Error(), "source toolchain go1.27.99") {
		t.Fatalf("unadmitted toolchain: code=%d err=%v", code, err)
	}
	if _, err := os.Stat(table); !os.IsNotExist(err) {
		t.Fatalf("refused refresh wrote the table: %v", err)
	}
	for _, version := range []string{"go1.27.1", "go1.27.2", "go1.27.3"} {
		table, code, err := refresh(version)
		if code != 0 || err != nil {
			t.Fatalf("admitted toolchain %s: code=%d err=%v", version, code, err)
		}
		if got, _ := os.ReadFile(table); !strings.Contains(string(got), `"goVersion": "`+version+`"`) {
			t.Fatalf("source goVersion %s not recorded:\n%s", version, got)
		}
	}
}

func TestAFPV0022RefreshFromHostedOutcomes(t *testing.T) {
	raw := write(t, "raw.log", `{"Action":"start","Package":"example.org/a"}
{"Action":"pass","Package":"example.org/a","Test":"TestA","Elapsed":9}
{"Action":"output","Package":"example.org/a","Output":"ok\n"}
{"Action":"pass","Package":"example.org/a","Elapsed":953.5}
`)
	hosted := write(t, "hosted.log", passed("example.org/b", "0.0004")+
		"{\"Action\":\"start\",\"Package\":\"example.org/none\"}\n"+
		"{\"Action\":\"skip\",\"Package\":\"example.org/none\",\"Elapsed\":0}\n")
	table := filepath.Join(t.TempDir(), "costs.json")
	if code, err := run("refresh", []string{"--table", table, "--shards", "2", "--revision", revision, "--run-url", runURL, raw, hosted}, nil); code != 0 || err != nil {
		t.Fatalf("refresh code=%d err=%v", code, err)
	}
	got, err := os.ReadFile(table)
	if err != nil {
		t.Fatal(err)
	}
	costs, ok := cishards.Costs(got)
	if !ok || len(costs) != 3 || costs["example.org/a"] != 953500 || costs["example.org/b"] != 1 || costs["example.org/none"] != 1 {
		t.Fatalf("costs=%v ok=%v", costs, ok)
	}
	if !strings.Contains(string(got), revision) || !strings.Contains(string(got), runURL) {
		t.Fatal("source run not recorded")
	}
	// A subset of one run's logs must not silently replace a complete table.
	if code, err := run("refresh", []string{"--table", table, "--shards", "1", "--revision", revision, "--run-url", runURL, raw}, nil); code != 2 || err == nil || !strings.Contains(err.Error(), "stale example.org/b") {
		t.Fatalf("partial refresh code=%d err=%v", code, err)
	}
	if after, _ := os.ReadFile(table); !bytes.Equal(after, got) {
		t.Fatal("refused partial refresh changed the table")
	}
	if code, err := run("refresh", []string{"--table", table, "--shards", "2", "--allow-removed", "--revision", revision, "--run-url", runURL, raw, hosted}, nil); code != 0 || err != nil {
		t.Fatalf("allowed refresh code=%d err=%v", code, err)
	}
	var out bytes.Buffer
	if code, err := run("check", []string{"--table", table, "--shards", "2", raw, hosted}, &out); code != 0 || err != nil || out.Len() != 0 {
		t.Fatalf("fresh table drifts: code=%d err=%v out=%s", code, err, out.String())
	}
}

func TestAFPV0022RefreshRefusesUnusableInput(t *testing.T) {
	pass := `{"Action":"pass","Package":"example.org/a","Elapsed":1}` + "\n"
	for name, tc := range map[string]struct{ log, revision, url string }{
		"failed-package":   {`{"Action":"fail","Package":"example.org/a","Elapsed":1}` + "\n", revision, runURL},
		"repeated-package": {pass + pass, revision, runURL},
		"no-outcome":       {`{"Action":"pass","Package":"example.org/a","Test":"TestA","Elapsed":1}` + "\n", revision, runURL},
		"short-revision":   {pass, "0123", runURL},
		"foreign-run":      {pass, revision, "https://example.org/runs/1"},
	} {
		t.Run(name, func(t *testing.T) {
			table := filepath.Join(t.TempDir(), "costs.json")
			code, err := run("refresh", []string{"--table", table, "--shards", "1", "--revision", tc.revision, "--run-url", tc.url, write(t, "log", tc.log)}, nil)
			if code != 2 || err == nil {
				t.Fatalf("code=%d err=%v", code, err)
			}
			if _, statErr := os.Stat(table); statErr == nil {
				t.Fatal("refused refresh wrote a table")
			}
		})
	}
}

func TestAFPV0022DriftCheck(t *testing.T) {
	table := write(t, "costs.json", `{"profile":"corvint-ci-package-costs/0","source":{"revision":"`+revision+`","runURL":"`+runURL+`","goVersion":"go1.27.1"},"milliseconds":{"example.org/slow":3220,"example.org/fast":910000,"example.org/same":100000,"example.org/tiny":10,"example.org/gone":5000}}`)
	log := write(t, "log", passed("example.org/slow", "953.5")+passed("example.org/fast", "98")+passed("example.org/same", "199")+passed("example.org/tiny", "9")+passed("example.org/new", "144"))
	var out bytes.Buffer
	code, err := run("check", []string{"--table", table, "--shards", "1", log}, &out)
	want := `drift example.org/fast recorded=910000ms observed=98000ms
drift example.org/slow recorded=3220ms observed=953500ms
missing example.org/new observed=144000ms
stale example.org/gone recorded=5000ms
`
	if code != 1 || err != nil || out.String() != want {
		t.Fatalf("code=%d err=%v out:\n%s", code, err, out.String())
	}
	out.Reset()
	if code, _ := run("check", []string{"--table", table, "--shards", "1", "--factor", "1.5", "--floor", "1s", log}, &out); code != 1 || !strings.Contains(out.String(), "drift example.org/same") || !strings.Contains(out.String(), "drift example.org/tiny") {
		t.Fatalf("stated bounds not applied: code=%d out:\n%s", code, out.String())
	}
	for _, factor := range []string{"NaN", "+Inf", "0.5"} {
		if code, err := run("check", []string{"--table", table, "--shards", "1", "--factor", factor, log}, &out); code != 2 || err == nil {
			t.Fatalf("factor %s code=%d err=%v", factor, code, err)
		}
	}
	if code, err := run("check", []string{"--table", write(t, "bad.json", `{"profile":"bad"}`), "--shards", "1", log}, &out); code != 2 || err == nil {
		t.Fatalf("invalid table code=%d err=%v", code, err)
	}
}

// AFP-V0-040: the advisory CI report exits 0 on findings, marks what misplaces a
// stated share of the ideal shard, and bounds its warning annotations.
func TestAFPV0040AdvisoryReportNeverFailsOnFindings(t *testing.T) {
	// Ideal shard: (1000+400+500+100+10)s / 2 = 1005s, so 10% is 100.5s.
	table := write(t, "costs.json", `{"profile":"corvint-ci-package-costs/0","source":{"revision":"`+revision+`","runURL":"`+runURL+`","goVersion":"go1.27.1"},"milliseconds":{"example.org/big":600000,"example.org/near":300000,"example.org/small":3000,"example.org/gone":900000}}`)
	shard0 := write(t, "shard-0.json", passed("example.org/big", "1000")+passed("example.org/near", "400"))
	shard1 := write(t, "shard-1.json", passed("example.org/new", "500")+passed("example.org/small", "100")+passed("example.org/tiny", "10"))
	summary := write(t, "summary.md", "earlier step\n")
	var out bytes.Buffer
	code, err := run("check", []string{"--table", table, "--advisory", "--shards", "2", "--summary", summary, shard0, shard1}, &out)
	if code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	// near drifts by 100s (ratio 1.33, under 10%): not reported. big drifts by 400s within
	// factor 2 but over 10%: material. new is a material missing package; small drifts beyond
	// factor 2 but is not material; tiny is a minor missing package; gone is stale, never material.
	// Shard 0 costs 1400s against a 900s prediction, 395s over the ideal shard: stale (AFP-V0-604).
	wantOut := "::warning title=CI shard balance stale::shard 0 observed 1400.0s against 900.0s predicted, 39%25 over the ideal shard 1005.0s; refresh " + table + " (AFP-V0-604)\n" +
		"::warning title=CI shard cost drift::missing example.org/new observed=500000ms misplaces 500.0s (50%25 of the ideal shard); refresh " + table + " (AFP-V0-040)\n" +
		"::warning title=CI shard cost drift::drift example.org/big recorded=600000ms observed=1000000ms misplaces 400.0s (40%25 of the ideal shard); refresh " + table + " (AFP-V0-040)\n" +
		"::warning title=CI shard cost drift::3 further findings (1 drift, 1 missing, 1 stale); see the job summary (AFP-V0-040)\n"
	if out.String() != wantOut {
		t.Fatalf("annotations:\n%s", out.String())
	}
	got, _ := os.ReadFile(summary)
	for _, want := range []string{
		"earlier step\n### CI shard cost table drift (advisory, AFP-V0-040)\n",
		"2 shard logs, 5 packages, 2010.0s observed; ideal shard 1005.0s. A finding is material when it misplaces at least 10% of the ideal shard (100.5s).",
		"| **missing** (material) | `example.org/new` | - | 500.0s | 500.0s | 49.8% |\n| **drift** (material) | `example.org/big` | 600.0s | 1000.0s | 400.0s | 39.8% |\n| drift | `example.org/small` | 3.0s | 100.0s | 97.0s | 9.7% |\n| missing | `example.org/tiny` | - | 10.0s | 10.0s | 1.0% |\n| stale | `example.org/gone` | 900.0s | - | 0.0s | 0.0% |\n",
	} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("summary lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(string(got), "example.org/near") {
		t.Fatalf("immaterial drift within factor reported:\n%s", got)
	}

	// A stated share applies; a clean table reports no finding.
	out.Reset()
	if code, _ := run("check", []string{"--table", table, "--advisory", "--shards", "2", "--share", "50", shard0, shard1}, &out); code != 0 || strings.Count(out.String(), "::warning") != 1 || strings.Contains(out.String(), "balance stale") {
		t.Fatalf("share 50: code=%d out:\n%s", code, out.String())
	}
	clean := filepath.Join(t.TempDir(), "clean.json")
	if code, err := run("refresh", []string{"--table", clean, "--shards", "2", "--revision", revision, "--run-url", runURL, shard0, shard1}, nil); code != 0 || err != nil {
		t.Fatal(code, err)
	}
	out.Reset()
	cleanSummary := filepath.Join(t.TempDir(), "clean.md")
	if code, err := run("check", []string{"--table", clean, "--advisory", "--shards", "2", "--summary", cleanSummary, shard0, shard1}, &out); code != 0 || err != nil || out.Len() != 0 {
		t.Fatalf("clean: code=%d err=%v out=%s", code, err, out.String())
	}
	if got, _ := os.ReadFile(cleanSummary); !strings.Contains(string(got), "No drift, missing or stale package.") {
		t.Fatalf("clean summary:\n%s", got)
	}
}

func TestAFPV0040AdvisoryWarningsStayWithinTheStepLimit(t *testing.T) {
	var table, log strings.Builder
	table.WriteString(`{"profile":"corvint-ci-package-costs/0","source":{"revision":"` + revision + `","runURL":"` + runURL + `","goVersion":"go1.27.1"},"milliseconds":{"example.org/kept":1000}}`)
	log.WriteString(passed("example.org/kept", "1"))
	for i := range 12 {
		log.WriteString(passed("example.org/new"+string(rune('a'+i)), "100"))
	}
	var out bytes.Buffer
	if code, err := run("check", []string{"--table", write(t, "costs.json", table.String()), "--advisory", "--shards", "1", "--share", "5", write(t, "log", log.String())}, &out); code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if n := strings.Count(out.String(), "::warning"); n != maxWarnings || !strings.Contains(out.String(), "3 further findings (0 drift, 3 missing, 0 stale)") {
		t.Fatalf("%d warnings:\n%s", n, out.String())
	}
}

// AFP-V0-040: strict validation admits every event a passing `go test -json` run emits.
func TestAFPV0040AdvisoryAcceptsAFullEventStream(t *testing.T) {
	table := write(t, "costs.json", `{"profile":"corvint-ci-package-costs/0","source":{"revision":"`+revision+`","runURL":"`+runURL+`","goVersion":"go1.27.1"},"milliseconds":{"example.org/a":1000,"example.org/b":1}}`)
	log := write(t, "shard-0.json", `{"ImportPath":"example.org/a","Action":"build-output","Output":"# example.org/a\n"}
{"Time":"2026-10-10T13:19:03Z","Action":"start","Package":"example.org/a"}

{"Action":"run","Package":"example.org/a","Test":"TestA"}
{"Action":"output","Package":"example.org/a","Test":"TestA","Output":"=== RUN   TestA\n","OutputType":"frame"}
{"Action":"attr","Package":"example.org/a","Test":"TestA","Key":"k","Value":"v"}
{"Action":"artifacts","Package":"example.org/a","Test":"TestA","Path":"/tmp/artifacts/TestA"}
{"Action":"pause","Package":"example.org/a","Test":"TestA"}
{"Action":"start","Package":"example.org/b"}
{"Action":"cont","Package":"example.org/a","Test":"TestA"}
{"Action":"bench","Package":"example.org/a","Test":"BenchmarkA","Output":"x"}
{"Action":"skip","Package":"example.org/a","Test":"TestSkipped","Elapsed":0}
{"Action":"pass","Package":"example.org/a","Test":"TestA","Elapsed":1}
{"Action":"output","Package":"example.org/a","Output":"ok\n"}
{"Action":"pass","Package":"example.org/a","Elapsed":1}
{"Action":"output","Package":"example.org/b","Output":"?   \texample.org/b\t[no test files]\n"}
{"Action":"skip","Package":"example.org/b","Elapsed":0}
`)
	var out bytes.Buffer
	if code, err := run("check", []string{"--table", table, "--advisory", "--shards", "1", log}, &out); code != 0 || err != nil || out.Len() != 0 {
		t.Fatalf("code=%d err=%v out=%s", code, err, out.String())
	}
}

// AFP-V0-040: input that cannot stand for one complete run abstains with its reason.
func TestAFPV0040AdvisoryAbstainsOnPartialOrUnusableInput(t *testing.T) {
	table := write(t, "costs.json", `{"profile":"corvint-ci-package-costs/0","source":{"revision":"`+revision+`","runURL":"`+runURL+`","goVersion":"go1.27.1"},"milliseconds":{"example.org/a":1000}}`)
	pass := write(t, "shard-0.json", passed("example.org/a", "900"))
	b := passed("example.org/b", "1")
	// The platform's own not-exist text: Windows says "The system cannot find
	// the file specified." rather than "no such file" (V1-2488).
	missing := filepath.Join(t.TempDir(), "shard-1.json")
	notExist := "no such file"
	var pathErr *fs.PathError
	if _, err := os.Open(missing); errors.As(err, &pathErr) {
		notExist = pathErr.Err.Error()
	}
	for name, tc := range map[string]struct {
		args   []string
		reason string
	}{
		"partial-run":      {[]string{"--table", table, pass}, "1 of 2 shard logs present"},
		"missing-artifact": {[]string{"--table", table, pass, missing}, notExist},
		"failed-package":   {[]string{"--table", table, pass, write(t, "shard-1.json", `{"Action":"fail","Package":"example.org/b","Elapsed":1}`+"\n")}, "package example.org/b failed"},
		"invalid-table":    {[]string{"--table", write(t, "bad.json", `{"profile":"bad"}`), pass, write(t, "shard-1.json", b)}, "cost table is invalid"},
		// Each of these has a valid table and a valid other shard, so only the stream decides.
		"empty-log":        {[]string{"--table", table, pass, write(t, "shard-1.json", "")}, "shard-1.json: no terminal package outcome"},
		"malformed-record": {[]string{"--table", table, pass, write(t, "shard-1.json", b+`{"Action":"output","Package":"example.org/c","Out`+"\n")}, "shard-1.json: line 3 is not a go test -json event"},
		"prefixed-record":  {[]string{"--table", table, pass, write(t, "shard-1.json", `2026-10-10T13:19:03Z {"Action":"pass","Package":"example.org/b","Elapsed":1}`+"\n")}, "shard-1.json: line 1 is not a go test -json event"},
		"unfinished":       {[]string{"--table", table, pass, write(t, "shard-1.json", `{"Action":"start","Package":"example.org/b"}`+"\n"+`{"Action":"start","Package":"example.org/c"}`+"\n"+`{"Action":"pass","Package":"example.org/c","Elapsed":1}`+"\n")}, "shard-1.json: package example.org/b started without a terminal outcome"},
		// Syntactically valid JSON that is not a go test -json event of a passing run.
		"empty-object":      {[]string{"--table", table, pass, write(t, "shard-1.json", b+"{}\n")}, `shard-1.json: line 3 has unknown Action ""`},
		"unknown-action":    {[]string{"--table", table, pass, write(t, "shard-1.json", b+`{"Action":"finish","Package":"example.org/b"}`+"\n")}, `line 3 has unknown Action "finish"`},
		"fail-no-package":   {[]string{"--table", table, pass, write(t, "shard-1.json", b+`{"Action":"fail"}`+"\n")}, "shard-1.json: line 3: fail event without Package"},
		"output-no-package": {[]string{"--table", table, pass, write(t, "shard-1.json", `{"Action":"output","Output":"ok\n"}`+"\n"+b)}, "line 1: output event without Package"},
		"test-failed":       {[]string{"--table", table, pass, write(t, "shard-1.json", `{"Action":"start","Package":"example.org/b"}`+"\n"+`{"Action":"fail","Package":"example.org/b","Test":"TestX","Elapsed":1}`+"\n"+`{"Action":"pass","Package":"example.org/b","Elapsed":1}`+"\n")}, "test TestX of package example.org/b failed"},
		"build-failed":      {[]string{"--table", table, pass, write(t, "shard-1.json", `{"ImportPath":"example.org/c [example.org/c.test]","Action":"build-fail"}`+"\n"+b)}, "build of example.org/c [example.org/c.test] failed"},
		"build-no-path":     {[]string{"--table", table, pass, write(t, "shard-1.json", `{"Action":"build-output","Output":"x"}`+"\n"+b)}, "line 1: build-output event without ImportPath"},
		"outcome-no-start":  {[]string{"--table", table, pass, write(t, "shard-1.json", `{"Action":"skip","Package":"example.org/b","Elapsed":0}`+"\n")}, "package example.org/b has a terminal outcome without a start"},
		"started-twice":     {[]string{"--table", table, pass, write(t, "shard-1.json", `{"Action":"start","Package":"example.org/b"}`+"\n"+b)}, "package example.org/b started twice"},
	} {
		t.Run(name, func(t *testing.T) {
			summary := filepath.Join(t.TempDir(), "summary.md")
			var out bytes.Buffer
			code, err := run("check", append([]string{"--advisory", "--shards", "2", "--summary", summary}, tc.args...), &out)
			got, _ := os.ReadFile(summary)
			if code != 2 || err == nil || out.Len() != 0 || !strings.Contains(string(got), "Abstained: ") || !strings.Contains(string(got), tc.reason) {
				t.Fatalf("code=%d err=%v out=%s summary:\n%s", code, err, out.String(), got)
			}
		})
	}
	for _, args := range [][]string{
		{"--advisory", "--shards", "0", pass},
		{"--advisory", "--shards", "1", "--share", "0", pass},
		{"--advisory", "--shards", "1", "--share", "101", pass},
	} {
		if code, err := run("check", append([]string{"--table", table}, args...), nil); code != 2 || err == nil {
			t.Fatalf("%v: code=%d err=%v", args, code, err)
		}
	}
	if code, err := run("refresh", []string{"--table", table, "--advisory", "--shards", "1", pass}, nil); code != 2 || err == nil {
		t.Fatalf("advisory refresh: code=%d err=%v", code, err)
	}
	if got := escape("50% done\r\nnext"); got != "50%25 done%0D%0Anext" {
		t.Fatalf("escape=%q", got)
	}
}

// passed is one finished package in a raw `go test -json` stream.
func passed(pkg, elapsed string) string {
	return `{"Action":"start","Package":"` + pkg + `"}` + "\n" + `{"Action":"pass","Package":"` + pkg + `","Elapsed":` + elapsed + `}` + "\n"
}

// sliceInputs writes an allow-list that admits example.org/split, a slice file
// that splits it into one named slice and the catch-all and also lists
// example.org/listed, which the allow-list does not admit, and a slice file that
// splits it into three slices.
func sliceInputs(t *testing.T) (allow, slices, slices3 string) {
	t.Helper()
	allow = write(t, "allow.json", `{"profile":"corvint-ci-test-split-allow/0","packages":{"example.org/split":"no test depends on another's order or state"}}`)
	source := `{"profile":"corvint-ci-test-slices/0","source":{"revision":"` + revision + `","runURL":"` + runURL + `","goVersion":"go1.27.1"},"packages":`
	slices = write(t, "slices.json", source+`{"example.org/split":{"named":[{"milliseconds":300000,"tests":["TestA"]}],"restMilliseconds":200000},"example.org/listed":{"named":[{"milliseconds":1000,"tests":["TestA"]}],"restMilliseconds":1000}}}`)
	slices3 = write(t, "slices3.json", source+`{"example.org/split":{"named":[{"milliseconds":300000,"tests":["TestA"]},{"milliseconds":100000,"tests":["TestB"]}],"restMilliseconds":100000}}}`)
	return allow, slices, slices3
}

// AFP-V0-041: refresh and check combine the test slices of a split package, one
// per log, into one cost, and refuse every other count, including a single
// outcome: a run made before the split, or a run that lost a slice's log, measures
// one slice. The declared shard count and the slice file, not the number of logs
// supplied, decide how many slices a package has, and every shard's log is needed.
func TestAFPV0041RefreshCombinesTestSlices(t *testing.T) {
	allow, slices, slices3 := sliceInputs(t)
	shard0 := write(t, "shard-0.log", passed("example.org/split", "300")+passed("example.org/a", "10"))
	shard1 := write(t, "shard-1.log", passed("example.org/split", "200.5")+passed("example.org/b", "20"))
	unsplit1 := write(t, "shard-1.log", passed("example.org/b", "20"))
	args := func(table, slices string, shards int, logs []string) []string {
		return append([]string{"--table", table, "--shards", strconv.Itoa(shards), "--allow", allow, "--slices", slices, "--revision", revision, "--run-url", runURL}, logs...)
	}
	good := filepath.Join(t.TempDir(), "costs.json")
	if code, err := run("refresh", args(good, slices, 2, []string{shard0, shard1}), nil); code != 0 || err != nil {
		t.Fatalf("sliced refresh code=%d err=%v", code, err)
	}
	got, _ := os.ReadFile(good)
	if costs, ok := cishards.Costs(got); !ok || len(costs) != 3 || costs["example.org/split"] != 500500 || costs["example.org/a"] != 10000 || costs["example.org/b"] != 20000 {
		t.Fatalf("costs=%v ok=%v", costs, ok)
	}
	var out bytes.Buffer
	if code, err := run("check", args(good, slices, 2, []string{shard0, shard1}), &out); code != 0 || err != nil || out.Len() != 0 {
		t.Fatalf("sliced check code=%d err=%v out=%s", code, err, out.String())
	}
	if code, err := run("refresh", append([]string{"--table", filepath.Join(t.TempDir(), "costs.json"), "--allow", allow, "--slices", slices, "--revision", revision, "--run-url", runURL}, shard0, shard1), nil); code != 2 || err == nil || !strings.Contains(err.Error(), "--shards N") {
		t.Fatalf("undeclared shards: code=%d err=%v", code, err)
	}
	for name, tc := range map[string]struct {
		slices string
		shards int // 0: one shard per log
		logs   []string
		reason string
	}{
		// One shard's log holds one slice: neither the log count nor a one-shard
		// declaration may turn it into the whole package.
		"only-one-log":      {slices, 2, []string{shard0}, "1 of 2 shard logs present"},
		"one-log-one-shard": {slices, 1, []string{shard0}, "package example.org/split is split into 2 test slices, more than --shards 1"},
		// Fewer logs than the slice file's three slices.
		"fewer-logs-than-slices":   {slices3, 3, []string{shard0, unsplit1}, "2 of 3 shard logs present"},
		"fewer-shards-than-slices": {slices3, 2, []string{shard0, unsplit1}, "package example.org/split is split into 3 test slices, more than --shards 2"},
		"pre-split-whole-run":      {slices, 0, []string{write(t, "shard-0.log", passed("example.org/split", "500")+passed("example.org/a", "10")), write(t, "shard-1.log", passed("example.org/b", "20"))}, "package example.org/split has 1 terminal outcomes for its 2 test slices"},
		"missing-second-slice":     {slices, 0, []string{shard0, write(t, "shard-1.log", passed("example.org/b", "20"))}, "package example.org/split has 1 terminal outcomes for its 2 test slices"},
		"no-slice-file":            {filepath.Join(t.TempDir(), "absent.json"), 0, []string{shard0, shard1}, "package example.org/split has two terminal outcomes"},
		"unsplit-twice":            {slices, 0, []string{shard0, write(t, "shard-1.log", passed("example.org/split", "200")+passed("example.org/a", "10"))}, "package example.org/a has two terminal outcomes"},
		"not-allowed":              {slices, 0, []string{write(t, "shard-0.log", passed("example.org/listed", "1")), write(t, "shard-1.log", passed("example.org/listed", "1"))}, "package example.org/listed has two terminal outcomes"},
		"extra-slice":              {slices, 0, []string{shard0, shard1, write(t, "shard-2.log", passed("example.org/split", "1"))}, "package example.org/split has 3 terminal outcomes for its 2 test slices"},
		"one-log-twice":            {slices, 0, []string{write(t, "shard-0.log", passed("example.org/split", "300")+passed("example.org/split", "200")), write(t, "shard-1.log", passed("example.org/b", "20"))}, "package example.org/split has two terminal outcomes"},
		"failed-slice":             {slices, 0, []string{shard0, write(t, "shard-1.log", `{"Action":"start","Package":"example.org/split"}`+"\n"+`{"Action":"fail","Package":"example.org/split","Elapsed":1}`+"\n")}, "package example.org/split failed"},
		"short-slice":              {slices, 0, []string{shard0, write(t, "shard-1.log", `{"Action":"start","Package":"example.org/split"}`+"\n"+passed("example.org/b", "20"))}, "package example.org/split started without a terminal outcome"},
	} {
		t.Run(name, func(t *testing.T) {
			shards := tc.shards
			if shards == 0 {
				shards = len(tc.logs)
			}
			table := filepath.Join(t.TempDir(), "costs.json")
			code, err := run("refresh", args(table, tc.slices, shards, tc.logs), nil)
			if code != 2 || err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("refresh code=%d err=%v", code, err)
			}
			if _, statErr := os.Stat(table); statErr == nil {
				t.Fatal("refused refresh wrote a table")
			}
			var out bytes.Buffer
			code, err = run("check", args(good, tc.slices, shards, tc.logs), &out)
			if code != 2 || err == nil || !strings.Contains(err.Error(), tc.reason) || out.Len() != 0 {
				t.Fatalf("check code=%d err=%v out=%s", code, err, out.String())
			}
		})
	}
}

// AFP-V0-041: the advisory drift report measures a split package as the sum of its
// slices and abstains on a failed, short or repeated slice like any other stream.
func TestAFPV0041AdvisoryCombinesTestSlices(t *testing.T) {
	allow, slices, slices3 := sliceInputs(t)
	tableOf := func(split string) string {
		return write(t, "costs.json", `{"profile":"corvint-ci-package-costs/0","source":{"revision":"`+revision+`","runURL":"`+runURL+`","goVersion":"go1.27.1"},"milliseconds":{"example.org/split":`+split+`,"example.org/a":10000,"example.org/b":20000}}`)
	}
	shard0 := write(t, "shard-0.json", passed("example.org/split", "300")+passed("example.org/a", "10"))
	shard1 := write(t, "shard-1.json", passed("example.org/split", "200.5")+passed("example.org/b", "20"))
	unsplit1 := write(t, "shard-1.json", passed("example.org/b", "20"))
	check := func(table, slices string, shards int, logs ...string) (int, string, string, error) {
		summary := filepath.Join(t.TempDir(), "summary.md")
		var out bytes.Buffer
		code, err := run("check", append([]string{"--advisory", "--shards", strconv.Itoa(shards), "--table", table, "--allow", allow, "--slices", slices, "--summary", summary}, logs...), &out)
		got, _ := os.ReadFile(summary)
		return code, out.String(), string(got), err
	}
	if code, out, summary, err := check(tableOf("500000"), slices, 2, shard0, shard1); code != 0 || err != nil || out != "" || !strings.Contains(summary, "2 shard logs, 3 packages, 530.5s observed") || !strings.Contains(summary, "No drift, missing or stale package.") {
		t.Fatalf("summed slices: code=%d err=%v out=%s summary:\n%s", code, err, out, summary)
	}
	// A table that holds one slice's time drifts by the other slice's.
	if code, out, _, err := check(tableOf("300000"), slices, 2, shard0, shard1); code != 0 || err != nil || !strings.Contains(out, "drift example.org/split recorded=300000ms observed=500500ms misplaces 200.5s") {
		t.Fatalf("one-slice table: code=%d err=%v out=%s", code, err, out)
	}
	for name, tc := range map[string]struct {
		slices string
		shards int // 0: one shard per log
		logs   []string
		reason string
	}{
		"only-one-log":             {slices, 2, []string{shard0}, "1 of 2 shard logs present"},
		"one-log-one-shard":        {slices, 1, []string{shard0}, "package example.org/split is split into 2 test slices, more than --shards 1"},
		"fewer-logs-than-slices":   {slices3, 3, []string{shard0, unsplit1}, "2 of 3 shard logs present"},
		"fewer-shards-than-slices": {slices3, 2, []string{shard0, unsplit1}, "package example.org/split is split into 3 test slices, more than --shards 2"},
		"no-slice-file":            {filepath.Join(t.TempDir(), "absent.json"), 0, []string{shard0, shard1}, "package example.org/split has two terminal outcomes"},
		"pre-split-whole-run":      {slices, 0, []string{write(t, "shard-0.json", passed("example.org/split", "500")+passed("example.org/a", "10")), write(t, "shard-1.json", passed("example.org/b", "20"))}, "package example.org/split has 1 terminal outcomes for its 2 test slices"},
		"missing-second-slice":     {slices, 0, []string{shard0, write(t, "shard-1.json", passed("example.org/b", "20"))}, "package example.org/split has 1 terminal outcomes for its 2 test slices"},
		"unsplit-twice":            {slices, 0, []string{shard0, write(t, "shard-1.json", passed("example.org/split", "200")+passed("example.org/a", "10"))}, "package example.org/a has two terminal outcomes"},
		"extra-slice":              {slices, 0, []string{shard0, shard1, write(t, "shard-2.json", passed("example.org/split", "1"))}, "package example.org/split has 3 terminal outcomes for its 2 test slices"},
		"test-failed":              {slices, 0, []string{shard0, write(t, "shard-1.json", `{"Action":"start","Package":"example.org/split"}`+"\n"+`{"Action":"fail","Package":"example.org/split","Test":"TestB","Elapsed":1}`+"\n"+`{"Action":"pass","Package":"example.org/split","Elapsed":1}`+"\n")}, "shard-1.json: test TestB of package example.org/split failed"},
		"unfinished":               {slices, 0, []string{shard0, write(t, "shard-1.json", `{"Action":"start","Package":"example.org/split"}`+"\n"+passed("example.org/b", "20"))}, "shard-1.json: package example.org/split started without a terminal outcome"},
	} {
		t.Run(name, func(t *testing.T) {
			shards := tc.shards
			if shards == 0 {
				shards = len(tc.logs)
			}
			code, out, summary, err := check(tableOf("500000"), tc.slices, shards, tc.logs...)
			if code != 2 || err == nil || out != "" || !strings.Contains(summary, "Abstained: ") || !strings.Contains(summary, tc.reason) {
				t.Fatalf("code=%d err=%v out=%s summary:\n%s", code, err, out, summary)
			}
		})
	}
}

// AFP-V0-043: both table-producing refresh and plain check use the strict
// completed-stream reader, and refusal preserves an existing table.
func assertStrictTableInputRefused(t *testing.T, suffix, reason string) {
	t.Helper()
	for _, mode := range []string{"refresh", "check"} {
		t.Run(mode, func(t *testing.T) {
			old, err := encode(map[string]int64{"example.org/a": 1000}, revision, runURL)
			if err != nil {
				t.Fatal(err)
			}
			table := write(t, "costs.json", string(old))
			log := write(t, "shard.json", passed("example.org/a", "1")+suffix)
			args := []string{"--table", table, "--shards", "1", "--revision", revision, "--run-url", runURL, log}
			var out bytes.Buffer
			code, err := run(mode, args, &out)
			if code != 2 || err == nil || !strings.Contains(err.Error(), reason) {
				t.Fatalf("code=%d err=%v; want refusal %q", code, err, reason)
			}
			after, err := os.ReadFile(table)
			if err != nil || !bytes.Equal(after, old) {
				t.Fatalf("refusal changed table: %v", err)
			}
		})
	}
}

func TestAFPV0043RefreshRefusesMalformedLine(t *testing.T) {
	assertStrictTableInputRefused(t, "not JSON\n", "not a go test -json event")
}

func TestAFPV0043RefreshRefusesTruncatedFinalEvent(t *testing.T) {
	assertStrictTableInputRefused(t, `{"Action":"start","Package":`, "not a go test -json event")
}

func TestAFPV0043RefreshRefusesUnfinishedPackage(t *testing.T) {
	assertStrictTableInputRefused(t, `{"Action":"start","Package":"example.org/b"}`+"\n", "started without a terminal outcome")
}

func TestAFPV0043RefreshRefusesEmptyShard(t *testing.T) {
	for _, mode := range []string{"refresh", "check"} {
		table := write(t, "costs.json", "retained table")
		logs := []string{write(t, "complete.json", passed("example.org/a", "1")), write(t, "empty.json", "\n")}
		var out bytes.Buffer
		code, err := run(mode, append([]string{"--table", table, "--shards", "2", "--revision", revision, "--run-url", runURL}, logs...), &out)
		if code != 2 || err == nil || !strings.Contains(err.Error(), "no terminal package outcome") {
			t.Fatalf("%s code=%d err=%v", mode, code, err)
		}
		after, err := os.ReadFile(table)
		if err != nil || string(after) != "retained table" {
			t.Fatalf("table changed: %v", err)
		}
	}
}

func TestAFPV0043RefreshValidStreamByteParity(t *testing.T) {
	logs := []string{
		passed("example.org/a", "953.5"),
		passed("example.org/b", "0.0004") +
			"{\"Action\":\"start\",\"Package\":\"example.org/none\"}\n" +
			"{\"Action\":\"skip\",\"Package\":\"example.org/none\",\"Elapsed\":0}\n",
	}
	old := newOutcomes()
	paths := []string{}
	for _, log := range logs {
		if err := observe(strings.NewReader(log), old); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, write(t, "shard.json", log))
	}
	// Preserve the old non-strict path's semantics, using the same unsplit inputs.
	allow, slices := write(t, "allow.json", "{}"), write(t, "slices.json", "{}")
	costs, err := combine(old, 2, allow, slices)
	if err != nil {
		t.Fatal(err)
	}
	want, err := encode(costs, revision, runURL)
	if err != nil {
		t.Fatal(err)
	}
	table := filepath.Join(t.TempDir(), "costs.json")
	args := []string{"--table", table, "--shards", "2", "--allow", allow, "--slices", slices, "--revision", revision, "--run-url", runURL}
	if code, err := run("refresh", append(args, paths...), nil); code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	got, err := os.ReadFile(table)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("table parity: err=%v\ngot %s\nwant %s", err, got, want)
	}
}

// STR-V0-056: refresh --history retains each refreshed table byte for byte
// under its UTC time and revision, keeps only the newest --keep retained
// names, never touches another file, writes nothing when refused, and the
// retained files plan through `corvint test shards` with no fallback.
func TestSTRV0056RefreshRetainsDatedTables(t *testing.T) {
	logs := write(t, "raw.log", passed("example.org/a", "0.5")+passed("example.org/b", "1.25"))
	dir := t.TempDir()
	history := filepath.Join(dir, "history")
	table := filepath.Join(dir, "costs.json")
	if err := os.MkdirAll(history, 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := map[string]string{"notes.txt": "keep", "20200101T000000Z-" + strings.Repeat("0", 40) + ".json.bak": "keep"}
	for name, body := range foreign {
		if err := os.WriteFile(filepath.Join(history, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	refresh := func(extra ...string) (int, error) {
		args := append([]string{"--table", table, "--shards", "1", "--run-url", runURL, "--history", history}, extra...)
		return run("refresh", append(args, logs), nil)
	}
	revisions := []string{strings.Repeat("1", 40), strings.Repeat("2", 40), strings.Repeat("3", 40)}
	times := []string{"2026-10-15T09:00:00Z", "2026-10-16T09:00:00Z", "2026-10-16T21:30:05Z"}
	names := []string{"20261015T090000Z-", "20261016T090000Z-", "20261016T213005Z-"}
	for i, rev := range revisions {
		if code, err := refresh("--revision", rev, "--at", times[i], "--keep", "2"); code != 0 || err != nil {
			t.Fatalf("refresh %d: code=%d err=%v", i, code, err)
		}
		tableBytes, err := os.ReadFile(table)
		if err != nil {
			t.Fatal(err)
		}
		retained, err := os.ReadFile(filepath.Join(history, names[i]+rev+".json"))
		if err != nil || !bytes.Equal(retained, tableBytes) {
			t.Fatalf("refresh %d: retained table differs from the table: %v", i, err)
		}
	}
	entries, err := os.ReadDir(history)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, entry := range entries {
		got = append(got, entry.Name())
	}
	want := []string{names[1] + revisions[1] + ".json", names[2] + revisions[2] + ".json"}
	for name := range foreign {
		want = append(want, name)
	}
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("retained %v, want %v", got, want)
	}
	for name, body := range foreign {
		if raw, err := os.ReadFile(filepath.Join(history, name)); err != nil || string(raw) != body {
			t.Fatalf("foreign file %s changed: %v", name, err)
		}
	}
	// The retained files are what `corvint test shards --durations` reads,
	// oldest first in name order.
	tables := []sessiontest.History{}
	for _, name := range []string{names[1] + revisions[1] + ".json", names[2] + revisions[2] + ".json"} {
		raw, err := os.ReadFile(filepath.Join(history, name))
		if err != nil {
			t.Fatal(err)
		}
		tables = append(tables, sessiontest.History{Name: name, Raw: raw})
	}
	listed, err := sessiontest.ListedFiles([]byte("example.org/a\nexample.org/b\n"))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := sessiontest.PlanShards(listed, 2, sessiontest.DefaultHistoryRuns, tables)
	if err != nil || plan.Fallback != "" || strings.Join(plan.History, ",") != revisions[1]+","+revisions[2] || plan.WeightMilliseconds["example.org/b"] != 1250 {
		t.Fatalf("retained tables do not plan: %v %+v", err, plan)
	}
	before, _ := os.ReadFile(table)
	for name, extra := range map[string][]string{
		"keep zero":       {"--revision", revisions[0], "--keep", "0"},
		"keep over bound": {"--revision", revisions[0], "--keep", "65"},
		"offset time":     {"--revision", revisions[0], "--at", "2026-10-17T10:00:00+01:00"},
		"malformed time":  {"--revision", revisions[0], "--at", "yesterday"},
		"uppercase rev":   {"--revision", strings.ToUpper("abcdef") + strings.Repeat("0", 34)},
		"short revision":  {"--revision", "0123"},
		"foreign run":     {"--revision", revisions[0], "--run-url", "https://example.org/runs/1"},
	} {
		if code, err := refresh(extra...); code != 2 || err == nil {
			t.Errorf("%s: code=%d err=%v", name, code, err)
		}
		if after, _ := os.ReadFile(table); !bytes.Equal(after, before) {
			t.Fatalf("%s: a refused refresh changed the table", name)
		}
		if entries, _ := os.ReadDir(history); len(entries) != len(want) {
			t.Fatalf("%s: a refused refresh changed the retained tables", name)
		}
	}
	for name, args := range map[string][]string{
		"at without history": {"--table", table, "--shards", "1", "--revision", revisions[0], "--run-url", runURL, "--at", times[0], logs},
		"history with check": {"--table", table, "--shards", "1", "--history", history, logs},
	} {
		mode := "refresh"
		if strings.Contains(name, "check") {
			mode = "check"
		}
		if code, err := run(mode, args, nil); code != 2 || err == nil {
			t.Errorf("%s: code=%d err=%v", name, code, err)
		}
	}
}

// TestAFPV0604ShardBalanceStaleTrigger proves the advisory report flags a stale
// table when the slowest shard exceeds the ideal shard by the material share and
// its prediction by as much, reports every shard's balance in the summary, and
// stays quiet once a refreshed table predicts the same imbalance.
func TestAFPV0604ShardBalanceStaleTrigger(t *testing.T) {
	// Ideal shard: (100+500)s / 2 = 300s, so 10% is 30s. Shard 1 costs 500s, 200s over
	// the ideal shard, against a 100s prediction: stale.
	table := write(t, "costs.json", `{"profile":"corvint-ci-package-costs/0","source":{"revision":"`+revision+`","runURL":"`+runURL+`","goVersion":"go1.27.1"},"milliseconds":{"example.org/a":100000,"example.org/b":100000}}`)
	shard0 := write(t, "shard-0.json", passed("example.org/a", "100"))
	shard1 := write(t, "shard-1.json", passed("example.org/b", "500"))
	summary := write(t, "summary.md", "")
	var out bytes.Buffer
	if code, err := run("check", []string{"--table", table, "--advisory", "--shards", "2", "--summary", summary, shard0, shard1}, &out); code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	want := "::warning title=CI shard balance stale::shard 1 observed 500.0s against 100.0s predicted, 67%25 over the ideal shard 300.0s; refresh " + table + " (AFP-V0-604)\n"
	if !strings.HasPrefix(out.String(), want) {
		t.Fatalf("annotations:\n%s", out.String())
	}
	got, _ := os.ReadFile(summary)
	for _, w := range []string{"| 0 | 100.0s | 100.0s | -66.7% |\n| 1 | 100.0s | 500.0s | 66.7% |\n", "**Stale:** shard 1 is the slowest, 200.0s over the ideal shard and 400.0s over its prediction."} {
		if !strings.Contains(string(got), w) {
			t.Fatalf("summary lacks %q:\n%s", w, got)
		}
	}

	// A refreshed table predicts the imbalance two packages cannot avoid: not stale.
	clean := filepath.Join(t.TempDir(), "clean.json")
	if code, err := run("refresh", []string{"--table", clean, "--shards", "2", "--revision", revision, "--run-url", runURL, shard0, shard1}, nil); code != 0 || err != nil {
		t.Fatal(code, err)
	}
	out.Reset()
	cleanSummary := filepath.Join(t.TempDir(), "clean.md")
	if code, err := run("check", []string{"--table", clean, "--advisory", "--shards", "2", "--summary", cleanSummary, shard0, shard1}, &out); code != 0 || err != nil || out.Len() != 0 {
		t.Fatalf("refreshed: code=%d err=%v out=%s", code, err, out.String())
	}
	if got, _ := os.ReadFile(cleanSummary); !strings.Contains(string(got), "The slowest shard is within the material share") {
		t.Fatalf("refreshed summary:\n%s", got)
	}

	// Balanced shards are never stale, however wrong the per-package costs.
	out.Reset()
	even := write(t, "shard-1b.json", passed("example.org/b", "100"))
	if code, err := run("check", []string{"--table", table, "--advisory", "--shards", "2", "--share", "50", shard0, even}, &out); code != 0 || err != nil || strings.Contains(out.String(), "balance stale") {
		t.Fatalf("balanced: code=%d err=%v out=%s", code, err, out.String())
	}
}
