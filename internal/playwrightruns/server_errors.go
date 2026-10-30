package playwrightruns

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Server-side exception attachment (docs/specs/observed-ui-evidence-v0.md, OUE-V0-050..057). A
// failed attempt whose errors name an HTTP 5xx from the system under test is matched to the
// server's own error record by method, path, status and time window (and request id when both
// sides carry one). The match attaches the exception class, its screened message and the first
// application frame, classifies a frame under a declared test-support path as
// environment/fixture, and groups repeats across runs. A failure with no matching record says
// "server error not found in log"; several distinct candidates read UNKNOWN.
const (
	ServerErrorsSchema   = "playwright-server-errors/0"
	ServerErrorLogSchema = "server-error-log/0"

	ServerErrorAttached = "ATTACHED"
	ServerErrorNotFound = "NOT_FOUND"
	ServerErrorUnknown  = "UNKNOWN"

	ClassEnvironmentFixture = "environment/fixture"
	ClassProduct            = "product"

	ServerErrorNotFoundReason = "server error not found in log"

	DefaultServerErrorWindow = 2 * time.Second

	maxServerErrorLogBytes = 64 << 20
	maxServerErrors        = 1 << 20
	maxServerFrames        = 64
)

// HTTPFailure is one HTTP 5xx response an attempt's error names. Method, Path and RequestID are
// "" when the message does not name them.
type HTTPFailure struct {
	Status    int
	Method    string
	Path      string
	RequestID string
}

var (
	httpStatusNear  = regexp.MustCompile(`(?i)(?:\bHTTP\b|\bstatus(?:\s*code)?\b|\breceived\b|\bresponse\b|\bgot\b)[^0-9\n]{0,24}\b(5[0-9]{2})\b`)
	httpStatusText  = regexp.MustCompile(`\b(5[0-9]{2})\s+(?:Internal Server Error|Not Implemented|Bad Gateway|Service Unavailable|Gateway Timeout)`)
	httpMethodURL   = regexp.MustCompile(`\b(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+(https?://[^\s"'<>,)]+|/[^\s"'<>,)]*)`)
	apiRequestCall  = regexp.MustCompile(`apiRequestContext\.(get|post|put|patch|delete|head)\b`)
	anyHTTPURL      = regexp.MustCompile(`https?://[^\s"'<>,)]+`)
	requestIDNear   = regexp.MustCompile(`(?i)\brequest[-_ ]?id\b["':=\s]{1,4}([A-Za-z0-9][A-Za-z0-9._-]{5,63})`)
	digitRun        = regexp.MustCompile(`[0-9]+`)
	nonAppFrameFile = regexp.MustCompile(`(?i)(^|/)(gems|vendor|node_modules|site-packages|dist-packages|bundle)/|/lib/ruby/|^<internal|^internal/|^node:|^/usr/(local/)?lib/`)
)

// httpFailure reads the 5xx response one error message names, if any.
func httpFailure(message string) (HTTPFailure, bool) {
	message = ansi.ReplaceAllString(message, "")
	f := HTTPFailure{}
	if m := httpStatusNear.FindStringSubmatch(message); m != nil {
		f.Status, _ = strconv.Atoi(m[1])
	} else if m := httpStatusText.FindStringSubmatch(message); m != nil {
		f.Status, _ = strconv.Atoi(m[1])
	} else {
		return f, false
	}
	if m := httpMethodURL.FindStringSubmatch(message); m != nil {
		f.Method, f.Path = m[1], requestPath(m[2])
	} else if m := apiRequestCall.FindStringSubmatch(message); m != nil {
		f.Method = strings.ToUpper(m[1])
		if u := anyHTTPURL.FindString(message); u != "" {
			f.Path = requestPath(u)
		}
	}
	if m := requestIDNear.FindStringSubmatch(message); m != nil {
		f.RequestID = m[1]
	}
	return f, true
}

// requestPath is a URL's path without query or fragment; "" when it cannot be read.
func requestPath(raw string) string {
	raw = strings.TrimRight(raw, ".;:")
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.Path == "" {
		return "/"
	}
	return u.Path
}

// ServerFrame is one frame of a server exception backtrace.
type ServerFrame struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function,omitempty"`
}

