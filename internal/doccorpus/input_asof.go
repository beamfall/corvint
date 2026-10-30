package doccorpus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// InputAsOf is a compiled input's declared position in its own upstream source
// at build time (DCP-V1-574): the source it was compiled from, the kind of
// position, and the value. Probe optionally names a primary-checkout file whose
// live content or modification time the reader compares with the value
// (DCP-V1-575). An undeclared point is a nil Input.AsOf and is omitted, so
// manifests without it keep their bytes.
type InputAsOf struct {
	Source string `json:"source"`
	Kind   string `json:"kind"`
	Value  string `json:"value"`
	Probe  string `json:"probe,omitempty"`
}

// InputFreshness is one declared compiled input's live position against its
// as-of point (DCP-V1-576). State is fresh, stale or unknown; a stale input
// whose lag is measurable reports it in LagUnit. Reason says what was or was
// not observed.
type InputFreshness struct {
	Path     string    `json:"path"`
	Revision string    `json:"revision"`
	Purpose  string    `json:"purpose"`
	AsOf     InputAsOf `json:"as_of"`
	State    string    `json:"state"`
	Live     string    `json:"live,omitempty"`
	Lag      *int64    `json:"lag,omitempty"`
	LagUnit  string    `json:"lag_unit,omitempty"`
	Reason   string    `json:"reason"`
}

// validInputAsOf names the rule a declared as-of point breaks, or returns "".
func validInputAsOf(in Input) string {
	if in.AsOf == nil {
		return ""
	}
	x := *in.AsOf
	if in.Purpose == "source" {
		return "as_of applies only to compiled inputs (provider, observation or evidence)"
	}
	if x.Source == "" || len(x.Source) > 128 || !utf8.ValidString(x.Source) || strings.ContainsFunc(x.Source, unicode.IsControl) {
		return "as_of source must be 1 to 128 bytes of UTF-8 without control characters"
	}
	switch x.Kind {
	case "timestamp":
		if _, err := time.Parse(time.RFC3339, x.Value); err != nil {
			return "as_of timestamp value must be RFC 3339"
		}
	case "sha256":
		if !wire.IsSha256(x.Value) {
			return "as_of sha256 value must be 64 lowercase hex characters"
		}
	case "sequence":
		if _, ok := canonicalSequence(x.Value); !ok {
			return "as_of sequence value must be a canonical unsigned decimal"
		}
	default:
		return "as_of kind must be timestamp, sha256 or sequence"
	}
	if x.Probe != "" && (!validPath(x.Probe) || gitMetadataPath(x.Probe)) {
		return "as_of probe must be a canonical relative path outside .git"
	}
	return ""
}

// gitMetadataPath reports whether any segment of path names Git metadata,
// ignoring case so a case-insensitive filesystem cannot alias ".GIT" to ".git".
func gitMetadataPath(path string) bool {
	return slices.ContainsFunc(strings.Split(path, "/"), func(seg string) bool { return strings.EqualFold(seg, ".git") })
}

func canonicalSequence(value string) (uint64, bool) {
	n, err := strconv.ParseUint(value, 10, 64)
	return n, err == nil && strconv.FormatUint(n, 10) == value
}

// declaredAsOf returns the inputs that declare an as-of point, in manifest order.
func declaredAsOf(a *Artifact) []Input {
	out := []Input{}
	for _, in := range a.Manifest.Inputs {
		if in.AsOf != nil {
			out = append(out, in)
		}
	}
	return out
}

// staticInputFreshness reports every declared input as unknown: a reader that
// probed no live source can never call a lagging input current (DCP-V1-576).
func staticInputFreshness(a *Artifact) []InputFreshness {
	out := []InputFreshness{}
	for _, in := range declaredAsOf(a) {
		out = append(out, InputFreshness{Path: in.Path, Revision: in.Revision, Purpose: in.Purpose, AsOf: *in.AsOf, State: "unknown", Reason: "no live reader probed the source"})
	}
	return out
}

// probeInputFreshness compares each declared input with its probe in the
// primary checkout at root. A probe it cannot read, or a position it cannot
// order, is unknown with its reason, never fresh; only the caller's context
// cancellation is an error (DCP-V1-575).
func probeInputFreshness(ctx context.Context, root string, a *Artifact) ([]InputFreshness, error) {
	out := []InputFreshness{}
	for _, in := range declaredAsOf(a) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out = append(out, probeInput(root, in))
	}
	return out, nil
}

