package testacceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/testvalidity"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

const acceptServerFixture = `const fs=require('node:fs'),http=require('node:http'),cp=require('node:child_process'),crypto=require('node:crypto');
const hash=b=>crypto.createHash('sha256').update(b).digest('hex');
const git=(...a)=>cp.execFileSync('/usr/bin/git',a,{encoding:'utf8'}).trim();
const source=fs.readFileSync('app.html');
const repository={rootCommit:git('rev-list','--max-parents=0','HEAD'),revision:git('rev-parse','HEAD'),tree:git('rev-parse','HEAD^{tree}'),dirtyState:'clean'};
const instance={kind:'process',id:String(process.pid),startGeneration:cp.execFileSync('/bin/ps',['-p',String(process.pid),'-o','lstart='],{encoding:'utf8',env:{PATH:'/usr/bin:/bin',LANG:'C',LC_ALL:'C'}}).trim()};
const state={profile:'corvint-application-attestation/0',repository,build:{kind:'source',digest:'sha256:'+hash(source)},configuration:{kind:'source',digest:'sha256:'+hash(fs.readFileSync('playwright.config.cjs'))},instance,health:{state:'healthy'}};
let input='';process.stdin.on('data',b=>input+=b);process.stdin.on('end',()=>{const response=Buffer.from(JSON.parse(input).response,'base64');http.createServer((req,res)=>{if(req.url==='/imports'){const imports={};for(const file of Object.keys(require.cache))imports[file]=hash(fs.readFileSync(file));res.setHeader('content-type','application/json');res.end(JSON.stringify(imports)+'\n');}else if(req.url==='/state'){res.setHeader('content-type','application/json');res.end(JSON.stringify(state)+'\n');}else {res.setHeader('content-type','text/html');res.end(response);}}).listen(4394,'127.0.0.1');});
`

const acceptObserverFixture = `const http=require('node:http'),cp=require('node:child_process'),fs=require('node:fs'),crypto=require('node:crypto');
let input='';process.stdin.on('data',b=>input+=b);process.stdin.on('end',async()=>{try {
const config=JSON.parse(input);
const get=url=>new Promise((resolve,reject)=>{http.get(url,r=>{if(r.statusCode!==200){r.resume();reject(Error('status'));return;}let b=[];r.on('data',x=>b.push(x));r.on('end',()=>resolve(Buffer.concat(b)));}).on('error',reject);});
const url=process.argv[2]==='attest'?'http://127.0.0.1:4394/':config.documentUrl;
const state=JSON.parse(await get(new URL('/state',url)));
const start=cp.execFileSync('/bin/ps',['-p',state.instance.id,'-o','lstart='],{encoding:'utf8',env:{PATH:'/usr/bin:/bin',LANG:'C',LC_ALL:'C'}}).trim();
if(start!==state.instance.startGeneration)throw Error('process');
if(process.argv[2]==='attest')process.stdout.write(JSON.stringify(state)+'\n');
else {const body=await get(url),serverImports=JSON.parse(await get(new URL('/imports',url))),observerImports={};for(const file of Object.keys(require.cache))observerImports[file]=crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');process.stdout.write(JSON.stringify({url,process:{pid:Number(state.instance.id),start},body:body.toString('base64'),serverImports:Object.fromEntries(Object.entries(serverImports).sort()),observerImports:Object.fromEntries(Object.entries(observerImports).sort())})+'\n');}
}catch(e){process.stderr.write('observer failed\n');process.exitCode=1;}});
`

// Helpers use the same consumer and falsifier executors as the companion.
// The staged native attestor observes the service and kernel, never stdin pins.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "attest" {
		acceptanceAttest()
		return
	}
	if role := os.Getenv("CORVINT_BEHAVIOR_COMMAND_ROLE"); role != "" {
		acceptanceHook(role)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "worker" {
		var job Job
		if json.NewDecoder(os.Stdin).Decode(&job) != nil {
			os.Exit(2)
		}
		json.NewEncoder(os.Stdout).Encode(RunWorker(context.Background(), job))
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "execute-receipt" {
		var plan behaviorfalsify.Plan
		if json.NewDecoder(os.Stdin).Decode(&plan) != nil {
			os.Exit(2)
		}
		receipt, e := behaviorfalsify.ExecuteReceipt(context.Background(), plan, plan.Digest, acceptTool())
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(2)
		}
		json.NewEncoder(os.Stdout).Encode(receipt)
		return
	}
	os.Exit(m.Run())
}
func acceptTool() behaviorfalsify.ToolIdentity {
	b, _ := os.ReadFile(os.Args[0])
	return behaviorfalsify.ToolIdentity{Name: "corvint-behavior-falsify", Version: "PTF-live-fixture", Revision: strings.Repeat("a", 40), Executable: Hash(b), SourceDirty: true}
}
func acceptanceAttest() {
	io.Copy(io.Discard, io.LimitReader(os.Stdin, 64<<10))
	client := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, e := client.Get("http://127.0.0.1:4394/state")
	if e != nil {
		os.Exit(2)
	}
	defer response.Body.Close()
	b, e := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if e != nil || response.StatusCode != 200 {
		os.Exit(2)
	}
	var a jstestprovider.ApplicationAttestation
	if json.Unmarshal(b, &a) != nil {
		os.Exit(2)
	}
	pid, e := strconv.Atoi(a.Instance.ID)
	if e != nil || pid <= 0 {
		os.Exit(2)
	}
	command := exec.Command("/bin/ps", "-p", a.Instance.ID, "-o", "lstart=")
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	observed, e := command.Output()
	if e != nil || strings.TrimSpace(string(observed)) != a.Instance.StartGeneration {
		os.Exit(2)
	}
	json.NewEncoder(os.Stdout).Encode(a)
}
func acceptanceHook(role string) {
	var invocation behaviorfalsify.Invocation
	if json.NewDecoder(os.Stdin).Decode(&invocation) != nil {
		os.Exit(2)
	}
	if role == "cleanup" {
		os.Remove("native.json")
		return
	}
	b, e := os.ReadFile("job.json")
	if e != nil {
		os.Exit(2)
	}
	var cfg jstestprovider.E2EConfig
	if json.Unmarshal(b, &cfg) != nil {
		os.Exit(2)
	}
	d, ok := responseDefinition(invocation.Control)
	if !ok {
		os.Exit(2)
	}
	cfg.Freshness.Mutation = &d
	// Remove the generic executor's private role/HOME before the bounded provider.
	os.Clearenv()
	for _, v := range SafeEnvironment() {
		parts := strings.SplitN(v, "=", 2)
		os.Setenv(parts[0], parts[1])
	}
	receipt, e := jstestprovider.RunFreshE2E(context.Background(), cfg)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(2)
	}
	native, e := jstestprovider.EncodeFreshness(receipt)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(2)
	}
	if len(receipt.Tests) != 1 {
		os.Exit(2)
	}
	// Derive the hook outcome from the actual selected native row, never the
	// fixture's requested mode. Infrastructure or ambiguous rows cannot qualify.
	row := receipt.Tests[0]
	outcome, state, failure := "passed", "passed", ""
	if jstestprovider.TargetAssertionFailure(row, invocation.Target.AssertionID) {
		outcome, state, failure = "failed", "failed", "assertion"
	} else if row.State != jstestprovider.StatePassed || len(row.Attempts) != 1 || row.Retries != 0 || row.Attempts[0].State != jstestprovider.StatePassed {
		os.Exit(2)
	}
	if os.WriteFile("native.json", native, 0600) != nil {
		os.Exit(2)
	}
	hash := Hash(native)
	hook := behaviorfalsify.HookReceipt{Schema: behaviorfalsify.HookSchema, PlanDigest: invocation.PlanDigest, PerturbationSHA256: invocation.Control.PerturbationSHA256, Target: invocation.Target, Runner: invocation.Runner, Attempt: invocation.Attempt, NativeReceiptSHA256: hash, TestOutcome: outcome, TargetObservation: behaviorfalsify.CriterionObservation{CriterionID: invocation.Target.CriterionID, AssertionID: invocation.Target.AssertionID, State: state, FailureKind: failure}, Unrelated: []behaviorfalsify.CriterionObservation{}, Setup: []behaviorfalsify.SetupObservation{}, Artifacts: []behaviorfalsify.Artifact{{Path: "native.json", SHA256: hash}}}
	json.NewEncoder(os.Stdout).Encode(hook)
}