// ServerError is one record of a server-error-log/0 file.
type ServerError struct {
	Time      time.Time
	Method    string
	Path      string
	Status    int
	RequestID string
	Class     string
	Message   string
	Frames    []ServerFrame
}

// ReadServerErrorLog parses a server-error-log/0 JSON-lines document: one object per line with
// time (RFC 3339), method, path, status, optional request_id, and exception {class, message,
// frames[{file, line, function}]}. Unknown members are refused.
func ReadServerErrorLog(data []byte) ([]ServerError, error) {
	if len(data) > maxServerErrorLogBytes {
		return nil, ErrTooLarge
	}
	out := []ServerError{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var rec struct {
			Time      string `json:"time"`
			Method    string `json:"method"`
			Path      string `json:"path"`
			Status    int    `json:"status"`
			RequestID string `json:"request_id"`
			Exception struct {
				Class   string        `json:"class"`
				Message string        `json:"message"`
				Frames  []ServerFrame `json:"frames"`
			} `json:"exception"`
		}
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		if dec.Decode(&rec) != nil || dec.Decode(new(json.RawMessage)) != io.EOF {
			return nil, ErrShape
		}
		at, err := time.Parse(time.RFC3339Nano, rec.Time)
		if err != nil || rec.Method == "" || rec.Path == "" || rec.Status < 100 || rec.Status > 599 || rec.Exception.Class == "" || len(rec.Exception.Class) > 256 || len(rec.RequestID) > 128 {
			return nil, ErrShape
		}
		if len(out) == maxServerErrors {
			return nil, ErrTooLarge
		}
		frames := rec.Exception.Frames
		if len(frames) > maxServerFrames {
			frames = frames[:maxServerFrames]
		}
		class, _ := secretscreen.Screen(rec.Exception.Class)
		out = append(out, ServerError{Time: at, Method: strings.ToUpper(rec.Method), Path: requestPath(rec.Path), Status: rec.Status,
			RequestID: rec.RequestID, Class: class, Message: FirstLine(rec.Exception.Message), Frames: frames})
	}
	if sc.Err() != nil {
		return nil, ErrTooLarge
	}
	return out, nil
}

// ReadServerErrorLogFile opens one regular server error log without blocking and parses it.
func ReadServerErrorLogFile(name string) ([]ServerError, error) {
	f, err := openNonblocking(name)
	if err != nil {
		return nil, ErrUnreadable
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrUnreadable
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxServerErrorLogBytes+1))
	if err != nil {
		return nil, ErrUnreadable
	}
	return ReadServerErrorLog(raw)
}

// ServerErrorOptions configures one attachment. SupportPaths are repository path prefixes of
// test-support code (seed builders, factories).
type ServerErrorOptions struct {
	Window       time.Duration
	SupportPaths []string
}

// ServerException is the attached part of a server error.
type ServerException struct {
	Class   string       `json:"class"`
	Message string       `json:"message"`
	Frame   *ServerFrame `json:"first_application_frame"`
}

// ServerErrorFailure is one failed attempt's 5xx and what it was matched to.
type ServerErrorFailure struct {
	Run            int              `json:"run"`
	Project        string           `json:"project"`
	File           string           `json:"file"`
	TitlePath      []string         `json:"titlePath"`
	Retry          int              `json:"retry"`
	Status         string           `json:"status"`
	HTTPStatus     int              `json:"http_status"`
	Method         string           `json:"method,omitempty"`
	Path           string           `json:"path,omitempty"`
	MatchBasis     string           `json:"match_basis"`
	State          string           `json:"state"`
	Reason         string           `json:"reason,omitempty"`
	Classification string           `json:"classification,omitempty"`
	Group          string           `json:"group,omitempty"`
	Exception      *ServerException `json:"exception,omitempty"`
	Candidates     int              `json:"candidates"`
}

// ServerErrorGroup is one recurring server-side cause across runs.
type ServerErrorGroup struct {
	ID             string          `json:"id"`
	Classification string          `json:"classification"`
	Exception      ServerException `json:"exception"`
	Occurrences    int             `json:"occurrences"`
	Runs           []int           `json:"runs"`
	Tests          []string        `json:"tests"`
}

