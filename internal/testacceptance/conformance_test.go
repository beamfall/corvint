package testacceptance

import (
	"context"
	"encoding/json"
	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/testvalidity"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestNEAV0001ClosedInput(t *testing.T) {
	t.Run("NEA-V0-001/closed-input", func(t *testing.T) {

		for _, data := range []string{`{"schema":"x","schema":"y"}`, `{"unknown":1}`, `{} {}`, strings.Repeat("x", InputLimit+1)} {
			var r Request
			if Decode([]byte(data), &r) == nil {
				t.Fatal("unsafe input admitted")
			}
		}
		var r Request
		if Decode([]byte(`{"schema":"x"}`), &r) != nil {
			t.Fatal("closed known shape refused")
		}
		if Validate(r) == nil {
			t.Fatal("empty request admitted")
		}
		if approval(r, Hash([]byte("other"))) == nil {
			t.Fatal("wrong approval admitted")
		}

	})
}
func TestNEAV0004UnknownsAndRejectionPrecedence(t *testing.T) {
	t.Run("NEA-V0-004/unknowns-and-precedence", func(t *testing.T) {

		r := Request{Environment: "local", Repeat: 2, Tests: []Test{{ID: "stable"}, {ID: "nonasserting"}, {ID: "flaky"}}}
		life := Cleanup{OwnedGroup: true, Descendants: &procgroup.DescendantObservation{Absent: true, Limitations: []string{"fast-detach"}}}
		rows := func(fail bool) []Row {
			out := []Row{}
			for _, id := range []string{"stable", "nonasserting", "flaky"} {
				state := jstestprovider.StatePassed
				if id == "flaky" && fail {
					state = jstestprovider.StateFailed
				}
				out = append(out, Row{ID: id, State: state, Attempts: 1, Validity: jstestprovider.ToTestProjection(jstestprovider.TestOutcome{State: state}), IdentityUnknown: []string{"test-id"}})
			}
			return out
		}
		report := Report{Runs: []Run{{Kind: "repeat", Rows: rows(false), Cleanup: life}, {Kind: "repeat", Rows: rows(true), Cleanup: life}}, Controls: []Control{{Status: "killed", Cleanup: life}, {Status: "survived", Cleanup: life}, {Status: "killed", Cleanup: life}}}
		classify(&report, r)
		for i, want := range []string{"blocked", "rejected", "rejected"} {
			if report.Assessments[i].Verdict != want {
				t.Fatalf("%d: %+v", i, report.Assessments[i])
			}
			if report.Assessments[i].Validity.Freshness.State != testvalidity.FreshnessUnknown {
				t.Fatal("freshness invented")
			}
		}
		if report.Verdict != "rejected" {
			t.Fatal("aggregate verdict absent")
		}
		if report.Assessments[2].Validity.Hygiene.State != report.Runs[1].Rows[2].Validity.Hygiene.State {
			t.Fatal("provider hygiene overwritten")
		}
		if !strings.Contains(report.Body, "V1-0556") || strings.Contains(report.Body, "synthetic-author-prose") {
			t.Fatal("fixed body lost qualification")
		}
		if report.Assessments[0].Repeats.Passed != 2 || report.Assessments[2].Repeats.Failed != 1 || report.Assessments[0].Cleanup != "observed-absent" {
			t.Fatalf("per-test repeat summary: %+v", report.Assessments)
		}
		if report.Assessments[2].Order.Status != "isolation-incomplete" || report.Assessments[0].Order.Status != "not-probed" {
			t.Fatalf("absent isolation evidence was not left incomplete: %+v", report.Assessments[2].Order)
		}

	})
}
func TestNEAV0006SanitizedNestedCleanup(t *testing.T) {
	t.Run("NEA-V0-006/sanitized-nested-cleanup", func(t *testing.T) {

		t.Setenv("GITHUB_TOKEN", "credential-sentinel")
		t.Setenv("NODE_OPTIONS", "credential-sentinel")
		temp := canonicalTemp(t)
		script := filepath.Join(temp, "parent.sh")
		if err := os.WriteFile(script, []byte("#!/bin/sh\n[ -z \"$GITHUB_TOKEN\" ] && [ -z \"$NODE_OPTIONS\" ] || exit 9\n/bin/sh -c 'sleep 30' &\nsleep 30\n"), 0700); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
		defer cancel()
		o := procgroup.Run(ctx, procgroup.Spec{Argv: []string{script}, Dir: temp, Env: SafeEnvironment(), Timeout: time.Second, ObserveDescendants: true})
		if !o.Started || !o.Cancelled || !o.WaitCompleted || !o.OwnedProcessGroupCleanup {
			t.Fatalf("cleanup not joined: %+v", o)
		}
		if o.DescendantObservation == nil || !o.DescendantObservation.Absent {
			t.Fatalf("observed descendants unretired: %+v", o.DescendantObservation)
		}

	})
}
func command(t *testing.T, dir string, argv ...string) string {
	t.Helper()
	c := exec.Command(argv[0], argv[1:]...)
	c.Dir = dir
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("command %s failed: %s", argv[0], b)
	}
	return strings.TrimSpace(string(b))
}
func write(t *testing.T, path, text string) {
	t.Helper()
	if e := os.WriteFile(path, []byte(text), 0700); e != nil {
		t.Fatal(e)
	}
}
func hashFile(t *testing.T, path string) string {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return Hash(b)
}

