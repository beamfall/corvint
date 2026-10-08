//go:build darwin || linux

package testrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

func TestExecutorHelper(t *testing.T) {
	if os.Getenv("CORVINT_EXEC_HELPER") != "1" {
		return
	}
	switch os.Getenv("CORVINT_EXEC_MODE") {
	case "graceful":
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt)
		exe, _ := os.Executable()
		child := exec.Command(exe, "-test.run=^TestExecutorHelper$")
		child.Env = append(os.Environ(), "CORVINT_EXEC_MODE=child")
		child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if e := child.Start(); e != nil {
			os.Exit(9)
		}
		if os.Getenv("CORVINT_GRACE_OVERFLOW") == "1" {
			go func() { fmt.Print(strings.Repeat("x", MaxReportBytes+1)) }()
		}
		<-signals
		_ = syscall.Kill(-child.Process.Pid, syscall.SIGKILL)
		_ = child.Wait()
		if path := os.Getenv("CORVINT_GRACE_MUTATE"); path != "" {
			_ = os.WriteFile(path, []byte("changed"), 0600)
		}
		fmt.Print("native cleanup complete")
		os.Exit(0)
	case "ignore":
		signal.Ignore(os.Interrupt)
		exe, _ := os.Executable()
		child := exec.Command(exe, "-test.run=^TestExecutorHelper$")
		child.Env = append(os.Environ(), "CORVINT_EXEC_MODE=child")
		if e := child.Start(); e != nil {
			os.Exit(9)
		}
		for {
			time.Sleep(time.Second)
		}

	case "mutate":
		if err := os.WriteFile(os.Getenv("CORVINT_EXEC_TARGET"), []byte("changed"), 0700); err != nil {
			os.Exit(9)
		}
	case "symlink":
		if err := os.Symlink(os.Getenv("CORVINT_EXEC_TARGET"), os.Getenv("CORVINT_EXEC_LINK")); err != nil {
			os.Exit(9)
		}
	case "parent":
		exe, _ := os.Executable()
		cmd := exec.Command(exe, "-test.run=^TestExecutorHelper$")
		cmd.Env = append(os.Environ(), "CORVINT_EXEC_MODE=child")
		if err := cmd.Start(); err != nil {
			os.Exit(9)
		}
		_ = os.WriteFile(os.Getenv("CORVINT_EXEC_TARGET")+".pid", []byte(fmt.Sprint(cmd.Process.Pid)), 0600)
		for {
			time.Sleep(time.Second)
		}
	case "child":
		f, err := os.OpenFile(os.Getenv("CORVINT_EXEC_TARGET"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			os.Exit(9)
		}
		for {
			_, _ = f.Write([]byte("x"))
			time.Sleep(10 * time.Millisecond)
		}
	case "detach":
		// A Setpgid child leaves the leader's group; its setsid child leaves
		// the session too. The marker is the readiness signal.
		exe, _ := os.Executable()
		child := exec.Command(exe, "-test.run=^TestExecutorHelper$")
		child.Env = append(os.Environ(), "CORVINT_EXEC_MODE=orphaner")
		child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if child.Start() != nil {
			os.Exit(9)
		}
		if os.Getenv("CORVINT_EXEC_EXIT") == "1" {
			for {
				if _, e := os.Stat(os.Getenv("CORVINT_EXEC_TARGET")); e == nil {
					fmt.Print("native result")
					os.Exit(0)
				}
				time.Sleep(5 * time.Millisecond)
			}
		}
		for {
			time.Sleep(time.Second)
		}
	case "orphaner":
		exe, _ := os.Executable()
		sleeper := exec.Command(exe, "-test.run=^TestExecutorHelper$")
		sleeper.Env = append(os.Environ(), "CORVINT_EXEC_MODE=sleep")
		sleeper.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if sleeper.Start() != nil {
			os.Exit(9)
		}
		target := os.Getenv("CORVINT_EXEC_TARGET")
		_ = os.WriteFile(target+".tmp", []byte(fmt.Sprintf("%d %d", os.Getpid(), sleeper.Process.Pid)), 0600)
		_ = os.Rename(target+".tmp", target)
		for {
			time.Sleep(time.Second)
		}
	case "sleep":
		time.Sleep(10 * time.Minute)
		os.Exit(0)
	case "env":
		fmt.Print(os.Getenv("PATH") + "\n" + os.Getenv("JAVA_HOME") + "\n" + os.Getenv("CORVINT_HOST_SECRET"))
	}
	fmt.Print("native result")
	os.Exit(0)
}
func executorRequest(t *testing.T) (Request, Invocation) {
	t.Helper()
	root := t.TempDir()
	// TempDir can use /var, which is a macOS symlink. The input binding intentionally
	// rejects symlink components, so use its canonical physical path.
	root, _ = filepath.EvalSymlinks(root)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	r := Request{Runner: "probe", Root: root, Executable: exe, ExecutableSha256: Digest(b), InputFiles: map[string]string{"source": Digest([]byte("source"))}, ReportDir: filepath.Join(root, "report"), TimeoutSeconds: 5}
	inv := Invocation{Argv: []string{"-test.run=^TestExecutorHelper$"}, Environment: map[string]string{"CORVINT_EXEC_HELPER": "1"}}
	return r, inv
}
func TestExecuteIndependentBindings(t *testing.T) {
	for _, target := range []string{"config", "reporter", "source", "tool"} {
		t.Run(target, func(t *testing.T) {
			r, inv := executorRequest(t)
			path := filepath.Join(r.Root, target)
			initial := []byte("source")
			if err := os.WriteFile(path, initial, 0700); err != nil {
				t.Fatal(err)
			}
			switch target {
			case "config":
				r.Config = target
				r.ConfigSha256 = Digest(initial)
			case "reporter":
				r.Reporter = path
				r.ReporterSha256 = Digest(initial)
			case "tool":
				r.Tools = map[string]Tool{"helper": {path, Digest(initial)}}
			}
			inv.Environment["CORVINT_EXEC_MODE"] = "mutate"
			inv.Environment["CORVINT_EXEC_TARGET"] = path
			out, err := Execute(context.Background(), r, inv)
			if err == nil || len(out.Input.ExecutionProblems) == 0 {
				t.Fatalf("mutation accepted: %+v %v", out, err)
			}
			o := Normalize(out.Input, Observation{Complete: true, RetryInformation: NotApplicable, Tests: []Test{{ID: "test", State: Passed, Attempts: []Attempt{{State: Passed}}}}})
			if o.Complete || o.Tests[0].State != Unknown || o.Tests[0].Attempts[0].State != Unknown {
				t.Fatalf("invalid result survived: %+v", o)
			}
		})
	}
	r, inv := executorRequest(t)
	r.Tools = map[string]Tool{"unused": {filepath.Join(r.Root, "missing"), strings.Repeat("0", 64)}}
	if out, err := Execute(context.Background(), r, inv); err == nil || len(out.Phases) != 0 {
		t.Fatalf("unverified unused tool admitted: %v", err)
	}
}
func TestExecuteEnvironmentAndArtifactBounds(t *testing.T) {
	t.Run("declared environment", func(t *testing.T) {
		r, inv := executorRequest(t)
		dir := filepath.Join(r.Root, "helpers")
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		helper := filepath.Join(dir, "java")
		if err := os.WriteFile(helper, []byte("helper"), 0700); err != nil {
			t.Fatal(err)
		}
		r.Tools = map[string]Tool{"java": {helper, Digest([]byte("helper"))}}
		t.Setenv("CORVINT_HOST_SECRET", "must not inherit")
		inv.Environment["CORVINT_EXEC_MODE"] = "env"
		inv.Environment["JAVA_HOME"] = dir
		out, err := Execute(context.Background(), r, inv)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Dir(r.Executable) + string(os.PathListSeparator) + dir + "\n" + dir + "\nnative result"
		if string(out.Input.Stdout) != want {
			t.Fatalf("environment %q != %q", out.Input.Stdout, want)
		}
	})
	for _, name := range []string{".phase-00-stdout", "decoded/report.json"} {
		t.Run(name, func(t *testing.T) {
			r, inv := executorRequest(t)
			outside := filepath.Join(r.Root, "outside")
			if err := os.WriteFile(outside, []byte("preserve"), 0600); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(r.ReportDir, name)
			if strings.HasPrefix(name, "decoded/") {
				// A parent-directory symlink is rejected by the rooted decoded write.
				link = filepath.Join(r.ReportDir, "decoded")
				inv.Phases = []Phase{{Kind: "TEST", Tool: "primary", Argv: inv.Argv, Environment: inv.Environment, StdoutReport: name}}
				inv.Argv = nil
			}
			inv.Environment["CORVINT_EXEC_MODE"] = "symlink"
			inv.Environment["CORVINT_EXEC_TARGET"] = outside
			inv.Environment["CORVINT_EXEC_LINK"] = link
			if _, err := Execute(context.Background(), r, inv); err == nil {
				t.Fatal("symlink accepted")
			}
			b, _ := os.ReadFile(outside)
			if string(b) != "preserve" {
				t.Fatal("external artifact overwritten")
			}
		})
	}
	t.Run("missing report pattern", func(t *testing.T) {
		r, inv := executorRequest(t)
		inv.ReportPatterns = []string{"missing-*.xml"}
		if _, err := Execute(context.Background(), r, inv); err == nil {
			t.Fatal("missing glob accepted")
		}
	})
}
func TestExecuteRetiresDescendants(t *testing.T) {
	for _, mode := range []string{"timeout", "interrupt"} {
		t.Run(mode, func(t *testing.T) {
			r, inv := executorRequest(t)
			r.TimeoutSeconds = 1
			heartbeat := filepath.Join(r.Root, "heartbeat")
			inv.Environment["CORVINT_EXEC_MODE"] = "parent"
			inv.Environment["CORVINT_EXEC_TARGET"] = heartbeat
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			if mode == "interrupt" {
				go func() {
					defer close(done)
					for {
						if _, e := os.Stat(heartbeat); e == nil {
							cancel()
							return
						}
						select {
						case <-ctx.Done():
							return
						case <-time.After(5 * time.Millisecond):
						}
					}
				}()
			} else {
				close(done)
			}
			out, err := Execute(ctx, r, inv)
			<-done
			if err != nil {
				t.Fatal(err)
			}
			if mode == "timeout" && !out.Input.TimedOut || mode == "interrupt" && !out.Input.Interrupted {
				t.Fatalf("lifecycle not retained: %+v", out.Input)
			}
			a, err := os.ReadFile(heartbeat)
			if err != nil {
				t.Fatal(err)
			}
			time.Sleep(100 * time.Millisecond)
			b, _ := os.ReadFile(heartbeat)
			if len(a) != len(b) {
				t.Fatal("descendant continued after executor returned")
			}
		})
	}
}