func probeInput(root string, in Input) InputFreshness {
	x := *in.AsOf
	f := InputFreshness{Path: in.Path, Revision: in.Revision, Purpose: in.Purpose, AsOf: x, State: "unknown"}
	if x.Probe == "" {
		f.Reason = "no probe declared"
		return f
	}
	unavailable := func(err error) InputFreshness {
		f.Reason = "probe " + x.Probe + " unavailable"
		if e := (*Error)(nil); errors.As(err, &e) && e.Code != "" {
			f.Reason += " (" + e.Code + ")"
		}
		return f
	}
	lag := func(n int64, unit string) { f.State, f.Lag, f.LagUnit = "stale", &n, unit }
	switch x.Kind {
	case "sha256":
		h := sha256.New()
		if err := streamCorpusFile(root, x.Probe, func(r io.Reader) error { _, err := io.Copy(h, r); return err }); err != nil {
			return unavailable(err)
		}
		f.Live = hex.EncodeToString(h.Sum(nil))
		if f.Live == x.Value {
			f.State, f.Reason = "fresh", "probe digest equals the as-of digest"
		} else {
			f.State, f.Reason = "stale", "probe digest differs from the as-of digest; lag is not measurable"
		}
	case "sequence":
		data, err := readFileBounded(root, x.Probe, 64)
		if err != nil {
			return unavailable(err)
		}
		live, ok := canonicalSequence(strings.TrimSpace(string(data)))
		pinned, _ := canonicalSequence(x.Value)
		switch {
		case !ok:
			f.Reason = "probe " + x.Probe + " holds no canonical sequence"
		case live > pinned:
			f.Live = strconv.FormatUint(live, 10)
			if d := live - pinned; d <= 1<<62 {
				lag(int64(d), "entries")
			} else {
				f.State = "stale"
			}
			f.Reason = "source sequence advanced past the as-of point"
		case live == pinned:
			f.Live, f.State, f.Reason = x.Value, "fresh", "probe sequence equals the as-of sequence"
		default:
			f.Live, f.Reason = strconv.FormatUint(live, 10), "probe sequence is behind the as-of point; the source cannot be ordered"
		}
	case "timestamp":
		info, err := StatCorpusFile(root, x.Probe)
		if err != nil {
			return unavailable(err)
		}
		pinned, _ := time.Parse(time.RFC3339, x.Value)
		modified := info.ModTime().UTC()
		f.Live = modified.Format(time.RFC3339Nano)
		if modified.After(pinned) {
			d := modified.Sub(pinned)
			lag(int64(d/time.Second), "seconds")
			f.Reason = "probe modified after the as-of time (modification time, not content)"
		} else {
			f.State, f.Reason = "fresh", "probe not modified after the as-of time (modification time, not content)"
		}
	}
	return f
}

// inputAsOfLimitations names each declared input that is not fresh, so every
// answer that used it carries the limitation (DCP-V1-576). Attribution is
// artifact-wide: any answer from the artifact counts as having used each
// declared input.
func inputAsOfLimitations(states []InputFreshness) []string {
	out := []string{}
	for _, s := range states {
		head := fmt.Sprintf("compiled input %s (%s) as of %s %s; ", s.Path, s.AsOf.Source, s.AsOf.Kind, s.AsOf.Value)
		switch s.State {
		case "stale":
			reason := "source has advanced"
			if s.Lag != nil {
				reason += fmt.Sprintf(" by %d %s", *s.Lag, s.LagUnit)
			}
			out = append(out, head+reason+": "+s.Reason)
		case "unknown":
			out = append(out, head+"source position NOT_OBSERVED: "+s.Reason)
		}
	}
	return out
}

// foldInputFreshness folds the per-input states into an artifact state: a
// stale input makes it stale and an unknown one makes fresh unknown.
func foldInputFreshness(state string, states []InputFreshness) string {
	for _, s := range states {
		switch {
		case s.State == "stale" && (state == "fresh" || state == "unknown"):
			state = "stale"
		case s.State == "unknown" && state == "fresh":
			state = "unknown"
		}
	}
	return state
}

// probeObservationKey is the lstat record of every declared probe, so a reader
// that reuses an overlay by observation re-probes when a probe changes, even an
// untracked or ignored one. A probe compared by content (sha256 or sequence)
// also contributes its content digest, so a rewrite that keeps size and
// modification time still changes the key. It is empty without probes. A probe
// modified inside the racy window gets a key no later observation can equal
// (DCP-V1-577).
func probeObservationKey(root string, a *Artifact) string {
	content := map[string]bool{}
	for _, in := range declaredAsOf(a) {
		if in.AsOf.Probe != "" {
			content[in.AsOf.Probe] = content[in.AsOf.Probe] || in.AsOf.Kind != "timestamp"
		}
	}
	if len(content) == 0 {
		return ""
	}
	probes := slices.Sorted(maps.Keys(content))
	now := time.Now()
	var b strings.Builder
	for _, p := range probes {
		record := "absent"
		if info, err := StatCorpusFile(root, p); err == nil {
			record = fmt.Sprintf("%d:%d:%o", info.Size(), info.ModTime().UnixNano(), info.Mode())
			if now.Sub(info.ModTime()) < 2*time.Second {
				record += fmt.Sprintf(":racy:%d", now.UnixNano())
			}
			if content[p] {
				h := sha256.New()
				if err := streamCorpusFile(root, p, func(r io.Reader) error { _, err := io.Copy(h, r); return err }); err != nil {
					record += ":unreadable"
				} else {
					record += ":" + hex.EncodeToString(h.Sum(nil))
				}
			}
		}
		fmt.Fprintf(&b, "%d:%s;%d:%s;", len(p), p, len(record), record)
	}
	return b.String()
}

func withProbeKey(key, probes string) string {
	if probes == "" {
		return key
	}
	return key + "\x00probes\x00" + probes
}
