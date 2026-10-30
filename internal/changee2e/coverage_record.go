package changee2e

import (
	"context"
	"encoding/json"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"
)

// Unrecorded reasons of a scenario's recorded coverage (CCE-V0-053).
const (
	UnrecordedHosted      = "hosted-test-without-screen"
	UnrecordedEnvironment = "declared-environment-serve"
	UnrecordedUnqualified = "head-pool-unqualified"
	UnrecordedNoPath      = "no-application-path-served"
)

// recordedCoverage is the application paths the in-process static server served while the head
// pool ran a compiled scenario, attributed to the screen its outcome assertion shows and pinned to
// their head blobs (CCE-V0-053).
type recordedCoverage struct {
	Screen     string        `json:"screen,omitempty"`
	Paths      []string      `json:"paths"`
	Pins       []coveragePin `json:"pins"`
	Receipts   []string      `json:"receipts"` // the head pool's qualified receipt digests
	Unrecorded string        `json:"unrecorded,omitempty"`
}

// coveragePin binds a recorded path to the blob the head revision holds for it.
type coveragePin struct {
	Path string `json:"path"`
	Blob string `json:"blob"`
}

// coverageRecorder collects the request paths one pool's static server answered.
type coverageRecorder struct {
	mu    sync.Mutex
	paths map[string]bool
}

func newCoverageRecorder() *coverageRecorder { return &coverageRecorder{paths: map[string]bool{}} }

func (r *coverageRecorder) record(p string) {
	r.mu.Lock()
	r.paths[p] = true
	r.mu.Unlock()
}

// recordFunc is nil for a nil recorder, so the pool runs unrecorded.
func (r *coverageRecorder) recordFunc() func(string) {
	if r == nil {
		return nil
	}
	return r.record
}

// recorded reads a scenario's recorded coverage from the head pool (CCE-V0-053): a hosted test
// has no screen, a declared environment serves outside the recorder, and an unqualified head pool
// pins nothing.
func (e *proveEnv) recorded(ctx context.Context, head string, tt testTree, sc Scenario, side sideProof, r *coverageRecorder) *recordedCoverage {
	c := &recordedCoverage{Screen: sc.Screen, Paths: []string{}, Pins: []coveragePin{}, Receipts: side.Receipts}
	switch {
	case sc.Screen == "":
		c.Unrecorded = UnrecordedHosted
	case e.environment != nil:
		c.Unrecorded = UnrecordedEnvironment
	case !side.Qualified || r == nil:
		c.Unrecorded = UnrecordedUnqualified
	}
	if c.Unrecorded != "" {
		return c
	}
	paths, pins, err := pinnedCoverage(ctx, e.root, head, tt, r.sorted())
	if err != nil {
		c.Unrecorded = "head-blobs-unreadable"
		return c
	}
	if len(paths) == 0 {
		c.Unrecorded = UnrecordedNoPath
	}
	c.Paths, c.Pins = paths, pins
	return c
}

func (r *coverageRecorder) sorted() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.paths))
	for p := range r.paths {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// recordingHandler serves through next and records the tree-relative path of every request
// answered 200 or 304; a directory request records its index.html, as http.FileServer serves it.
func recordingHandler(next http.Handler, record func(string)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, req)
		if sw.status != http.StatusOK && sw.status != http.StatusNotModified {
			return
		}
		p := req.URL.Path
		if strings.HasSuffix(p, "/") {
			p += "index.html"
		}
		if p = strings.TrimPrefix(path.Clean("/"+p), "/"); p != "" {
			record(p)
		}
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// pinnedCoverage keeps the served paths that are application files at head, outside the staged
// test tree and node_modules, and pins each to its head blob (CCE-V0-053).
func pinnedCoverage(ctx context.Context, root, head string, tt testTree, served []string) ([]string, []coveragePin, error) {
	var candidates []string
	for _, p := range served {
		if _, test := tt.files[p]; test || p == "node_modules" || strings.HasPrefix(p, "node_modules/") {
			continue
		}
		candidates = append(candidates, p)
	}
	blobs, err := headBlobs(ctx, root, head, candidates)
	if err != nil {
		return nil, nil, err
	}
	paths, pins := []string{}, []coveragePin{}
	for _, p := range candidates {
		if oid, ok := blobs[p]; ok {
			paths = append(paths, p)
			pins = append(pins, coveragePin{Path: p, Blob: oid})
		}
	}
	return paths, pins, nil
}

// headBlobs returns the blob ID head holds for each of paths that is a regular blob there.
func headBlobs(ctx context.Context, root, head string, paths []string) (map[string]string, error) {
	out := map[string]string{}
	for len(paths) > 0 {
		n := min(len(paths), 256)
		raw, err := git(ctx, root, append([]string{"ls-tree", "-z", "--full-tree", head, "--"}, paths[:n]...)...)
		if err != nil {
			return nil, err
		}
		for _, rec := range strings.Split(string(raw), "\x00") {
			meta, name, ok := strings.Cut(rec, "\t")
			if f := strings.Fields(meta); ok && len(f) == 3 && f[1] == "blob" {
				out[name] = f[2]
			}
		}
		paths = paths[n:]
	}
	return out, nil
}

// CoverageFromProof derives an e2e-runtime-coverage/0 document bound to the proof's head from the
// recorded coverage of an e2e-prove/0 document (CCE-V0-053): one entry per screen with the union
// of its recorded paths and their pins, and the proof digest. `--coverage` consumes it
// (CCE-V0-045..047).
func CoverageFromProof(proof []byte) ([]byte, error) {
	var r proveReport
	if err := json.Unmarshal(proof, &r); err != nil || r.Schema != ProveSchema || r.Head == "" {
		return nil, refuse("e2e-coverage-invalid", "not an %s document", ProveSchema)
	}
	byScreen := map[string]map[string]string{}
	for _, s := range r.Scenarios {
		c := s.RecordedCoverage
		if c == nil || c.Screen == "" || c.Unrecorded != "" {
			continue
		}
		if byScreen[c.Screen] == nil {
			byScreen[c.Screen] = map[string]string{}
		}
		for _, pin := range c.Pins {
			byScreen[c.Screen][pin.Path] = pin.Blob
		}
	}
	doc := runtimeCoverage{Schema: CoverageSchema, Revision: r.Head, ProofDigest: r.Digest, Screens: []coverageScreen{}}
	for screen, pins := range byScreen {
		e := coverageScreen{Screen: screen, Paths: []string{}, Pins: []coveragePin{}}
		for p := range pins {
			e.Paths = append(e.Paths, p)
		}
		sort.Strings(e.Paths)
		for _, p := range e.Paths {
			e.Pins = append(e.Pins, coveragePin{Path: p, Blob: pins[p]})
		}
		doc.Screens = append(doc.Screens, e)
	}
	sort.Slice(doc.Screens, func(i, j int) bool { return doc.Screens[i].Screen < doc.Screens[j].Screen })
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