// This live qualification is opt-in because the optional companion never
// installs browsers. A skip is NOT_RUN, not a qualified browser result.
func TestNEAV0002ActualBrowserAssessment(t *testing.T) {
	t.Run("NEA-V0-002/actual-browser-repeats", func(t *testing.T) {

		cli := os.Getenv("CORVINT_ACCEPTANCE_PLAYWRIGHT_CLI")
		if cli == "" {
			t.Skip("actual browser qualification NOT_RUN: explicit installed Playwright CLI required")
		}
		cacheDir, cacheErr := os.UserCacheDir()
		if cacheErr != nil {
			t.Fatal(cacheErr)
		}
		cli, e := filepath.EvalSymlinks(cli)
		if e != nil {
			t.Fatal(e)
		}
		node, e := exec.LookPath("node")
		if e != nil {
			t.Fatal(e)
		}
		node, e = filepath.EvalSymlinks(node)
		if e != nil {
			t.Fatal(e)
		}
		module := filepath.Join(filepath.Dir(cli), "test.mjs")
		browserModule := filepath.Join(filepath.Dir(cli), "index.mjs")
		repo := canonicalTemp(t)
		workspace := canonicalTemp(t)
		tools := canonicalTemp(t)
		state := filepath.Join(tools, "counter")
		// The actual tests remain unchanged for every negative control. Only the
		// approved disposable server response changes.
		testText := `import {test,expect} from '` + module + `';
 test('stable',async({page})=>{await page.goto('http://127.0.0.1:4173/stable');await page.getByRole('button',{name:'Increment'}).click();await expect(page.locator('output')).toHaveText('1',{timeout:500});});
 test('nonasserting',async({page})=>{await page.goto('http://127.0.0.1:4173/stable');await page.getByRole('button',{name:'Increment'}).click();});
 test('flaky',async({page})=>{await page.goto('http://127.0.0.1:4173/flaky');await page.getByRole('button',{name:'Increment'}).click();await expect(page.locator('output')).toHaveText('1',{timeout:500});});
`
		write(t, filepath.Join(repo, "new.spec.mjs"), testText)
		write(t, filepath.Join(repo, "playwright.config.mjs"), `export default {testDir:'.',testMatch:'new.spec.mjs',retries:0,workers:1,use:{headless:true},projects:[{name:'chromium',use:{browserName:'chromium'}}]};`)
		write(t, filepath.Join(repo, "package.json"), `{"type":"module"}`)
		write(t, filepath.Join(repo, "package-lock.json"), `{"lockfileVersion":3}`)
		write(t, filepath.Join(repo, "contract.md"), "Criterion counter increments to one; negative response must fail its assertion.\n")
		server := `import {createServer} from 'node:http';import fs from 'node:fs';const state=process.argv[2];const s=createServer((req,res)=>{let value=1;if(req.url==='/flaky'){let n=fs.existsSync(state)?Number(fs.readFileSync(state,'utf8')):0;fs.writeFileSync(state,String(n+1));value=n%2===0?1:2;}res.end('<button>Increment</button><output>0</output><script>document.querySelector("button").onclick=()=>document.querySelector("output").textContent='+value+'</script>');});s.listen(4173,'127.0.0.1');const close=()=>s.close(()=>process.exit());process.on('SIGTERM',close);process.on('SIGINT',close);`
		write(t, filepath.Join(repo, "server.mjs"), server)
		command(t, repo, "git", "init", "-q")
		command(t, repo, "git", "config", "user.email", "fixture@example.invalid")
		command(t, repo, "git", "config", "user.name", "Fixture")
		command(t, repo, "git", "add", ".")
		command(t, repo, "git", "commit", "-qm", "fixture")
		pin := Repository{Root: repo, Commit: command(t, repo, "git", "rev-parse", "HEAD"), Tree: command(t, repo, "git", "rev-parse", "HEAD^{tree}")}
		source := filepath.Clean(filepath.Join("..", ".."))
		exe := filepath.Join(tools, "corvint-tests-accept")
		falsify := filepath.Join(tools, "corvint-behavior-falsify")
		command(t, source, "go", "build", "-o", exe, "./cmd/corvint-tests-accept")
		command(t, source, "go", "build", "-o", falsify, "./cmd/corvint-behavior-falsify")
		// The approved hook is fixture code, with actual Playwright runs and retained
		// native report bytes. Writer credential sentinels must be absent in both roles.
		hook := filepath.Join(tools, "hook")
		cleanup := filepath.Join(tools, "cleanup")
		js := filepath.Join(tools, "hook.mjs")
		hookJS := `import fs from 'node:fs';import {createServer} from 'node:http';import {spawn} from 'node:child_process';import crypto from 'node:crypto';
 if(process.env.GITHUB_TOKEN||process.env.NODE_OPTIONS)process.exit(9);
 let data='';for await(const c of process.stdin)data+=c;const inv=JSON.parse(data);
 const server=createServer((req,res)=>res.end('<button id="increment">Increment</button><output>0</output><script>document.querySelector("button").onclick=()=>document.querySelector("output").textContent=2</script>'));
 let child;const close=async()=>{if(child&&!child.killed)child.kill('SIGTERM');await new Promise(r=>server.close(r));};
 process.on('SIGTERM',()=>close().then(()=>process.exit(143)));process.on('SIGINT',()=>close().then(()=>process.exit(130)));
 try{await new Promise(r=>server.listen(4173,'127.0.0.1',r));child=spawn(` + quoted(node) + `,[` + quoted(cli) + `,'test','--config',` + quoted(filepath.Join(repo, "playwright.config.mjs")) + `,'--reporter=json','--retries=0','--workers=1','--trace=off','--output',process.cwd()+'/out','--grep',inv.target.test_title+'$'],{cwd:` + quoted(repo) + `,env:{...process.env,PLAYWRIGHT_BROWSERS_PATH:` + quoted(filepath.Join(cacheDir, "ms-playwright")) + `}});let raw='';for await(const chunk of child.stdout)raw+=chunk;let err='';for await(const chunk of child.stderr)err+=chunk;await new Promise(r=>child.exitCode!==null?r():child.once('exit',r));
 const native=JSON.parse(raw);let spec;for(const suite of native.suites){for(const row of suite.specs??[])if(row.title===inv.target.test_title)spec=row;}if(!spec)throw Error('missing native test');const result=spec.tests[0].results[0];const passed=result.status==='passed';const assertion=!passed&&JSON.stringify(result.errors??result.error??{}).includes('toHaveText');if(!passed&&!assertion)throw Error('not target assertion');
 fs.writeFileSync('native.json',raw);const digest='sha256:'+crypto.createHash('sha256').update(raw).digest('hex');console.log(JSON.stringify({schema:'corvint-browser-behavior-control-run/0',plan_digest:inv.plan_digest,perturbation_sha256:inv.control.perturbation_sha256,target:inv.target,runner:inv.runner,attempt:inv.attempt,retry:result.retry,native_receipt_sha256:digest,test_outcome:passed?'passed':'failed',target_observation:{criterion_id:inv.target.criterion_id,assertion_id:inv.target.assertion_id,state:passed?'passed':'failed',failure_kind:passed?'':'assertion'},unrelated:[],setup:[{id:'browser-navigation',state:'passed'}],artifacts:[{path:'native.json',sha256:digest}]}));
 }finally{await close();}
`
		write(t, js, hookJS)
		write(t, hook, "#!/bin/sh\nexec "+node+" "+js+"\n")
		write(t, cleanup, "#!/bin/sh\n[ -z \"$GITHUB_TOKEN\" ] && [ -z \"$NODE_OPTIONS\" ] || exit 9\nrm -rf native.json out\n")
		marker := filepath.Join(workspace, behaviorfalsify.MarkerName)
		write(t, marker, "disposable fixture\n")
		version := strings.TrimSpace(command(t, repo, node, "--input-type=module", "-e", "import {chromium} from '"+browserModule+"';const b=await chromium.launch();console.log(b.version());await b.close();"))
		r := Request{Schema: RequestSchema, Product: pin, TestRepository: pin, Config: filepath.Join(repo, "playwright.config.mjs"), Package: filepath.Join(repo, "package.json"), Lockfile: filepath.Join(repo, "package-lock.json"), Runner: Command{Argv: []string{node, cli}, ExecutableSHA256: hashFile(t, node), EntrypointSHA256: hashFile(t, cli)}, Server: Command{Argv: []string{node, filepath.Join(repo, "server.mjs"), state}, ExecutableSHA256: hashFile(t, node)}, ReadyURL: "http://127.0.0.1:4173/", AppBuildDir: repo, RunnerVersion: "1.63.0", Environment: "disposable-loopback", Repeat: 2, TimeoutSeconds: 30}
		for _, file := range []string{"new.spec.mjs", "playwright.config.mjs", "package.json", "package-lock.json", "server.mjs", "contract.md"} {
			p := filepath.Join(repo, file)
			r.Inputs = append(r.Inputs, File{Path: p, SHA256: hashFile(t, p)})
		}
		for i, title := range []string{"stable", "nonasserting", "flaky"} {
			target := behaviorfalsify.TargetIdentity{ContractID: "counter", ContractSHA256: hashFile(t, filepath.Join(repo, "contract.md")), CriterionID: "counter-one", AssertionID: "output-one", ApplicationRevision: pin.Commit, TestRevision: pin.Commit, DocumentationRevision: pin.Commit, ContractFile: "contract.md", TestID: title, TestFile: "new.spec.mjs", TestLine: i + 2, TestTitle: title, Project: "chromium"}
			plan, e := behaviorfalsify.BuildPlan(behaviorfalsify.Request{Target: target, Runner: behaviorfalsify.RunnerIdentity{Runner: "playwright", RunnerVersion: "1.63.0", Browser: "chromium", BrowserVersion: version, ConfigFile: "playwright.config.mjs", ConfigSHA256: hashFile(t, r.Config), EnvironmentSHA256: Hash([]byte("{}"))}, Controls: []behaviorfalsify.ControlSpec{{ID: "response-two", Kind: behaviorfalsify.ChangedFixtureValue, Disposition: "run", Definition: map[string]string{"response": "two"}, Hook: &behaviorfalsify.Command{Path: hook, ExecutableSHA256: hashFile(t, hook)}, RequiredSetup: []string{"browser-navigation"}}}, Cleanup: behaviorfalsify.Command{Path: cleanup, ExecutableSHA256: hashFile(t, cleanup)}, DisposableRoot: workspace, Repositories: behaviorfalsify.RepositoryRoots{Application: repo, Test: repo, Documentation: repo}, MarkerSHA256: hashFile(t, marker), Attempts: 1, TimeoutSeconds: 30, WallClockSeconds: 60, ExternalState: "none"})
			if e != nil {
				t.Fatal(e)
			}
			tool := behaviorfalsify.ToolIdentity{Name: "corvint-behavior-falsify", Version: "1.0.0-rc.1", Revision: command(t, source, "git", "rev-parse", "HEAD"), Executable: hashFile(t, falsify), SourceDirty: strings.Contains(command(t, source, "go", "version", "-m", falsify), "vcs.modified=true")}
			r.Tests = append(r.Tests, Test{ID: title, File: filepath.Join(repo, "new.spec.mjs"), Line: i + 2, Title: title, Project: "chromium", Control: &plan, Tool: &tool, ControlCommand: &Command{Argv: []string{falsify}, ExecutableSHA256: tool.Executable}})
		}
		t.Setenv("GITHUB_TOKEN", "credential-sentinel")
		t.Setenv("NODE_OPTIONS", "credential-sentinel")
		if e := Validate(r); e != nil {
			t.Fatal(e)
		}
		report, e := Execute(context.Background(), r, Digest(r), exe, "fixture-build")
		if e != nil {
			t.Fatal(e)
		}
		b, _ := json.MarshalIndent(report, "", "  ")
		if output := os.Getenv("CORVINT_ACCEPTANCE_REPORT"); output != "" {
			write(t, output, string(b))
		}
		for i, want := range []string{"blocked", "rejected", "rejected"} {
			if report.Assessments[i].Verdict != want {
				t.Fatalf("%s want%s report%s", r.Tests[i].ID, want, b)
			}
		}
		t.Run("NEA-V0-005/requested-order-probes", func(t *testing.T) {
			if len(report.Runs) != 6 || report.Runs[2].Kind != "probe-original" || report.Runs[3].Kind != "probe-reversed" || report.Runs[4].Kind != "probe-isolated" || report.Runs[5].TestID != "flaky" {
				t.Fatalf("order evidence missing: %s", b)
			}
			t.Run("NEA-V0-008/attached-order-evidence", func(t *testing.T) {
				// The fixture server alternates per request across all runs, so the
				// single-file probes cannot vary order and isolation stays mixed.
				o := report.Assessments[2].Order
				if o.RequestedFileOrder != "not-varied" || o.Status != "nondeterministic-in-isolation" || report.Assessments[0].Order.Status != "not-probed" {
					t.Fatalf("flaky order evidence not attached: %+v", o)
				}
				for _, a := range report.Assessments {
					if a.Repeats.Passed+a.Repeats.Failed+a.Repeats.Other != r.Repeat || a.Repeats.MinDurationMS == nil || a.Cleanup == "" {
						t.Fatalf("per-test repeat evidence incomplete: %+v", a)
					}
				}
			})
			t.Run("NEA-V0-003/actual-negative-controls", func(t *testing.T) {
				if report.Controls[0].Status != "killed" || report.Controls[1].Status != "survived" {
					t.Fatalf("actual controls did not qualify: %s", b)
				}
			})
		})
		for _, run := range report.Runs {
			if run.ObservedSchedule != nil {
				t.Fatal("owned provider invented schedule")
			}
			want := 3
			if run.Kind == "probe-isolated" {
				want = 1
			}
			if len(run.Rows) != want || len(run.Reasons) != 0 {
				t.Fatalf("missing actual new test or isolation not observed: %s", b)
			}
		}
		t.Run("NEA-V0-007/fixed-body", func(t *testing.T) {
			if !strings.Contains(report.Body, "| `stable` | blocked |") || !strings.Contains(report.Body, "| `nonasserting` | rejected |") || !strings.Contains(report.Body, "| `flaky` | rejected |") {
				t.Fatal("fixed body missing verdicts")
			}
			if !strings.Contains(report.Body, pin.Commit) || !strings.Contains(report.Body, "fixture-build") || strings.Contains(report.Body, "Increment") {
				t.Fatalf("fixed body lost bindings or admitted source text: %s", report.Body)
			}
		})
		t.Logf("actual browser negative assessment PASS Node=%s Chromium=%s; accepted freshness UNKNOWN; exact report retained when requested", report.Runs[0].NodeVersion, version)

	})
}
func quoted(s string) string { b, _ := json.Marshal(s); return string(b) }

