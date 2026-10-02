package testacceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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
	if len(receipt.Tests) != 1 || !jstestprovider.TargetAssertionFailure(receipt.Tests[0], invocation.Target.AssertionID) {
		os.Exit(2)
	}
	if os.WriteFile("native.json", native, 0600) != nil {
		os.Exit(2)
	}
	hash := Hash(native)
	hook := behaviorfalsify.HookReceipt{Schema: behaviorfalsify.HookSchema, PlanDigest: invocation.PlanDigest, PerturbationSHA256: invocation.Control.PerturbationSHA256, Target: invocation.Target, Runner: invocation.Runner, Attempt: invocation.Attempt, NativeReceiptSHA256: hash, TestOutcome: "failed", TargetObservation: behaviorfalsify.CriterionObservation{CriterionID: invocation.Target.CriterionID, AssertionID: invocation.Target.AssertionID, State: "failed", FailureKind: "assertion"}, Unrelated: []behaviorfalsify.CriterionObservation{}, Setup: []behaviorfalsify.SetupObservation{}, Artifacts: []behaviorfalsify.Artifact{{Path: "native.json", SHA256: hash}}}
	json.NewEncoder(os.Stdout).Encode(hook)
}

// PTF-V0-006/007/008/009/010: actual two-repeat consumer + native control join.
func TestPTFV0ConsumerActualLive(t *testing.T) {
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
	write("counter.spec.cjs", `const {test,expect}=require('@playwright/test');test('counter one',async({page})=>{await page.goto('/');await page.locator('#inc').click();const imports={};for(const file of Object.keys(require.cache))imports[file]=require('node:crypto').createHash('sha256').update(require('node:fs').readFileSync(file)).digest('hex');process.stdout.write('CORVINT-PTF-IMPORTS '+JSON.stringify(imports)+'\n');await expect(page.locator('#count'),'PTF-ASSERTION:counter-one').toHaveText('1');});`)
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
	request := Request{Schema: FreshRequestSchema, Product: repo, TestRepository: repo, Tests: []Test{{ID: "counter", File: filepath.Join(root, "counter.spec.cjs"), Line: 1, Title: "counter one", Project: "chromium"}}, Config: configPin.Path, Package: filepath.Join(root, "package.json"), Lockfile: filepath.Join(root, "package-lock.json"), Runner: cmd(fresh.Runner), Server: cmd(fresh.Server), ReadyURL: fresh.DocumentURL, RunnerVersion: "1.63.0", Environment: "PTF-live", Repeat: 2, TimeoutSeconds: 30, Freshness: &FreshnessOptions{Provider: fresh, Attestation: Command{Argv: []string{exe, "attest"}, ExecutableSHA256: pin(exe).SHA256}, AttestationConfiguration: pin(attestationConfig)}}
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
