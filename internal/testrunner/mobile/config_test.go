package mobile

import (
	"encoding/json"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The installed WDIO parser and session sanitizer determine the effective
// schedule/connection; inspecting generated JavaScript alone cannot prove them.
func TestNativeConfigIsolation(t *testing.T) {
	modules := os.Getenv("CORVINT_WDIO_MODULE_ROOT")
	if modules == "" {
		t.Skip("explicit installed WDIO modules required")
	}
	node := os.Getenv("CORVINT_NODE")
	if node == "" {
		t.Fatal("explicit Node required")
	}
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if e := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	r := request()
	r.Root = root
	r.Config = "supplied.mjs"
	r.ReportDir = filepath.Join(root, "reports")
	r.Reporter = filepath.Join(modules, "@wdio/json-reporter/build/index.js")
	write("test.cjs", "")
	write("other.cjs", "")
	write("supplied.mjs", `export const config={suite:'extra',suites:{extra:[`+js(filepath.Join(root, "other.cjs"))+`]},specFileRetries:3,repeat:4,mochaOpts:{timeout:15000,retries:8},before:()=>{throw new Error('unqualified hook');},capabilities:[{platformName:'Android','appium:automationName':'UiAutomator2','appium:udid':'emulator-5580','appium:appPackage':'local.corvint.mobileproof','appium:appActivity':'.ProofActivity',hostname:'outside.invalid',port:4444,sessionId:'foreign',specs:[`+js(filepath.Join(root, "other.cjs"))+`],alwaysMatch:{platformName:'Android','appium:automationName':'UiAutomator2','appium:udid':'emulator-5554'}}]};`)
	v, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	write("generated.mjs", string(v.Files["appium.config.mjs"]))
	write("probe.mjs", `import {config} from './generated.mjs';
import {ConfigParser} from `+js(fileURL(filepath.Join(modules, "@wdio/config/build/node/index.js")))+`;
import {DEFAULT_CONFIGS} from `+js(fileURL(filepath.Join(modules, "@wdio/config/build/index.js")))+`;
import {DEFAULTS} from `+js(fileURL(filepath.Join(modules, "webdriver/build/index.js")))+`;
function sanitizeCaps(capabilities,filterOut){const caps='alwaysMatch' in capabilities?capabilities.alwaysMatch:capabilities;const keys=[...Object.keys(DEFAULT_CONFIGS()),...Object.keys(DEFAULTS)];return Object.keys(caps).filter(k=>!keys.includes(k)===!filterOut).reduce((o,k)=>(o[k]=caps[k],o),{});}
const c=config.capabilities[0],effective={...config,...sanitizeCaps(c,true),capabilities:sanitizeCaps(c)};
const parser=new ConfigParser(`+js(filepath.Join(root, "generated.mjs"))+`);await parser.initialize({spec:[`+js(filepath.Join(root, "test.cjs"))+`]});
console.log(JSON.stringify({host:effective.hostname,port:effective.port,device:effective.capabilities['appium:udid'],specs:parser.getSpecs(),hooks:!!config.before,suite:!!config.suite,retries:config.specFileRetries,mochaRetries:config.mochaOpts.retries,repeat:config.repeat??0,session:effective.sessionId??''}));`)
	b, e := exec.Command(node, filepath.Join(root, "probe.mjs")).CombinedOutput()
	if e != nil {
		t.Fatalf("%v: %s", e, b)
	}
	var got struct {
		Host                          string
		Port                          int
		Device                        string
		Specs                         []string
		Hooks, Suite                  bool
		Retries, MochaRetries, Repeat int
		Session                       string
	}
	if e = json.Unmarshal(b, &got); e != nil {
		t.Fatalf("%v: %s", e, b)
	}
	if got.Host != "127.0.0.1" || got.Port != 4723 || got.Device != "emulator-5580" || len(got.Specs) != 1 || got.Specs[0] != fileURL(filepath.Join(root, "test.cjs")) || got.Hooks || got.Suite || got.Retries != 0 || got.MochaRetries != 0 || got.Repeat != 0 || got.Session != "" {
		t.Fatalf("native isolation failed: %s", b)
	}
	t.Logf("native schedule and session config: %s", b)
}

func TestNativeSpecInventory(t *testing.T) {
	b, e := readFixture("testdata", "pass.json")
	if e != nil {
		t.Fatal(e)
	}
	for _, specs := range []any{nil, []string{}, []string{"file:///one.cjs", "file:///two.cjs"}, []string{"relative.cjs"}, []string{"file://foreign/one.cjs"}, []string{"file:///one.ts"}, []string{"file:///a/../one.cjs"}} {
		var doc map[string]any
		if e = json.Unmarshal(b, &doc); e != nil {
			t.Fatal(e)
		}
		doc["specs"] = specs
		changed, _ := json.Marshal(doc)
		in := tr.Input{Target: "emulator-5580", SourceRoot: "/private/tmp/cem10-build/mobile/fixture", Selectors: []string{"pass.cjs"}, Runner: Runner, Reports: map[string][]byte{reportName: changed}}
		if o, e := Parse(in); e == nil && o.Complete {
			t.Fatalf("invalid native inventory admitted: %v", specs)
		}
	}
}

func TestCallerBoundIdentity(t *testing.T) {
	b, e := readFixture("testdata", "pass.json")
	if e != nil {
		t.Fatal(e)
	}
	valid := func() tr.Input {
		return tr.Input{Target: "emulator-5580", SourceRoot: "/private/tmp/cem10-build/mobile/fixture", Selectors: []string{"pass.cjs"}, Runner: Runner, Reports: map[string][]byte{reportName: b}}
	}
	o, e := Parse(valid())
	if e != nil || !o.Complete || o.Tests[0].File != "/private/tmp/cem10-build/mobile/fixture/pass.cjs" {
		t.Fatal(o, e)
	}
	for _, mutate := range []func(*tr.Input){func(in *tr.Input) { in.Target = "emulator-5554" }, func(in *tr.Input) { in.Target = "" }, func(in *tr.Input) { in.SourceRoot = "/other" }, func(in *tr.Input) { in.Selectors = []string{"other.cjs"} }, func(in *tr.Input) { in.Selectors = nil }, func(in *tr.Input) { in.Selectors = []string{"pass.cjs", "other.cjs"} }} {
		in := valid()
		mutate(&in)
		if o, e := Parse(in); e == nil && o.Complete {
			t.Fatal("caller contradiction admitted", in)
		}
	}
}
