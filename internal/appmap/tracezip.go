package appmap

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"sort"
	"strings"
)

// Playwright trace zips (ANL-V0-022, ANL-V0-068). The archive is read in memory and never
// extracted: only root context trace entries (`trace.trace` or `N-trace.trace`, at most 16, one per
// browser context) are decompressed, each as a bounded line stream. Navigations are derived from
// main-frame snapshot URLs, page by page; a change between an action's before snapshot and its input
// or after snapshot on the same page is attributed to that action, any other change carries no
// action and so reads UNKNOWN.
var (
	traceZipLimit      = 64 << 20 // archive bytes
	traceEntryLimit    = 64 << 20 // decompressed context-trace bytes
	traceLineLimit     = 16 << 20 // one NDJSON record
	traceZipEntryLimit = 65536
)

const TraceZipSource = "playwright-trace-zip"

var traceEntryName = regexp.MustCompile(`^([0-9]+-)?trace\.trace$`)
var traceMethod = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{0,63}$`)

type traceRecord struct {
	Type     string          `json:"type"`
	CallID   string          `json:"callId"`
	Method   string          `json:"method"`
	Params   json.RawMessage `json:"params"`
	Snapshot *struct {
		CallID       string `json:"callId"`
		SnapshotName string `json:"snapshotName"`
		FrameURL     string `json:"frameUrl"`
		PageID       string `json:"pageId"`
		IsMainFrame  bool   `json:"isMainFrame"`
	} `json:"snapshot"`
}

// LoadTraceZip derives playwright-navigation-events/0 documents from a trace zip, one per page
// that has a main-frame snapshot, ordered by context entry (root entry first, then by its number)
// and then by each page's first snapshot. Call and page IDs are keyed by their entry prefix, so two
// contexts never share an action or a page. The caller names the test key; the archive does not
// carry one. When the zip holds more than one page each document names its page, so its visits
// are ordered on that page alone. raw is the archive, for the trace digest.
func LoadTraceZip(name, testKey string) ([]NavigationEvents, []byte, error) {
	bad := func(msg string) error { return verifyError("appmap-invalid-query", "%s", msg) }
	if !validTestKey(testKey) {
		return nil, nil, bad("--test-key must be a non-empty key without spaces or control characters")
	}
	raw, ok := readRegular(name, traceZipLimit)
	if !ok {
		return nil, nil, bad("--trace zip must be a regular file of at most 64 MiB, unchanged while read")
	}
	// Every central directory record starts with this signature, so its count bounds the entries
	// the reader would allocate; refuse before building the directory.
	if bytes.Count(raw, []byte("PK\x01\x02")) > traceZipEntryLimit {
		return nil, nil, bad("--trace is not a readable zip of at most 65536 entries")
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil || len(zr.File) > traceZipEntryLimit {
		return nil, nil, bad("--trace is not a readable zip of at most 65536 entries")
	}
	var entries []*zip.File
	for _, f := range zr.File {
		if traceEntryName.MatchString(f.Name) {
			entries = append(entries, f)
		}
	}
	if len(entries) == 0 {
		return nil, nil, bad("trace zip needs a root context trace (trace.trace or N-trace.trace)")
	}
	if len(entries) > maxTraceContexts {
		return nil, nil, bad("trace zip holds more than 16 context traces")
	}
	if duplicateEntryName(entries) {
		return nil, nil, bad("trace zip holds two context traces with one name")
	}
	sort.SliceStable(entries, func(i, j int) bool { return traceEntryLess(entries[i].Name, entries[j].Name) })
	type pageEvents struct {
		doc NavigationEvents
		cur string
	}
	// IDs are keyed by context entry index, so equal IDs in two contexts (or an ID that spells
	// another context's prefix) never meet; a before URL is also keyed by its page.
	type ctxID struct {
		entry int
		id    string
	}
	type pageCall struct {
		entry      int
		page, call string
	}
	var pages []*pageEvents
	byPage := map[ctxID]*pageEvents{}
	actions := map[ctxID]*NavigationAction{}
	beforeURL := map[pageCall]string{}
	snapshots, events := 0, 0
	for ei, entry := range entries {
		if entry.UncompressedSize64 > uint64(traceEntryLimit) {
			return nil, nil, bad("trace zip context trace exceeds 64 MiB")
		}
		prefix := strings.TrimSuffix(entry.Name, "trace.trace")
		rc, err := entry.Open()
		if err != nil {
			return nil, nil, bad("trace zip context trace unreadable")
		}
		limited := &io.LimitedReader{R: rc, N: int64(traceEntryLimit) + 1}
		sc := bufio.NewScanner(limited)
		sc.Buffer(make([]byte, 0, min(64<<10, traceLineLimit)), traceLineLimit)
		for sc.Scan() {
			line := bytes.TrimSpace(sc.Bytes())
			if len(line) == 0 {
				continue
			}
			var r traceRecord
			if json.Unmarshal(line, &r) != nil || r.Type == "" {
				rc.Close()
				return nil, nil, bad("trace zip context trace has an invalid record")
			}
			switch {
			case r.Type == "before" && r.CallID != "":
				if !traceMethod.MatchString(r.Method) {
					continue
				}
				var p struct {
					Selector string `json:"selector"`
				}
				_ = json.Unmarshal(r.Params, &p)
				a := &NavigationAction{Kind: r.Method}
				if len(p.Selector) <= 1024 {
					a.Selector = p.Selector
				}
				actions[ctxID{ei, r.CallID}] = a
			case r.Type == "frame-snapshot" && r.Snapshot != nil && r.Snapshot.IsMainFrame:
				s := r.Snapshot
				snapshots++
				pe := byPage[ctxID{ei, s.PageID}]
				if pe == nil {
					if len(pages) == maxTraceFiles {
						rc.Close()
						return nil, nil, bad("trace zip has more than 64 pages")
					}
					pe = &pageEvents{doc: NavigationEvents{Schema: NavigationEventsSchema, TestKey: testKey, Page: prefix + s.PageID, Events: []NavigationEvent{}}}
					byPage[ctxID{ei, s.PageID}] = pe
					pages = append(pages, pe)
				}
				phase, call, _ := strings.Cut(s.SnapshotName, "@")
				if call == "" {
					call = s.CallID
				}
				at := pageCall{ei, s.PageID, call}
				if phase == "before" {
					beforeURL[at] = s.FrameURL
				}
				if s.FrameURL == "" || len(s.FrameURL) > 4096 {
					rc.Close()
					return nil, nil, bad("trace zip snapshot URL must be 1 to 4096 bytes")
				}
				if s.FrameURL == pe.cur {
					continue
				}
				prev := pe.cur
				pe.cur = s.FrameURL
				if strings.HasPrefix(s.FrameURL, "about:") {
					continue
				}
				ev := NavigationEvent{Kind: "navigate", URL: s.FrameURL}
				if b, ok := beforeURL[at]; ok && (phase == "input" || phase == "after") && b == prev {
					if a := actions[ctxID{ei, call}]; a != nil {
						c := *a
						ev.Action = &c
					}
				}
				if events == maxTraceEvents {
					rc.Close()
					return nil, nil, bad("trace zip has more than 4096 navigations")
				}
				events++
				pe.doc.Events = append(pe.doc.Events, ev)
			}
		}
		rc.Close()
		if sc.Err() != nil || limited.N <= 0 {
			return nil, nil, bad("trace zip context trace exceeds its byte or line bound")
		}
	}
	if snapshots == 0 {
		return nil, nil, bad("trace zip has no main-frame snapshots; record the trace with snapshots on")
	}
	docs := make([]NavigationEvents, 0, len(pages))
	for _, pe := range pages {
		if len(pages) == 1 {
			pe.doc.Page = ""
		}
		docs = append(docs, pe.doc)
	}
	return docs, raw, nil
}

// maxTraceContexts bounds the context trace entries one zip may hold (as OUE-V0 records do).
const maxTraceContexts = 16

// traceEntryLess orders the root `trace.trace` before `N-trace.trace` entries, and those by the
// value of N compared as a decimal string, so no N overflows into the root's place.
func traceEntryLess(a, b string) bool {
	na, nb := traceEntryNumber(a), traceEntryNumber(b)
	if (na == "") != (nb == "") {
		return na == ""
	}
	if len(na) != len(nb) {
		return len(na) < len(nb)
	}
	return na < nb
}

// traceEntryNumber is N without leading zeros ("0" for zero), or "" for the root entry.
func traceEntryNumber(name string) string {
	n, ok := strings.CutSuffix(name, "-trace.trace")
	if !ok {
		return ""
	}
	if n = strings.TrimLeft(n, "0"); n == "" {
		return "0"
	}
	return n
}

// duplicateEntryName reports two context entries with one name, which would share call IDs.
func duplicateEntryName(entries []*zip.File) bool {
	seen := map[string]bool{}
	for _, f := range entries {
		if seen[f.Name] {
			return true
		}
		seen[f.Name] = true
	}
	return false
}
