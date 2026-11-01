// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ASD-V0-778 (V1-4166, V1-3580): a replayed change's Result is a function of
// the planner and selector binaries, this tool, the change itself, the cost
// table, the go and git on PATH and the environment they read, because each
// change's base and head are immutable commits. resultCache keys one record per
// change on exactly those bytes, so a second replay with byte-identical binaries
// (doc-gates, then batch-precheck, on one tree; or any tree whose binaries build
// to the same bytes) reads the recorded Result instead of replaying the change.
// A miss, an unreadable record or any doubt replays; the cache never narrows.
const (
	cacheSchema     = "corvint-selection-baseline-cache/1"
	cacheMaxRecords = 512
)

type resultCache struct {
	dir   string
	stamp []byte // digest of every input shared by the run's changes
}

type cacheRecord struct {
	Schema string `json:"schema"`
	Key    string `json:"key"`
	Result Result `json:"result"`
}

// openCache returns the run's cache, or nil and the reason it is off.
func openCache(ctx context.Context, root, corvint, selector string) (*resultCache, string) {
	if os.Getenv("SELECTION_BASELINE_CACHE") == "off" || os.Getenv("CORVINT_GATE_LEDGER") == "off" {
		return nil, "SELECTION_BASELINE_CACHE=off or CORVINT_GATE_LEDGER=off"
	}
	dir := os.Getenv("SELECTION_BASELINE_CACHE_DIR")
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return nil, "no user cache directory: " + err.Error()
		}
		dir = filepath.Join(base, "corvint", "selection-baseline")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err.Error()
	}
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return nil, dir + " is not a private directory"
	}
	self, err := os.Executable()
	if err != nil {
		return nil, "own executable: " + err.Error()
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s\n", cacheSchema)
	for _, file := range []string{corvint, selector, self, filepath.Join(root, ".github", "cishards", "package-costs.json")} {
		sum, err := fileDigest(file)
		if err != nil {
			return nil, err.Error()
		}
		fmt.Fprintf(h, "file %s\n", sum)
	}
	module, err := moduleOf(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, err.Error()
	}
	fmt.Fprintf(h, "module %s\n", module)
	for _, tool := range [][]string{{"go", "version"}, {"git", "--version"}} {
		out, err := run(ctx, root, tool[0], tool[1:]...)
		if err != nil {
			return nil, err.Error()
		}
		fmt.Fprintf(h, "%s %s\n", tool[0], strings.TrimSpace(out))
	}
	for _, kv := range cacheEnvironment(os.Environ()) {
		fmt.Fprintf(h, "env %s\n", kv)
	}
	return &resultCache{dir: dir, stamp: h.Sum(nil)}, ""
}

// cacheEnvironment is every GO*, CGO_*, CORVINT_*, GIT_* and SELECTION_BASELINE_*
// variable, sorted, except the trace sinks, which change no output, and the
// cache's own switches.
func cacheEnvironment(env []string) []string {
	var kept []string
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		switch {
		case strings.HasPrefix(name, "GIT_TRACE"), name == "SELECTION_BASELINE_CACHE", name == "SELECTION_BASELINE_CACHE_DIR", name == "SELECTION_BASELINE_JOBS":
			continue
		case strings.HasPrefix(name, "GO"), strings.HasPrefix(name, "CGO_"), strings.HasPrefix(name, "CORVINT_"), strings.HasPrefix(name, "GIT_"), strings.HasPrefix(name, "SELECTION_BASELINE_"):
			kept = append(kept, kv)
		}
	}
	sort.Strings(kept)
	return kept
}

func fileDigest(name string) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// key is the record key of change c: the run's stamp and the whole change entry.
func (c *resultCache) key(ch change) string {
	entry, _ := json.Marshal(ch)
	h := sha256.New()
	h.Write(c.stamp)
	fmt.Fprintf(h, "change %s\n", entry)
	return hex.EncodeToString(h.Sum(nil))
}

func (c *resultCache) path(key string) string { return filepath.Join(c.dir, key+".json") }

// load returns the recorded Result for key; anything unreadable is a miss.
func (c *resultCache) load(key string) (Result, bool) {
	data, err := os.ReadFile(c.path(key))
	if err != nil {
		return Result{}, false
	}
	var rec cacheRecord
	if err := json.Unmarshal(data, &rec); err != nil || rec.Schema != cacheSchema || rec.Key != key {
		return Result{}, false
	}
	now := time.Now()
	_ = os.Chtimes(c.path(key), now, now) // prune keeps the records still in use
	return rec.Result, true
}

// store records r under key atomically; a failure only costs a later replay.
func (c *resultCache) store(key string, r Result) error {
	data, err := json.Marshal(cacheRecord{Schema: cacheSchema, Key: key, Result: r})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(c.dir, ".record-")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), c.path(key))
}

// prune keeps the newest cacheMaxRecords records.
func (c *resultCache) prune() {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	type aged struct {
		name string
		at   time.Time
	}
	var records []aged
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if info, err := e.Info(); err == nil {
			records = append(records, aged{e.Name(), info.ModTime()})
		}
	}
	if len(records) <= cacheMaxRecords {
		return
	}
	sort.Slice(records, func(i, j int) bool { return records[i].at.After(records[j].at) })
	for _, r := range records[cacheMaxRecords:] {
		os.Remove(filepath.Join(c.dir, r.name))
	}
}
