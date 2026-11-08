// SPDX-License-Identifier: Apache-2.0
package testevidencedoc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testevidence"
)

const maxReadBytes = 32 << 20
const maxFiles = 512

// Read only uses committed Git objects and existing record artifacts. No stored
// inventory/admission contract exists in this release, so it cannot certify a
// fresh planned-run context or a complete inventory. Never borrow that context
// from the records. The pure Project API also serves independently bound inputs.
func Read(ctx context.Context, root string, q Query) Document {
	if q.Validate() != nil {
		return Unknown(q, "invalid-query")
	}
	if root == "" {
		root = "."
	}
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return Unknown(q, "revision-unresolved")
	}
	rev, err := repo.Resolve(ctx, q.Revision)
	if err != nil {
		return Unknown(q, "revision-unresolved")
	}
	q.Revision = rev
	in := runnerselection.Input{Head: q.Revision, Base: q.Revision, Repository: "local", Repositories: map[string]bool{"local": true}, DiffComplete: true}
	if q.Base != "" {
		b, e := repo.Resolve(ctx, q.Base)
		if e != nil {
			return Unknown(q, "diff-unresolved")
		}
		q.Base = b
		in.Base = q.Base
		// Without admitted qualification every runner is FULL. Still bind the real
		// diff paths; undecoded/unsupported entries remain explicitly incomplete.
		changes, e := repo.DeltaPaths(ctx, q.Base, q.Revision)
		if e != nil {
			in.DiffComplete = false
			in.DiffReason = "diff-unresolved"
		} else {
			for _, change := range changes {
				kind := changeKind(change)
				in.Changes = append(in.Changes, runnerselection.Change{Repository: "local", Path: change.Path, Kind: kind, Decoded: kind != ""})
			}
		}
	}
	records, reason := readRecords(root, q.Index)
	// Names are labels, never independently trusted runner identities. Nil
	// inventory and qualification reach the selector unchanged.
	names := map[string]bool{}
	for _, r := range records {
		if q.Test != "" && r.Test != q.Test || q.Explain != "" && r.Test != q.Explain {
			continue
		}
		names[r.Runner.Plugin] = true
	}
	if len(names) == 0 {
		names[""] = true
	}
	for name := range names {
		in.Runners = append(in.Runners, runnerselection.Runner{Name: name})
	}
	sort.Slice(in.Runners, func(i, j int) bool { return in.Runners[i].Name < in.Runners[j].Name })
	return Project(q, in, records, reason)
}

// changeKind admits only understood Git entry modes and transitions. Unknown
// forms remain undecoded; the selector must retain uncertainty (RSP-V0-051).
func changeKind(change gitauth.DeltaPath) string {
	known := func(entry gitauth.TreeEntry) bool {
		switch entry.Mode {
		case "100644", "100755", "120000":
			return entry.Type == "blob"
		case "160000":
			return entry.Type == "commit"
		}
		return false
	}
	switch {
	case change.Old == (gitauth.TreeEntry{}) && known(change.New):
		return "add"
	case change.New == (gitauth.TreeEntry{}) && known(change.Old):
		return "delete"
	case known(change.Old) && known(change.New) && change.Old.Mode[:3] == change.New.Mode[:3]:
		return "modify"
	}
	return ""
}

// readRegular rejects links, directories and special files before opening. A
// rooted handle confines races and every byte read is bounded.
func readRegular(root *os.Root, name string, limit int) ([]byte, error) {
	parts := strings.Split(name, "/")
	parent := root
	defer func() {
		if parent != root {
			parent.Close()
		}
	}()
	for _, part := range parts[:len(parts)-1] {
		info, e := parent.Lstat(part)
		if e != nil {
			return nil, e
		}
		if !info.IsDir() {
			return nil, errors.New("unsafe-path")
		}
		next, e := parent.OpenRoot(part)
		if e != nil {
			return nil, e
		}
		opened, e := next.Stat(".")
		if e != nil || !os.SameFile(info, opened) {
			next.Close()
			return nil, errors.New("path-changed")
		}
		if parent != root {
			parent.Close()
		}
		parent = next
	}
	name = parts[len(parts)-1]
	info, e := parent.Lstat(name)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Size() > int64(limit) {
		return nil, errors.New("unsafe-or-capped-file")
	}
	f, e := parent.Open(name)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	now, e := f.Stat()
	if e != nil || !os.SameFile(info, now) {
		return nil, errors.New("file-changed")
	}
	data, e := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if len(data) > limit {
		return nil, errors.New("read-cap-exceeded")
	}
	return data, e
}

func readRecords(root, index string) ([]testevidence.Record, string) {
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, "records-unreadable"
	}
	defer r.Close()
	records := []testevidence.Record{}
	reason := ""
	total := 0
	if index != "" {
		data, e := readRegular(r, index, maxReadBytes)
		if e != nil {
			return nil, "index-unreadable"
		}
		total += len(data)
		ix, e := linecov.Decode(bytes.NewReader(data))
		if e != nil {
			return nil, "index-malformed"
		}
		records, e = ix.EvidenceRecords("local")
		if e != nil {
			return nil, "index-malformed"
		}
	}
	// Do not traverse a linked .corvint or evidence directory.
	info, e := r.Lstat(".corvint")
	if errors.Is(e, fs.ErrNotExist) {
		return records, reason
	}
	if e != nil || !info.IsDir() {
		return records, "records-unreadable"
	}
	c, e := r.OpenRoot(".corvint")
	if e != nil {
		return records, "records-unreadable"
	}
	defer c.Close()
	now, e := c.Stat(".")
	if e != nil || !os.SameFile(info, now) {
		return records, "records-unreadable"
	}
	info, e = c.Lstat("test-evidence")
	if errors.Is(e, fs.ErrNotExist) {
		return records, reason
	}
	if e != nil || !info.IsDir() {
		return records, "records-unreadable"
	}
	dir, e := c.OpenRoot("test-evidence")
	if e != nil {
		return records, "records-unreadable"
	}
	defer dir.Close()
	now, e = dir.Stat(".")
	if e != nil || !os.SameFile(info, now) {
		return records, "records-unreadable"
	}
	listing, e := dir.Open(".")
	if e != nil {
		return records, "records-unreadable"
	}
	defer listing.Close()
	entries, e := listing.ReadDir(maxFiles + 1)
	if e != nil && !errors.Is(e, io.EOF) {
		return records, "records-unreadable"
	}
	if len(entries) > maxFiles {
		return nil, "record-cap-exceeded"
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, e := readRegular(dir, entry.Name(), testevidence.MaxRecordBytes)
		if e != nil {
			reason = "records-unreadable"
			continue
		}
		total += len(data)
		if total > maxReadBytes {
			return nil, "record-cap-exceeded"
		}
		var header struct {
			Format string `json:"format"`
			Schema string `json:"schema"`
		}
		if json.Unmarshal(data, &header) != nil {
			reason = "malformed-record"
			continue
		}
		// Existing provider receipts share this directory; they are not T1 records.
		if header.Format != testevidence.Format {
			reason = "unsupported-record"
			continue
		}
		record, e := testevidence.Decode(data)
		if e != nil {
			reason = "malformed-record"
			continue
		}
		records = append(records, record)
	}
	return records, reason
}