func canonicalTemp(t *testing.T) string {
	t.Helper()
	p, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	return p
}

func TestNEAV0001ImmutableGitInputs(t *testing.T) {
	repo := canonicalTemp(t)
	script := filepath.Join(repo, "runner")
	write(t, script, "#!/bin/sh\nexit 0\n")
	for _, p := range []string{"new.spec.js", "config.js", "package.json", "lock.json"} {
		write(t, filepath.Join(repo, p), "{}\n")
	}
	command(t, repo, "git", "init", "-q")
	command(t, repo, "git", "config", "user.email", "fixture@example.invalid")
	command(t, repo, "git", "config", "user.name", "Fixture")
	command(t, repo, "git", "add", ".")
	command(t, repo, "git", "commit", "-qm", "pinned")
	pin := Repository{Root: repo, Commit: command(t, repo, "git", "rev-parse", "HEAD"), Tree: command(t, repo, "git", "rev-parse", "HEAD^{tree}")}
	r := Request{Schema: RequestSchema, Product: pin, TestRepository: pin, Config: filepath.Join(repo, "config.js"), Package: filepath.Join(repo, "package.json"), Lockfile: filepath.Join(repo, "lock.json"), Runner: Command{Argv: []string{script}, ExecutableSHA256: hashFile(t, script)}, Server: Command{Argv: []string{script}, ExecutableSHA256: hashFile(t, script)}, ReadyURL: "http://127.0.0.1:4173/", AppBuildDir: repo, RunnerVersion: "fixture", Environment: "local", Repeat: 2, TimeoutSeconds: 1, Tests: []Test{{ID: "counter", File: filepath.Join(repo, "new.spec.js"), Line: 1, Title: "counter"}}}
	for _, p := range []string{"new.spec.js", "config.js", "package.json", "lock.json"} {
		path := filepath.Join(repo, p)
		r.Inputs = append(r.Inputs, File{Path: path, SHA256: hashFile(t, path)})
	}
	if e := approval(r, Digest(r)); e != nil {
		t.Fatal(e)
	}
	original := r.Tests[0].ID
	r.Tests[0].ID = "counter\nmalicious"
	if Validate(r) == nil {
		t.Fatal("body interpolation admitted")
	}
	r.Tests[0].ID = original
	old := r.ReadyURL
	r.ReadyURL = "https://example.invalid/"
	if Validate(r) == nil {
		t.Fatal("nonloopback admitted")
	}
	r.ReadyURL = old
	r.Inputs[0].SHA256 = Hash([]byte("wrong"))
	if Validate(r) == nil {
		t.Fatal("hash drift admitted")
	}
	r.Inputs[0].SHA256 = hashFile(t, r.Inputs[0].Path)
	// A local replacement ref must not reinterpret the immutable pinned commit.
	emptyTree := command(t, repo, "git", "mktree")
	replacement := command(t, repo, "git", "commit-tree", emptyTree, "-m", "replacement")
	command(t, repo, "git", "replace", pin.Commit, replacement)
	if command(t, repo, "git", "rev-parse", "HEAD^{tree}") == pin.Tree {
		t.Fatal("fixture replacement not active")
	}
	if actual, e := git(repo, "rev-parse", "HEAD^{tree}"); e != nil || actual != pin.Tree {
		t.Fatal("replacement object changed immutable observation")
	}
	if e := Validate(r); e != nil {
		t.Fatalf("replacement-free source check failed: %v", e)
	}
	write(t, r.Tests[0].File, "changed\n")
	if Validate(r) == nil {
		t.Fatal("dirty source admitted")
	}
}

