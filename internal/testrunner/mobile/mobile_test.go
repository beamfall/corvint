package mobile

import (
	"encoding/json"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"os"
	"strings"
	"testing"
)

func TestActualNativeAppiumReport(t *testing.T) {
	b, e := os.ReadFile("testdata/mixed.json")
	if e != nil {
		t.Fatal(e)
	}
	in := tr.Input{Target: "emulator-5580", SourceRoot: "/private/tmp/cem10-build/mobile/fixture", Selectors: []string{"mixed.cjs"}, Runner: Runner, ExitCode: 1, Reports: map[string][]byte{reportName: b}}
	o, e := Parse(in)
	if e != nil || !o.Complete || len(o.Tests) != 3 {
		t.Fatal(o, e)
	}
	counts := map[string]int{}
	for _, row := range o.Tests {
		counts[row.State]++
	}
	if counts[tr.Passed] != 1 || counts[tr.Failed] != 1 || counts[tr.Skipped] != 1 {
		t.Fatal(counts)
	}
	for _, field := range []string{"automationName", "platformName", "udid", "deviceUDID", "sessionId"} {
		var d map[string]any
		json.Unmarshal(b, &d)
		d["capabilities"].(map[string]any)[field] = "contradiction"
		changed, _ := json.Marshal(d)
		in.Reports[reportName] = changed
		if o, e := Parse(in); e == nil && o.Complete {
			t.Fatal("invalid capability admitted", field)
		}
	}
}
func request() tr.Request {
	return tr.Request{Runner: Runner, Root: "/source", Executable: "/tools/wdio", ExecutableSha256: strings.Repeat("a", 64), Config: "config.mjs", ConfigSha256: strings.Repeat("b", 64), Reporter: "/tools/reporter.js", ReporterSha256: strings.Repeat("c", 64), ReportDir: "/fresh", Project: "http://127.0.0.1:4723/", Target: "emulator-5580", Selectors: []string{"test.cjs"}, InputFiles: map[string]string{"test.cjs": strings.Repeat("d", 64)}}
}
func TestFixedProfile(t *testing.T) {
	r := request()
	v, e := Build(r)
	if e != nil || len(v.Files) != 1 || len(v.ReportPaths) != 1 || v.ReportPaths[0] != reportName {
		t.Fatal(v, e)
	}
	s := string(v.Files["appium.config.mjs"])
	for _, want := range []string{"UiAutomator2", "emulator-5580", "127.0.0.1", "maxInstances:1", "services:[]", "connectionRetryCount:0", "appium-wdio.json"} {
		if !strings.Contains(s, want) {
			t.Fatal(want)
		}
	}
	for _, mutate := range []func(*tr.Request){func(r *tr.Request) { r.Project = "http://evil:4723/" }, func(r *tr.Request) { r.Project = "http://127.0.0.1:4723/wd/hub" }, func(r *tr.Request) { r.Project = "http://127.0.0.1:04723/" }, func(r *tr.Request) { r.Target = "any" }, func(r *tr.Request) { r.Selectors = []string{"../x.cjs"} }, func(r *tr.Request) { r.InputFiles = nil }, func(r *tr.Request) { r.ReporterSha256 = "" }, func(r *tr.Request) { r.ReportFiles = []string{"foreign.json"} }} {
		r := request()
		mutate(&r)
		if _, e := Build(r); e == nil {
			t.Fatal("invalid fixed profile admitted", r)
		}
	}
}

func TestNativeCaseMatrixAndBoundaries(t *testing.T) {
	for _, c := range []struct {
		name     string
		exit     int
		complete bool
	}{{"mixed", 1, true}, {"pass", 0, true}, {"skip", 0, true}, {"zero", 0, false}, {"setup", 1, false}, {"collection", 1, false}} {
		t.Run(c.name, func(t *testing.T) {
			b, e := os.ReadFile("testdata/" + c.name + ".json")
			if e != nil {
				t.Fatal(e)
			}
			in := tr.Input{Target: "emulator-5580", SourceRoot: "/private/tmp/cem10-build/mobile/fixture", Selectors: []string{c.name + ".cjs"}, Runner: Runner, ExitCode: c.exit, Reports: map[string][]byte{reportName: b}}
			o, e := Parse(in)
			o = tr.Normalize(in, o)
			if o.Complete != c.complete || (c.complete && e != nil) {
				t.Fatal(o, e)
			}
		})
	}
	b, e := os.ReadFile("testdata/pass.json")
	if e != nil {
		t.Fatal(e)
	}
	for _, flag := range []string{"timeout", "overflow", "interrupted", "wrong-report", "wrong-exit"} {
		t.Run(flag, func(t *testing.T) {
			in := tr.Input{Target: "emulator-5580", SourceRoot: "/private/tmp/cem10-build/mobile/fixture", Selectors: []string{"pass.cjs"}, Runner: Runner, Reports: map[string][]byte{reportName: b}}
			switch flag {
			case "timeout":
				in.TimedOut = true
			case "overflow":
				in.Overflow = true
			case "interrupted":
				in.Interrupted = true
			case "wrong-report":
				in.Reports = map[string][]byte{"foreign.json": b}
			case "wrong-exit":
				in.ExitCode = 1
			}
			o, _ := Parse(in)
			if tr.Normalize(in, o).Complete {
				t.Fatal("incomplete boundary admitted")
			}
		})
	}
}
