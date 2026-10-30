package opencodequalification

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInvalidHostInvalidatesQualification(t *testing.T) {
	t.Run("GOC-V0-008 AHI-032 invalid host cannot preserve previous PASS", func(t *testing.T) {
		for _, kind := range []string{"missing", "launcher"} {
			t.Run(kind, func(t *testing.T) {
				source := t.TempDir()
				if e := os.Mkdir(filepath.Join(source, "integrations"), 0700); e != nil {
					t.Fatal(e)
				}
				record := filepath.Join(source, "integrations/opencode-qualification.json")
				if e := writeRecord(context.Background(), record, Object{"result": "PASS"}, nil); e != nil {
					t.Fatal(e)
				}
				host := filepath.Join(source, "host")
				if kind == "launcher" {
					if e := os.WriteFile(host, []byte("#!/bin/sh\nexit 0\n"), 0700); e != nil {
						t.Fatal(e)
					}
				}
				output := t.TempDir()
				if e := Run(context.Background(), []string{"--source", source, "--output", output, "--host", host, "--corvint", "/bin/sh"}); e == nil {
					t.Fatal("invalid host accepted")
				}
				got, e := readObject(record)
				if e != nil || got["result"] != "FAIL" {
					t.Fatalf("%v %v", got, e)
				}
				paths, _ := filepath.Glob(output + "/qualification-*/previous-record.json")
				if len(paths) != 1 {
					t.Fatal(paths)
				}
				previous, e := readObject(paths[0])
				if e != nil || previous["result"] != "PASS" {
					t.Fatalf("%v %v", previous, e)
				}
			})
		}
	})
}

func TestObservedHostVersionRetriesOnlyEmptyStdout(t *testing.T) {
	count := 0
	version, err := observedHostVersion(func(attempt int) (string, error) {
		count++
		if attempt == 0 {
			return "", nil
		}
		return "opencode v2.0.18\n", nil
	})
	if err != nil || version != "opencode v2.0.18\n" || count != 2 {
		t.Fatalf("version=%q count=%d err=%v", version, count, err)
	}
	count = 0
	if _, err := observedHostVersion(func(int) (string, error) { count++; return "", nil }); err == nil || count != 3 {
		t.Fatalf("empty version probe did not refuse after three attempts: count=%d err=%v", count, err)
	}
}