func TestNEAV0001BoundedGitCapture(t *testing.T) {
	dir := canonicalTemp(t)
	write(t, filepath.Join(dir, "git"), "#!/bin/sh\n/usr/bin/yes x\n")
	t.Setenv("PATH", dir)
	if _, err := git(dir, "status"); err == nil {
		t.Fatal("unbounded capture admitted")
	}
}

func TestNEAV0008AttachedOrderEvidence(t *testing.T) {
	t.Run("NEA-V0-008/attached-order-evidence", func(t *testing.T) {
		r := Request{Environment: "local", Repeat: 2, Tests: []Test{{ID: "a", File: "/r/a.spec.mjs"}, {ID: "b", File: "/r/b.spec.mjs"}}}
		life := Cleanup{OwnedGroup: true, Descendants: &procgroup.DescendantObservation{Absent: true}}
		row := func(id string, state jstestprovider.ExecutionState) Row {
			return Row{ID: id, State: state, Attempts: 1, DurationMS: 10}
		}
		pass, fail := jstestprovider.StatePassed, jstestprovider.StateFailed
		base := []Run{
			{Kind: "repeat", Rows: []Row{row("a", pass), row("b", pass)}, Cleanup: life},
			{Kind: "repeat", Rows: []Row{row("a", pass), row("b", fail)}, Cleanup: life},
			{Kind: "probe-original", Rows: []Row{row("a", pass), row("b", fail)}, Cleanup: life},
			{Kind: "probe-reversed", Rows: []Row{row("a", pass), row("b", pass)}, Cleanup: life},
		}
		isolated := func(states ...jstestprovider.ExecutionState) []Run {
			out := append([]Run{}, base...)
			for _, s := range states {
				out = append(out, Run{Kind: "probe-isolated", TestID: "b", Rows: []Row{row("b", s)}, Cleanup: life})
			}
			return out
		}
		for name, c := range map[string]struct {
			runs []Run
			want string
		}{
			"order-dependent":  {isolated(pass, pass), "failures-not-reproduced-in-isolation"},
			"nondeterministic": {isolated(pass, fail), "nondeterministic-in-isolation"},
			"fails-alone":      {isolated(fail, fail), "fails-in-isolation"},
			"missing":          {isolated(pass), "isolation-incomplete"},
			"not-decided":      {isolated(pass, jstestprovider.StateSkipped), "isolation-incomplete"},
		} {
			o := orderEvidence(c.runs, r, "b")
			if o.Status != c.want || o.RequestedFileOrder != "outcome-differs" || o.OriginalState != "failed" || o.ReversedState != "passed" {
				t.Fatalf("%s: %+v", name, o)
			}
		}
		if o := orderEvidence(isolated(pass, pass), r, "a"); o.Status != "not-probed" || len(o.IsolatedStates) != 0 {
			t.Fatalf("stable test received another test's isolation evidence: %+v", o)
		}
		// Isolation runs for another test do not carry that test's cleanup onto this one.
		runs := isolated(pass, pass)
		runs[len(runs)-1].Cleanup = Cleanup{OwnedGroup: true, Descendants: &procgroup.DescendantObservation{}}
		report := Report{Runs: runs, Controls: []Control{{Status: "killed", Cleanup: life}, {Status: "killed", Cleanup: life}}}
		classify(&report, r)
		if report.Assessments[0].Cleanup != "observed-absent" || report.Assessments[1].Cleanup != "survivors" || report.Assessments[1].Verdict != "rejected" {
			t.Fatalf("isolation cleanup attribution: %+v", report.Assessments)
		}
		for _, title := range []string{"chromium new.spec.mjs b", "chromium new.spec.mjs b @smoke @fast"} {
			if !regexp.MustCompile(isolationPattern("b")).MatchString(title) {
				t.Fatalf("isolation pattern missed %q", title)
			}
		}
		if regexp.MustCompile(isolationPattern("b")).MatchString("chromium new.spec.mjs ab") || regexp.MustCompile(isolationPattern("a.b")).MatchString("chromium x axb") {
			t.Fatal("isolation pattern matched another title")
		}
	})
}