// acceptanceLiveRequest freezes each fixture before any native execution.
func acceptanceLiveRequest(t *testing.T, mode string, repeats int) (Request, string) {
	t.Helper()
	modules := os.Getenv("CORVINT_PTF_MODULES")
	if modules == "" {
		if os.Getenv("CORVINT_PTF_REQUIRE_LIVE") == "1" {
			t.Fatal("live modules required")
		}
		t.Skip("live qualification not requested")
	}
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Fatal("tuple unavailable")
	}
	ptfLivePort(t)
	modules, e := filepath.EvalSymlinks(modules)
	if e != nil {
		t.Fatal(e)
	}
	root := canonicalTemp(t)
	out := canonicalTemp(t)
	workspace := canonicalTemp(t)
	write := func(name, body string) {
		t.Helper()
		if e := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	write(".gitignore", "node_modules\n")
	write("app.html", `<button id="inc" onclick="document.querySelector('#count').textContent=1">add</button><p id="count">0</p>`)
	write("server.cjs", acceptServerFixture)
	write("observer.cjs", acceptObserverFixture)
	quoted, _ := json.Marshal(filepath.Join(out, "browser-output"))
	write("playwright.config.cjs", `module.exports={testDir:__dirname,testMatch:'counter.spec.cjs',outputDir:`+string(quoted)+`,projects:[{name:'chromium',use:{browserName:'chromium',headless:true,baseURL:'http://127.0.0.1:4394/'}}]};`)
	assertion := "await expect(page.locator('#count'),'PTF-ASSERTION:counter-one').toHaveText('1');"
	switch mode {
	case "stable":
	case "nonasserting":
		assertion = "await page.locator('#count').textContent();"
	case "flaky":
		assertion = "await expect(page.locator('#count'),'PTF-ASSERTION:counter-one').toHaveText(Math.random()<0.5?'1':'2');"
	default:
		t.Fatal("unknown fixture mode")
	}
	write("counter.spec.cjs", `const {test,expect}=require('@playwright/test');test('counter one',async({page})=>{await page.goto('/');await page.locator('#inc').click();const imports={};for(const file of Object.keys(require.cache))imports[file]=require('node:crypto').createHash('sha256').update(require('node:fs').readFileSync(file)).digest('hex');process.stdout.write('CORVINT-PTF-IMPORTS '+JSON.stringify(imports)+'\n');`+assertion+`});`)
	write("contract.md", "# Counter\nThe registered counter-one assertion observes count one after clicking add.\n")
	write("package.json", `{"private":true,"devDependencies":{"@playwright/test":"1.63.0"}}`)
	write("package-lock.json", `{"lockfileVersion":3,"packages":{}}`)
	if e := os.Symlink(modules, filepath.Join(root, "node_modules")); e != nil {
		t.Fatal(e)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=PTF fixture", "-c", "user.email=ptf@example.invalid", "commit", "-qm", "fixture"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if b, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("git: %v %s", e, b)
		}
	}
	get := func(args ...string) string {
		v, e := git(root, args...)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	repo := Repository{Root: root, Commit: get("rev-parse", "HEAD"), Tree: get("rev-parse", "HEAD^{tree}")}
	expected := jstestprovider.ApplicationRepositoryIdentity{RootCommit: get("rev-list", "--max-parents=0", "HEAD"), Revision: repo.Commit, Tree: repo.Tree, DirtyState: "clean"}
	pin := func(path string) File {
		t.Helper()
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		return File{path, Hash(b)}
	}
	node, e := exec.LookPath("node")
	if e != nil {
		t.Fatal(e)
	}
	node, e = filepath.EvalSymlinks(node)
	if e != nil {
		t.Fatal(e)
	}
	command := func(args ...string) jstestprovider.FreshCommandIdentity {
		return jstestprovider.FreshCommandIdentity{Argv: args, ExecutableDigest: strings.TrimPrefix(pin(args[0]).SHA256, "sha256:"), EntrypointDigest: strings.TrimPrefix(pin(args[1]).SHA256, "sha256:")}
	}
	source, _ := os.ReadFile(filepath.Join(root, "app.html"))
	sourcePin := pin(filepath.Join(root, "app.html"))
	configPin := pin(filepath.Join(root, "playwright.config.cjs"))
	observerConfig := filepath.Join(out, "observer.json")
	os.WriteFile(observerConfig, []byte("{\"documentUrl\":\"http://127.0.0.1:4394/\"}\n"), 0600)
	attestationConfig := filepath.Join(out, "attestation.json")
	attestation := struct {
		Profile     string                                           `json:"profile"`
		Expectation jstestprovider.ApplicationAttestationExpectation `json:"expectation"`
	}{"corvint-application-attestation-config/0", jstestprovider.ApplicationAttestationExpectation{Repository: jstestprovider.ApplicationRepositoryExpectation{RootCommit: expected.RootCommit, Revision: repo.Commit, Tree: repo.Tree, DirtyPolicy: "require-clean"}, Build: jstestprovider.ApplicationArtifactIdentity{Kind: "source", Digest: sourcePin.SHA256}, Configuration: jstestprovider.ApplicationArtifactIdentity{Kind: "source", Digest: configPin.SHA256}, InstanceKind: "process"}}
	b, _ := json.Marshal(attestation)
	os.WriteFile(attestationConfig, append(b, '\n'), 0600)
	fresh := jstestprovider.FreshnessConfig{ProductDir: root, ProductExpected: expected, TestExpected: expected, ArtifactPath: "app.html", SourceDigest: strings.TrimPrefix(sourcePin.SHA256, "sha256:"), DocumentURL: "http://127.0.0.1:4394/", Runner: command(node, filepath.Join(modules, "playwright", "cli.js")), Server: command(node, filepath.Join(root, "server.cjs")), Observer: command(node, filepath.Join(root, "observer.cjs"), "fresh"), ObserverConfigFile: observerConfig, ObserverConfigDigest: strings.TrimPrefix(pin(observerConfig).SHA256, "sha256:")}
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		t.Fatal(e)
	}
	cmd := func(c jstestprovider.FreshCommandIdentity) Command {
		return Command{Argv: c.Argv, ExecutableSHA256: "sha256:" + c.ExecutableDigest, EntrypointSHA256: "sha256:" + c.EntrypointDigest}
	}
	request := Request{Schema: FreshRequestSchema, Product: repo, TestRepository: repo, Tests: []Test{{ID: "counter", File: filepath.Join(root, "counter.spec.cjs"), Line: 1, Title: "counter one", Project: "chromium"}}, Config: configPin.Path, Package: filepath.Join(root, "package.json"), Lockfile: filepath.Join(root, "package-lock.json"), Runner: cmd(fresh.Runner), Server: cmd(fresh.Server), ReadyURL: fresh.DocumentURL, RunnerVersion: "1.63.0", Environment: "PTF-live", Repeat: repeats, TimeoutSeconds: 30, Freshness: &FreshnessOptions{Provider: fresh, Attestation: Command{Argv: []string{exe, "attest"}, ExecutableSHA256: pin(exe).SHA256}, AttestationConfiguration: pin(attestationConfig)}}
	for _, name := range []string{"app.html", "server.cjs", "observer.cjs", "counter.spec.cjs", "playwright.config.cjs", "package.json", "package-lock.json", "contract.md"} {
		request.Inputs = append(request.Inputs, pin(filepath.Join(root, name)))
	}
	fresh.Dependencies = acceptLiveDependencies(t, modules, append(append([]File{}, request.Inputs...), pin(observerConfig), pin(attestationConfig)))
	fresh.DependencyDigest = jstestprovider.FreshDependencyDigest(fresh.Dependencies)
	request.Freshness.Provider = fresh
	cfg := jstestprovider.E2EConfig{Config: jstestprovider.Config{Dir: root, TestFiles: []string{request.Tests[0].File}, ConfigFile: request.Config, PackageJSON: request.Package, Lockfile: request.Lockfile, RunnerName: "playwright", RunnerVersion: "1.63.0", DeclaredEnvKeys: []string{"PATH", "LANG", "LC_ALL", "TMPDIR"}, Timeout: 30 * time.Second}, ServerArgv: fresh.Server.Argv, ServerReadyURL: fresh.DocumentURL, TestArgv: []string{"--retries=0", "--workers=1", "--forbid-only", regexp.QuoteMeta(request.Tests[0].File)}, ApplicationAttestation: &jstestprovider.ApplicationAttestationProvider{Argv: request.Freshness.Attestation.Argv, ConfigFile: attestationConfig, Timeout: 5 * time.Second}, Freshness: &fresh}
	b, _ = json.Marshal(cfg)
	os.WriteFile(filepath.Join(workspace, "job.json"), b, 0600)
	os.WriteFile(filepath.Join(workspace, behaviorfalsify.MarkerName), []byte("private PTF fixture\n"), 0600)
	mutant := bytes.Replace(source, []byte("textContent=1"), []byte("textContent=2"), 1)
	hook := behaviorfalsify.Command{Path: exe, ExecutableSHA256: pin(exe).SHA256}
	falsification := behaviorfalsify.Request{Target: behaviorfalsify.TargetIdentity{ContractID: "counter-contract", ContractSHA256: pin(filepath.Join(root, "contract.md")).SHA256, CriterionID: "count-one", AssertionID: "counter-one", ApplicationRevision: repo.Commit, TestRevision: repo.Commit, DocumentationRevision: repo.Commit, ContractFile: "contract.md", TestID: "counter", TestFile: "counter.spec.cjs", TestLine: 1, TestTitle: "counter one", Project: "chromium"}, Runner: behaviorfalsify.RunnerIdentity{Runner: "playwright", RunnerVersion: "1.63.0", Browser: "chromium", BrowserVersion: "Google Chrome for Testing 153.0.8010.12", ConfigFile: "playwright.config.cjs", ConfigSHA256: configPin.SHA256, EnvironmentSHA256: Hash([]byte("{}"))}, Controls: []behaviorfalsify.ControlSpec{{ID: "response-counter-two", Kind: behaviorfalsify.ChangedFixtureValue, Disposition: "run", Definition: map[string]string{"artifactPath": "app.html", "sourceSha256": fresh.SourceDigest, "from": "textContent=1", "to": "textContent=2", "mutantSha256": strings.TrimPrefix(Hash(mutant), "sha256:")}, Hook: &hook, UnrelatedCriteria: []string{}, RequiredSetup: []string{}}}, Cleanup: hook, DisposableRoot: workspace, Repositories: behaviorfalsify.RepositoryRoots{Application: root, Test: root, Documentation: root}, MarkerSHA256: pin(filepath.Join(workspace, behaviorfalsify.MarkerName)).SHA256, Attempts: 1, TimeoutSeconds: 60, WallClockSeconds: 120, ExternalState: "none"}
	plan, e := behaviorfalsify.BuildPlan(falsification)
	if e != nil {
		t.Fatal(e)
	}
	tool := acceptTool()
	request.Tests[0].Control = &plan
	request.Tests[0].Tool = &tool
	request.Tests[0].ControlCommand = &Command{Argv: []string{exe}, ExecutableSHA256: tool.Executable}
	return request, exe
}

