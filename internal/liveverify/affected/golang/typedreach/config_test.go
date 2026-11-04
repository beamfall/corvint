// SPDX-License-Identifier: Apache-2.0

package typedreach

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// executorConfig probes the build configuration of a plain go test run in
// root, as the test process's own environment selects it.
func executorConfig(t *testing.T, root string, env, flags []string) Config {
	t.Helper()
	config, err := ProbeConfig(context.Background(), root, append(os.Environ(), env...), flags)
	if err != nil {
		t.Fatal(err)
	}
	return config
}

// TestAFPV02372ExecutorConfigSelectsConstrainedSource loads a package with a
// tagged and an anti-tagged test file under the configuration the executor
// selects: each configuration keeps exactly the test functions its build
// compiles, and Load without a configuration still refuses (AFP-V0-2372).
func TestAFPV02372ExecutorConfigSelectsConstrainedSource(t *testing.T) {
	root := reviewFixture(t, map[string]string{
		"lib/lib.go":         "package lib\nfunc A() int { return 1 }\nfunc B() int { return 2 }\n",
		"p/p.go":             "package p\n",
		"p/p_test.go":        "package p\nimport (\"testing\"; \"example.com/review/lib\")\nfunc TestA(t *testing.T) { lib.A() }\nfunc TestControl(t *testing.T) {}\n",
		"p/tagged_test.go":   "//go:build floortag\n\npackage p\nimport (\"testing\"; \"example.com/review/lib\")\nfunc TestTagged(t *testing.T) { lib.A() }\n",
		"p/untagged_test.go": "//go:build !floortag\n\npackage p\nimport (\"testing\"; \"example.com/review/lib\")\nfunc TestUntagged(t *testing.T) { lib.A() }\n",
	})
	if _, err := Load(root, "example.com/review", "p"); !errors.Is(err, ErrUnmodelled) {
		t.Fatalf("constrained source without a configuration must refuse: %v", err)
	}
	plain := executorConfig(t, root, nil, nil)
	tagged := executorConfig(t, root, nil, []string{"-tags=floortag"})
	if plain.Identity() == tagged.Identity() {
		t.Fatalf("the tag does not change the configuration identity: %s", plain.Identity())
	}
	for _, tc := range []struct {
		name         string
		config       Config
		kept, absent string
	}{
		{"tagged", tagged, "TestTagged", "TestUntagged"},
		{"untagged", plain, "TestUntagged", "TestTagged"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := LoadFor(tc.config, root, "example.com/review", "p")
			if err != nil {
				t.Fatal(err)
			}
			result := Analyze(program, []Declaration{{Package: "example.com/review/lib", Name: "A"}}, nil)
			_, kept := result.Kept[tc.kept]
			_, absent := result.Kept[tc.absent]
			if result.Whole != "" || !kept || absent || result.Tests != 3 || !slices.Equal(result.Dropped, []string{"TestControl"}) {
				t.Fatalf("%s configuration: %+v", tc.name, result)
			}
		})
	}
}

// TestAFPV02376BaseConstraintChangeRefuses reads the base of a file whose
// build constraint changed: the two trees could compile different source, so
// the base load fails closed, while an unchanged constraint loads
// (AFP-V0-2376).
func TestAFPV02376BaseConstraintChangeRefuses(t *testing.T) {
	head := "//go:build !floortag\n\npackage lib\nfunc A() int { return 1 }\n"
	root := reviewFixture(t, map[string]string{
		"lib/lib.go":  head,
		"p/p_test.go": "package p\nimport (\"testing\"; \"example.com/review/lib\")\nfunc TestA(t *testing.T) { lib.A() }\n",
	})
	config := executorConfig(t, root, nil, nil)
	changed := map[string][]byte{"lib/lib.go": []byte("package lib\nfunc A() int { return 0 }\n")}
	if _, err := LoadBaseFor(config, root, "example.com/review", "p", changed); !errors.Is(err, ErrUnmodelled) || !strings.Contains(err.Error(), "changes its build constraint") {
		t.Fatalf("a changed constraint must refuse the base: %v", err)
	}
	same := map[string][]byte{"lib/lib.go": []byte(strings.Replace(head, "return 1", "return 0", 1))}
	if _, err := LoadBaseFor(config, root, "example.com/review", "p", same); err != nil {
		t.Fatalf("an unchanged constraint must load: %v", err)
	}
}

