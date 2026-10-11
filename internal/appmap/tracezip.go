package appmap

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"
)

// Playwright trace zips (ANL-V0-022). The archive is read in memory and never extracted: only the
// one root context trace entry (`trace.trace` or `N-trace.trace`) is decompressed, as a bounded
// line stream. Navigations are derived from main-frame snapshot URLs; a change between an action's
// before snapshot and its input or after snapshot is attributed to that action, any other change
// carries no action and so reads UNKNOWN.
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

// LoadTraceZip derives one playwright-navigation-events/0 document from a trace zip. The caller
// names the test key; the archive does not carry one. raw is the archive, for the trace digest.
func LoadTraceZip(name, testKey string) (NavigationEvents, []byte, error) {
	bad := func(msg string) error { return verifyError("appmap-invalid-query", "%s", msg) }
	none := NavigationEvents{}
	if !validTestKey(testKey) {
		return none, nil, bad("--test-key must be a non-empty key without spaces or control characters")
	}
	raw, ok := readRegular(name, traceZipLimit)
	if !ok {
		return none, nil, bad("--trace zip must be a regular file of at most 64 MiB, unchanged while read")
	}
	// Every central directory record starts with this signature, so its count bounds the entries
	// the reader would allocate; refuse before building the directory.
	if bytes.Count(raw, []byte("PK\x01\x02")) > traceZipEntryLimit {
		return none, nil, bad("--trace is not a readable zip of at most 65536 entries")
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil || len(zr.File) > traceZipEntryLimit {
		return none, nil, bad("--trace is not a readable zip of at most 65536 entries")
	}
	var entry *zip.File
	for _, f := range zr.File {
		if traceEntryName.MatchString(f.Name) {
			if entry != nil {
				return none, nil, bad("trace zip holds more than one context trace")
			}
			entry = f
		}
	}
	if entry == nil || entry.UncompressedSize64 > uint64(traceEntryLimit) {
		return none, nil, bad("trace zip needs one root context trace of at most 64 MiB")
	}
	rc, err := entry.Open()
	if err != nil {
		return none, nil, bad("trace zip context trace unreadable")
	}
	defer rc.Close()
	limited := &io.LimitedReader{R: rc, N: int64(traceEntryLimit) + 1}
	sc := bufio.NewScanner(limited)
	sc.Buffer(make([]byte, 0, min(64<<10, traceLineLimit)), traceLineLimit)
	doc := NavigationEvents{Schema: NavigationEventsSchema, TestKey: testKey, Events: []NavigationEvent{}}
	actions := map[string]*NavigationAction{}
	beforeURL := map[string]string{}
	page, cur, snapshots := "", "", 0
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var r traceRecord
		if json.Unmarshal(line, &r) != nil || r.Type == "" {
			return none, nil, bad("trace zip context trace has an invalid record")
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
			actions[r.CallID] = a
		case r.Type == "frame-snapshot" && r.Snapshot != nil && r.Snapshot.IsMainFrame:
			s := r.Snapshot
			if page == "" {
				page = s.PageID
			} else if s.PageID != page {
				return none, nil, bad("trace zip records more than one page; supply the reporter JSON")
			}
			snapshots++
			phase, call, _ := strings.Cut(s.SnapshotName, "@")
			if call == "" {
				call = s.CallID
			}
			if phase == "before" {
				beforeURL[call] = s.FrameURL
			}
			if s.FrameURL == "" || len(s.FrameURL) > 4096 {
				return none, nil, bad("trace zip snapshot URL must be 1 to 4096 bytes")
			}
			if s.FrameURL == cur {
				continue
			}
			prev := cur
			cur = s.FrameURL
			if strings.HasPrefix(s.FrameURL, "about:") {
				continue
			}
			ev := NavigationEvent{Kind: "navigate", URL: s.FrameURL}
			if b, ok := beforeURL[call]; ok && (phase == "input" || phase == "after") && b == prev {
				if a := actions[call]; a != nil {
					c := *a
					ev.Action = &c
				}
			}
			if len(doc.Events) == maxTraceEvents {
				return none, nil, bad("trace zip has more than 4096 navigations")
			}
			doc.Events = append(doc.Events, ev)
		}
	}
	if sc.Err() != nil || limited.N <= 0 {
		return none, nil, bad("trace zip context trace exceeds its byte or line bound")
	}
	if snapshots == 0 {
		return none, nil, bad("trace zip has no main-frame snapshots; record the trace with snapshots on")
	}
	return doc, raw, nil
}