// PTF-V0-006/007/008/009/010: actual two-repeat consumer + native control join.
func TestPTFV0ConsumerActualLive(t *testing.T) {
	request, exe := acceptanceLiveRequest(t, "stable", 2)
	report, e := Execute(context.Background(), request, Digest(request), exe, "experimental-live-fixture")
	if e != nil {
		t.Fatal(e)
	}
	if report.Verdict != "accepted" || len(report.Assessments) != 1 || report.Assessments[0].Validity.Strength.State != testvalidity.StrengthKilled || report.Assessments[0].Repeats.Passed != 2 {
		b, _ := json.MarshalIndent(report, "", "  ")
		t.Fatalf("consumer: %s", b)
	}

	// Retain actual native evidence when the operator requests a proof directory.
	if dir := os.Getenv("CORVINT_PTF_EVIDENCE_DIR"); dir != "" {
		if !filepath.IsAbs(dir) {
			t.Fatal("proof directory must be absolute")
		}
		if e := os.MkdirAll(dir, 0700); e != nil {
			t.Fatal(e)
		}
		b, _ := json.MarshalIndent(report, "", "  ")
		if e := os.WriteFile(filepath.Join(dir, "consumer-report.json"), append(b, '\n'), 0600); e != nil {
			t.Fatal(e)
		}
	}
	for _, name := range []string{"generic-valid-opaque", "generic-valid-other-assertion", "generic-valid-observer-drift", "generic-valid-locale", "generic-valid-viewport", "generic-valid-full-title", "early-stale", "early-unknown", "duplicate-repeat-ordinal"} {
		t.Run(name, func(t *testing.T) {
			b, _ := json.Marshal(report)
			var altered Report
			json.Unmarshal(b, &altered)
			switch name {
			case "generic-valid-opaque":
				replaceFreshNative(t, &altered.Controls[0], []byte("{}\n"))
			case "generic-valid-other-assertion", "generic-valid-observer-drift", "generic-valid-locale", "generic-valid-viewport", "generic-valid-full-title":
				native, e := jstestprovider.DecodeFreshness(altered.Controls[0].Evidence.RawAttempts[0].NativeBytes)
				if e != nil {
					t.Fatal(e)
				}
				switch name {
				case "generic-valid-other-assertion":
					native.Tests[0].FailureMessage = strings.ReplaceAll(native.Tests[0].FailureMessage, "PTF-ASSERTION:counter-one", "PTF-ASSERTION:other")
				case "generic-valid-observer-drift":
					native.Freshness.ObserverConfigBefore = strings.Repeat("b", 64)
					native.Freshness.ObserverConfigAfter = strings.Repeat("b", 64)
				default:
					if name == "generic-valid-full-title" {
						native.Tests[0].FullName += " renamed"
					} else {
						var use map[string]any
						if json.Unmarshal(native.Tests[0].Project.Use, &use) != nil {
							t.Fatal("use decode")
						}
						if name == "generic-valid-locale" {
							use["locale"] = "fr-FR"
						} else {
							use["viewport"] = map[string]int{"width": 811, "height": 633}
						}
						native.Tests[0].Project.Use, _ = json.Marshal(use)
					}
					native.Tests[0].ID = acceptNativeID(native.Identity, native.Tests[0])
				}
				raw, e := jstestprovider.EncodeFreshness(native)
				if e != nil {
					t.Fatal(e)
				}
				replaceFreshNative(t, &altered.Controls[0], raw)
			case "early-stale", "early-unknown":
				native, e := jstestprovider.DecodeFreshness(altered.Runs[0].NativeReceipt)
				if e != nil {
					t.Fatal(e)
				}
				if name == "early-stale" {
					native.Freshness.ArtifactAfter = strings.Repeat("b", 64)
				} else {
					native.Freshness.ProductAfter = nil
				}
				raw, e := jstestprovider.EncodeFreshness(native)
				if e != nil {
					t.Fatal(e)
				}
				altered.Runs[0].NativeReceipt = raw
				altered.Runs[0].ReceiptSHA256 = Hash(raw)
				altered.Runs[0].Rows[0].Validity = jstestprovider.ReceiptTestProjection(native, native.Tests[0])
			case "duplicate-repeat-ordinal":
				altered.Runs[1].Ordinal = altered.Runs[0].Ordinal
			}
			classify(&altered, request)
			if altered.Verdict == "accepted" {
				t.Fatal("altered native evidence accepted")
			}
			if strings.HasPrefix(name, "generic-valid") && altered.Assessments[0].Validity.Strength.State != testvalidity.StrengthNotMeasured {
				t.Fatal("opaque or mismatched generic kill supplied native strength")
			}
			if name == "early-stale" && (altered.Verdict != "rejected" || altered.Assessments[0].Validity.Freshness.State != testvalidity.FreshnessStale) {
				t.Fatal("later pass erased early stale")
			}
			if name == "early-unknown" && altered.Assessments[0].Validity.Freshness.State != testvalidity.FreshnessUnknown {
				t.Fatal("later pass erased early unknown")
			}
		})
	}
	// A generic kill cannot survive missing native data or an approved-target drift.
	for name, mutate := range map[string]func(*Report){"opaque-native": func(r *Report) { r.Controls[0].Evidence.RawAttempts[0].NativeBytes = []byte("{}") }, "first-baseline-absent": func(r *Report) { r.Runs[0].NativeReceipt = nil }, "projection-forged": func(r *Report) { r.Runs[0].Rows[0].Validity.Freshness.State = testvalidity.FreshnessStale }} {
		t.Run(name, func(t *testing.T) {
			b, _ := json.Marshal(report)
			var altered Report
			json.Unmarshal(b, &altered)
			mutate(&altered)
			classify(&altered, request)
			if altered.Verdict == "accepted" {
				t.Fatal("accepted changed evidence")
			}
		})
	}
}

