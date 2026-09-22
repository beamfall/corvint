package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotLatencyReusesCopiesAndRegistersBeforeRetrieval(t *testing.T) {
	t.Run("TCP-V0-021", testSnapshotLatencyReusesCopiesAndRegistersBeforeRetrieval)
}

func testSnapshotLatencyReusesCopiesAndRegistersBeforeRetrieval(t *testing.T) {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	source := fixtureSnapshot(t)
	configuration := fixtureOptions(t, source)
	configuration.arms = map[string]bool{"context": true}
	configuration.snapshotLatency = true
	configuration.registrationPath = filepath.Join(t.TempDir(), "registration.json")
	script := "#!/bin/sh\nif [ \"$1\" = --version ]; then echo test; exit; fi\nmkdir -p \"$2/.corvint/index\"\nprintf '*\\n' > \"$2/.corvint/.gitignore\"\n"
	if err := os.WriteFile(configuration.corvintGo, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	roots := map[string]bool{}
	calls := 0
	retrieve := func(ctx context.Context, binary, root, task string, limit int) (arm, error) {
		if _, err := os.Stat(configuration.registrationPath); err != nil {
			t.Fatal("retrieval before registration", err)
		}
		roots[root] = true
		calls++
		if err := cleanWorkspace(ctx, root); err != nil {
			t.Fatal(err)
		}
		state := "OBSERVED_MISS"
		if _, err := os.Stat(filepath.Join(root, ".corvint", "index")); err == nil {
			state = "OBSERVED_HIT"
		}
		return arm{Ranked: []string{"tests/help_test.rs"}, State: "READY", CacheState: state}, nil
	}
	report, err := bench(context.Background(), configuration, nil, retrieve, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || calls != 8 {
		t.Fatalf("roots=%d calls=%d", len(roots), calls)
	}
	for _, item := range report["details"].([]sampleReport) {
		answer := item.Arms["context"]
		if answer.CacheState != "OBSERVED_HIT" || answer.ColdMillis <= 0 {
			t.Fatalf("answer=%+v", answer)
		}
	}
	if _, err := bench(context.Background(), configuration, nil, retrieve, nil, nil); err == nil {
		t.Fatal("overwrote registration")
	}
	if calls != 8 {
		t.Fatal("retrieved before refusing duplicate registration")
	}
	if _, err := os.Stat(filepath.Join(source, ".corvint")); !os.IsNotExist(err) {
		t.Fatal("source modified", err)
	}
	for root := range roots {
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatal("scratch retained", err)
		}
	}
}

func TestSnapshotLatencyRestoresHiddenSnapshotOnCancellation(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, ".corvint", "index")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(directory, "snapshot.gob")
	if err := os.WriteFile(file, []byte("snapshot"), 0o644); err != nil {
		t.Fatal(err)
	}
	retrieve := func(context.Context, string, string, string, int) (arm, error) { return arm{}, context.Canceled }
	_, err := prepareSnapshotSample(context.Background(), options{}, sample{}, root, retrieve)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != "snapshot" {
		t.Fatalf("snapshot lost: %q %v", data, err)
	}
}

func TestSnapshotLatencyRefusesUnknownOrDifferentRanking(t *testing.T) {
	for _, diagnostic := range []string{"", "corvint-bench-snapshot: hit=true\nother", "corvint-bench-snapshot: hit=maybe"} {
		if observedSnapshotState(diagnostic) != "UNKNOWN" {
			t.Fatal(diagnostic)
		}
	}
	cold := arm{Ranked: []string{"a"}, State: "READY"}
	warm := arm{Ranked: []string{"b"}, State: "READY", CacheState: "OBSERVED_HIT"}
	if err := validateSnapshotPair(cold, warm); err == nil {
		t.Fatal("accepted differing ranks")
	}
	warm.Ranked = cold.Ranked
	warm.CacheState = "PRIMED_SHARED"
	if err := validateSnapshotPair(cold, warm); err == nil {
		t.Fatal("inferred a hit")
	}
}

func TestBenchRejectsDirtySampleBeforeNextRetrieval(t *testing.T) {
	configuration := fixtureOptions(t, fixtureSnapshot(t))
	configuration.arms = map[string]bool{"context": true}
	calls := 0
	retrieve := func(ctx context.Context, binary, root, task string, limit int) (arm, error) {
		calls++
		return arm{}, os.WriteFile(filepath.Join(root, "leaked.txt"), []byte("dirty"), 0o644)
	}
	_, err := bench(context.Background(), configuration, nil, retrieve, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "left dirty") || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestRegistrationOutputAliasesRefuseBeforeRetrievalOrWrites(t *testing.T) {
	for _, kind := range []string{"identical", "clean-absolute", "relative-absolute", "hardlink", "symlink", "parent-symlink", "dangling-symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			registration := filepath.Join(root, "registration.json")
			output := registration
			const preserved = "original registration\n"
			existing := kind == "hardlink" || kind == "symlink"
			if existing {
				if err := os.WriteFile(registration, []byte(preserved), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			switch kind {
			case "clean-absolute":
				output = root + "/./registration.json"
			case "relative-absolute":
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				output, err = filepath.Rel(cwd, registration)
				if err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				output = filepath.Join(root, "report.json")
				if err := os.Link(registration, output); err != nil {
					t.Fatal(err)
				}
			case "symlink", "dangling-symlink":
				output = filepath.Join(root, "report.json")
				if err := os.Symlink("registration.json", output); err != nil {
					t.Fatal(err)
				}
			case "parent-symlink":
				alias := filepath.Join(root, "alias")
				if err := os.Symlink(root, alias); err != nil {
					t.Fatal(err)
				}
				output = filepath.Join(alias, "registration.json")
			}
			configuration := fixtureOptions(t, fixtureSnapshot(t))
			configuration.registrationPath = registration
			configuration.output = output
			configuration.arms = map[string]bool{"context": true}
			calls := 0
			retrieve := func(context.Context, string, string, string, int) (arm, error) { calls++; return arm{}, nil }
			_, err := bench(context.Background(), configuration, nil, retrieve, nil, nil)
			if err == nil || !strings.Contains(err.Error(), "distinct files") || calls != 0 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
			data, err := os.ReadFile(registration)
			if existing {
				if err != nil || string(data) != preserved {
					t.Fatalf("registration changed: %q %v", data, err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatalf("registration was written before refusal: %q %v", data, err)
			}
		})
	}
}

func TestRegistrationOutputAllowsDistinctFiles(t *testing.T) {
	root := t.TempDir()
	registration := filepath.Join(root, "registration.json")
	output := filepath.Join(root, "report.json")
	configuration := options{registrationPath: registration, output: output}
	if err := validateRegistrationOutput(configuration); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{registration, output} {
		if err := os.WriteFile(file, []byte("separate"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateRegistrationOutput(configuration); err != nil {
		t.Fatal(err)
	}
}
