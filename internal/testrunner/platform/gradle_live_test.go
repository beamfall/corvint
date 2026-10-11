//go:build darwin || linux

package platform

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

const gradleLiveProof = `package proof;

import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.assertEquals;

class ProofTest {
    @Test void passes() { assertEquals(2, 1 + 1); }
    @Test void fails() { assertEquals(2, 1); }
    @Disabled("fixture") @Test void skipped() {}
}
`

// The hang body records its worker and ancestor process identities before it
// blocks, so the witness can prove those exact processes were retired.
const gradleLiveHang = `package proof;

import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;

class HangTest {
    @Test void hangs() throws Exception {
        StringBuilder ids = new StringBuilder();
        for (ProcessHandle p = ProcessHandle.current(); p != null; p = p.parent().orElse(null)) {
            ids.append(p.pid()).append('\n');
            if (ids.length() > 64) break;
        }
        Path marker = Path.of(System.getProperty("corvint.marker"));
        Files.writeString(marker.resolveSibling("ready.tmp"), ids.toString());
        Files.move(marker.resolveSibling("ready.tmp"), marker);
        Thread.sleep(600_000);
    }
}
`

// TestGradleLiveOwnership is the opt-in actual TRE-V0-042/043 witness. The
// operator names an installed Gradle launcher (CORVINT_TEST_GRADLE), a Java
// executable (CORVINT_TEST_JAVA) and a directory of JUnit Jupiter jars
// (CORVINT_TEST_GRADLE_LIBS); nothing is downloaded. Each run uses the shared
// executor with a per-run Gradle user home and must leave no owned process.
func TestGradleLiveOwnership(t *testing.T) {
	gradle, java, libs := os.Getenv("CORVINT_TEST_GRADLE"), os.Getenv("CORVINT_TEST_JAVA"), os.Getenv("CORVINT_TEST_GRADLE_LIBS")
	if gradle == "" || java == "" || libs == "" {
		t.Skip("installed Gradle, Java and JUnit libraries not admitted")
	}
	pin := func(path string) tr.Tool {
		path, e := filepath.EvalSymlinks(path)
		if e != nil {
			t.Fatal(e)
		}
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		return tr.Tool{Executable: path, Sha256: tr.Digest(b)}
	}
	launcher, jvm := pin(gradle), pin(java)
	// The Gradle start script needs basic POSIX utilities on the declared PATH.
	tools := map[string]tr.Tool{"java": jvm, "xargs": pin("/usr/bin/xargs")}
	jars, e := filepath.Glob(filepath.Join(libs, "*.jar"))
	if e != nil || len(jars) == 0 {
		t.Fatalf("no JUnit jars in %s: %v", libs, e)
	}
	out := os.Getenv("CORVINT_TEST_GRADLE_OUT")
	run := func(t *testing.T, name string, selectors []string, timeout int, interrupt bool) (tr.Execution, tr.Observation, error, []int) {
		root, e := filepath.EvalSymlinks(t.TempDir())
		if e != nil {
			t.Fatal(e)
		}
		marker := filepath.Join(root, "ready")
		files := map[string][]byte{
			"app/settings.gradle":                    []byte("rootProject.name = 'app'\n"),
			"app/build.gradle":                       []byte("plugins { id 'java' }\ndependencies { testImplementation fileTree(dir: 'libs', include: '*.jar') }\ntest { useJUnitPlatform(); systemProperty 'corvint.marker', '" + marker + "' }\n"),
			"app/src/test/java/proof/ProofTest.java": []byte(gradleLiveProof),
			"app/src/test/java/proof/HangTest.java":  []byte(gradleLiveHang),
		}
		for _, jar := range jars {
			b, e := os.ReadFile(jar)
			if e != nil {
				t.Fatal(e)
			}
			files["app/libs/"+filepath.Base(jar)] = b
		}
		inputs := map[string]string{}
		for n, b := range files {
			p := filepath.Join(root, filepath.FromSlash(n))
			if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, b, 0600); e != nil {
				t.Fatal(e)
			}
			inputs[n] = tr.Digest(b)
		}
		r := tr.Request{Runner: "gradle-junit", Root: root, Project: "app", Config: filepath.Join(root, "app", "build.gradle"), ConfigSha256: inputs["app/build.gradle"], Target: ":test", Selectors: selectors, Executable: launcher.Executable, ExecutableSha256: launcher.Sha256, Tools: tools, InputFiles: inputs, ReportDir: filepath.Join(root, "reports"), TimeoutSeconds: timeout}
		v, e := Build(r)
		if e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ready := make(chan []int, 1)
		go func() {
			for ctx.Err() == nil {
				if b, e := os.ReadFile(marker); e == nil {
					var ids []int
					for _, f := range strings.Fields(string(b)) {
						if n, e := strconv.Atoi(f); e == nil {
							ids = append(ids, n)
						}
					}
					ready <- ids
					if interrupt {
						cancel()
					}
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
			ready <- nil
		}()
		x, execErr := tr.Execute(ctx, r, v)
		cancel()
		ids := <-ready
		var o tr.Observation
		var parseErr error
		if execErr == nil {
			o, parseErr = Parse(x.Input)
			o = tr.Normalize(x.Input, o)
		}
		if _, e := os.Stat(filepath.Join(r.ReportDir, ".home", ".gradle", "daemon")); e != nil {
			t.Fatalf("Gradle did not use the per-run user home: %v", e)
		}
		if out != "" {
			errText := func(e error) string {
				if e == nil {
					return ""
				}
				return e.Error()
			}
			raw, e := json.MarshalIndent(struct {
				Request      tr.Request
				Invocation   tr.Invocation
				Execution    tr.Execution
				Stdout       string
				Stderr       string
				Observation  tr.Observation
				ExecuteError string
				ParseError   string
				ReadyIDs     []int
			}{r, v, x, string(x.Input.Stdout), string(x.Input.Stderr), o, errText(execErr), errText(parseErr), ids}, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(out, name+".json"), append(raw, '\n'), 0600); e != nil {
				t.Fatal(e)
			}
		}
		if parseErr != nil {
			t.Fatalf("%v stderr=%s", parseErr, x.Input.Stderr)
		}
		return x, o, execErr, ids
	}
	// alive reports a recorded descendant still present; ids[0] is the test
	// worker and the chain stops before the executor (this process) or init.
	alive := func(ids []int) []int {
		var live []int
		for _, id := range ids {
			if id == os.Getpid() || id <= 1 {
				break
			}
			if e := syscall.Kill(id, 0); e == nil || !errors.Is(e, syscall.ESRCH) {
				live = append(live, id)
			}
		}
		return live
	}
	t.Run("outcomes", func(t *testing.T) {
		x, o, e, _ := run(t, "outcomes", []string{"proof.ProofTest#passes", "proof.ProofTest#fails", "proof.ProofTest#skipped"}, 300, false)
		if e != nil {
			t.Fatalf("%v stderr=%s", e, x.Input.Stderr)
		}
		states := map[string]string{}
		for _, c := range o.Tests {
			states[c.ID] = c.State
		}
		want := map[string]string{"proof.ProofTest#passes()": tr.Passed, "proof.ProofTest#fails()": tr.Failed, "proof.ProofTest#skipped()": tr.Skipped}
		if x.Input.ExitCode != 1 || !o.Complete || len(states) != 3 {
			t.Fatalf("exit=%d %+v", x.Input.ExitCode, o)
		}
		for id, s := range want {
			if states[id] != s {
				t.Fatalf("%s=%q want %q: %v", id, states[id], s, states)
			}
		}
		if x.Retirement == nil || !x.Retirement.Clean() {
			t.Fatalf("retirement %+v", x.Retirement)
		}
	})
	t.Run("no match", func(t *testing.T) {
		x, o, e, _ := run(t, "no-match", []string{"proof.ProofTest#absent"}, 300, false)
		if e == nil && o.Complete {
			t.Fatalf("no-match selection completed: %+v", o)
		}
		if x.Input.ExitCode != 1 {
			t.Fatalf("exit=%d", x.Input.ExitCode)
		}
	})
	for _, tc := range []struct {
		name      string
		timeout   int
		interrupt bool
	}{{"timeout", 90, false}, {"interruption", 300, true}} {
		t.Run(tc.name, func(t *testing.T) {
			x, o, e, ids := run(t, tc.name, []string{"proof.HangTest#hangs"}, tc.timeout, tc.interrupt)
			if len(ids) < 2 {
				t.Fatalf("hang body never became ready: %v %v stderr=%s", ids, e, x.Input.Stderr)
			}
			if o.Complete || x.Input.ExitCode != -1 || (tc.interrupt && !x.Input.Interrupted) || (!tc.interrupt && !x.Input.TimedOut) {
				t.Fatalf("killed run reported as %+v exit=%d timedOut=%v interrupted=%v", o, x.Input.ExitCode, x.Input.TimedOut, x.Input.Interrupted)
			}
			if live := alive(ids); len(live) != 0 {
				t.Fatalf("owned Gradle processes survived: %v of %v", live, ids)
			}
			if x.Retirement == nil || len(x.Retirement.Unretired) != 0 {
				t.Fatalf("retirement %+v", x.Retirement)
			}
		})
	}
}
