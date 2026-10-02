package jstestprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

const freshServerFixture = `const fs=require('node:fs'),http=require('node:http'),cp=require('node:child_process'),crypto=require('node:crypto');
const hash=b=>crypto.createHash('sha256').update(b).digest('hex');
const git=(...a)=>cp.execFileSync('/usr/bin/git',a,{encoding:'utf8'}).trim();
const source=fs.readFileSync('app.html');
const repository={rootCommit:git('rev-list','--max-parents=0','HEAD'),revision:git('rev-parse','HEAD'),tree:git('rev-parse','HEAD^{tree}'),dirtyState:'clean'};
const instance={kind:'process',id:String(process.pid),startGeneration:cp.execFileSync('/bin/ps',['-p',String(process.pid),'-o','lstart='],{encoding:'utf8',env:{PATH:'/usr/bin:/bin',LANG:'C',LC_ALL:'C'}}).trim()};
const state={profile:'corvint-application-attestation/0',repository,build:{kind:'source',digest:'sha256:'+hash(source)},configuration:{kind:'source',digest:'sha256:'+hash(fs.readFileSync('playwright.config.cjs'))},instance,health:{state:'healthy'}};
let input='';process.stdin.on('data',b=>input+=b);process.stdin.on('end',()=>{const response=Buffer.from(JSON.parse(input).response,'base64');http.createServer((req,res)=>{if(req.url==='/imports'){const imports={};for(const file of Object.keys(require.cache))imports[file]=hash(fs.readFileSync(file));res.setHeader('content-type','application/json');res.end(JSON.stringify(imports)+'\n');}else if(req.url==='/state'){res.setHeader('content-type','application/json');res.end(JSON.stringify(state)+'\n');}else {res.setHeader('content-type','text/html');res.end(response);}}).listen(4394,'127.0.0.1');});
`

