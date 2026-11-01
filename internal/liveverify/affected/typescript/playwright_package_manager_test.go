package typescript

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TJAA-V0-780 (V1-3498): every Playwright argv launches through the repository root's package
// manager; a non-npm manager is bound into the source digest, and npm repositories keep their
// npx argv and digest.
func TestPlaywrightArgvFollowsPackageManager_V13498(t *testing.T) {
	npmRoot := playwrightFixture(t)
	npm, err := SelectPlaywright(npmRoot, "playwright.config.ts", discoveryFixtureRevision, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertDiscoveryFallback(t, npm, "MISSING")
	for name, c := range map[string]struct {
		files  map[string]string
		launch []string
	}{
		"packageManager member": {map[string]string{"package.json": `{"packageManager":"pnpm@11.28.5","devDependencies":{"@playwright/test":"1.61.0"}}`}, []string{"pnpm", "exec"}},
		"pnpm lockfile":         {map[string]string{"pnpm-lock.yaml": "lockfileVersion: '9.0'\n"}, []string{"pnpm", "exec"}},
		"yarn lockfile":         {map[string]string{"yarn.lock": "\n"}, []string{"yarn"}},
		"npm lockfile":          {map[string]string{"package-lock.json": "{}\n"}, []string{"npx"}},
		"conflicting lockfiles": {map[string]string{"package-lock.json": "{}\n", "pnpm-lock.yaml": "lockfileVersion: '9.0'\n"}, []string{"npx"}},
	} {
		t.Run(name, func(t *testing.T) {
			root := playwrightFixture(t)
			for path, body := range c.files {
				write(t, root, path, body)
			}
			plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := append(append([]string{}, c.launch...), "playwright", "test", "--config=playwright.config.ts")
			if !slices.Equal(plan.FallbackArgv, want) {
				t.Fatalf("fallbackArgv = %v, want %v", plan.FallbackArgv, want)
			}
			if len(plan.Projects) == 0 {
				t.Fatal("fixture has no projects")
			}
			for _, project := range plan.Projects {
				if !slices.Equal(project.Argv[:len(c.launch)], c.launch) || project.Argv[len(c.launch)] != "playwright" {
					t.Fatalf("project argv = %v", project.Argv)
				}
			}
			if plan.Narrowed != nil {
				for _, project := range plan.Narrowed.Projects {
					if !slices.Equal(project.Argv[:len(c.launch)], c.launch) {
						t.Fatalf("narrowing project argv = %v", project.Argv)
					}
				}
			}
			// A lockfile is no observed source, so only the manager identity can move the digest.
			if (name == "pnpm lockfile" || name == "yarn lockfile") && plan.SourceDigest == npm.SourceDigest {
				t.Fatalf("a %v manager is not bound into the source digest", c.launch)
			}
		})
	}
}

// TJAA-V0-780 review fix: the argv use the manager the plan's final source check bound into its
// digest. A lockfile change after that check yields neither Yarn argv nor a digest that disagrees
// with them: the plan keeps the pnpm argv and the pnpm-bound digest.
func TestPlaywrightArgvUseTheValidatedManager_V13498(t *testing.T) {
	root := playwrightFixture(t)
	write(t, root, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
	pnpm, err := ObservePlaywrightSources(root, "playwright.config.ts")
	if err != nil {
		t.Fatal(err)
	}
	playwrightAfterSourceCheck.Store(root, func() {
		if err := os.Remove(filepath.Join(root, "pnpm-lock.yaml")); err != nil {
			t.Error(err)
		}
		write(t, root, "yarn.lock", "\n")
	})
	defer playwrightAfterSourceCheck.Delete(root)
	plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SourceDigest != pnpm {
		t.Fatalf("source digest %s, want the pnpm-bound %s", plan.SourceDigest, pnpm)
	}
	if want := []string{"pnpm", "exec", "playwright", "test", "--config=playwright.config.ts"}; !slices.Equal(plan.FallbackArgv, want) {
		t.Fatalf("fallbackArgv = %v, want %v", plan.FallbackArgv, want)
	}
}
