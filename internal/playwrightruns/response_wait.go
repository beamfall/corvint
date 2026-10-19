package playwrightruns

// Response-wait timeout diagnosis (docs/specs/test-source-analysis-v0.md, TSA-V0-200..205). A
// browser test that times out waiting for a response is read against the run's request log: a
// HAR browser network log or a JSON-lines server request log. The reading says whether a request
// matching the wait's predicate reached the log during the wait, and when none did, which failed
// requests on the same page came before the deadline. Nothing here runs a process, opens a
// network connection or writes a file.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// ResponseWaitSchema names the `corvint runs response-wait` report (TSA-V0-200).
const ResponseWaitSchema = "corvint-runs-response-wait/0"

// Response-wait verdicts (TSA-V0-201). The set is closed.
const (
	WaitRequestNeverSent = "REQUEST_NEVER_SENT"
	WaitSlowResponse     = "SLOW_RESPONSE"
	WaitResponseInWait   = "RESPONSE_IN_WAIT"
	WaitUnknown          = "UNKNOWN"
)

// Request-log sources and the reasons an UNKNOWN verdict carries (TSA-V0-200, TSA-V0-204).
const (
	LogSourceHAR       = "HAR"
	LogSourceServer    = "SERVER_LOG"
	WaitLogUnavailable = "request-log-unavailable"
	WaitLogUnreadable  = "request-log-unreadable"
	WaitLogEmpty       = "request-log-empty"
	WaitLogNotCovering = "request-log-starts-after-wait"
	WaitNoResponse     = "no-response-recorded"
	FailureScopePage   = "PAGE"
	FailureScopeLog    = "LOG"
	maxRequestLogBytes = 64 << 20
	maxLoggedRequests  = 200000
	maxPrecedingFailed = 5
	maxDurationMs      = 7 * 24 * 60 * 60 * 1000
)

// LoggedRequest is one request of a run's request log. Duration is negative when the log holds
// no completed response for it; Status is 0 then.
type LoggedRequest struct {
	Started  time.Time
	Method   string
	URL      string
	Status   int
	Duration time.Duration
	Page     string
}

// PageStart is one page of a HAR log: its id and when it started.
type PageStart struct {
	ID      string
	Started time.Time
}

// RequestLog is a parsed run request log in start order.
type RequestLog struct {
	Source   string
	Pages    []PageStart
	Requests []LoggedRequest
}

// ResponseWait is the wait to diagnose: the predicate a response had to satisfy (a URL substring
// and, when set, the method) and the window from Start to Start+Timeout.
type ResponseWait struct {
	URLContains string
	Method      string
	Start       time.Time
	Timeout     time.Duration
}

// WaitRequest is one request named by a response-wait report. The URL keeps scheme, host and path
// only; query and fragment are dropped and the rest passes the secret screen.
type WaitRequest struct {
	Started    string `json:"started"`
	Method     string `json:"method"`
	URL        string `json:"url"`
	Status     int    `json:"status"`
	DurationMs *int64 `json:"durationMs"`
}

// ResponseWaitReport is the deterministic diagnosis of one timed-out response wait (TSA-V0-201).
type ResponseWaitReport struct {
	Schema            string        `json:"schema"`
	Verdict           string        `json:"verdict"`
	Reason            string        `json:"reason,omitempty"`
	LogSource         string        `json:"logSource,omitempty"`
	WaitStart         string        `json:"waitStart"`
	WaitDeadline      string        `json:"waitDeadline"`
	Matched           *WaitRequest  `json:"matched,omitempty"`
	LateByMs          *int64        `json:"lateByMs,omitempty"`
	FailureScope      string        `json:"failureScope,omitempty"`
	Page              string        `json:"page,omitempty"`
	PrecedingFailures []WaitRequest `json:"precedingFailures"`
}