// ServerErrorCounts is the denominator: Failures = Attached + NotFound + Unknown.
type ServerErrorCounts struct {
	Failures           int `json:"failures_with_5xx"`
	Attached           int `json:"attached"`
	EnvironmentFixture int `json:"environment_fixture"`
	NotFound           int `json:"not_found"`
	Unknown            int `json:"unknown"`
	Groups             int `json:"groups"`
}

// ServerErrorReport is the playwright-server-errors/0 document.
type ServerErrorReport struct {
	Schema       string               `json:"schema"`
	WindowMS     int64                `json:"window_ms"`
	SupportPaths []string             `json:"support_paths"`
	Counts       ServerErrorCounts    `json:"counts"`
	Failures     []ServerErrorFailure `json:"failures"`
	Groups       []ServerErrorGroup   `json:"groups"`
}

// firstApplicationFrame skips library, vendored and runtime frames.
func firstApplicationFrame(frames []ServerFrame) *ServerFrame {
	for _, f := range frames {
		file := strings.ReplaceAll(f.File, "\\", "/")
		if file == "" || nonAppFrameFile.MatchString(file) {
			continue
		}
		screened, _ := secretscreen.Screen(file)
		fn, _ := secretscreen.Screen(f.Function)
		return &ServerFrame{File: screened, Line: f.Line, Function: fn}
	}
	return nil
}

func underSupportPath(file string, prefixes []string) bool {
	file = strings.TrimPrefix(strings.ReplaceAll(file, "\\", "/"), "./")
	for _, p := range prefixes {
		p = strings.Trim(strings.TrimPrefix(p, "./"), "/")
		if p == "" {
			continue
		}
		if strings.HasPrefix(file, p+"/") || strings.Contains(file, "/"+p+"/") {
			return true
		}
	}
	return false
}

func serverGroupKey(e ServerError, frame *ServerFrame) string {
	at := ""
	if frame != nil {
		at = frame.File + ":" + strconv.Itoa(frame.Line)
	}
	sum := sha256.Sum256([]byte(e.Class + "\x00" + digitRun.ReplaceAllString(e.Message, "N") + "\x00" + at))
	return "server-error:" + hex.EncodeToString(sum[:8])
}

// AttachServerErrors matches every failed attempt that names an HTTP 5xx to the server error
// log (OUE-V0-051..056). requests is an optional HAR or JSON-lines request log used only to
// name the method and path a failure message leaves out.
func AttachServerErrors(runs []*Run, errs []ServerError, requests *RequestLog, o ServerErrorOptions) ServerErrorReport {
	if o.Window <= 0 {
		o.Window = DefaultServerErrorWindow
	}
	support := append([]string{}, o.SupportPaths...)
	sort.Strings(support)
	rep := ServerErrorReport{Schema: ServerErrorsSchema, WindowMS: o.Window.Milliseconds(), SupportPaths: support, Failures: []ServerErrorFailure{}, Groups: []ServerErrorGroup{}}
	groups := map[string]*ServerErrorGroup{}
	groupRuns := map[string]map[int]bool{}
	groupTests := map[string]map[string]bool{}
	for ri, run := range runs {
		for _, t := range run.Tests {
			for _, res := range t.Results {
				if res.Status != "failed" && res.Status != "timedOut" {
					continue
				}
				for _, hf := range res.HTTPFailures {
					row := ServerErrorFailure{Run: ri + 1, Project: t.Project, File: t.File, TitlePath: t.TitlePath, Retry: res.Retry, Status: res.Status,
						HTTPStatus: hf.Status, Method: hf.Method}
					row.Path, _ = secretscreen.Screen(hf.Path)
					attachOne(&row, hf, res, errs, requests, o, support)
					switch row.State {
					case ServerErrorAttached:
						rep.Counts.Attached++
						if row.Classification == ClassEnvironmentFixture {
							rep.Counts.EnvironmentFixture++
						}
						g := groups[row.Group]
						if g == nil {
							g = &ServerErrorGroup{ID: row.Group, Classification: row.Classification, Exception: *row.Exception}
							groups[row.Group], groupRuns[row.Group], groupTests[row.Group] = g, map[int]bool{}, map[string]bool{}
						}
						g.Occurrences++
						groupRuns[row.Group][ri+1] = true
						groupTests[row.Group][t.Key()] = true
					case ServerErrorNotFound:
						rep.Counts.NotFound++
					default:
						rep.Counts.Unknown++
					}
					rep.Failures = append(rep.Failures, row)
				}
			}
		}
	}
	for id, g := range groups {
		for r := range groupRuns[id] {
			g.Runs = append(g.Runs, r)
		}
		sort.Ints(g.Runs)
		for k := range groupTests[id] {
			g.Tests = append(g.Tests, k)
		}
		sort.Strings(g.Tests)
		rep.Groups = append(rep.Groups, *g)
	}
	sort.Slice(rep.Groups, func(i, j int) bool {
		if rep.Groups[i].Occurrences != rep.Groups[j].Occurrences {
			return rep.Groups[i].Occurrences > rep.Groups[j].Occurrences
		}
		return rep.Groups[i].ID < rep.Groups[j].ID
	})
	rep.Counts.Failures = len(rep.Failures)
	rep.Counts.Groups = len(rep.Groups)
	return rep
}

