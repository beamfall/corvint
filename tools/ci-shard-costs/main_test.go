package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/.github/cishards"
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
func TestAFPV0022RefreshFromHostedOutcomes(t *testing.T) {
	raw := write(t, "raw.log", `{"Action":"pass","Package":"example.org/a","Test":"TestA","Elapsed":9}
{"Action":"output","Package":"example.org/a","Output":"ok\n"}
{"Action":"pass","Package":"example.org/a","Elapsed":953.5}
`)
	hosted := write(t, "hosted.log", "shard (1)\tGo tests\t2026-10-04T11:21:18Z {\"Action\":\"pass\",\"Package\":\"example.org/b\",\"Elapsed\":0.0004}\n"+
		"shard (1)\tGo tests\t2026-10-04T11:21:19Z {\"Action\":\"skip\",\"Package\":\"example.org/none\",\"Elapsed\":0}\n"+
		"shard (1)\tGo tests\t2026-10-04T11:21:19Z not json {\n")
	table := filepath.Join(t.TempDir(), "costs.json")
	if code, err := run("refresh", []string{"--table", table, "--revision", revision, "--run-url", runURL, raw, hosted}, nil); code != 0 || err != nil {
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
	if code, err := run("refresh", []string{"--table", table, "--revision", revision, "--run-url", runURL, raw}, nil); code != 2 || err == nil {
		t.Fatalf("partial refresh code=%d err=%v", code, err)
	}
	if after, _ := os.ReadFile(table); !bytes.Equal(after, got) {
		t.Fatal("refused partial refresh changed the table")
	}
	if code, err := run("refresh", []string{"--table", table, "--allow-removed", "--revision", revision, "--run-url", runURL, raw, hosted}, nil); code != 0 || err != nil {
		t.Fatalf("allowed refresh code=%d err=%v", code, err)
	}
	var out bytes.Buffer
	if code, err := run("check", []string{"--table", table, raw, hosted}, &out); code != 0 || err != nil || out.Len() != 0 {
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
			code, err := run("refresh", []string{"--table", table, "--revision", tc.revision, "--run-url", tc.url, write(t, "log", tc.log)}, nil)
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
	log := write(t, "log", `{"Action":"pass","Package":"example.org/slow","Elapsed":953.5}
{"Action":"pass","Package":"example.org/fast","Elapsed":98}
{"Action":"pass","Package":"example.org/same","Elapsed":199}
{"Action":"pass","Package":"example.org/tiny","Elapsed":9}
{"Action":"pass","Package":"example.org/new","Elapsed":144}
`)
	var out bytes.Buffer
	code, err := run("check", []string{"--table", table, log}, &out)
	want := `drift example.org/fast recorded=910000ms observed=98000ms
drift example.org/slow recorded=3220ms observed=953500ms
missing example.org/new observed=144000ms
stale example.org/gone recorded=5000ms
`
	if code != 1 || err != nil || out.String() != want {
		t.Fatalf("code=%d err=%v out:\n%s", code, err, out.String())
	}
	out.Reset()
	if code, _ := run("check", []string{"--table", table, "--factor", "1.5", "--floor", "1s", log}, &out); code != 1 || !strings.Contains(out.String(), "drift example.org/same") || !strings.Contains(out.String(), "drift example.org/tiny") {
		t.Fatalf("stated bounds not applied: code=%d out:\n%s", code, out.String())
	}
	for _, factor := range []string{"NaN", "+Inf", "0.5"} {
		if code, err := run("check", []string{"--table", table, "--factor", factor, log}, &out); code != 2 || err == nil {
			t.Fatalf("factor %s code=%d err=%v", factor, code, err)
		}
	}
	if code, err := run("check", []string{"--table", write(t, "bad.json", `{"profile":"bad"}`), log}, &out); code != 2 || err == nil {
		t.Fatalf("invalid table code=%d err=%v", code, err)
	}
}

// AFP-V0-040: the advisory CI report exits 0 on findings, marks what misplaces a
// stated share of the ideal shard, and bounds its warning annotations.
func TestAFPV0040AdvisoryReportNeverFailsOnFindings(t *testing.T) {
	// Ideal shard: (1000+400+500+100+10)s / 2 = 1005s, so 10% is 100.5s.
	table := write(t, "costs.json", `{"profile":"corvint-ci-package-costs/0","source":{"revision":"`+revision+`","runURL":"`+runURL+`","goVersion":"go1.27.1"},"milliseconds":{"example.org/big":600000,"example.org/near":300000,"example.org/small":3000,"example.org/gone":900000}}`)
	shard0 := write(t, "shard-0.json", `{"Action":"pass","Package":"example.org/big","Elapsed":1000}
{"Action":"pass","Package":"example.org/near","Elapsed":400}
`)
	shard1 := write(t, "shard-1.json", `{"Action":"pass","Package":"example.org/new","Elapsed":500}
{"Action":"pass","Package":"example.org/small","Elapsed":100}
{"Action":"pass","Package":"example.org/tiny","Elapsed":10}
`)
	summary := write(t, "summary.md", "earlier step\n")
	var out bytes.Buffer
	code, err := run("check", []string{"--table", table, "--advisory", "--shards", "2", "--summary", summary, shard0, shard1}, &out)
	if code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	// near drifts by 100s (ratio 1.33, under 10%): not reported. big drifts by 400s within
	// factor 2 but over 10%: material. new is a material missing package; small drifts beyond
	// factor 2 but is not material; tiny is a minor missing package; gone is stale, never material.
	wantOut := "::warning title=CI shard cost drift::missing example.org/new observed=500000ms misplaces 500.0s (50%25 of the ideal shard); refresh " + table + " (AFP-V0-040)\n" +
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
	if code, _ := run("check", []string{"--table", table, "--advisory", "--shards", "2", "--share", "50", shard0, shard1}, &out); code != 0 || strings.Count(out.String(), "::warning") != 1 {
		t.Fatalf("share 50: code=%d out:\n%s", code, out.String())
	}
	clean := filepath.Join(t.TempDir(), "clean.json")
	if code, err := run("refresh", []string{"--table", clean, "--revision", revision, "--run-url", runURL, shard0, shard1}, nil); code != 0 || err != nil {
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
	log.WriteString(`{"Action":"pass","Package":"example.org/kept","Elapsed":1}` + "\n")
	for i := range 12 {
		log.WriteString(`{"Action":"pass","Package":"example.org/new` + string(rune('a'+i)) + `","Elapsed":100}` + "\n")
	}
	var out bytes.Buffer
	if code, err := run("check", []string{"--table", write(t, "costs.json", table.String()), "--advisory", "--shards", "1", "--share", "5", write(t, "log", log.String())}, &out); code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if n := strings.Count(out.String(), "::warning"); n != maxWarnings || !strings.Contains(out.String(), "3 further findings (0 drift, 3 missing, 0 stale)") {
		t.Fatalf("%d warnings:\n%s", n, out.String())
	}
}

// AFP-V0-040: input that cannot stand for one complete run abstains with its reason.
func TestAFPV0040AdvisoryAbstainsOnPartialOrUnusableInput(t *testing.T) {
	table := write(t, "costs.json", `{"profile":"corvint-ci-package-costs/0","source":{"revision":"`+revision+`","runURL":"`+runURL+`","goVersion":"go1.27.1"},"milliseconds":{"example.org/a":1000}}`)
	pass := write(t, "shard-0.json", `{"Action":"pass","Package":"example.org/a","Elapsed":900}`+"\n")
	for name, tc := range map[string]struct {
		args   []string
		reason string
	}{
		"partial-run":      {[]string{"--table", table, pass}, "1 of 2 shard logs present"},
		"missing-artifact": {[]string{"--table", table, pass, filepath.Join(t.TempDir(), "shard-1.json")}, "no such file"},
		"failed-package":   {[]string{"--table", table, pass, write(t, "shard-1.json", `{"Action":"fail","Package":"example.org/b","Elapsed":1}`+"\n")}, "package example.org/b failed"},
		"invalid-table":    {[]string{"--table", write(t, "bad.json", `{"profile":"bad"}`), pass, write(t, "shard-1.json", `{"Action":"pass","Package":"example.org/b","Elapsed":1}`+"\n")}, "cost table is invalid"},
		// Each of these has a valid table and a valid other shard, so only the stream decides.
		"empty-log":        {[]string{"--table", table, pass, write(t, "shard-1.json", "")}, "shard-1.json: no terminal package outcome"},
		"malformed-record": {[]string{"--table", table, pass, write(t, "shard-1.json", `{"Action":"pass","Package":"example.org/b","Elapsed":1}`+"\n"+`{"Action":"output","Package":"example.org/c","Out`+"\n")}, "shard-1.json: line 2 is not a go test -json event"},
		"prefixed-record":  {[]string{"--table", table, pass, write(t, "shard-1.json", `2026-10-10T13:19:03Z {"Action":"pass","Package":"example.org/b","Elapsed":1}`+"\n")}, "shard-1.json: line 1 is not a go test -json event"},
		"unfinished":       {[]string{"--table", table, pass, write(t, "shard-1.json", `{"Action":"start","Package":"example.org/b"}`+"\n"+`{"Action":"start","Package":"example.org/c"}`+"\n"+`{"Action":"pass","Package":"example.org/c","Elapsed":1}`+"\n")}, "shard-1.json: package example.org/b started without a terminal outcome"},
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