func TestNEAV0009ChangeRequestBody(t *testing.T) {
	t.Run("NEA-V0-009/change-request-body", func(t *testing.T) {
		oid := strings.Repeat("a", 40)
		r := Request{Environment: "local", Repeat: 2, Tests: []Test{{ID: "flaky", File: "/r/a.spec.mjs", Title: "synthetic-author-prose"}}}
		life := Cleanup{OwnedGroup: true, Descendants: &procgroup.DescendantObservation{Absent: true}}
		report := Report{Product: Repository{Commit: oid, Tree: oid}, TestRepository: Repository{Commit: strings.Repeat("b", 40), Tree: oid}, Environment: "local", Build: "corvint-tests-accept experimental/0 revision=abc dirty=false", ExecutableSHA256: "sha256:" + strings.Repeat("c", 64), RequestDigest: "sha256:" + strings.Repeat("d", 64), Unknowns: []string{"provider-per-test-freshness"},
			Runs: []Run{
				{Kind: "repeat", Rows: []Row{{ID: "flaky", State: jstestprovider.StatePassed, Attempts: 1, DurationMS: 812.4}}, Cleanup: life},
				{Kind: "repeat", Rows: []Row{{ID: "flaky", State: jstestprovider.StateFailed, Attempts: 1, DurationMS: 950.6}}, Cleanup: life, Reasons: []string{"bad|reason\ninjected"}},
				{Kind: "probe-isolated", TestID: "flaky", Rows: []Row{{ID: "flaky", State: jstestprovider.StatePassed, Attempts: 1}}, Cleanup: life},
				{Kind: "probe-isolated", TestID: "flaky", Rows: []Row{{ID: "flaky", State: jstestprovider.StatePassed, Attempts: 1}}, Cleanup: life},
			},
			Controls: []Control{{Status: "survived", Cleanup: life}}}
		classify(&report, r)
		for _, want := range []string{"**Overall verdict: rejected**", "| Product revision | `" + oid, "| Test-repository revision | `" + strings.Repeat("b", 40), "| Environment | `local` |", "| Corvint build | `corvint-tests-accept experimental/0 revision=abc dirty=false` |", "| `flaky` | rejected | 1/1/0 | 812-951 | SURVIVED | observed-absent | failures-not-reproduced-in-isolation | not-varied | passed, passed |", "negative-control-survived", "UNVALIDATED", "V1-0556"} {
			if !strings.Contains(report.Body, want) {
				t.Fatalf("body missing %q:\n%s", want, report.Body)
			}
		}
		if strings.Contains(report.Body, "synthetic-author-prose") || strings.Contains(report.Body, "bad|reason") {
			t.Fatalf("body admitted unvalidated text:\n%s", report.Body)
		}
		report.Build = "evil`|build\n"
		if body := renderBody(report, r); !strings.Contains(body, "| Corvint build | `UNVALIDATED` |") {
			t.Fatalf("unvalidated build rendered:\n%s", body)
		}
	})
}

