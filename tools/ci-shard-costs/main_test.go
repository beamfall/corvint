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
