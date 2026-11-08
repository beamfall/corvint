// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/langplugin/runnernames"
	"github.com/Beamfall/corvint/internal/testrunner/platform"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/testevidence"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

var (
	jvmClass    = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*(\.[A-Za-z_$][A-Za-z0-9_$]*)*$`)
	jvmMethodID = regexp.MustCompile(`^\[engine:junit-jupiter\]/\[class:([A-Za-z_$][A-Za-z0-9_$.]*)\]/\[method:([A-Za-z_$][A-Za-z0-9_$]*)\(\)\]$`)
	jvmSummary  = regexp.MustCompile(`^\[\s*([0-9]+) (containers found|tests found)\s*\]$`)
)

func jvmDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'f' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func jvmRelative(s string) bool {
	return s != "" && s != "." && path.Clean(s) == s && !strings.HasPrefix(s, "/") && s != ".." && !strings.HasPrefix(s, "../") && !strings.ContainsAny(s, "\\:\x00\r\n") && utf8.ValidString(s)
}

func jvmBlob(b testevidence.Blob) bool {
	r := testevidence.New("validation", "validation", strings.Repeat("a", 40))
	r.Loaded = testevidence.Component[testevidence.Blob]{State: testevidence.Present, Items: []testevidence.Blob{b}}
	return r.ValidateComponents() == nil
}

// jvmInventory parses ConsoleLauncher 1.11's --details=flat discovery output.
// Display names are deliberately opaque. Only simple Jupiter method IDs can
// be related without ambiguity to Gradle/Surefire's class#method report IDs.
// Factories, templates, nested/unknown engines and runtime-only tests refuse the
// whole inventory. Final native counts detect truncation, including at a line
// boundary; no successful process exit is inferred from stdout alone.
func jvmInventory(key tr.InventoryKey, output string, classes map[string]testevidence.Blob) (tr.RunnerInventory, error) {
	fail := func(reason string) (tr.RunnerInventory, error) {
		return tr.RunnerInventory{Key: key, Reason: "jvm-inventory:" + reason}, fmt.Errorf("jvm-inventory:%s", reason)
	}
	if len(output) == 0 || len(output) > tr.MaxReportBytes || !utf8.ValidString(output) || !strings.HasSuffix(output, "\n") || strings.ContainsAny(output, "\x00\x1b") {
		return fail("missing-malformed-or-capped")
	}
	inv := tr.RunnerInventory{Key: key, Complete: true}
	seen := map[string]bool{}
	containers, tests, wantContainers, wantTests := 0, 0, -1, -1
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if summary := jvmSummary.FindStringSubmatch(line); summary != nil {
			n, err := strconv.Atoi(summary[1])
			if err != nil || n > tr.MaxTests*2 {
				return fail("count-cap")
			}
			if summary[2] == "containers found" && wantContainers == -1 && wantTests == -1 {
				wantContainers = n
			} else if summary[2] == "tests found" && wantContainers >= 0 && wantTests == -1 {
				wantTests = n
			} else {
				return fail("summary-divergent")
			}
			continue
		}
		if wantContainers >= 0 {
			return fail("trailing-discovery")
		}
		pos := strings.LastIndex(line, " ([engine:")
		if pos < 1 || !strings.HasSuffix(line, ")") {
			return fail("unparseable-discovery")
		}
		id := line[pos+2 : len(line)-1]
		if seen[id] || len(seen) >= tr.MaxTests*2 {
			return fail("duplicate-or-capped")
		}
		seen[id] = true
		const engine = "[engine:junit-jupiter]"
		if id == engine || id == "[engine:junit-vintage]" || id == "[engine:junit-platform-suite]" {
			containers++
			continue
		}
		if strings.HasPrefix(id, engine+"/[class:") && strings.HasSuffix(id, "]") && strings.Count(id, "/") == 1 {
			class := strings.TrimSuffix(strings.TrimPrefix(id, engine+"/[class:"), "]")
			if !jvmClass.MatchString(class) || !jvmBlob(classes[class]) {
				return fail("class-source-unresolved")
			}
			containers++
			continue
		}
		method := jvmMethodID.FindStringSubmatch(id)
		if method == nil || !jvmBlob(classes[method[1]]) || !seen[engine+"/[class:"+method[1]+"]"] || !seen[engine] {
			return fail("unsupported-or-unmapped-test")
		}
		tests++
		if key.Runner != runnernames.RunnerJunitPlatform {
			id = method[1] + "#" + method[2]
		}
		inv.Tests = append(inv.Tests, tr.InventoryTest{ID: id, Path: classes[method[1]].Path.Lexical})
	}
	if tests != wantTests || containers != wantContainers || tests == 0 {
		return fail("missing-or-divergent-summary")
	}
	canonical, err := tr.CanonicalInventory(inv, key)
	if err != nil {
		return fail(err.Error())
	}
	return canonical, nil
}

// jvmGradleInventory consumes the selected native task's complete dry-run XML
// artifact set (a single suite or a testsuites envelope). A ConsoleLauncher
// inventory is insufficient even when its own counts are internally consistent.
// The host owes fresh artifact capture and successful process retirement, as
// for native execution. Unsupported Gradle dry-run versions yield no inventory.
func jvmGradleInventory(key tr.InventoryKey, output string, classes map[string]testevidence.Blob) (tr.RunnerInventory, error) {
	fail := func() (tr.RunnerInventory, error) {
		return tr.RunnerInventory{Key: key, Reason: "jvm-inventory:unsupported-or-unmapped-test"}, fmt.Errorf("jvm-inventory:unsupported-or-unmapped-test")
	}
	if len(output) == 0 || len(output) > tr.MaxReportBytes || !utf8.ValidString(output) {
		return fail()
	}
	suites, err := jvmSuites(output)
	if err != nil {
		return fail()
	}
	inv := tr.RunnerInventory{Key: key, Complete: true}
	for _, suite := range suites {
		observed, err := platform.Parse(tr.Input{Runner: key.Runner, Reports: map[string][]byte{"TEST-captured.xml": []byte(suite)}})
		if err != nil || !observed.Complete {
			return fail()
		}
		for _, test := range observed.Tests {
			class, method, ok := strings.Cut(test.ID, "#")
			// Gradle reports native method names (including optional parentheses).
			// Runtime display names/templates remain unsupported in this slice.
			if !ok || !jvmClass.MatchString(class) || strings.Contains(class, "$") || !jvmClass.MatchString(strings.TrimSuffix(method, "()")) || strings.Contains(method, ".") || !jvmBlob(classes[class]) || test.State != tr.Skipped {
				return fail()
			}
			inv.Tests = append(inv.Tests, tr.InventoryTest{ID: test.ID, Path: classes[class].Path.Lexical})
			if len(inv.Tests) > tr.MaxTests {
				return fail()
			}
		}
	}
	if len(inv.Tests) == 0 {
		return fail()
	}
	canonical, err := tr.CanonicalInventory(inv, key)
	if err != nil {
		return fail()
	}
	return canonical, nil
}

// JVMGradleTaskScript is staged and pinned by the host as a Gradle init script.
// It lists only native Test tasks with a unique Java source-set association;
// aggregates (including fastGate) are omitted. Unknown associations are named,
// never guessed from the task name. Classpath files still need core immutable
// blob resolution/classification; these lines alone do not attest completeness.
const JVMGradleTaskScript = `gradle.projectsEvaluated {
 println('CORVINT_JVM_BEGIN')
 def count = 0
 gradle.rootProject.allprojects.sort { it.path }.each { p ->
  def sets = p.extensions.findByName('sourceSets')
  p.tasks.withType(org.gradle.api.tasks.testing.Test).sort { it.path }.each { t ->
   def matches = sets == null ? [] : sets.findAll { s -> s.output.classesDirs.files == t.testClassesDirs.files }
   if (matches.size() != 1) {
    println('CORVINT_JVM_UNRESOLVED\t' + t.path)
   } else {
    def s = matches[0]
    count++
    println('CORVINT_JVM_TASK\t' + t.path + '\t' + s.name)
    s.compileClasspath.files.sort { it.path }.each { f ->
     println('CORVINT_JVM_COMPILE\t' + t.path + '\t' + f.absolutePath)
    }
   }
  }
 }
 println('CORVINT_JVM_END\t' + count)
}
`

// JVMGradleTask retains one native Test task and its source-set compile inputs.
// The core resolves each classpath file to immutable repository blobs or a
// separately classified immutable toolchain before setting SourceSet.Complete.
type JVMGradleTask struct {
	SourceSet        string
	CompileClasspath []string
}

// JVMGradleTasks parses the fixed init script's bounded task listing. Gradle's
// ordinary log lines outside the markers are diagnostic only; the host must
// independently require a successful uncapped process. Missing end/count,
// unresolved associations and malformed rows refuse the complete listing.
func JVMGradleTasks(output string) (map[string]JVMGradleTask, error) {
	fail := func() (map[string]JVMGradleTask, error) {
		return nil, fmt.Errorf("jvm-task-list-unresolved-malformed-or-capped")
	}
	if len(output) == 0 || len(output) > tr.MaxReportBytes || !utf8.ValidString(output) || !strings.HasSuffix(output, "\n") || strings.ContainsAny(output, "\x00\x1b") {
		return fail()
	}
	tasks := map[string]JVMGradleTask{}
	started, ended, rows := false, false, 0
	for _, line := range strings.Split(output, "\n") {
		if line == "CORVINT_JVM_BEGIN" {
			if started {
				return fail()
			}
			started = true
			continue
		}
		if !started || ended {
			if strings.Contains(line, "CORVINT_JVM_") {
				return fail()
			}
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) == 2 && f[0] == "CORVINT_JVM_END" {
			n, err := strconv.Atoi(f[1])
			if err != nil || n != len(tasks) || strconv.Itoa(n) != f[1] {
				return fail()
			}
			ended = true
			continue
		}
		rows++
		if rows > tr.MaxTests*8 || len(f) != 3 || !jvmTask.MatchString(f[1]) {
			return fail()
		}
		switch f[0] {
		case "CORVINT_JVM_TASK":
			if !jvmClass.MatchString(f[2]) || tasks[f[1]].SourceSet != "" || len(tasks) >= tr.MaxTests {
				return fail()
			}
			tasks[f[1]] = JVMGradleTask{SourceSet: f[2], CompileClasspath: []string{}}
		case "CORVINT_JVM_COMPILE":
			task, ok := tasks[f[1]]
			if !ok || !filepath.IsAbs(f[2]) || strings.ContainsAny(f[2], "\r\x00") {
				return fail()
			}
			for _, old := range task.CompileClasspath {
				if old == f[2] {
					return fail()
				}
			}
			task.CompileClasspath = append(task.CompileClasspath, f[2])
			tasks[f[1]] = task
		default:
			return fail()
		}
	}
	if !started || !ended || len(tasks) == 0 {
		return fail()
	}
	return tasks, nil
}

var jvmTask = regexp.MustCompile(`^(:[A-Za-z_][A-Za-z0-9_-]*)+$`)