type methodPath struct{ method, path string }

func attachOne(row *ServerErrorFailure, hf HTTPFailure, res Result, errs []ServerError, requests *RequestLog, o ServerErrorOptions, support []string) {
	if !res.HasStartTime {
		row.State, row.Reason, row.MatchBasis = ServerErrorUnknown, "result start time missing from the report", "none"
		return
	}
	from := res.StartTime.Add(-o.Window)
	to := res.StartTime.Add(time.Duration(res.DurationMS)*time.Millisecond + o.Window)
	inWindow := func(at time.Time) bool { return !at.Before(from) && !at.After(to) }
	wanted := map[methodPath]bool{}
	row.MatchBasis = "method-path-time"
	switch {
	case hf.Method != "" && hf.Path != "":
		wanted[methodPath{hf.Method, hf.Path}] = true
	case requests != nil:
		for _, r := range requests.Requests {
			if r.Status == hf.Status && inWindow(r.Started) && (hf.Method == "" || r.Method == hf.Method) {
				wanted[methodPath{strings.ToUpper(r.Method), requestPath(r.URL)}] = true
			}
		}
		row.MatchBasis = "request-log-method-path-time"
	}
	if len(wanted) == 0 {
		// Whatever the failure names still constrains the match: a known method or path is
		// never dropped because the other half is missing.
		switch {
		case hf.Method != "":
			row.MatchBasis = "method-time"
		case hf.Path != "":
			row.MatchBasis = "path-time"
		default:
			row.MatchBasis = "status-time"
		}
	}
	if hf.RequestID != "" {
		row.MatchBasis += "+request-id"
	}
	byGroup := map[string]ServerError{}
	candidates := 0
	for _, e := range errs {
		if !inWindow(e.Time) || e.Status != hf.Status || (hf.Method != "" && e.Method != hf.Method) || (hf.Path != "" && e.Path != hf.Path) {
			continue
		}
		if len(wanted) > 0 && !wanted[methodPath{e.Method, e.Path}] {
			continue
		}
		if hf.RequestID != "" && e.RequestID != "" && hf.RequestID != e.RequestID {
			continue
		}
		candidates++
		byGroup[serverGroupKey(e, firstApplicationFrame(e.Frames))] = e
	}
	row.Candidates = candidates
	switch len(byGroup) {
	case 0:
		row.State, row.Reason = ServerErrorNotFound, ServerErrorNotFoundReason
	case 1:
		for id, e := range byGroup {
			frame := firstApplicationFrame(e.Frames)
			row.State, row.Group = ServerErrorAttached, id
			row.Exception = &ServerException{Class: e.Class, Message: e.Message, Frame: frame}
			switch {
			case frame == nil:
				row.Classification, row.Reason = ClassProduct, "no application frame; not classified as test support"
			case underSupportPath(frame.File, support):
				row.Classification = ClassEnvironmentFixture
			default:
				row.Classification = ClassProduct
			}
		}
	default:
		row.State, row.Reason = ServerErrorUnknown, "ambiguous: "+strconv.Itoa(len(byGroup))+" distinct server errors match"
	}
}