const freshObserverFixture = `const http=require('node:http'),cp=require('node:child_process'),fs=require('node:fs'),crypto=require('node:crypto');
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

func freshLiveEnvironment() []string {
	return []string{"PATH=/opt/homebrew/bin:/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TMPDIR=/tmp"}
}

// PTF-V0-002/003/004/009/010: this is a separately opt-in actual consuming-path
// fixture, never a transfer of the old provider tuple qualification.
func TestPTFV0ActualLive(t *testing.T) {
	modules := os.Getenv("CORVINT_PTF_MODULES")
	if modules == "" {
		if os.Getenv("CORVINT_PTF_REQUIRE_LIVE") == "1" {
			t.Fatal("PTF live modules required")
		}
		t.Skip("PTF live qualification not requested")
	}
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Fatal("PTF tuple unavailable")
	}
	ptfLivePort(t)
	for _, mode := range []string{"stable", "source-drift", "test-source-drift", "dependency-drift", "mutant", "foreign-port", "old-artifact", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			out, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			node, err := exec.LookPath("node")
			if err != nil {
				t.Fatal(err)
			}
			node, err = filepath.EvalSymlinks(node)
			if err != nil {
				t.Fatal(err)
			}

			fixtureModules := modules
			if mode == "dependency-drift" {
				fixtureModules = filepath.Join(out, "node_modules")
				for _, pkg := range []string{"playwright", "playwright-core", "@playwright/test"} {
					copyFreshLiveTree(t, filepath.Join(modules, pkg), filepath.Join(fixtureModules, pkg))
				}
			}
			write := func(name, body string) {
				t.Helper()
				if e := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); e != nil {
					t.Fatal(e)
				}
			}
			write(".gitignore", "node_modules\n")
			write("app.html", `<button id="inc" onclick="document.querySelector('#count').textContent=1">add</button><p id="count">0</p>`)
			server := freshServerFixture
			if mode == "old-artifact" {
				server = strings.Replace(server, "Buffer.from(JSON.parse(input).response,'base64')", "Buffer.from('old served html')", 1)
			}
			write("server.cjs", server)
			write("observer.cjs", freshObserverFixture)
			quoted, _ := json.Marshal(filepath.Join(out, "browser-output"))
			write("playwright.config.cjs", `module.exports={testDir:__dirname,testMatch:'counter.spec.cjs',outputDir:`+string(quoted)+`,projects:[{name:'chromium',use:{browserName:'chromium',headless:true,baseURL:'http://127.0.0.1:4394/'}}]};`)
			drift := ""
			if mode == "source-drift" {
				drift = `require('node:fs').appendFileSync(require('node:path').join(__dirname,'app.html'),'<!-- changed -->');`
			}

			if mode == "test-source-drift" {
				drift = `require('node:fs').appendFileSync(__filename,'\n// actual test source drift\n');`
			}
			if mode == "dependency-drift" {
				target, _ := json.Marshal(filepath.Join(fixtureModules, "playwright", "lib", "program.js"))
				drift = `require('node:fs').appendFileSync(` + string(target) + `,'\n// actual imported runner drift\n');`
			}
			if mode == "cancel" {
				marker, _ := json.Marshal(filepath.Join(out, "browser-started"))
				drift = `require('node:fs').writeFileSync(` + string(marker) + `,'started');await new Promise(resolve=>setTimeout(resolve,30000));`
			}
			write("counter.spec.cjs", `const {test,expect}=require('@playwright/test');test('counter one',async({page})=>{await page.goto('/');`+drift+`await page.locator('#inc').click();const imports={};for(const file of Object.keys(require.cache))imports[file]=require('node:crypto').createHash('sha256').update(require('node:fs').readFileSync(file)).digest('hex');process.stdout.write('CORVINT-PTF-IMPORTS '+JSON.stringify(imports)+'\n');await expect(page.locator('#count'),'PTF-ASSERTION:counter-one').toHaveText('1');});`)
			write("package.json", `{"private":true,"devDependencies":{"@playwright/test":"1.63.0"}}`)
			write("package-lock.json", `{"lockfileVersion":3,"packages":{}}`)
			if err := os.Symlink(fixtureModules, filepath.Join(root, "node_modules")); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=PTF fixture", "-c", "user.email=ptf@example.invalid", "commit", "-qm", "fixture"}} {
				cmd := exec.Command("git", args...)
				cmd.Dir = root
				if b, e := cmd.CombinedOutput(); e != nil {
					t.Fatalf("git %v: %v %s", args, e, b)
				}
			}
			repo, reason := observeTestRepository(context.Background(), root)
			if reason != "" {
				t.Fatal(reason)
			}
			command := func(args ...string) FreshCommandIdentity {
				e := FreshCommandIdentity{Argv: args}
				o := freshCommandObserve(e)
				if o == nil {
					t.Fatal("command unavailable")
				}
				return *o
			}
			digest := func(path string) string {
				b, e := os.ReadFile(path)
				if e != nil {
					t.Fatal(e)
				}
				return sha256Hex(b)
			}
			observerConfig := filepath.Join(out, "observer.json")
			if e := os.WriteFile(observerConfig, []byte("{\"documentUrl\":\"http://127.0.0.1:4394/\"}\n"), 0600); e != nil {
				t.Fatal(e)
			}
			expectation := ApplicationAttestationExpectation{Repository: ApplicationRepositoryExpectation{RootCommit: repo.RootCommit, Revision: repo.Revision, Tree: repo.Tree, DirtyPolicy: "require-clean"}, Build: ApplicationArtifactIdentity{Kind: "source", Digest: "sha256:" + digest(filepath.Join(root, "app.html"))}, Configuration: ApplicationArtifactIdentity{Kind: "source", Digest: "sha256:" + digest(filepath.Join(root, "playwright.config.cjs"))}, InstanceKind: "process"}
			attestationConfig := filepath.Join(out, "attestation.json")
			writeCanonicalFixture(t, attestationConfig, applicationAttestationConfig{Profile: applicationAttestationConfigProfile, Expectation: expectation})
			cfg := E2EConfig{Config: Config{Dir: root, TestFiles: []string{filepath.Join(root, "counter.spec.cjs")}, ConfigFile: filepath.Join(root, "playwright.config.cjs"), PackageJSON: filepath.Join(root, "package.json"), Lockfile: filepath.Join(root, "package-lock.json"), RunnerName: "playwright", RunnerVersion: "1.63.0", DeclaredEnvKeys: []string{"PATH", "LANG", "LC_ALL", "TMPDIR"}, Timeout: 30 * time.Second}, ServerArgv: []string{node, filepath.Join(root, "server.cjs")}, ServerReadyURL: "http://127.0.0.1:4394/", TestArgv: []string{"--workers=1", "--retries=0", "--forbid-only"}, ApplicationAttestation: &ApplicationAttestationProvider{Argv: []string{os.Args[0], "-test.run=^TestPTFV0AttestationHelper$"}, ConfigFile: attestationConfig}, Freshness: &FreshnessConfig{ProductDir: root, ProductExpected: *repo, TestExpected: *repo, ArtifactPath: "app.html", SourceDigest: strings.TrimPrefix(expectation.Build.Digest, "sha256:"), DocumentURL: "http://127.0.0.1:4394/", Runner: command(node, filepath.Join(fixtureModules, "playwright", "cli.js")), Server: command(node, filepath.Join(root, "server.cjs")), Observer: command(node, filepath.Join(root, "observer.cjs"), "fresh"), ObserverConfigFile: observerConfig, ObserverConfigDigest: digest(observerConfig)}}
			cfg.Freshness.Dependencies = freshLiveDependencies(t, fixtureModules, []string{filepath.Join(root, "app.html"), filepath.Join(root, "server.cjs"), filepath.Join(root, "observer.cjs"), filepath.Join(root, "counter.spec.cjs"), filepath.Join(root, "playwright.config.cjs"), filepath.Join(root, "package.json"), filepath.Join(root, "package-lock.json"), observerConfig, attestationConfig})
			cfg.Freshness.DependencyDigest = FreshDependencyDigest(cfg.Freshness.Dependencies)
			if mode == "mutant" {
				source, e := os.ReadFile(filepath.Join(root, "app.html"))
				if e != nil {
					t.Fatal(e)
				}
				mutant := bytes.Replace(source, []byte("textContent=1"), []byte("textContent=2"), 1)
				cfg.Freshness.Mutation = &ResponseMutationDefinition{ArtifactPath: "app.html", SourceSHA256: sha256Hex(source), From: "textContent=1", To: "textContent=2", MutantSHA256: sha256Hex(mutant)}
			}

			if mode == "cancel" {
				cfg.TestArgv = append(cfg.TestArgv, "--corvint-fixture-cancel")
			}
			if mode == "foreign-port" {
				foreignCtx, foreignCancel := context.WithCancel(context.Background())
				foreignDone := make(chan procgroup.Observation, 1)
				source, _ := os.ReadFile(filepath.Join(root, "app.html"))
				input, _ := json.Marshal(struct {
					Response []byte `json:"response"`
				}{source})
				go func() {
					foreignDone <- procgroup.Run(foreignCtx, procgroup.Spec{Argv: cfg.ServerArgv, Dir: root, Env: freshLiveEnvironment(), Stdin: append(input, '\n'), Timeout: 45 * time.Second, OutputLimit: 64 << 10, ObserveDescendants: true})
				}()
				t.Cleanup(func() {
					foreignCancel()
					o := <-foreignDone
					if !o.OwnedProcessGroupCleanup || !freshDescendantsAbsent(o.DescendantObservation) {
						t.Errorf("foreign fixture cleanup: %+v", o)
					}
				})
				if e := externalReady(context.Background(), cfg.ServerReadyURL, 5*time.Second); e != nil {
					t.Fatal(e)
				}
			}
			b, e := json.Marshal(cfg)
			if e != nil {
				t.Fatal(e)
			}
			job := filepath.Join(out, "job.json")
			if e := os.WriteFile(job, b, 0600); e != nil {
				t.Fatal(e)
			}
			o := procgroup.Run(context.Background(), procgroup.Spec{Argv: []string{os.Args[0], "-test.run=^TestPTFV0WorkerHelper$", "--", job}, Dir: root, Env: freshLiveEnvironment(), Timeout: 45 * time.Second, OutputLimit: 4 << 20, ObserveDescendants: true})
			if o.ExitStatus != 0 || o.Cancelled || o.TimedOut || !o.OwnedProcessGroupCleanup || !freshDescendantsAbsent(o.DescendantObservation) {
				t.Fatalf("worker: %+v\n%s", o, string(o.Stderr))
			}
			var r Receipt
			if e := json.Unmarshal(o.Stdout, &r); e != nil {
				t.Fatalf("receipt: %v %s %s", e, o.Stdout, o.Stderr)
			}
			if r.Freshness == nil || r.Freshness.ServerGone == nil || !*r.Freshness.ServerGone {
				t.Fatalf("server cleanup %+v", r)
			}
			if mode == "foreign-port" || mode == "old-artifact" {
				if FreshnessCurrency(r, TestOutcome{}) != testvalidity.FreshnessStale || len(r.Tests) != 0 {
					t.Fatalf("false current or runner reached stale service: %+v", r)
				}
				return
			}
			if mode == "cancel" {
				if !r.Cancelled || !freshDescendantsAbsent(r.DescendantObservation) || FreshnessCurrency(r, TestOutcome{}) != testvalidity.FreshnessUnknown {
					t.Fatalf("cancel evidence: %+v", r)
				}
				if _, e := os.Stat(filepath.Join(out, "browser-started")); e != nil {
					t.Fatal("actual browser did not start before cancellation")
				}
				return
			}
			if len(r.Tests) != 1 {
				data, _ := json.MarshalIndent(r, "", "  ")
				t.Fatalf("native row not observed: %s", data)
			}
			got := ReceiptTestProjection(r, r.Tests[0])
			want := testvalidity.FreshnessCurrent
			if mode == "source-drift" || mode == "test-source-drift" || mode == "dependency-drift" {
				want = testvalidity.FreshnessStale
			}
			if got.Freshness.State != want {
				data, _ := json.MarshalIndent(r, "", "  ")
				t.Fatalf("got %+v want %s\n%s", got, want, data)
			}

			if mode == "dependency-drift" {
				f := r.Freshness
				path := filepath.Join(fixtureModules, "playwright", "lib", "program.js")
				if f.DependenciesBefore.Files[path] == f.DependenciesAfter.Files[path] || f.Runner.Before.ExecutableDigest != f.Runner.After.ExecutableDigest || f.Runner.Before.EntrypointDigest != f.Runner.After.EntrypointDigest || r.Identity.RunnerVersion != "1.63.0" || !reflect.DeepEqual(r.TestRepositoryAtStart, r.TestRepositoryAtPublish) {
					t.Fatal("drift did not isolate imported implementation with stable entry/Git/version pins")
				}
				after, e := cfg.identity(r.Identity.Argv)
				if e != nil || after.PackageDigest != r.Identity.PackageDigest {
					t.Fatal("package/lock bytes changed in dependency drift")
				}
			}
			if mode == "test-source-drift" {
				path := filepath.Join(root, "counter.spec.cjs")
				if r.Freshness.ArtifactBefore != r.Freshness.ArtifactAfter || r.Freshness.DependenciesBefore.Files[path] == r.Freshness.DependenciesAfter.Files[path] {
					t.Fatal("distinct test-file drift not observed")
				}
			}
			if mode == "mutant" {
				if got.Execution.State != testvalidity.ExecutionFailed || !TargetAssertionFailure(r.Tests[0], "counter-one") {
					t.Fatalf("wrong target assertion: %+v", r.Tests[0])
				}
			}
			if mode == "stable" {
				if _, e := EncodeFreshness(r); e != nil {
					t.Fatal(e)
				}
				if got.Execution.State != testvalidity.ExecutionPassed {
					t.Fatalf("execution %+v", got)
				}
			}
		})
	}
}

func TestPTFV0WorkerHelper(t *testing.T) {
	if len(os.Args) < 3 || os.Args[len(os.Args)-2] != "--" {
		return
	}
	b, e := os.ReadFile(os.Args[len(os.Args)-1])
	if e != nil {
		os.Exit(2)
	}
	var cfg E2EConfig
	if json.Unmarshal(b, &cfg) != nil {
		os.Exit(2)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if len(cfg.TestArgv) > 0 && cfg.TestArgv[len(cfg.TestArgv)-1] == "--corvint-fixture-cancel" {
		cfg.TestArgv = cfg.TestArgv[:len(cfg.TestArgv)-1]
		go func() {
			deadline := time.NewTimer(10 * time.Second)
			defer deadline.Stop()
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-deadline.C:
					cancel()
					return
				case <-ticker.C:
					if _, e := os.Stat(filepath.Join(filepath.Dir(cfg.ApplicationAttestation.ConfigFile), "browser-started")); e == nil {
						cancel()
						return
					}
				}
			}
		}()
	}
	r, e := RunFreshE2E(ctx, cfg)
	if e != nil {
		os.Stderr.WriteString(e.Error())
		os.Exit(2)
	}
	json.NewEncoder(os.Stdout).Encode(r)
	os.Exit(0)
}

// The native helper is deliberately compatible with the existing /1 staged
// executable boundary. It reads the service and kernel process identity; no
// expected repository/build values are copied from its configuration stdin.
func TestPTFV0AttestationHelper(t *testing.T) {
	if os.Args[0] == "" || !strings.Contains(strings.Join(os.Args, " "), "-test.run=^TestPTFV0AttestationHelper$") {
		return
	}
	if _, e := io.ReadAll(io.LimitReader(os.Stdin, 64<<10)); e != nil {
		os.Exit(2)
	}
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
	var a ApplicationAttestation
	if decodeCanonical(b, &a) != nil {
		os.Exit(2)
	}
	pid, e := strconv.Atoi(a.Instance.ID)
	if e != nil {
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	process, e := freshProcessObserve(ctx, pid)
	if e != nil || process.Start != a.Instance.StartGeneration {
		os.Exit(2)
	}
	json.NewEncoder(os.Stdout).Encode(a)
	os.Exit(0)
}

func copyFreshLiveTree(t *testing.T, source, target string) {
	t.Helper()
	if e := filepath.WalkDir(source, func(path string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(source, path)
		if e != nil {
			return e
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0700)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("fixture dependency symlink")
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		return os.WriteFile(dest, b, 0600)
	}); e != nil {
		t.Fatal(e)
	}
}

func freshLiveDependencies(t *testing.T, modules string, inputs []string) *FreshDependencyManifest {
	t.Helper()
	m := &FreshDependencyManifest{Roots: []string{filepath.Join(modules, "@playwright/test"), filepath.Join(modules, "playwright"), filepath.Join(modules, "playwright-core")}, Files: map[string]string{}}
	o := ObserveFreshDependencies(m)
	if len(o.Failures) != 0 {
		t.Fatal(o.Failures)
	}
	m.Files = o.Files
	for _, path := range inputs {
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		m.Files[path] = sha256Hex(b)
	}
	if !FreshDependenciesMatch(m, ObserveFreshDependencies(m)) {
		t.Fatal("dependency manifest not complete")
	}
	return m
}
