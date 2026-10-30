package main

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func sqliteFilePath(dir string) string { return filepath.Join(dir, "snapshot.db") }

// BuildSQLite writes the corpus into a SQLite database: one table per
// record kind, path-indexed, matching DNIP-IDX-009's required safety
// baseline.
func BuildSQLite(dir string, c *Corpus) error {
	db, err := sql.Open("sqlite", sqliteFilePath(dir))
	if err != nil {
		return err
	}
	defer db.Close()

	ddl := []string{
		`PRAGMA journal_mode=OFF`,
		`PRAGMA synchronous=OFF`,
		`CREATE TABLE files (path TEXT PRIMARY KEY, blob_hash TEXT)`,
		`CREATE TABLE symbols (path TEXT, name TEXT, line INTEGER)`,
		`CREATE TABLE imports (path TEXT, imp TEXT)`,
		`CREATE TABLE markers (path TEXT, line INTEGER, text TEXT)`,
		`CREATE TABLE vocabulary (term TEXT)`,
	}
	for _, stmt := range ddl {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	insFile, err := tx.Prepare(`INSERT INTO files(path, blob_hash) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer insFile.Close()
	insSym, err := tx.Prepare(`INSERT INTO symbols(path, name, line) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insSym.Close()
	insImp, err := tx.Prepare(`INSERT INTO imports(path, imp) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer insImp.Close()
	insMk, err := tx.Prepare(`INSERT INTO markers(path, line, text) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insMk.Close()
	insVocab, err := tx.Prepare(`INSERT INTO vocabulary(term) VALUES (?)`)
	if err != nil {
		return err
	}
	defer insVocab.Close()

	for _, p := range c.Paths {
		rec := c.Files[p]
		if _, err := insFile.Exec(rec.Path, rec.BlobHash); err != nil {
			return err
		}
		for _, s := range rec.Symbols {
			if _, err := insSym.Exec(rec.Path, s.Name, s.Line); err != nil {
				return err
			}
		}
		for _, imp := range rec.Imports {
			if _, err := insImp.Exec(rec.Path, imp); err != nil {
				return err
			}
		}
		for _, m := range rec.Markers {
			if _, err := insMk.Exec(rec.Path, m.Line, m.Text); err != nil {
				return err
			}
		}
	}
	for _, term := range c.Vocabulary {
		if _, err := insVocab.Exec(term); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	idx := []string{
		`CREATE INDEX idx_symbols_path ON symbols(path)`,
		`CREATE INDEX idx_imports_path ON imports(path)`,
		`CREATE INDEX idx_markers_path ON markers(path)`,
	}
	for _, stmt := range idx {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	_, err = db.Exec(`VACUUM`)
	return err
}

// OpenSQLiteReadOnly opens the database read-only and immutable, matching
// DNIP-IDX-009's safety-baseline mode.
func OpenSQLiteReadOnly(dir string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro&immutable=1", sqliteFilePath(dir))
	return sql.Open("sqlite", dsn)
}

// SQLiteReadWhole reads every table back into a Corpus: the "whole" read.
func SQLiteReadWhole(db *sql.DB) (*Corpus, error) {
	c := &Corpus{Files: make(map[string]Record)}

	fileRows, err := db.Query(`SELECT path, blob_hash FROM files`)
	if err != nil {
		return nil, err
	}
	for fileRows.Next() {
		var rec Record
		if err := fileRows.Scan(&rec.Path, &rec.BlobHash); err != nil {
			fileRows.Close()
			return nil, err
		}
		c.Files[rec.Path] = rec
		c.Paths = append(c.Paths, rec.Path)
	}
	fileRows.Close()

	symRows, err := db.Query(`SELECT path, name, line FROM symbols`)
	if err != nil {
		return nil, err
	}
	for symRows.Next() {
		var path, name string
		var line int
		if err := symRows.Scan(&path, &name, &line); err != nil {
			symRows.Close()
			return nil, err
		}
		rec := c.Files[path]
		rec.Symbols = append(rec.Symbols, Symbol{Name: name, Line: line})
		c.Files[path] = rec
	}
	symRows.Close()

	impRows, err := db.Query(`SELECT path, imp FROM imports`)
	if err != nil {
		return nil, err
	}
	for impRows.Next() {
		var path, imp string
		if err := impRows.Scan(&path, &imp); err != nil {
			impRows.Close()
			return nil, err
		}
		rec := c.Files[path]
		rec.Imports = append(rec.Imports, imp)
		c.Files[path] = rec
	}
	impRows.Close()

	mkRows, err := db.Query(`SELECT path, line, text FROM markers`)
	if err != nil {
		return nil, err
	}
	for mkRows.Next() {
		var path, text string
		var line int
		if err := mkRows.Scan(&path, &line, &text); err != nil {
			mkRows.Close()
			return nil, err
		}
		rec := c.Files[path]
		rec.Markers = append(rec.Markers, Marker{Line: line, Text: text})
		c.Files[path] = rec
	}
	mkRows.Close()

	vocabRows, err := db.Query(`SELECT term FROM vocabulary`)
	if err != nil {
		return nil, err
	}
	for vocabRows.Next() {
		var term string
		if err := vocabRows.Scan(&term); err != nil {
			vocabRows.Close()
			return nil, err
		}
		c.Vocabulary = append(c.Vocabulary, term)
	}
	vocabRows.Close()
	return c, nil
}

// SQLiteReadOneTable reads only the imports table, across every path.
func SQLiteReadOneTable(db *sql.DB) (map[string][]string, error) {
	rows, err := db.Query(`SELECT path, imp FROM imports`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]string)
	for rows.Next() {
		var path, imp string
		if err := rows.Scan(&path, &imp); err != nil {
			return nil, err
		}
		out[path] = append(out[path], imp)
	}
	return out, nil
}

// SQLiteReadOnePath reads every table's rows for one path, using the
// path indexes built in BuildSQLite.
func SQLiteReadOnePath(db *sql.DB, target string) (Record, error) {
	var rec Record
	rec.Path = target
	row := db.QueryRow(`SELECT blob_hash FROM files WHERE path = ?`, target)
	if err := row.Scan(&rec.BlobHash); err != nil {
		return Record{}, err
	}

	symRows, err := db.Query(`SELECT name, line FROM symbols WHERE path = ?`, target)
	if err != nil {
		return Record{}, err
	}
	for symRows.Next() {
		var s Symbol
		if err := symRows.Scan(&s.Name, &s.Line); err != nil {
			symRows.Close()
			return Record{}, err
		}
		rec.Symbols = append(rec.Symbols, s)
	}
	symRows.Close()

	impRows, err := db.Query(`SELECT imp FROM imports WHERE path = ?`, target)
	if err != nil {
		return Record{}, err
	}
	for impRows.Next() {
		var imp string
		if err := impRows.Scan(&imp); err != nil {
			impRows.Close()
			return Record{}, err
		}
		rec.Imports = append(rec.Imports, imp)
	}
	impRows.Close()

	mkRows, err := db.Query(`SELECT line, text FROM markers WHERE path = ?`, target)
	if err != nil {
		return Record{}, err
	}
	for mkRows.Next() {
		var m Marker
		if err := mkRows.Scan(&m.Line, &m.Text); err != nil {
			mkRows.Close()
			return Record{}, err
		}
		rec.Markers = append(rec.Markers, m)
	}
	mkRows.Close()
	return rec, nil
}
