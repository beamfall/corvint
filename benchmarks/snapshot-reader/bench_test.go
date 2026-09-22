package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
)

// fixture holds one scale cell's corpus and its three on-disk encodings,
// built once and reused across every benchmark permutation for that cell.
type fixture struct {
	corpus     *Corpus
	gobPath    string
	packDir    string
	packV2Dir  string
	sqliteDir  string
	targetPath string // one path used for every "one-path" read
}

var (
	fixturesMu  sync.Mutex
	fixtures    = map[int]*fixture{}
	fixtureDirs []string // every directory getFixture created, removed by TestMain
)

// TestMain removes the fixture directories after every test and benchmark has
// run. Fixtures are cached per process across benchmark re-invocations, so a
// per-benchmark b.TempDir would delete them while still cached.
func TestMain(m *testing.M) {
	code := m.Run()
	for _, dir := range fixtureDirs {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintf(os.Stderr, "remove fixture %s: %v\n", dir, err)
			code = 1
		}
	}
	os.Exit(code)
}

func getFixture(tb testing.TB, n int) *fixture {
	fixturesMu.Lock()
	defer fixturesMu.Unlock()
	if f, ok := fixtures[n]; ok {
		return f
	}

	corpus := GenerateCorpus(n, 42)
	dir, err := os.MkdirTemp("", fmt.Sprintf("snapshot-reader-%d-", n))
	if err != nil {
		tb.Fatalf("mkdir temp: %v", err)
	}
	fixtureDirs = append(fixtureDirs, dir)

	f := &fixture{corpus: corpus}
	f.gobPath = filepath.Join(dir, "snapshot.gob")
	if err := WriteGobSnapshot(f.gobPath, corpus); err != nil {
		tb.Fatalf("write gob snapshot: %v", err)
	}

	f.packDir = filepath.Join(dir, "pack")
	if err := BuildPack(f.packDir, corpus); err != nil {
		tb.Fatalf("build pack: %v", err)
	}

	f.packV2Dir = filepath.Join(dir, "packv2")
	if err := BuildPackV2(f.packV2Dir, corpus); err != nil {
		tb.Fatalf("build pack v2: %v", err)
	}

	f.sqliteDir = filepath.Join(dir, "sqlite")
	if err := os.MkdirAll(f.sqliteDir, 0o755); err != nil {
		tb.Fatalf("mkdir sqlite dir: %v", err)
	}
	if err := BuildSQLite(f.sqliteDir, corpus); err != nil {
		tb.Fatalf("build sqlite: %v", err)
	}

	sortedPaths := append([]string(nil), corpus.Paths...)
	sort.Strings(sortedPaths)
	f.targetPath = sortedPaths[len(sortedPaths)/2]

	fixtures[n] = f
	return f
}

var scaleCells = []int{10_000, 100_000}

func BenchmarkSnapshotRead(b *testing.B) {
	for _, n := range scaleCells {
		n := n
		b.Run(fmt.Sprintf("files=%d", n), func(b *testing.B) {
			f := getFixture(b, n)
			b.Run("gob", func(b *testing.B) { benchGob(b, f) })
			b.Run("sqlite", func(b *testing.B) { benchSQLite(b, f) })
			b.Run("pack", func(b *testing.B) { benchPack(b, f, f.packDir, openPackV1) })
			b.Run("packv2", func(b *testing.B) { benchPack(b, f, f.packV2Dir, openPackV2) })
		})
	}
}

// --- gob: no random access, so "warm" means reusing an already-decoded
// corpus in memory rather than a cheaper re-decode. ---

func benchGob(b *testing.B, f *fixture) {
	b.Run("whole/cold", func(b *testing.B) {
		var bytesRead int64
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, n, err := LoadGobWhole(f.gobPath)
			if err != nil {
				b.Fatal(err)
			}
			bytesRead = n
		}
		b.ReportMetric(float64(bytesRead), "bytes/op")
	})
	// No whole/warm cell for gob: gob has no random access and no
	// persistent reader to reuse, so the only thing a "warm" whole read
	// could measure is holding onto the already-decoded value -- zero
	// work, not a re-decode -- while SQLite/pack's warm cells reuse an
	// open handle but still re-query and re-decode. Publishing a gob
	// warm/whole number next to those would compare a no-op against real
	// work, so it is omitted rather than reported as if comparable.

	b.Run("one-table/cold", func(b *testing.B) {
		var bytesRead int64
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, n, err := LoadGobOneTable(f.gobPath)
			if err != nil {
				b.Fatal(err)
			}
			bytesRead = n
		}
		b.ReportMetric(float64(bytesRead), "bytes/op")
	})
	b.Run("one-table/warm", func(b *testing.B) {
		c, n, err := LoadGobWhole(f.gobPath)
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			out := make(map[string][]string, len(c.Files))
			for p, rec := range c.Files {
				out[p] = rec.Imports
			}
			_ = out
		}
		b.ReportMetric(float64(n), "bytes/op")
	})

	b.Run("one-path/cold", func(b *testing.B) {
		var bytesRead int64
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, n, err := LoadGobOnePath(f.gobPath, f.targetPath)
			if err != nil {
				b.Fatal(err)
			}
			bytesRead = n
		}
		b.ReportMetric(float64(bytesRead), "bytes/op")
	})
	b.Run("one-path/warm", func(b *testing.B) {
		c, n, err := LoadGobWhole(f.gobPath)
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = c.Files[f.targetPath]
		}
		b.ReportMetric(float64(n), "bytes/op")
	})
}

