package opencodequalification

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEvidenceReadsRefuseOverBoundFiles(t *testing.T) {
	t.Run("AHI-032 V1-0747 bounded evidence reads", func(t *testing.T) {
		dir := t.TempDir()
		write := func(name string, data []byte) string {
			t.Helper()
			p := filepath.Join(dir, name)
			if err := os.WriteFile(p, data, 0600); err != nil {
				t.Fatal(err)
			}
			return p
		}
		over := write("over.jsonl", bytes.Repeat([]byte("\n"), maxEvidenceBytes+1))
		if rows, err := readRows(over); !errors.Is(err, errEvidenceBound) || rows != nil {
			t.Fatalf("bound+1 rows: %v %v", rows, err)
		}
		if x, err := readObject(over); !errors.Is(err, errEvidenceBound) || x != nil {
			t.Fatalf("bound+1 object: %v %v", x, err)
		}
		exact := write("exact.jsonl", bytes.Repeat([]byte("\n"), maxEvidenceBytes))
		if rows, err := readRows(exact); err != nil || len(rows) != 0 {
			t.Fatalf("exact-bound rows: %v %v", rows, err)
		}
		// Rows are open Object maps: a member no gate names is accepted, not refused.
		open := write("open.json", []byte(`{"event":"x","futureField":1}`))
		if x, err := readObject(open); err != nil || x["futureField"] != float64(1) {
			t.Fatalf("unknown member: %v %v", x, err)
		}
	})
}