func acceptCanonicalJSON(value any) []byte {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	encoder.Encode(value)
	return bytes.TrimSuffix(b.Bytes(), []byte("\n"))
}
func replaceFreshNative(t *testing.T, c *Control, bytes []byte) {
	t.Helper()
	e := c.Evidence
	raw := &e.RawAttempts[0]
	raw.NativeBytes = bytes
	raw.NativeSHA256 = Hash(bytes)
	var hook behaviorfalsify.HookReceipt
	if behaviorfalsify.Decode(raw.HookBytes, &hook) != nil {
		t.Fatal("hook decode")
	}
	hook.NativeReceiptSHA256 = raw.NativeSHA256
	hook.Artifacts[0].SHA256 = raw.NativeSHA256
	raw.HookBytes = append(acceptCanonicalJSON(hook), '\n')
	raw.HookSHA256 = Hash(raw.HookBytes)
	attempt := &e.Report.Results[0].Attempts[0]
	attempt.Receipt = &hook
	attempt.RetainedArtifacts = hook.Artifacts
	attempt.HookProcess.StdoutSHA256 = raw.HookSHA256
	e.Report.Digest = ""
	e.Report.Digest = Hash(acceptCanonicalJSON(e.Report))
	e.Digest = ""
	e.Digest = Hash(acceptCanonicalJSON(e))
	c.ReceiptDigest = e.Digest
	if err := behaviorfalsify.VerifyReceipt(*e, c.PlanDigest, e.Tool.Executable); err != nil {
		t.Fatalf("generic kill should remain valid: %v", err)
	}
}