func TestV10689BoundedWitnessCleanupCodec(t *testing.T) {
	const limitation = "Process rows are a bounded witness sample; omitted historical or resident identities are not an exhaustive process list."
	for _, state := range []string{"absent", "survivor", "failure"} {
		t.Run(state, func(t *testing.T) {
			observation := &procgroup.DescendantObservation{Scope: "observed-pid-start-identities", IntervalMS: 20, Absent: true, Processes: []procgroup.ObservedProcess{{PID: 42, ParentPID: 1, Start: "witness", State: "Z"}}, Failures: []string{}, Limitations: []string{limitation}}
			if state == "survivor" {
				observation.Absent = false
			}
			if state == "failure" {
				observation.Failures = []string{"resident overflow"}
			}
			original := Cleanup{OwnedGroup: true, Descendants: observation}
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			var decoded Cleanup
			if err := Decode(data, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Descendants == nil || len(decoded.Descendants.Processes) != 1 || len(decoded.Descendants.Limitations) != 1 || decoded.Descendants.Limitations[0] != limitation {
				t.Fatal("bounded witness or omission lost")
			}
			want := map[string]string{"absent": "observed-absent", "survivor": "survivors", "failure": "unknown"}[state]
			if got := cleanupState(decoded); got != want {
				t.Fatalf("got %s want %s", got, want)
			}
		})
	}
}

// Exercise the existing aggregate refusal through Execute, without a live
// provider: cancellation prevents worker launches but does not bypass encoding.
func TestV10689AggregateReportBound(t *testing.T) {
	repo := canonicalTemp(t)
	script := filepath.Join(repo, "runner")
	write(t, script, "#!/bin/sh\nexit 0\n")
	for _, p := range []string{"new.spec.js", "config.js", "package.json", "lock.json"} {
		write(t, filepath.Join(repo, p), "{}\n")
	}
	command(t, repo, "git", "init", "-q")
	command(t, repo, "git", "config", "user.email", "fixture@example.invalid")
	command(t, repo, "git", "config", "user.name", "Fixture")
	command(t, repo, "git", "add", ".")
	command(t, repo, "git", "commit", "-qm", "pinned")
	pin := Repository{Root: repo, Commit: command(t, repo, "git", "rev-parse", "HEAD"), Tree: command(t, repo, "git", "rev-parse", "HEAD^{tree}")}
	r := Request{Schema: RequestSchema, Product: pin, TestRepository: pin, Config: filepath.Join(repo, "config.js"), Package: filepath.Join(repo, "package.json"), Lockfile: filepath.Join(repo, "lock.json"), Runner: Command{Argv: []string{script}, ExecutableSHA256: hashFile(t, script)}, Server: Command{Argv: []string{script}, ExecutableSHA256: hashFile(t, script)}, ReadyURL: "http://127.0.0.1:4173/", AppBuildDir: repo, RunnerVersion: "fixture", Environment: "local", Repeat: 2, TimeoutSeconds: 1, Tests: []Test{{ID: "counter", File: filepath.Join(repo, "new.spec.js"), Line: 1, Title: "counter"}}}
	for _, p := range []string{"new.spec.js", "config.js", "package.json", "lock.json"} {
		path := filepath.Join(repo, p)
		r.Inputs = append(r.Inputs, File{Path: path, SHA256: hashFile(t, path)})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err := Execute(ctx, r, Digest(r), script, "bounded-fixture")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(report)
	if err != nil || len(data) > ReportLimit {
		t.Fatal("fitting aggregate refused")
	}
	if _, err := Execute(ctx, r, Digest(r), script, strings.Repeat("b", ReportLimit)); err == nil || err.Error() != "report-bound" {
		t.Fatalf("oversized aggregate did not refuse: %v", err)
	}
}
