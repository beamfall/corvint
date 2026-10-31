package kotlin

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// KAT-V0-761 (V1-2054): the default version catalog is a build configuration
// unit of its own build, so a catalog change selects every test under that
// build root (a nested build's tests too: conservative), a nested build's
// catalog selects only that build's tests, and neither is unowned.
func TestVersionCatalogIsABuildConfigurationOfItsBuild_KATV0761(t *testing.T) {
	root := t.TempDir()
	write(t, root, "settings.gradle.kts", "rootProject.name = \"fixture\"\n")
	write(t, root, "gradle/libs.versions.toml", "[versions]\nkotlin = \"2.2.0\"\n[libraries]\njunit = { module = \"junit:junit\", version = \"4.13.2\" }\n")
	write(t, root, "app/build.gradle.kts", "dependencies { testImplementation(libs.junit) }\n")
	write(t, root, "app/src/main/kotlin/app/App.kt", "package app\nclass App\n")
	write(t, root, "app/src/test/kotlin/app/AppTest.kt", junit("app", "AppTest", "app.App"))
	write(t, root, "core/src/test/kotlin/core/CoreTest.kt", junit("core", "CoreTest", "core.Core"))
	write(t, root, "ui/gradle/libs.versions.toml", "[versions]\nkotlin = \"2.2.0\"\n")
	write(t, root, "ui/kit/src/test/kotlin/kit/KitTest.kt", junit("kit", "KitTest", "kit.Kit"))
	language := New()
	if !language.Owns("gradle/libs.versions.toml") || !language.Owns("ui/gradle/libs.versions.toml") || language.Owns("libs.versions.toml") || language.Owns("gradle/other.toml") {
		t.Fatal("catalog ownership is not exactly gradle/libs.versions.toml")
	}
	graph, err := affected.Build(context.Background(), root, language)
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"gradle/libs.versions.toml"})
	want := []string{"app/src/test/kotlin/app/AppTest.kt", "core/src/test/kotlin/core/CoreTest.kt", "ui/kit/src/test/kotlin/kit/KitTest.kt"}
	if got := plan.SelectedTests(); !equal(got, want) {
		t.Fatalf("selected=%v want=%v", got, want)
	}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == affected.UnknownUnownedDirtyPath {
			t.Fatalf("catalog stays unowned: %+v", plan.Unknown)
		}
	}
	nested := affected.Select(graph, []string{"ui/gradle/libs.versions.toml"})
	if got := nested.SelectedTests(); !equal(got, []string{"ui/kit/src/test/kotlin/kit/KitTest.kt"}) {
		t.Fatalf("nested catalog selected=%v", got)
	}
}

// KAT-V0-762 (V1-2054): a test in a com.android.test module's src/main, whether
// the plugin is applied by id or by a catalog alias, is the typed android-test
// unit with the typed frontier, never kotlin:unknown.
func TestAndroidTestModuleYieldsATypedUnit_KATV0762(t *testing.T) {
	root := t.TempDir()
	write(t, root, "settings.gradle.kts", "rootProject.name = \"fixture\"\n")
	write(t, root, "gradle/libs.versions.toml", "[plugins]\nandroid-test = { id = \"com.android.test\", version = \"8.7.0\" }\n")
	write(t, root, "benchmark-app/build.gradle.kts", "plugins { id(\"com.android.test\") }\n")
	write(t, root, "benchmark-app/src/main/kotlin/bench/StartupBenchmark.kt", junit("bench", "StartupBenchmark", "bench.Startup"))
	write(t, root, "baselineprofile-app/build.gradle.kts", "plugins { alias(libs.plugins.android.test) }\n")
	write(t, root, "baselineprofile-app/src/main/kotlin/profile/ProfileGenerator.kt", junit("profile", "ProfileGenerator", "profile.Profile"))
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, unit := range result.Units {
		if strings.HasPrefix(unit.ID, "kotlin:unknown") {
			t.Fatalf("untyped unit %s", unit.ID)
		}
		if len(unit.Tests) > 0 {
			ids = append(ids, unit.ID)
		}
	}
	want := []string{
		"kotlin:android-test:baselineprofile-app:unknown:profile.ProfileGenerator",
		"kotlin:android-test:benchmark-app:unknown:bench.StartupBenchmark",
	}
	if !equal(ids, want) {
		t.Fatalf("test units=%v want=%v", ids, want)
	}
	if !contains(result.Frontier, FrontierAndroidTestModule) || contains(result.Frontier, FrontierCustomSourceSet) {
		t.Fatalf("frontier=%v, want %s and no custom-source-set", result.Frontier, FrontierAndroidTestModule)
	}
}

// KAT-V0-763 (V1-2055): an includeBuild outside the root is the named
// cross-repository frontier carrying the included path; one inside the root
// is not.
func TestIncludeBuildOutsideTheRootIsANamedFrontier_KATV0763(t *testing.T) {
	root := t.TempDir()
	write(t, root, "settings.gradle.kts", "rootProject.name = \"app\"\nincludeBuild(\"../beamfall-android-ui\")\nincludeBuild(\"build-logic\")\n")
	write(t, root, "nested/settings.gradle", "includeBuild '../../shared'\n")
	write(t, root, "app/src/main/kotlin/app/App.kt", "package app\nclass App\n")
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{FrontierCrossRepositoryBuild + ":../beamfall-android-ui", FrontierCrossRepositoryBuild + ":../../shared"} {
		if !contains(result.Frontier, want) {
			t.Errorf("frontier %v lacks %s", result.Frontier, want)
		}
	}
	if contains(result.Frontier, FrontierCrossRepositoryBuild+":build-logic") {
		t.Errorf("frontier %v names the in-root build-logic", result.Frontier)
	}
}
