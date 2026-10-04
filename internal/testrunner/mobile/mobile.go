// Package mobile observes a bounded experimental Appium Android runner tuple.
// The operator owns the server and emulator; native reports do not prove authority.
package mobile

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/dynamic"
)

const Runner = "appium-uiautomator2-wdio"
const reportName = "appium-wdio.json"

var digest = regexp.MustCompile(`^[0-9a-f]{64}$`)
var emulatorID = regexp.MustCompile(`^emulator-[1-9][0-9]{3,4}$`)
var sessionID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func Runners() []string { return []string{Runner} }
func literal(name string) bool {
	return name != "" && name != "." && len(name) <= 1024 && filepath.IsLocal(name) && filepath.ToSlash(filepath.Clean(name)) == name && !strings.HasPrefix(name, "-") && !strings.ContainsAny(name, "\\\x00\r\n*?[]{}!,")
}
func fileURL(name string) string { return (&url.URL{Scheme: "file", Path: name}).String() }
func js(v any) string            { b, _ := json.Marshal(v); return string(b) }

// Build fixes the endpoint, emulator, framework and native report. JavaScript
// configuration/specs remain trusted executable project code, not a sandbox.
func Build(r tr.Request) (tr.Invocation, error) {
	v := tr.Invocation{Format: r.Runner, SuccessExitCodes: []int{0}, FailureExitCodes: []int{1}, Environment: map[string]string{}, Files: map[string][]byte{}}
	if r.Runner != Runner || !filepath.IsAbs(r.Root) || !filepath.IsAbs(r.Executable) || !digest.MatchString(r.ExecutableSha256) || !filepath.IsAbs(r.ReportDir) || filepath.Clean(r.ReportDir) != r.ReportDir || !emulatorID.MatchString(r.Target) {
		return v, fmt.Errorf("Appium requires pinned executable, root, fresh report directory and explicit emulator UDID")
	}
	endpoint, e := url.Parse(r.Project)
	if e != nil || endpoint.Scheme != "http" || endpoint.Hostname() != "127.0.0.1" || endpoint.User != nil || endpoint.Path != "/" || endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.RawPath != "" {
		return v, fmt.Errorf("Appium endpoint must be canonical http://127.0.0.1:PORT/")
	}
	port, e := strconv.Atoi(endpoint.Port())
	if e != nil || port < 1 || port > 65535 || r.Project != "http://127.0.0.1:"+strconv.Itoa(port)+"/" {
		return v, fmt.Errorf("Appium requires one explicit canonical loopback port")
	}
	if len(r.Selectors) != 1 || !literal(r.Selectors[0]) || !digest.MatchString(r.InputFiles[r.Selectors[0]]) {
		return v, fmt.Errorf("Appium requires one exact pinned JavaScript spec")
	}
	ext := filepath.Ext(r.Selectors[0])
	if ext != ".js" && ext != ".cjs" && ext != ".mjs" {
		return v, fmt.Errorf("Appium profile admits JavaScript specs only")
	}
	config := r.Config
	if !filepath.IsAbs(config) {
		if !literal(config) {
			return v, fmt.Errorf("invalid Appium config path")
		}
		config = filepath.Join(r.Root, config)
	}
	if filepath.Clean(config) != config || !digest.MatchString(r.ConfigSha256) || !filepath.IsAbs(r.Reporter) || filepath.Clean(r.Reporter) != r.Reporter || !digest.MatchString(r.ReporterSha256) || len(r.ReportFiles) != 0 {
		return v, fmt.Errorf("Appium requires pinned configuration and native WDIO JSON reporter; report path is fixed")
	}
	wrapper := `import * as imported from ` + js(fileURL(config)) + `;
import JSONReporter from ` + js(fileURL(r.Reporter)) + `;
const supplied=imported.config??imported.default?.config;
if(!supplied||!Array.isArray(supplied.capabilities)||supplied.capabilities.length!==1)throw new Error("one explicit Appium capability required");
const input=supplied.capabilities[0];
if(!input||input.platformName!=="Android"||input["appium:automationName"]!=="UiAutomator2"||input["appium:udid"]!==` + js(r.Target) + `)throw new Error("unqualified Appium device/driver tuple");
// Copy only qualified data fields: WDIO lifts connection fields and alwaysMatch
// from capabilities, and merges named suites with positional specs.
const capability={platformName:"Android","appium:automationName":"UiAutomator2","appium:udid":` + js(r.Target) + `};
for(const key of ["appium:appPackage","appium:appActivity"]){const value=input[key];if(typeof value!=="string"||!value||value.length>256||!/^[A-Za-z0-9_.$]+$/.test(value))throw new Error("explicit Android app identity required");capability[key]=value;}
for(const key of ["appium:noReset","appium:skipUnlock","appium:skipDeviceInitialization","appium:disableWindowAnimation"]){if(key in input){if(typeof input[key]!=="boolean")throw new Error("invalid Appium boolean");capability[key]=input[key];}}
for(const key of ["appium:newCommandTimeout","appium:uiautomator2ServerLaunchTimeout"]){if(key in input){if(!Number.isSafeInteger(input[key])||input[key]<1||input[key]>120000)throw new Error("invalid Appium timeout");capability[key]=input[key];}}
const timeout=supplied.mochaOpts?.timeout??15000;if(!Number.isSafeInteger(timeout)||timeout<1||timeout>120000)throw new Error("invalid Mocha timeout");
export const config={runner:"local",protocol:"http",hostname:"127.0.0.1",port:` + strconv.Itoa(port) + `,path:"/",maxInstances:1,logLevel:"error",framework:"mocha",services:[],specFileRetries:0,connectionRetryCount:0,mochaOpts:{timeout,retries:0},capabilities:[capability],specs:[` + js(filepath.Join(r.Root, r.Selectors[0])) + `],reporters:[[JSONReporter,{outputDir:` + js(r.ReportDir) + `,outputFileFormat:()=>` + js(reportName) + `}]]};
`
	v.Files["appium.config.mjs"] = []byte(wrapper)
	v.Argv = []string{"run", filepath.Join(r.ReportDir, "appium.config.mjs"), "--spec", filepath.Join(r.Root, r.Selectors[0])}
	v.ReportPaths = []string{reportName}
	return v, nil
}

