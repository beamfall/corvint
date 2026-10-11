package platform

import (
	"slices"
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// TestGradlePerRunUserHomeAndDaemonOwnership binds TRE-V0-042 and TRE-V0-043:
// the fixed Gradle invocation names a per-run user home beneath the fresh
// executor HOME, keeps the single-use daemon mode, never addresses ambient
// daemons and retires escaped descendants. Maven is unchanged.
func TestGradlePerRunUserHomeAndDaemonOwnership(t *testing.T) {
	r := tr.Request{Root: "/source", Executable: "/tools/gradle/bin/gradle", ExecutableSha256: strings.Repeat("a", 64), ReportDir: "/fresh", Project: "app", Config: "/source/app/build.gradle", ConfigSha256: strings.Repeat("b", 64), Target: ":test", Runner: "gradle-junit", Tools: map[string]tr.Tool{"java": {Executable: "/jdk/bin/java", Sha256: strings.Repeat("c", 64)}}}
	i, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	at := slices.Index(i.Argv, "--gradle-user-home")
	if at < 0 || at+1 >= len(i.Argv) || i.Argv[at+1] != "/fresh/.home/.gradle" || at > slices.Index(i.Argv, "--project-dir") {
		t.Fatalf("Gradle user home not scoped to the run: %v", i.Argv)
	}
	if !slices.Contains(i.Argv, "--no-daemon") || !slices.Contains(i.Argv, "--offline") || !i.RetireDetachedDescendants {
		t.Fatalf("single-use daemon ownership not fixed: %+v", i)
	}
	for _, arg := range i.Argv {
		switch arg {
		case "--daemon", "--stop", "--status", "--foreground", "--continuous":
			t.Fatalf("ambient daemon control in fixed argv: %v", i.Argv)
		}
		if strings.HasPrefix(arg, "-Dorg.gradle.daemon") || strings.HasPrefix(arg, "-Dgradle.user.home") {
			t.Fatalf("plan-controlled daemon identity: %v", i.Argv)
		}
	}
	if _, ok := i.Environment["GRADLE_USER_HOME"]; ok || len(i.Environment) != 1 {
		t.Fatalf("environment must carry only JAVA_HOME: %v", i.Environment)
	}
	r.Runner, r.Config, r.Target = "maven-surefire", "/source/app/pom.xml", ""
	m, e := Build(r)
	if e != nil || m.RetireDetachedDescendants || slices.Contains(m.Argv, "--gradle-user-home") {
		t.Fatalf("Maven profile changed: %+v %v", m, e)
	}
}