// ReadRequestLog parses a HAR document (an object with log.entries) or a JSON-lines server log
// (one object per line with time, method, url or path, status and durationMs) (TSA-V0-200).
func ReadRequestLog(data []byte) (*RequestLog, error) {
	if len(data) > maxRequestLogBytes {
		return nil, ErrTooLarge
	}
	trimmed := bytes.TrimSpace(data)
	var har struct {
		Log *struct {
			Pages []struct {
				ID      string `json:"id"`
				Started string `json:"startedDateTime"`
			} `json:"pages"`
			Entries []struct {
				PageRef string   `json:"pageref"`
				Started string   `json:"startedDateTime"`
				Time    *float64 `json:"time"`
				Request struct {
					Method string `json:"method"`
					URL    string `json:"url"`
				} `json:"request"`
				Response struct {
					Status int `json:"status"`
				} `json:"response"`
			} `json:"entries"`
		} `json:"log"`
	}
	if json.Unmarshal(trimmed, &har) == nil && har.Log != nil {
		out := &RequestLog{Source: LogSourceHAR}
		for _, p := range har.Log.Pages {
			started, err := time.Parse(time.RFC3339Nano, p.Started)
			if err != nil {
				return nil, ErrShape
			}
			out.Pages = append(out.Pages, PageStart{ID: p.ID, Started: started})
		}
		if len(har.Log.Entries) > maxLoggedRequests {
			return nil, ErrTooLarge
		}
		for _, e := range har.Log.Entries {
			started, err := time.Parse(time.RFC3339Nano, e.Started)
			if err != nil || e.Request.URL == "" || e.Time == nil || *e.Time > maxDurationMs {
				return nil, ErrShape
			}
			duration := time.Duration(*e.Time * float64(time.Millisecond))
			if e.Response.Status == 0 || *e.Time < 0 {
				duration = -1
			}
			out.Requests = append(out.Requests, LoggedRequest{Started: started, Method: strings.ToUpper(e.Request.Method),
				URL: e.Request.URL, Status: e.Response.Status, Duration: duration, Page: e.PageRef})
		}
		out.sort()
		return out, nil
	}
	out := &RequestLog{Source: LogSourceServer}
	for rest := trimmed; len(rest) > 0; {
		line := rest
		if i := bytes.IndexByte(rest, '\n'); i >= 0 {
			line, rest = rest[:i], rest[i+1:]
		} else {
			rest = nil
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var e struct {
			Time       string   `json:"time"`
			Method     string   `json:"method"`
			URL        string   `json:"url"`
			Path       string   `json:"path"`
			Status     int      `json:"status"`
			DurationMs *float64 `json:"durationMs"`
		}
		if json.Unmarshal(line, &e) != nil {
			return nil, ErrShape
		}
		started, err := time.Parse(time.RFC3339Nano, e.Time)
		target := e.URL
		if target == "" {
			target = e.Path
		}
		if err != nil || target == "" {
			return nil, ErrShape
		}
		duration := time.Duration(-1)
		if e.DurationMs != nil && *e.DurationMs > maxDurationMs {
			return nil, ErrShape
		}
		if e.DurationMs != nil && *e.DurationMs >= 0 && e.Status != 0 {
			duration = time.Duration(*e.DurationMs * float64(time.Millisecond))
		}
		if len(out.Requests) >= maxLoggedRequests {
			return nil, ErrTooLarge
		}
		out.Requests = append(out.Requests, LoggedRequest{Started: started, Method: strings.ToUpper(e.Method),
			URL: target, Status: e.Status, Duration: duration})
	}
	out.sort()
	return out, nil
}

func (l *RequestLog) sort() {
	sort.SliceStable(l.Requests, func(i, j int) bool { return l.Requests[i].Started.Before(l.Requests[j].Started) })
	sort.SliceStable(l.Pages, func(i, j int) bool { return l.Pages[i].Started.Before(l.Pages[j].Started) })
}

// DiagnoseResponseWait reads one timed-out wait against log (TSA-V0-201..204). A nil log, or one
// with no request, abstains as UNKNOWN; it never reports a request as unsent without a log that
// covers the wait.
func DiagnoseResponseWait(log *RequestLog, wait ResponseWait, reason string) ResponseWaitReport {
	deadline := wait.Start.Add(wait.Timeout)
	report := ResponseWaitReport{Schema: ResponseWaitSchema, WaitStart: wait.Start.UTC().Format(time.RFC3339Nano),
		WaitDeadline: deadline.UTC().Format(time.RFC3339Nano), PrecedingFailures: []WaitRequest{}}
	if log == nil {
		report.Verdict, report.Reason = WaitUnknown, reason
		return report
	}
	report.LogSource = log.Source
	if len(log.Requests) == 0 {
		report.Verdict, report.Reason = WaitUnknown, WaitLogEmpty
		return report
	}
	if log.Requests[0].Started.After(deadline) {
		report.Verdict, report.Reason = WaitUnknown, WaitLogNotCovering
		return report
	}
	page := ""
	for _, p := range log.Pages {
		if !p.Started.After(wait.Start) {
			page = p.ID
		}
	}
	// The first eligible match is reported, unless a later one answered within the window.
	var first *LoggedRequest
	for i, r := range log.Requests {
		if !matchesWait(r, wait) || r.Started.After(deadline) {
			continue
		}
		// A response completing before the wait began cannot satisfy it.
		if r.Duration >= 0 && r.Started.Add(r.Duration).Before(wait.Start) {
			continue
		}
		if r.Duration >= 0 && !r.Started.Add(r.Duration).After(deadline) {
			first = &log.Requests[i]
			break
		}
		if first == nil {
			first = &log.Requests[i]
		}
	}
	if first != nil {
		r := *first
		matched := waitRequest(r)
		report.Matched = &matched
		switch {
		case r.Duration < 0:
			report.Verdict, report.Reason = WaitSlowResponse, WaitNoResponse
		case r.Started.Add(r.Duration).After(deadline):
			late := r.Started.Add(r.Duration).Sub(deadline).Milliseconds()
			report.Verdict, report.LateByMs = WaitSlowResponse, &late
		default:
			report.Verdict = WaitResponseInWait
		}
		return report
	}
	report.Verdict = WaitRequestNeverSent
	report.FailureScope, report.Page = FailureScopeLog, page
	if page != "" {
		report.FailureScope = FailureScopePage
	}
	failures := []WaitRequest{}
	for _, r := range log.Requests {
		if r.Started.After(deadline) || r.Status < 400 || r.Status > 599 || (page != "" && r.Page != page) {
			continue
		}
		failures = append(failures, waitRequest(r))
	}
	if len(failures) > maxPrecedingFailed {
		failures = failures[len(failures)-maxPrecedingFailed:]
	}
	report.PrecedingFailures = failures
	return report
}

func matchesWait(r LoggedRequest, wait ResponseWait) bool {
	if wait.Method != "" && !strings.EqualFold(r.Method, wait.Method) {
		return false
	}
	return strings.Contains(r.URL, wait.URLContains)
}

func waitRequest(r LoggedRequest) WaitRequest {
	out := WaitRequest{Started: r.Started.UTC().Format(time.RFC3339Nano), Method: r.Method, URL: displayURL(r.URL), Status: r.Status}
	if r.Duration >= 0 {
		ms := r.Duration.Milliseconds()
		out.DurationMs = &ms
	}
	return out
}

// displayURL keeps scheme, host and path; query, fragment and user information are dropped.
func displayURL(raw string) string {
	shown := raw
	if parsed, err := url.Parse(raw); err == nil {
		parsed.RawQuery, parsed.Fragment, parsed.User = "", "", nil
		shown = parsed.String()
	} else if i := strings.IndexAny(raw, "?#"); i >= 0 {
		shown = raw[:i]
	}
	shown, _ = secretscreen.Screen(shown)
	return shown
}

// ReadRequestLogFile opens one regular request-log file without blocking and parses it.
func ReadRequestLogFile(name string) (*RequestLog, error) {
	f, err := openNonblocking(name)
	if err != nil {
		return nil, ErrUnreadable
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrUnreadable
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxRequestLogBytes+1))
	if err != nil {
		return nil, ErrUnreadable
	}
	return ReadRequestLog(raw)
}
