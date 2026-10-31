package appmap

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// Blocker 404 cross-check (AMAP-V0-140..146): a blocked or fixture-gap classification, or a run
// failure note, that cites an HTTP 404 for a request is resolved against the compiled Rails route
// table. A request no route matches is flagged BLOCKER-404-NO-ROUTE with the nearest routes, a
// matched route whose handler lacks the action reuses ROUTE-ACTION-MISSING, a matched route with a
// defined action is accepted, and anything the table cannot decide is uncertain with a reason.
const (
	BlockerSchema      = "application-map-blocker/0"
	RuleBlocker404     = "BLOCKER-404-NO-ROUTE"
	maxBlockerNote     = 16384
	maxBlockerRequests = 16
	maxNearestRoutes   = 3
)

var (
	blocker404     = regexp.MustCompile(`(^|[^0-9A-Za-z])404([^0-9A-Za-z]|$)`)
	blockerRequest = regexp.MustCompile(`(^|[^A-Za-z])(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+(?:https?://[^/\s]+)?(/[^\s"'<>` + "`" + `]*)`)
)

type citedRequest struct {
	line       int
	verb, path string
}

type nearRoute struct {
	Verb        string `json:"verb"`
	Path        string `json:"path"`
	Declaration string `json:"declaration"`
	Controller  string `json:"controller"`
	Action      string `json:"action"`
}

type blockerRow struct {
	Request     string      `json:"request"`
	NoteLine    int         `json:"note_line"`
	Verdict     string      `json:"verdict"` // accepted, flagged, uncertain
	Rule        string      `json:"rule,omitempty"`
	Route       *nearRoute  `json:"route,omitempty"`
	HandlerFile string      `json:"handler_file,omitempty"`
	Basis       string      `json:"basis,omitempty"`
	Reason      string      `json:"reason,omitempty"`
	At          string      `json:"at,omitempty"`
	Nearest     []nearRoute `json:"nearest,omitempty"`
}

// BlockerReport is the blocker document; Flagged decides the exit status (AMAP-V0-145).
type BlockerReport struct {
	Document []byte
	Verdict  string
	Flagged  int
}

// citedRequests reads the requests a note cites with a 404: on each note line holding a 404
// token, every METHOD /path pair, normalised and deduplicated in note order (AMAP-V0-141).
func citedRequests(note string) ([]citedRequest, error) {
	out, seen := []citedRequest{}, map[string]bool{}
	for i, line := range strings.Split(note, "\n") {
		if !blocker404.MatchString(line) {
			continue
		}
		for _, m := range blockerRequest.FindAllStringSubmatch(line, -1) {
			p := m[3]
			if j := strings.IndexAny(p, "?#"); j >= 0 {
				p = p[:j]
			}
			p = strings.TrimRight(p, ".,;:)]}")
			if len(p) > 1 {
				p = strings.TrimRight(p, "/")
			}
			if p == "" {
				p = "/"
			}
			key := m[2] + " " + p
			if seen[key] {
				continue
			}
			if len(out) == maxBlockerRequests {
				return nil, &gokernel.Error{Code: "appmap-invalid-query", Message: "the note cites more than 16 distinct 404 requests"}
			}
			seen[key] = true
			out = append(out, citedRequest{i + 1, m[2], p})
		}
	}
	return out, nil
}