func TestExecuteFixedTemplatesAndReportInventory(t *testing.T) {
	t.Run("template mutation", func(t *testing.T) {
		r, inv := executorRequest(t)
		inv.Files = map[string][]byte{"owned-reporter": []byte("fixed")}
		inv.Environment["CORVINT_EXEC_MODE"] = "mutate"
		inv.Environment["CORVINT_EXEC_TARGET"] = filepath.Join(r.ReportDir, "owned-reporter")
		out, err := Execute(context.Background(), r, inv)
		if err == nil || len(out.Input.ExecutionProblems) == 0 {
			t.Fatalf("template drift admitted: %+v %v", out, err)
		}
	})
	for _, which := range []string{"request", "invocation", "patterns"} {
		t.Run("duplicate "+which, func(t *testing.T) {
			r, inv := executorRequest(t)
			switch which {
			case "request":
				r.ReportFiles = []string{"a", "a"}
			case "invocation":
				inv.ReportPaths = []string{"a", "a"}
			case "patterns":
				inv.ReportPatterns = []string{"*.xml", "*.xml"}
			}
			out, err := Execute(context.Background(), r, inv)
			if err == nil || len(out.Phases) != 0 {
				t.Fatalf("duplicate inventory executed: %+v %v", out, err)
			}
		})
	}
	t.Run("intentional cross inventory overlap", func(t *testing.T) {
		r, inv := executorRequest(t)
		inv.Phases = []Phase{{Kind: "TEST", Tool: "primary", Argv: inv.Argv, Environment: inv.Environment, StdoutReport: "native.xml"}}
		inv.Argv = nil
		inv.ReportPaths = []string{"native.xml"}
		r.ReportFiles = []string{"native.xml"}
		inv.ReportPatterns = []string{"*.xml"}
		out, err := Execute(context.Background(), r, inv)
		if err != nil || len(out.Input.Reports) != 1 {
			t.Fatalf("overlap refused: %+v %v", out, err)
		}
	})
}
func TestBoundedReportEnumeration(t *testing.T) {
	base := t.TempDir()
	for i := 0; i < MaxReports+1; i++ {
		if err := os.WriteFile(filepath.Join(base, fmt.Sprintf("%03d.xml", i)), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	scanned := 0
	if _, err = boundedReportGlob(context.Background(), root, "*.xml", &scanned); err == nil {
		t.Fatal("unbounded match inventory")
	}
	scanned = MaxTests
	if _, err = boundedReportGlob(context.Background(), root, "*.absent", &scanned); err == nil {
		t.Fatal("nonmatching scan not bounded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scanned = 0
	if _, err = boundedReportGlob(ctx, root, "*.xml", &scanned); err != context.Canceled || scanned != 0 {
		t.Fatalf("cancelled enumeration continued: %d %v", scanned, err)
	}
	outside := t.TempDir()
	if err = os.Symlink(outside, filepath.Join(base, "linked")); err != nil {
		t.Fatal(err)
	}
	scanned = 0
	if _, err = boundedReportGlob(context.Background(), root, "linked/*.xml", &scanned); err == nil || scanned != 0 {
		t.Fatal("symlink parent traversed")
	}
	if err = checkReportInventory(Request{}, Invocation{ReportPatterns: []string{"*/report.xml"}}); err == nil {
		t.Fatal("wildcard parent accepted")
	}
}

// Runtime jars are individually pinned artifacts, not report/source payloads.
func TestLargePinnedArtifactSeparateFromSourceBound(t *testing.T) {
	root, resolveErr := filepath.EvalSymlinks(t.TempDir())
	if resolveErr != nil {
		t.Fatal(resolveErr)
	}
	data := bytes.Repeat([]byte{0x5a}, MaxReportBytes+1)
	name := filepath.Join(root, "runtime.jar")
	if e := os.WriteFile(name, data, 0600); e != nil {
		t.Fatal(e)
	}
	if e := checkPinnedFile(root, "runtime.jar", Digest(data)); e != nil {
		t.Fatal(e)
	}
	r, e := os.OpenRoot(root)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	if e = checkInputs(r, map[string]string{"runtime.jar": Digest(data)}); e == nil {
		t.Fatal("source inventory byte bound widened")
	}
	if e = checkPinnedFile(root, name, Digest([]byte("changed"))); e == nil {
		t.Fatal("changed runtime accepted")
	}
	f, e := os.Create(filepath.Join(root, "oversized.jar"))
	if e != nil {
		t.Fatal(e)
	}
	e = f.Truncate(MaxPinnedArtifactBytes + 1)
	f.Close()
	if e != nil {
		t.Fatal(e)
	}
	if e = checkPinnedFile(root, "oversized.jar", Digest(nil)); e == nil {
		t.Fatal("oversized pinned artifact accepted")
	}
}

func TestExecuteRetainsCallerSelectionMetadata(t *testing.T) {
	r, inv := executorRequest(t)
	r.Target = "declared-target"
	r.Project = "retained-source-file"
	r.Selectors = []string{"./package/..."}
	out, e := Execute(context.Background(), r, inv)
	if e != nil {
		t.Fatal(e)
	}
	r.Selectors[0] = "changed-after-execution"
	if out.Input.Target != "declared-target" || out.Input.SourceRoot != r.Root || out.Input.SourceFile != "retained-source-file" || len(out.Input.Selectors) != 1 || out.Input.Selectors[0] != "./package/..." {
		t.Fatalf("caller metadata was lost, interpreted, or aliased: %+v", out.Input)
	}
}

func TestGracefulInterruptLifecycle(t *testing.T) {
	for _, mode := range []string{"timeout", "interrupt", "overflow", "postbinding", "uncooperative", "normal"} {
		t.Run(mode, func(t *testing.T) {
			r, inv := executorRequest(t)
			r.TimeoutSeconds = 1
			inv.GracefulInterrupt = true
			target := filepath.Join(r.Root, "heartbeat")
			inv.Environment["CORVINT_EXEC_TARGET"] = target
			inv.Environment["CORVINT_EXEC_MODE"] = "graceful"
			if mode == "uncooperative" {
				inv.Environment["CORVINT_EXEC_MODE"] = "ignore"
			}
			if mode == "normal" {
				// The race runtime waits one second before a successful helper exit.
				r.TimeoutSeconds = 5
				inv.Environment["CORVINT_EXEC_MODE"] = ""
			}
			if mode == "overflow" {
				inv.Environment["CORVINT_GRACE_OVERFLOW"] = "1"
			}
			if mode == "postbinding" {
				inv.Environment["CORVINT_GRACE_MUTATE"] = filepath.Join(r.Root, "source")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			if mode == "interrupt" {
				go func() {
					defer close(done)
					for {
						if _, e := os.Stat(target); e == nil {
							cancel()
							return
						}
						select {
						case <-ctx.Done():
							return
						case <-time.After(time.Millisecond * 5):
						}
					}
				}()
			} else {
				close(done)
			}
			start := time.Now()
			out, e := Execute(ctx, r, inv)
			cancel()
			<-done
			if time.Since(start) > 8*time.Second {
				t.Fatal("graceful fallback exceeded bound")
			}
			if mode == "postbinding" {
				if e == nil || len(out.Input.ExecutionProblems) == 0 {
					t.Fatal("post-cleanup input mutation escaped binding")
				}
			} else if e != nil {
				t.Fatal(e)
			}
			if mode == "normal" {
				if out.Input.ExitCode != 0 || out.Input.TimedOut || out.Input.Interrupted {
					t.Fatal(out.Input)
				}
				return
			}
			if mode == "interrupt" && !out.Input.Interrupted {
				t.Fatal("missing interruption")
			}
			if mode == "overflow" && !out.Input.Overflow {
				t.Fatal("missing overflow")
			}
			if mode == "timeout" && !out.Input.TimedOut {
				t.Fatal("missing timeout")
			}
			o := Normalize(out.Input, Observation{Complete: true, RetryInformation: NotApplicable, Tests: []Test{{ID: "test", State: Passed}}})
			if o.Complete {
				t.Fatal("cancelled observation admitted complete")
			}
			{
				a, _ := os.ReadFile(target)
				time.Sleep(50 * time.Millisecond)
				b, _ := os.ReadFile(target)
				if len(a) != len(b) {
					t.Fatal("detached child survived native cleanup")
				}
				if mode != "uncooperative" && mode != "overflow" && mode != "postbinding" && !strings.Contains(string(out.Input.Stdout), "native cleanup complete") {
					t.Fatal("native shutdown did not finish")
				}
			}
		})
	}
}

func TestHistoricalPlanByteIdentity(t *testing.T) {
	// Original retained Go and Playwright plans predate gracefulInterrupt.
	for _, raw := range []string{
		`{"profile":"corvint-test-runner-plan/0","request":{"expectedTests":["example.invalid/clamp::TestClampRepair","example.invalid/clamp::TestClampPreservation","example.invalid/clamp::TestClampNewBehavior"],"target":"","inputFiles":{"clamp.go":"ea671f58af1018e9197485ad9d6e2a12325f283b1c9fb3ffbbd33e1bde9425a6","go.mod":"ab0bea7f420c7b25873fe8c0eefe8e67b73f7276ad0e4893a92f42dc9a77a4ed","oracle_test.go":"3aad89ef4a04ec32a3b6bc66d6dd7beee126e70c82852fd6ecea867ae1f0117c"},"runner":"go-test","root":"/private/tmp/cem10-build/coherent-join/demo/candidate/sample","executable":"/opt/homebrew/Cellar/go/1.27.1/libexec/bin/go","executableSha256":"548608a910c46de32c65a3934f461b1787acf6ddd371044826068d8503b8509b","selectors":[".::TestClampRepair",".::TestClampPreservation",".::TestClampNewBehavior"],"project":".","config":"","configSha256":"","reporter":"","reporterSha256":"","reportFiles":null,"tools":null,"reportDir":"/private/tmp/cem10-build/coherent-join/runner-reports","timeoutSeconds":120},"invocation":{"outcomeNeutralExitCodes":null,"successExitCodes":[0],"failureExitCodes":[1],"argv":["test","-json","-count=1","-run","^(TestClampRepair|TestClampPreservation|TestClampNewBehavior)$","."],"phases":null,"files":null,"reportPaths":null,"reportPatterns":null,"environment":{"GOPROXY":"off","GOSUMDB":"off","GOTOOLCHAIN":"local"},"format":"go-test"}}`,
		`{"profile":"corvint-test-runner-plan/0","request":{"expectedTests":["eb96dfe6f8759b4426a9-1bad61f11ddefe3ea7aa::pinned-chrome"],"target":"","inputFiles":{"collection.spec.cjs":"bc556099b34ca66d7e59ee602755cdc8d89e0f8fe6b1d6d20e15ba25fc1ed60a","mixed.spec.cjs":"5a03a2c1d59b5bba15adffbd82b00b6dec459d6b9fe5e027c578f4a421b49343","pass.spec.cjs":"abea6773acd117be6c8e43753e889d5052246543ea3e0b0134fd1b19e71b4bb4","playwright.config.cjs":"d540a1a4316c6a04d4b2c12a367ea0a9e41aabadf9a7b46869ef8150e2b4610a","timeout.spec.cjs":"003dd8827c4fb416afa4b5dc5cc13f89bdb87ac00d7a429ee4fabbc8ecf74166","zero.spec.cjs":"fc4def867de8e018110d0711cda4b223e03f7401b19f3bfdb43aeb19b57bc5bb"},"runner":"playwright","root":"/private/tmp/cem10-build/browser-qualification/timeout-proof/fixture","executable":"/private/tmp/cem10-build/browser-qualification/timeout-proof/playwright.cjs","executableSha256":"588ae2e798d064e58bd41976cf3a1c52c9637bd2310a92b88508ef82dbba4e49","selectors":["timeout.spec.cjs"],"project":"pinned-chrome","config":"/private/tmp/cem10-build/browser-qualification/timeout-proof/fixture/playwright.config.cjs","configSha256":"d540a1a4316c6a04d4b2c12a367ea0a9e41aabadf9a7b46869ef8150e2b4610a","reporter":"","reporterSha256":"","reportFiles":null,"tools":{"chrome":{"executable":"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome","sha256":"4e0c9634bbafc4007a5d5d72cfcb7101fda2ac519daf7cf2beefcbbed4136361"},"node":{"executable":"/opt/homebrew/Cellar/node@22/22.23.3/bin/node","sha256":"0c61847de20184ae3f35c24be3a79682e25fccccccbcbd316b7e224e24fa8637"}},"reportDir":"/private/tmp/cem10-build/browser-qualification/timeout-proof/reports-timeout","timeoutSeconds":6},"invocation":{"outcomeNeutralExitCodes":null,"successExitCodes":[0],"failureExitCodes":[1],"argv":["test","--reporter=json","--config","/private/tmp/cem10-build/browser-qualification/timeout-proof/fixture/playwright.config.cjs","--project=pinned-chrome","^/private/tmp/cem10-build/browser-qualification/timeout-proof/fixture/timeout\\.spec\\.cjs$"],"phases":null,"files":{},"reportPaths":["playwright.json"],"reportPatterns":null,"environment":{"PLAYWRIGHT_JSON_OUTPUT_FILE":"/private/tmp/cem10-build/browser-qualification/timeout-proof/reports-timeout/playwright.json"},"format":"playwright"}}
`,
	} {
		var plan PlanDocument
		if e := DecodeDocument([]byte(raw), &plan); e != nil {
			t.Fatal(e)
		}
		var compact bytes.Buffer
		if e := json.Compact(&compact, []byte(raw)); e != nil {
			t.Fatal(e)
		}
		before := append([]byte{}, compact.Bytes()...)
		after, e := json.Marshal(plan)
		if e != nil || !bytes.Equal(before, after) {
			t.Fatalf("historical PlanDocument identity changed: %v", e)
		}
		var old map[string]json.RawMessage
		_ = json.Unmarshal(before, &old)
		encoded, _ := json.Marshal(plan.Invocation)
		if !bytes.Equal(old["invocation"], encoded) {
			t.Fatal("historical Invocation identity changed")
		}
		plan.Invocation.GracefulInterrupt = false
		encoded, _ = json.Marshal(plan)
		if !bytes.Equal(before, encoded) {
			t.Fatal("explicit false changed old identity")
		}
		plan.Invocation.GracefulInterrupt = true
		encoded, _ = json.Marshal(plan)
		if bytes.Equal(before, encoded) || !bytes.Contains(encoded, []byte(`"gracefulInterrupt":true`)) {
			t.Fatal("new lifecycle not bound into plan identity")
		}
		plan.Invocation.GracefulInterrupt = false
		plan.Invocation.RetireDetachedDescendants = false
		if encoded, _ = json.Marshal(plan); !bytes.Equal(before, encoded) {
			t.Fatal("explicit false retirement changed old identity")
		}
		plan.Invocation.RetireDetachedDescendants = true
		encoded, _ = json.Marshal(plan)
		if bytes.Equal(before, encoded) || !bytes.Contains(encoded, []byte(`"retireDetachedDescendants":true`)) {
			t.Fatal("detached retirement not bound into plan identity")
		}
	}
}

func TestExecuteExplicitPrimaryTestWithoutArguments(t *testing.T) {
	exe, err := exec.LookPath("pwd")
	if err != nil {
		t.Fatal(err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	request := func() Request {
		r, _ := executorRequest(t)
		r.Executable = exe
		r.ExecutableSha256 = Digest(binary)
		return r
	}
	r := request()
	inv := Invocation{Phases: []Phase{{Kind: "TEST", Tool: "primary", Argv: []string{}}}, SuccessExitCodes: []int{0}}
	result, err := Execute(context.Background(), r, inv)
	if err != nil || len(result.Phases) != 1 || result.Phases[0].ExitCode != 0 || string(result.Input.Stdout) != r.Root+"\n" || result.Phases[0].StdoutSha256 != Digest(result.Input.Stdout) {
		t.Fatal(result, err)
	}
	for _, tc := range []struct {
		name string
		inv  Invocation
		want string
	}{
		{"implicit-empty", Invocation{}, "empty runner invocation"},
		{"ambiguous", Invocation{Argv: []string{"-L"}, Phases: inv.Phases}, "ambiguous invocation phases"},
		{"empty-build", Invocation{Phases: []Phase{{Kind: "BUILD", Tool: "primary", Argv: []string{}}}}, "argv bound"},
		{"empty-discover", Invocation{Phases: []Phase{{Kind: "DISCOVER", Tool: "primary", Argv: []string{}}}}, "argv bound"},
		{"empty-decode", Invocation{Phases: []Phase{{Kind: "DECODE", Tool: "primary", Argv: []string{}}}}, "argv bound"},
		{"empty-auxiliary", Invocation{Phases: []Phase{{Kind: "TEST", Tool: "auxiliary", Argv: []string{}}}}, "argv bound"},
		{"implicit-primary-name", Invocation{Phases: []Phase{{Kind: "TEST", Argv: []string{}}}}, "argv bound"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := request()
			r.Tools = map[string]Tool{"auxiliary": {Executable: exe, Sha256: Digest(binary)}}
			result, err := Execute(context.Background(), r, tc.inv)
			if err == nil || !strings.Contains(err.Error(), tc.want) || len(result.Phases) != 0 {
				t.Fatal(result, err)
			}
		})
	}
}

// TestExecuteRefusesUnprovenRetirement keeps TRE-V0-025 closed off Darwin:
// a plan requesting detached retirement is refused before launch.
func TestExecuteRefusesUnprovenRetirement(t *testing.T) {
	if groupreap.RetirementSupported {
		t.Skip("platform retirement is proved by the Darwin tests")
	}
	r, inv := executorRequest(t)
	inv.RetireDetachedDescendants = true
	if _, err := Execute(context.Background(), r, inv); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unproven retirement admitted: %v", err)
	}
	if _, err := os.Stat(r.ReportDir); err == nil {
		t.Fatal("refusal created the report directory")
	}
}