// This adversarial resealing reproduces the provider's documented ID preimage;
// it is test-only and cannot manufacture an execution observation.
func acceptNativeID(identity jstestprovider.Identity, t jstestprovider.TestOutcome) string {
	identity.Argv = append([]string{}, identity.Argv...)
	for i, arg := range identity.Argv {
		if strings.HasPrefix(arg, "--config=") {
			identity.Argv[i] = "--config=" + identity.ConfigFile
		}
	}
	data, _ := json.Marshal(struct {
		Identity jstestprovider.Identity
		Project  *jstestprovider.ProjectIdentity
		Anchor   *jstestprovider.Anchor
		FullName string
	}{identity, t.Project, t.Anchor, t.FullName})
	return strings.TrimPrefix(Hash(data), "sha256:")
}

func acceptLiveDependencies(t *testing.T, modules string, inputs []File) *jstestprovider.FreshDependencyManifest {
	t.Helper()
	m := &jstestprovider.FreshDependencyManifest{Roots: []string{filepath.Join(modules, "@playwright/test"), filepath.Join(modules, "playwright"), filepath.Join(modules, "playwright-core")}, Files: map[string]string{}}
	o := jstestprovider.ObserveFreshDependencies(m)
	if len(o.Failures) != 0 {
		t.Fatal(o.Failures)
	}
	m.Files = o.Files
	for _, pin := range inputs {
		m.Files[pin.Path] = strings.TrimPrefix(pin.SHA256, "sha256:")
	}
	if !jstestprovider.FreshDependenciesMatch(m, jstestprovider.ObserveFreshDependencies(m)) {
		t.Fatal("dependency manifest not complete")
	}
	return m
}