// --- sqlite: "cold" opens a fresh *sql.DB per op, "warm" reuses one open
// connection. modernc.org/sqlite exposes no ReaderAt hook, so bytes/op is
// not reported for this encoding -- see README.md. ---

func benchSQLite(b *testing.B, f *fixture) {
	b.Run("whole/cold", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			db, err := OpenSQLiteReadOnly(f.sqliteDir)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := SQLiteReadWhole(db); err != nil {
				b.Fatal(err)
			}
			db.Close()
		}
	})
	b.Run("whole/warm", func(b *testing.B) {
		db, err := OpenSQLiteReadOnly(f.sqliteDir)
		if err != nil {
			b.Fatal(err)
		}
		defer db.Close()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := SQLiteReadWhole(db); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("one-table/cold", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			db, err := OpenSQLiteReadOnly(f.sqliteDir)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := SQLiteReadOneTable(db); err != nil {
				b.Fatal(err)
			}
			db.Close()
		}
	})
	b.Run("one-table/warm", func(b *testing.B) {
		db, err := OpenSQLiteReadOnly(f.sqliteDir)
		if err != nil {
			b.Fatal(err)
		}
		defer db.Close()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := SQLiteReadOneTable(db); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("one-path/cold", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			db, err := OpenSQLiteReadOnly(f.sqliteDir)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := SQLiteReadOnePath(db, f.targetPath); err != nil {
				b.Fatal(err)
			}
			db.Close()
		}
	})
	b.Run("one-path/warm", func(b *testing.B) {
		db, err := OpenSQLiteReadOnly(f.sqliteDir)
		if err != nil {
			b.Fatal(err)
		}
		defer db.Close()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := SQLiteReadOnePath(db, f.targetPath); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// packCodec is the read surface pack v1 and pack v2 share, so both run
// through the same benchmark cells and byte counting.
type packCodec interface {
	ReadWhole() (*Corpus, error)
	ReadOneTable() (map[string][]string, error)
	ReadOnePath(target string) (Record, error)
	BytesRead() int64
	Close() error
}

func openPackV1(dir string) (packCodec, error) {
	r, err := OpenPack(dir)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func openPackV2(dir string) (packCodec, error) {
	r, err := OpenPackV2(dir)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// --- pack v1/v2: "cold" opens a fresh reader (fresh manifest read, fresh
// file handle, zeroed byte counter) per op; "warm" reuses one open reader,
// and bytes/op is the counter's total divided by b.N. ---

func benchPack(b *testing.B, f *fixture, dir string, open func(string) (packCodec, error)) {
	b.Run("whole/cold", func(b *testing.B) {
		var bytesRead int64
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r, err := open(dir)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := r.ReadWhole(); err != nil {
				b.Fatal(err)
			}
			bytesRead = r.BytesRead()
			r.Close()
		}
		b.ReportMetric(float64(bytesRead), "bytes/op")
	})
	b.Run("whole/warm", func(b *testing.B) {
		r, err := open(dir)
		if err != nil {
			b.Fatal(err)
		}
		defer r.Close()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := r.ReadWhole(); err != nil {
				b.Fatal(err)
			}
		}
		b.ReportMetric(float64(r.BytesRead())/float64(b.N), "bytes/op")
	})

	b.Run("one-table/cold", func(b *testing.B) {
		var bytesRead int64
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r, err := open(dir)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := r.ReadOneTable(); err != nil {
				b.Fatal(err)
			}
			bytesRead = r.BytesRead()
			r.Close()
		}
		b.ReportMetric(float64(bytesRead), "bytes/op")
	})
	b.Run("one-table/warm", func(b *testing.B) {
		r, err := open(dir)
		if err != nil {
			b.Fatal(err)
		}
		defer r.Close()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := r.ReadOneTable(); err != nil {
				b.Fatal(err)
			}
		}
		b.ReportMetric(float64(r.BytesRead())/float64(b.N), "bytes/op")
	})

	b.Run("one-path/cold", func(b *testing.B) {
		var bytesRead int64
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r, err := open(dir)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := r.ReadOnePath(f.targetPath); err != nil {
				b.Fatal(err)
			}
			bytesRead = r.BytesRead()
			r.Close()
		}
		b.ReportMetric(float64(bytesRead), "bytes/op")
	})
	b.Run("one-path/warm", func(b *testing.B) {
		r, err := open(dir)
		if err != nil {
			b.Fatal(err)
		}
		defer r.Close()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := r.ReadOnePath(f.targetPath); err != nil {
				b.Fatal(err)
			}
		}
		b.ReportMetric(float64(r.BytesRead())/float64(b.N), "bytes/op")
	})
}