// Parse requires the native Android/UiAutomator2 session tuple in addition to
// WebdriverIO completion/case checks. The session is observed, not authenticated.
func Parse(in tr.Input) (tr.Observation, error) {
	empty := tr.Observation{Runner: in.Runner, RetryInformation: tr.NotReported}
	if in.Runner != Runner {
		return empty, fmt.Errorf("unsupported mobile runner")
	}
	if len(in.Reports) != 1 || in.Reports[reportName] == nil {
		return empty, fmt.Errorf("one fixed native Appium report required")
	}
	native := in
	native.Runner = "webdriverio"
	o, e := dynamic.Parse(native)
	o.Runner = in.Runner
	if e != nil {
		return o, e
	}
	var doc map[string]json.RawMessage
	if e = json.Unmarshal(in.Reports[reportName], &doc); e != nil {
		return empty, e
	}
	if !emulatorID.MatchString(in.Target) || !filepath.IsAbs(in.SourceRoot) || filepath.Clean(in.SourceRoot) != in.SourceRoot || len(in.Selectors) != 1 || !literal(in.Selectors[0]) {
		return empty, fmt.Errorf("Appium observation requires caller-bound emulator, source root and one selected spec")
	}
	var specs []string
	if e = json.Unmarshal(doc["specs"], &specs); e != nil || len(specs) != 1 {
		return empty, fmt.Errorf("native Appium report requires exactly one spec")
	}
	spec, e := url.Parse(specs[0])
	if e != nil || spec.Scheme != "file" || spec.Host != "" || spec.User != nil || spec.RawQuery != "" || spec.Fragment != "" || !filepath.IsAbs(spec.Path) || filepath.Clean(spec.Path) != spec.Path || fileURL(spec.Path) != specs[0] {
		return empty, fmt.Errorf("native Appium spec must be one canonical absolute file URI")
	}
	switch filepath.Ext(spec.Path) {
	case ".js", ".cjs", ".mjs":
	default:
		return empty, fmt.Errorf("native Appium spec is not qualified JavaScript")
	}
	if spec.Path != filepath.Join(in.SourceRoot, in.Selectors[0]) {
		return empty, fmt.Errorf("native Appium spec contradicts caller-bound selection")
	}
	var caps map[string]json.RawMessage
	if e = json.Unmarshal(doc["capabilities"], &caps); e != nil {
		return empty, e
	}
	value := func(name string) string { var v string; _ = json.Unmarshal(caps[name], &v); return v }
	var framework string
	_ = json.Unmarshal(doc["framework"], &framework)
	if framework != "mocha" || value("platformName") != "Android" || value("automationName") != "UiAutomator2" || value("udid") != in.Target || value("deviceUDID") != value("udid") || !sessionID.MatchString(value("sessionId")) {
		return empty, fmt.Errorf("native report lacks qualified Appium Android emulator/session identity")
	}
	for i, t := range o.Tests {
		if t.Name == "" || !strings.HasPrefix(t.ID, value("sessionId")+"::") {
			return empty, fmt.Errorf("native case/session identity mismatch")
		}
		o.Tests[i].File = spec.Path
	}
	return o, nil
}