// PTF-V0-006/008 and NEA-V0-003/004: both actual baseline rows and the
// faulted-response control pass; the complete native join measures survival.
func TestPTFV0ParentSurvivedActualLive(t *testing.T) {
	request, exe := acceptanceLiveRequest(t, "nonasserting", 2)
	report, err := Execute(context.Background(), request, Digest(request), exe, "experimental-parent-source-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("CORVINT_PTF_EVIDENCE_DIR"); dir != "" {
		if !filepath.IsAbs(dir) {
			t.Fatal("proof directory must be absolute")
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		for name, value := range map[string]any{"source-request.json": request, "source-report.json": report} {
			raw, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), append(raw, '\n'), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(report.Assessments) != 1 || len(report.Controls) != 1 {
		t.Fatal("missing actual assessment/control")
	}
	control := report.Controls[0]
	if control.Evidence == nil || behaviorfalsify.VerifyReceipt(*control.Evidence, request.Tests[0].Control.Digest, request.Tests[0].Tool.Executable) != nil {
		t.Fatal("actual generic survivor evidence did not verify")
	}
	if control.Status != "survived" || len(control.Evidence.RawAttempts) != 1 || len(control.Evidence.Report.CoverageGaps) != 1 || control.Evidence.Report.CoverageGaps[0] != "response-counter-two" {
		t.Fatalf("expected exact actual survivor and gap: status=%s", control.Status)
	}
	assessment := report.Assessments[0]
	if report.Verdict != "rejected" || assessment.Repeats.Passed != 2 || assessment.Validity.Strength.State != testvalidity.StrengthSurvived {
		t.Fatalf("actual nonasserting N2: verdict=%s passed=%d strength=%s reasons=%v", report.Verdict, assessment.Repeats.Passed, assessment.Validity.Strength.State, assessment.Reasons)
	}
}

func parentProofWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func parentProofJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	parentProofWrite(t, path, append(data, '\n'))
}

func parentGoExecutable(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(path)
}

// Compare complete independently formatted lines, never a second rendering
// through renderBody. A token elsewhere cannot hide corruption of its row.
func parentBodyBinding(report Report, repeats int) error {
	expected := []string{
		fmt.Sprintf("**Overall verdict: %s**", report.Verdict),
		fmt.Sprintf("| Product revision | `%s` (tree `%s`) |", report.Product.Commit, report.Product.Tree),
		fmt.Sprintf("| Test-repository revision | `%s` (tree `%s`) |", report.TestRepository.Commit, report.TestRepository.Tree),
		fmt.Sprintf("| Environment | `%s` |", report.Environment),
		fmt.Sprintf("| Corvint build | `%s` |", report.Build),
		fmt.Sprintf("| Companion executable | `%s` |", report.ExecutableSHA256),
		fmt.Sprintf("| Request digest | `%s` |", report.RequestDigest),
		fmt.Sprintf("| Repeats | %d per test, retries 0, workers 1 |", repeats),
		"Unknowns: " + strings.Join(report.Unknowns, ", "),
	}
	for _, a := range report.Assessments {
		minimum, maximum, isolated := "-", "-", "-"
		if a.Repeats.MinDurationMS != nil {
			minimum = fmt.Sprintf("%.0f", *a.Repeats.MinDurationMS)
		}
		if a.Repeats.MaxDurationMS != nil {
			maximum = fmt.Sprintf("%.0f", *a.Repeats.MaxDurationMS)
		}
		if len(a.Order.IsolatedStates) > 0 {
			isolated = strings.Join(a.Order.IsolatedStates, ", ")
		}
		expected = append(expected,
			fmt.Sprintf("| `%s` | %s | %d/%d/%d | %s-%s | %s | %s | %s | %s | %s |", a.ID, a.Verdict, a.Repeats.Passed, a.Repeats.Failed, a.Repeats.Other, minimum, maximum, a.Validity.Strength.State, a.Cleanup, a.Order.Status, a.Order.RequestedFileOrder, isolated),
			fmt.Sprintf("- `%s`: %s", a.ID, strings.Join(a.Reasons, ", ")))
	}
	lines := strings.Split(report.Body, "\n")
	for _, line := range expected {
		count := 0
		for _, actual := range lines {
			if actual == line {
				count++
			}
		}
		if count != 1 {
			return fmt.Errorf("body requires exactly one complete line %q; found %d", line, count)
		}
	}
	if strings.Contains(report.Body, "UNVALIDATED") {
		return fmt.Errorf("body contains unvalidated output")
	}
	return nil
}

// Capture the actual command and lifecycle before asserting success, so a
// failed qualification retains its bounded output and cleanup uncertainty.
func parentProofCommand(t *testing.T, dir, proof, name string, limit int, timeout time.Duration, argv ...string) []byte {
	t.Helper()
	o := procgroup.Run(t.Context(), procgroup.Spec{Argv: argv, Dir: dir, Env: os.Environ(), Timeout: timeout, OutputLimit: limit, StderrLimit: 1 << 20, ObserveDescendants: true})
	parentProofWrite(t, filepath.Join(proof, name+".stdout"), o.Stdout)
	parentProofWrite(t, filepath.Join(proof, name+".stderr"), o.Stderr)
	errText := ""
	if o.Err != nil {
		errText = o.Err.Error()
	}
	parentProofJSON(t, filepath.Join(proof, name+".process.json"), map[string]any{"argv": argv, "dir": dir, "error": errText, "started": o.Started, "waitCompleted": o.WaitCompleted, "exitObserved": o.ExitObserved, "exit": o.ExitStatus, "overflow": o.OutputOverflow, "cleanup": cleanup(o), "stdoutSHA256": Hash(o.Stdout), "stderrSHA256": Hash(o.Stderr)})
	if o.Err != nil || !o.Started || !o.WaitCompleted || !o.ExitObserved || o.ExitStatus != 0 || o.OutputOverflow || o.Cancelled || o.TimedOut || cleanupState(cleanup(o)) != "observed-absent" {
		t.Fatalf("%s did not qualify; retained command output and process observation in %s", name, proof)
	}
	return o.Stdout
}

// parentStandalone executes the real public companion, with this test binary
// retained only as the separately pinned attestation/falsification fixture.
// N32 has a persistent exclusive marker; failure never permits a pattern retry.
func parentStandalone(t *testing.T, mode string, repeats int) Report {
	t.Helper()
	companion, proof := os.Getenv("CORVINT_PTF_ACCEPT_COMPANION"), os.Getenv("CORVINT_PTF_EVIDENCE_DIR")
	if companion == "" || proof == "" || os.Getenv("CORVINT_PTF_MODULES") == "" {
		if os.Getenv("CORVINT_PTF_REQUIRE_LIVE") == "1" {
			t.Fatal("standalone live companion, evidence directory and modules required")
		}
		t.Skip("standalone live qualification not requested")
	}
	if !filepath.IsAbs(companion) || !filepath.IsAbs(proof) || filepath.Dir(companion) != filepath.Clean(proof) {
		t.Fatal("companion must be directly inside absolute evidence directory")
	}
	marker := filepath.Join(proof, "standalone-flaky-n32-started.json")
	if repeats == 32 {
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("N32 already attempted or marker unreadable; preserve the original batch")
		}
	}
	if err := os.MkdirAll(proof, 0700); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := git(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	readGit := func(args ...string) string {
		t.Helper()
		value, err := git(root, args...)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	head, tree := readGit("rev-parse", "HEAD"), readGit("rev-parse", "HEAD^{tree}")
	if readGit("status", "--porcelain=v1") != "" {
		t.Fatal("standalone qualification requires final clean source/spec/CEM target")
	}
	cem, err := os.ReadFile(filepath.Join(root, ".corvint", "change.cem.json"))
	if err != nil {
		t.Fatal("committed final CEM required: ", err)
	}
	if strings.TrimSpace(string(cem)) != readGit("show", "HEAD:.corvint/change.cem.json") {
		t.Fatal("working CEM differs from committed final binding")
	}
	name := "standalone-" + mode
	goExecutable := parentGoExecutable(t)
	parentProofCommand(t, root, proof, name+"-build", 1<<20, 3*time.Minute, goExecutable, "build", "-trimpath", "-o", companion, "./cmd/corvint-tests-accept")
	buildInfo := parentProofCommand(t, root, proof, name+"-buildinfo", 1<<20, time.Minute, goExecutable, "version", "-m", companion)
	if !bytes.Contains(buildInfo, []byte("vcs.revision="+head)) || !bytes.Contains(buildInfo, []byte("vcs.modified=false")) {
		t.Fatal("actual companion lacks clean final revision build identity")
	}
	executable, err := os.ReadFile(companion)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := acceptanceLiveRequest(t, mode, repeats)
	requestBytes := append(acceptCanonicalJSON(r), '\n')
	if len(requestBytes) > InputLimit {
		t.Fatal("actual standalone request exceeds original 4MiB limit")
	}
	requestPath := filepath.Join(proof, name+"-request.json")
	parentProofWrite(t, requestPath, requestBytes)
	planBytes := parentProofCommand(t, root, proof, name+"-plan", 1<<20, time.Minute, companion, "plan", "--plan", requestPath)
	var planned struct {
		Digest        string `json:"digest"`
		Qualification string `json:"qualification"`
	}
	if err := json.Unmarshal(planBytes, &planned); err != nil || planned.Digest != Digest(r) {
		t.Fatal("actual standalone plan did not match the immutable request")
	}
	argv := []string{companion, "accept", "--plan", requestPath, "--approve-plan", planned.Digest, "--new", r.Tests[0].File, "--repeat", strconv.Itoa(repeats), "--environment", r.Environment}
	binding := map[string]any{"sourceCommit": head, "sourceTree": tree, "committedCEMSHA256": Hash(cem), "executableSHA256": Hash(executable), "requestSHA256": Hash(requestBytes), "requestDigest": planned.Digest, "fixtureProduct": r.Product, "fixtureTests": r.TestRepository, "inputs": r.Inputs, "repeat": repeats, "argv": argv, "qualification": "NOT_YET_OBSERVED"}
	parentProofJSON(t, filepath.Join(proof, name+"-binding.json"), binding)
	if repeats == 32 {
		// Do not remove this marker, including after failure or interruption.
		f, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(binding)
		if err == nil {
			_, err = f.Write(append(data, '\n'))
		}
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			t.Fatal("N32 marker write failed; batch remains consumed", err, closeErr)
		}
	}
	raw := parentProofCommand(t, root, proof, name+"-accept", ReportLimit, 25*time.Minute, argv...)
	parentProofWrite(t, filepath.Join(proof, name+"-report.json"), raw)
	var report Report
	if len(raw) > ReportLimit || json.Unmarshal(raw, &report) != nil {
		t.Fatal("actual CLI did not emit a complete report within16MiB")
	}
	parentProofWrite(t, filepath.Join(proof, name+"-body.md"), []byte(report.Body))
	// Retain an explicit nonqualification until every assertion below passes.
	binding["qualification"] = "NOT_QUALIFIED"
	binding["reportSHA256"], binding["reportBytes"], binding["bodySHA256"] = Hash(raw), len(raw), Hash([]byte(report.Body))
	parentProofJSON(t, filepath.Join(proof, name+"-result.json"), binding)
	if readGit("rev-parse", "HEAD") != head || readGit("status", "--porcelain=v1") != "" {
		t.Fatal("final target changed during qualification")
	}
	if report.Schema != FreshSchema || report.RequestDigest != planned.Digest || report.Product != r.Product || report.TestRepository != r.TestRepository || report.Environment != r.Environment || report.ExecutableSHA256 != Hash(executable) || report.Build != "corvint-tests-accept experimental/0 revision="+head+" dirty=false" {
		t.Fatal("actual CLI report lost source/request/product/environment/executable binding")
	}
	if len(report.Assessments) != 1 || len(report.Controls) != 1 {
		t.Fatal("actual CLI omitted assessment or control")
	}
	a := report.Assessments[0]
	if a.Validity.Freshness.State != testvalidity.FreshnessCurrent || a.Cleanup != "observed-absent" || a.Repeats.Other != 0 || a.Repeats.Passed+a.Repeats.Failed != repeats || a.Repeats.MinDurationMS == nil || a.Repeats.MaxDurationMS == nil {
		t.Fatal("incomplete actual repeats/durations/cleanup")
	}
	counts := map[string]int{}
	for _, run := range report.Runs {
		counts[run.Kind]++
		if len(run.Rows) != 1 || run.Rows[0].ID != r.Tests[0].ID || run.Rows[0].Retries != 0 || run.Rows[0].Attempts != 1 || run.Rows[0].DurationMS < 0 || cleanupState(run.Cleanup) != "observed-absent" || run.Cleanup.Cancelled || run.Cleanup.TimedOut || len(run.Reasons) != 0 {
			t.Fatal("actual run did not retain one zero-retry row with duration and cleanup")
		}
		if run.Rows[0].State != jstestprovider.StatePassed && run.Rows[0].State != jstestprovider.StateFailed {
			t.Fatal("actual run outcome incomplete")
		}
		native, err := jstestprovider.DecodeFreshness(run.NativeReceipt)
		if err != nil || Hash(run.NativeReceipt) != run.ReceiptSHA256 || !freshRequestBinding(native, r) {
			t.Fatal("actual run native receipt binding incomplete")
		}
		row, ok := freshNativeRow(native, r.Tests[0])
		if !ok || jstestprovider.FreshnessCurrency(native, row) != testvalidity.FreshnessCurrent {
			t.Fatal("actual run native freshness is not current")
		}
		if run.Kind == "probe-isolated" && (run.TestID != r.Tests[0].ID || !slices.Equal(run.RequestedFiles, []string{r.Tests[0].File})) {
			t.Fatal("isolation probe lost requested test binding")
		}
	}
	if counts["repeat"] != repeats {
		t.Fatal("actual CLI repeat rows incomplete")
	}
	c := report.Controls[0]
	if c.Evidence == nil || behaviorfalsify.VerifyReceipt(*c.Evidence, r.Tests[0].Control.Digest, r.Tests[0].Tool.Executable) != nil {
		t.Fatal("actual CLI control lacks verified generic evidence")
	}
	if cleanupState(c.Cleanup) != "observed-absent" || c.Cleanup.Cancelled || c.Cleanup.TimedOut || !slices.Contains([]string{testvalidity.StrengthKilled, testvalidity.StrengthSurvived}, a.Validity.Strength.State) {
		t.Fatal("actual CLI control did not measure native strength with cleanup")
	}
	if err := parentBodyBinding(report, repeats); err != nil {
		t.Fatal(err)
	}
	if mode == "flaky" {
		if a.Repeats.Passed == 0 || a.Repeats.Failed == 0 {
			t.Fatal("NOT_QUALIFIED: single immutable N32 batch did not observe both outcomes; do not rerun")
		}
		if report.Verdict != "rejected" || !slices.Contains(a.Reasons, "test-failed") || counts["probe-original"] != 1 || counts["probe-reversed"] != 1 || counts["probe-isolated"] != IsolationRepeats || len(report.Runs) != repeats+2+IsolationRepeats {
			t.Fatal("flaky verdict or actual order/isolation runs incomplete")
		}
		decided := func(s string) bool {
			return s == string(jstestprovider.StatePassed) || s == string(jstestprovider.StateFailed)
		}
		if a.Order.RequestedFileOrder != "not-varied" || !decided(a.Order.OriginalState) || !decided(a.Order.ReversedState) || len(a.Order.IsolatedStates) != IsolationRepeats {
			t.Fatal("single-file order probe must retain its limitation and actual isolated outcomes")
		}
		for _, state := range a.Order.IsolatedStates {
			if !decided(state) {
				t.Fatal("isolation outcome incomplete")
			}
		}
		if !slices.Contains([]string{"fails-in-isolation", "failures-not-reproduced-in-isolation", "nondeterministic-in-isolation"}, a.Order.Status) {
			t.Fatal("isolation classification incomplete")
		}
	} else {
		wantVerdict, wantStrength := "accepted", testvalidity.StrengthKilled
		if mode == "nonasserting" {
			wantVerdict, wantStrength = "rejected", testvalidity.StrengthSurvived
		}
		if report.Verdict != wantVerdict || a.Verdict != wantVerdict || a.Validity.Strength.State != wantStrength || a.Repeats.Passed != 2 || len(report.Runs) != 2 || a.Order.Status != "not-probed" {
			t.Fatal("actual standalone N2 verdict/strength mismatch")
		}
		if mode == "nonasserting" && (!slices.Contains(a.Reasons, "negative-control-survived") || !slices.Equal(c.Evidence.Report.CoverageGaps, []string{"response-counter-two"})) {
			t.Fatal("native survivor reason or exact generic gap erased")
		}
		if mode == "stable" && len(c.Evidence.Report.CoverageGaps) != 0 {
			t.Fatal("original all-killed control acquired a gap")
		}
	}
	binding["qualification"] = "OBSERVED_PASS"
	binding["reportSHA256"], binding["reportBytes"], binding["bodySHA256"] = Hash(raw), len(raw), Hash([]byte(report.Body))
	parentProofJSON(t, filepath.Join(proof, name+"-result.json"), binding)
	return report
}

// PTF-V0-006/008 and NEA-V0-003/004/006/007/009: public companion N2 witnesses.
func TestPTFV0ParentStandaloneStableAndSurvivedLive(t *testing.T) {
	for _, mode := range []string{"stable", "nonasserting"} {
		t.Run(mode, func(t *testing.T) { parentStandalone(t, mode, 2) })
	}
}

// PTF-V0-008 and NEA-V0-002/004/005/006/007/008/009: one final immutable N32 batch, including
// real original/reversed/isolation observations; never retry for a pattern.
func TestPTFV0ParentStandaloneFlakyN32Live(t *testing.T) {
	parentStandalone(t, "flaky", 32)
}

// This optional discriminator builds and inspects a binary only. It never
// starts the companion, a provider, a browser, a fixture server or a live batch.
func TestPTFV0ParentBuildToolDiscriminator(t *testing.T) {
	root, proof := os.Getenv("CORVINT_PTF_BUILD_PROBE_ROOT"), os.Getenv("CORVINT_PTF_BUILD_PROBE_PROOF")
	if root == "" && proof == "" {
		t.Skip("bounded build discriminator not requested")
	}
	if !filepath.IsAbs(root) || !filepath.IsAbs(proof) {
		t.Fatal("absolute clean source root and private proof directory required")
	}
	status, err := git(root, "status", "--porcelain=v1")
	if err != nil || status != "" {
		t.Fatal("build discriminator source must be clean", err)
	}
	head, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(proof, 0700); err != nil {
		t.Fatal(err)
	}
	// Retain the original discriminator: a relative executable cannot start.
	bad := procgroup.Run(t.Context(), procgroup.Spec{Argv: []string{"go", "version"}, Dir: root, Env: os.Environ(), Timeout: time.Minute, OutputLimit: 1 << 20})
	if bad.Started || bad.Err == nil {
		t.Fatal("relative-executable guard was weakened")
	}
	parentProofJSON(t, filepath.Join(proof, "relative-go-refusal.json"), map[string]any{"argv": []string{"go", "version"}, "started": bad.Started, "error": bad.Err.Error()})
	goExecutable := parentGoExecutable(t)
	binary := filepath.Join(proof, "build-discriminator-companion")
	parentProofCommand(t, root, proof, "absolute-go-build", 1<<20, 3*time.Minute, goExecutable, "build", "-trimpath", "-o", binary, "./cmd/corvint-tests-accept")
	info := parentProofCommand(t, root, proof, "absolute-go-buildinfo", 1<<20, time.Minute, goExecutable, "version", "-m", binary)
	if !bytes.Contains(info, []byte("vcs.revision="+head)) || !bytes.Contains(info, []byte("vcs.modified=false")) {
		t.Fatal("built binary lost actual clean source identity")
	}
	end, err := git(root, "status", "--porcelain=v1")
	if err != nil || end != "" {
		t.Fatal("build discriminator modified source", err)
	}
}