// TestAFPV02372BuildFlagsClassifiesExecutorFlags keeps the flags that select
// source, drops the run-only ones and names the first flag the model does not
// know (AFP-V0-2372).
func TestAFPV02372BuildFlagsClassifiesExecutorFlags(t *testing.T) {
	selecting, unknown := BuildFlags([]string{"-exec", "/opt/confine", "-race", "-p", "1", "-count=1", "-tags", "a,b", "-json", "-timeout=30m", "-mod=readonly"})
	if unknown != "" || !slices.Equal(selecting, []string{"-tags=a,b", "-race"}) {
		t.Fatalf("selecting %q unknown %q", selecting, unknown)
	}
	// The last occurrence wins, as in the go command.
	if selecting, unknown := BuildFlags([]string{"-race", "-tags=a", "-race=false", "-tags=b"}); unknown != "" || !slices.Equal(selecting, []string{"-tags=b"}) {
		t.Fatalf("last wins: selecting %q unknown %q", selecting, unknown)
	}
	for _, flag := range []string{"-toolexec=x", "-overlay=o.json", "-gcflags=all=-N", "-pgo=auto", "-coverpkg=all", "-mod=vendor", "-race=1", "./cmd"} {
		if _, unknown := BuildFlags([]string{"-count=1", flag}); unknown != flag {
			t.Fatalf("%s: unknown %q", flag, unknown)
		}
	}
	if _, unknown := BuildFlags([]string{"-tags"}); unknown != "-tags" {
		t.Fatalf("a missing value: unknown %q", unknown)
	}
}

// TestAFPV02372ProbeConfigReadsTheExecutorEnvironment probes the go command
// under the executor's environment and flags: CGO_ENABLED, -tags and -race
// each reach the configuration, and an unmodelled GOFLAGS entry refuses
// (AFP-V0-2372).
func TestAFPV02372ProbeConfigReadsTheExecutorEnvironment(t *testing.T) {
	root := reviewFixture(t, map[string]string{"p/p.go": "package p\n"})
	if config := executorConfig(t, root, []string{"CGO_ENABLED=0"}, nil); config.CgoEnabled {
		t.Fatalf("CGO_ENABLED=0 not read: %+v", config)
	}
	if config := executorConfig(t, root, nil, []string{"-tags=floortag"}); !slices.Contains(config.BuildTags, "floortag") {
		t.Fatalf("-tags not read: %+v", config)
	}
	if config := executorConfig(t, root, []string{"GOFLAGS=-tags=fromenv"}, nil); !slices.Contains(config.BuildTags, "fromenv") {
		t.Fatalf("GOFLAGS -tags not read: %+v", config)
	}
	if config := executorConfig(t, root, []string{"CGO_ENABLED=1"}, []string{"-race"}); !slices.Contains(config.ToolTags, "race") {
		t.Fatalf("-race not read: %+v", config)
	}
	if _, err := ProbeConfig(context.Background(), root, append(os.Environ(), "GOFLAGS=-toolexec=/bin/true"), nil); err == nil || !strings.Contains(err.Error(), "-toolexec") {
		t.Fatalf("an unmodelled GOFLAGS entry must refuse: %v", err)
	}
	// GOFLAGS written by go env -w lives in the GOENV file, read only when
	// the environment sets no GOFLAGS; both the gate and the context read it.
	goenv := func(content string) []string {
		path := filepath.Join(t.TempDir(), "go.env")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		env := slices.DeleteFunc(os.Environ(), func(entry string) bool { return strings.HasPrefix(entry, "GOFLAGS=") })
		return append(env, "GOENV="+path)
	}
	if config, err := ProbeConfig(context.Background(), root, goenv("GOFLAGS=-tags=fromgoenv\n"), nil); err != nil || !slices.Contains(config.BuildTags, "fromgoenv") {
		t.Fatalf("go env -w GOFLAGS -tags not read: %+v %v", config, err)
	}
	if _, err := ProbeConfig(context.Background(), root, goenv("GOFLAGS=-toolexec=/bin/true\n"), nil); err == nil || !strings.Contains(err.Error(), "-toolexec") {
		t.Fatalf("an unmodelled go env -w GOFLAGS entry must refuse: %v", err)
	}
	// A microarchitecture level is a tool tag, so it changes the identity.
	v1 := executorConfig(t, root, []string{"GOARCH=amd64", "GOAMD64=v1"}, nil)
	v3 := executorConfig(t, root, []string{"GOARCH=amd64", "GOAMD64=v3"}, nil)
	if !slices.Contains(v3.ToolTags, "amd64.v3") || v1.Identity() == v3.Identity() {
		t.Fatalf("GOAMD64 not bound: %+v / %+v", v1, v3)
	}
}