func requestSegments(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// routeMatches reports whether a request path matches a route path: literal segments compare
// exactly, `:param` takes one segment holding no dot, `*glob` takes the rest (at least one
// segment), and a trailing `.format` on the request is ignored as Rails' optional format is
// (AMAP-V0-142).
func routeMatches(route string, req []string) bool {
	if routeMatchSegments(requestSegments(route), req) {
		return true
	}
	if n := len(req); n > 0 {
		if dot := strings.LastIndexByte(req[n-1], '.'); dot > 0 {
			trimmed := append(append([]string{}, req[:n-1]...), req[n-1][:dot])
			return routeMatchSegments(requestSegments(route), trimmed)
		}
	}
	return false
}

func routeMatchSegments(route, req []string) bool {
	for i, seg := range route {
		if strings.HasPrefix(seg, "*") {
			return len(req) > i
		}
		if i >= len(req) {
			return false
		}
		if strings.HasPrefix(seg, ":") {
			if req[i] == "" || strings.Contains(req[i], ".") {
				return false
			}
			continue
		}
		if seg != req[i] {
			return false
		}
	}
	return len(route) == len(req)
}

// verbMatches reports whether a request method is served by a route verb (`PATCH|PUT` lists
// several; HEAD is served by GET routes, as Rails does).
// underMount reports whether a request path lies at or under a mount path.
func underMount(mount string, req []string) bool {
	prefix := requestSegments(mount)
	return len(req) >= len(prefix) && routeMatchSegments(prefix, req[:len(prefix)])
}

func verbMatches(routeVerb, method string) bool {
	for _, v := range strings.Split(routeVerb, "|") {
		if v == method || method == "HEAD" && v == "GET" {
			return true
		}
	}
	return false
}

// nearness is the number of request segments a route matches in order (a longest common
// subsequence where `:param` matches any segment), used to rank the nearest routes (AMAP-V0-144).
func nearness(route, req []string) int {
	dp := make([][]int, len(route)+1)
	for i := range dp {
		dp[i] = make([]int, len(req)+1)
	}
	for i := 1; i <= len(route); i++ {
		for j := 1; j <= len(req); j++ {
			r := route[i-1]
			switch {
			case r == req[j-1] || strings.HasPrefix(r, ":") || strings.HasPrefix(r, "*"):
				dp[i][j] = dp[i-1][j-1] + 1
			case dp[i-1][j] >= dp[i][j-1]:
				dp[i][j] = dp[i-1][j]
			default:
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	return dp[len(route)][len(req)]
}

func toNear(rt railsRoute) nearRoute {
	return nearRoute{Verb: rt.verb, Path: rt.path, Declaration: rt.declaration(), Controller: controllerConstant(rt.ctrl), Action: rt.action}
}

// nearestRoutes ranks compiled routes by shared literal segments with the request: routes
// serving the method first, then more matched segments, then the smaller length difference,
// then declaration order; at most three routes sharing a literal segment are named.
func nearestRoutes(routes []railsRoute, method string, req []string) []nearRoute {
	type scored struct {
		i, score, literal, diff int
		verb                    bool
	}
	cands := []scored{}
	for i, rt := range routes {
		seg := requestSegments(rt.path)
		literal := 0
		for _, s := range seg {
			if !strings.HasPrefix(s, ":") && !strings.HasPrefix(s, "*") {
				for _, r := range req {
					if r == s {
						literal++
						break
					}
				}
			}
		}
		if literal == 0 {
			continue
		}
		diff := len(seg) - len(req)
		if diff < 0 {
			diff = -diff
		}
		cands = append(cands, scored{i, nearness(seg, req), literal, diff, verbMatches(rt.verb, method)})
	}
	sort.SliceStable(cands, func(a, b int) bool {
		x, y := cands[a], cands[b]
		switch {
		case x.verb != y.verb:
			return x.verb
		case x.score != y.score:
			return x.score > y.score
		case x.literal != y.literal:
			return x.literal > y.literal
		case x.diff != y.diff:
			return x.diff < y.diff
		}
		return x.i < y.i
	})
	out := []nearRoute{}
	for _, c := range cands {
		if len(out) == maxNearestRoutes {
			break
		}
		out = append(out, toNear(routes[c.i]))
	}
	return out
}

// CheckBlocker cross-checks the 404 requests a blocker note cites against the route table
// (AMAP-V0-140..146). It reads committed objects of --root at the map revision only and writes
// nothing.
func CheckBlocker(ctx context.Context, m *Map, note string, o Options) (*BlockerReport, error) {
	budget, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(note) == "" || len(note) > maxBlockerNote || strings.IndexByte(note, 0) >= 0 {
		return nil, &gokernel.Error{Code: "appmap-invalid-query", Message: "--note must be 1..16384 bytes of text"}
	}
	cited, err := citedRequests(note)
	if err != nil {
		return nil, err
	}
	r := repo{ctx: ctx, root: o.Root}
	rev, err := r.resolve(m.Revision)
	if err != nil || rev != m.Revision {
		return nil, orPlatform(err, &gokernel.Error{Code: "appmap-invalid-revision", Message: "the map revision is not a commit in --root"})
	}
	man, err := readManifestAt(r, m, nil)
	if err != nil {
		return nil, err
	}
	compiled, err := compileRailsRoutes(r, rev, man)
	if err != nil {
		return nil, err
	}
	rootOf := make([]string, len(compiled.routes))
	for root, idx := range compiled.roots {
		for _, i := range idx {
			rootOf[i] = root
		}
	}
	incomplete := ""
	if len(compiled.checked) == 0 {
		incomplete = "no-server-route-table"
	} else if len(compiled.abstain) > 0 {
		n := compiled.abstain[0]
		incomplete = n.file + ":" + strconv.Itoa(n.line) + " " + n.reason
	}
	apps := map[string]*railsApp{}
	reads := newAppReadBudget()
	rows := []any{}
	counts := map[string]int{}
	for _, c := range cited {
		row := blockerRow{Request: c.verb + " " + c.path, NoteLine: c.line}
		req := requestSegments(c.path)
		matched := -1
		for i, rt := range compiled.routes {
			if verbMatches(rt.verb, c.verb) && routeMatches(rt.path, req) {
				matched = i
				break
			}
		}
		var opaque *opaqueRoute
		for i, op := range compiled.opaque {
			if op.verb == "" && underMount(op.path, req) || op.verb != "" && verbMatches(op.verb, c.verb) && routeMatches(op.path, req) {
				opaque = &compiled.opaque[i]
				break
			}
		}
		switch {
		case strings.ContainsAny(c.path, "{}$*") || strings.Contains(c.path, "/:"):
			row.Verdict, row.Reason = "uncertain", "non-literal-request-path"
		case matched >= 0:
			rt := compiled.routes[matched]
			near := toNear(rt)
			row.Route = &near
			app := apps[rootOf[matched]]
			if app == nil {
				root := rootOf[matched]
				app, err = loadRailsApp(r, rev, root, []string{"app"}, func(p string) bool {
					return strings.HasSuffix(p, ".rb") && (strings.HasPrefix(p, appJoin(root, "app/controllers/")) || strings.HasPrefix(p, appJoin(root, "app/models/concerns/")))
				}, reads)
				if err != nil {
					return nil, err
				}
				apps[root] = app
			}
			v := app.resolveAction(rt.ctrl, rt.action)
			row.HandlerFile, row.Basis = v.file, v.basis
			switch v.status {
			case "defined":
				row.Verdict = "accepted"
			case "missing":
				row.Verdict, row.Rule = "flagged", RuleRouteActionMissing
			default:
				row.Verdict, row.Reason, row.At = "uncertain", v.reason, v.at
			}
		case opaque != nil:
			row.Verdict, row.Reason, row.At = "uncertain", opaque.reason, opaque.file+":"+strconv.Itoa(opaque.line)
		case incomplete != "":
			row.Verdict, row.Reason, row.At = "uncertain", "route-table-incomplete", incomplete
			row.Nearest = nearestRoutes(compiled.routes, c.verb, req)
		default:
			row.Verdict, row.Rule = "flagged", RuleBlocker404
			row.Nearest = nearestRoutes(compiled.routes, c.verb, req)
		}
		counts[row.Verdict]++
		rows = append(rows, row)
	}
	verdict := "not-applicable"
	for _, v := range []string{"accepted", "uncertain", "flagged"} {
		if counts[v] > 0 {
			verdict = v
		}
	}
	head := []field{{"schema", BlockerSchema}, {"app", m.App}, {"map_revision", m.Revision}, {"map_digest", m.Digest},
		{"budget", budget}, {"full", o.Full}, {"rule", RuleBlocker404}, {"routers_checked", compiled.checked},
		{"routes", len(compiled.routes)}, {"route_table_incomplete", incomplete != ""}, {"cited", len(cited)},
		{"verdict", verdict}, {"accepted", counts["accepted"]}, {"flagged", counts["flagged"]}, {"uncertain", counts["uncertain"]}}
	doc, err := render(head, []section{{"requests", rows}}, budget)
	if err != nil {
		return nil, err
	}
	return &BlockerReport{Document: doc, Verdict: verdict, Flagged: counts["flagged"]}, nil
}
