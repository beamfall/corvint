package console

import (
	"encoding/json"
	"html/template"
	"strconv"
	"strings"
)

// page is the whole surface. It is server-rendered with no client framework,
// no build step and no external asset: every byte is served from this
// process, so LAC-V0-002's "no outbound connection" holds for the page as
// well as for the server.
//
// Every interpolation goes through html/template's contextual escaping, which
// is what makes ticket titles, review text and agent output inert
// (LAC-V0-020). Nothing here linkifies text, and no value reaches an
// attribute or script context.
var page = template.Must(template.New("page").Funcs(template.FuncMap{
	"or_dash": func(s string) string {
		if strings.TrimSpace(s) == "" {
			return "—"
		}
		return s
	},
	"refusalOf": func(heading string, source Source, err string) Refusal {
		return Refusal{Heading: heading, Err: err, Source: source}
	},
	"refusalOfEnvelope": func(heading string, source Source, e *Envelope) Refusal {
		return Refusal{Heading: heading, Outcome: e.Outcome, Codes: e.Codes, Warnings: e.Warnings, Source: source}
	},
	// originField names the record fields that answer "where does this ticket
	// come from". Everything else stays on the record itself.
	"originField": func(key string) bool {
		switch key {
		case "source", "createdAt", "updatedAt", "updatedBy", "supersedes", "supersededBy",
			"requirementRefs", "acceptanceCriteria", "labels", "body", "shadowOverlay":
			return true
		}
		return false
	},
	// render turns one decoded JSON value into text. It never emits markup:
	// every value it returns is escaped by the template that prints it.
	"render": renderValue,
}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
{{if .RefreshSeconds}}<meta http-equiv="refresh" content="{{.RefreshSeconds}}">{{end}}
<title>{{.Title}} · Corvint Console</title>
<style>
:root{color-scheme:dark;--bg:#101820;--fg:#e6edf5;--dim:#a7b2bf;--faint:#7f8b99;
--line:#2d3946;--line-strong:#435163;--card:#17212b;--soft:#202b36;--accent:#7fa5ff;
--accent-strong:#adc4ff;--accent-soft:#20345f;--on-accent:#0d1b35;--warn:#f0c36a;
--warnbg:#2d281b;--bad:#ff938c;--badbg:#351f22;--ok:#75d8a2;--okbg:#183326;
--unstated:#9aa6b3;--unstatedbg:#202933;--shadow:0 1px 3px rgba(3,8,13,.3)}
*{box-sizing:border-box}
html{background:var(--bg)}
body{margin:0;font:14px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif;
background:var(--bg);color:var(--fg);text-rendering:optimizeLegibility}
a{color:var(--accent);text-underline-offset:2px}
a:hover{color:var(--accent-strong)}
a:focus-visible,button:focus-visible,input:focus-visible,textarea:focus-visible,summary:focus-visible{
outline:3px solid #d9e4ff;outline-offset:2px}
header{background:var(--card);border-bottom:1px solid var(--line);position:sticky;top:0;z-index:5}
.header-inner{max-width:1600px;margin:0 auto;padding:13px clamp(16px,3vw,32px);display:grid;
grid-template-columns:auto 1fr;gap:9px 30px;align-items:end}
.brand-lockup{display:flex;align-items:baseline;gap:10px;white-space:nowrap}
.brand{font-size:11px;font-weight:750;letter-spacing:.08em;text-transform:uppercase;color:var(--accent)}
header h1{margin:0;font-size:20px;line-height:1.2;font-weight:720;letter-spacing:-.015em}
header nav{display:flex;gap:4px;align-items:center;justify-content:flex-end;flex-wrap:wrap}
header nav a{color:var(--dim);font-size:13px;font-weight:600;text-decoration:none;padding:6px 9px;border-radius:6px}
header nav a:hover{color:var(--fg);background:var(--soft)}
header nav a[aria-current=page]{color:var(--accent-strong);background:var(--accent-soft)}
.repo-meta{grid-column:1/-1;color:var(--dim);font-size:11px;display:flex;gap:7px;flex-wrap:wrap;
border-top:1px solid var(--soft);padding-top:8px}
.repo-meta code{color:var(--fg)}
main{padding:24px clamp(16px,3vw,32px) 48px;max-width:1600px;margin:0 auto;min-height:calc(100vh - 145px)}
.page-intro{display:flex;justify-content:space-between;gap:24px;align-items:flex-start;margin:2px 0 20px}
.page-intro h2{font-size:24px;line-height:1.2;letter-spacing:-.025em;margin:2px 0 6px}
.page-intro p{margin:0;color:var(--dim);max-width:70ch}
.eyebrow{font-size:11px!important;text-transform:uppercase;letter-spacing:.1em;font-weight:750;color:var(--accent)!important}
.button-link{display:inline-flex;align-items:center;justify-content:center;min-height:38px;padding:7px 13px;
border:1px solid var(--line-strong);border-radius:7px;background:var(--card);text-decoration:none;font-weight:650;white-space:nowrap}
.board-summary{display:flex;align-items:center;gap:8px;flex-wrap:wrap;color:var(--dim);font-size:12px;margin-bottom:12px}
.board{display:flex;gap:14px;overflow-x:auto;padding:2px 2px 14px;align-items:flex-start;scrollbar-color:#bac2cc transparent}
.col{flex:0 0 320px;background:#1d2833;border:1px solid var(--line);border-radius:10px;padding:10px;min-height:100px}
.col h2{margin:2px 2px 10px;font-size:11px;letter-spacing:.08em;text-transform:uppercase;color:var(--dim);display:flex;justify-content:space-between}
.col.unmapped{background:var(--warnbg);outline:1px solid #66572f}
.card{background:var(--card);border:1px solid var(--line);border-radius:8px;padding:12px;margin-bottom:9px;box-shadow:var(--shadow)}
.card:hover{border-color:#50637a}
.card a{color:var(--fg);text-decoration:none;font-weight:675;line-height:1.35;display:block}
.card a:hover{color:var(--accent)}
.meta{margin-top:8px;font-size:11px;color:var(--dim);display:flex;gap:6px;flex-wrap:wrap;align-items:center}
.pill,.tag{border:1px solid var(--line);border-radius:999px;padding:2px 7px;font-size:11px;line-height:1.4;background:#1d2833;color:var(--dim)}
.pill.priority{background:var(--accent-soft);border-color:#3b5793;color:var(--accent-strong);font-weight:700}
.refusal{background:var(--badbg);border:1px solid #6e3b3f;border-left:4px solid var(--bad);
padding:13px 15px;border-radius:8px;margin-bottom:16px}
.refusal h2{margin:0 0 6px;font-size:13px;color:var(--bad)}
.note{background:var(--warnbg);border:1px solid #61542e;border-radius:8px;padding:11px 13px;margin-bottom:16px}
.historical{display:flex;gap:9px;align-items:baseline;color:var(--warn);font-size:12px}
.panel,.roadmap-group{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:16px 18px;margin-bottom:16px;box-shadow:var(--shadow)}
.panel h2{margin:0 0 12px;font-size:12px;text-transform:uppercase;letter-spacing:.075em;color:var(--dim)}
.panel-subtitle{font-size:12px;color:var(--dim);margin:-5px 0 12px}
details.panel{padding:0}
details.panel>summary{cursor:pointer;list-style:none;padding:15px 18px;font-weight:680;display:flex;align-items:center;justify-content:space-between;gap:16px}
details.panel>summary::-webkit-details-marker{display:none}
details.panel>summary::after{content:"+";color:var(--dim);font-size:20px;font-weight:400}
details.panel[open]>summary{border-bottom:1px solid var(--line)}
details.panel[open]>summary::after{content:"−"}
.details-body{padding:16px 18px}
.summary-hint{font-weight:400;color:var(--dim);font-size:12px}
table{border-collapse:collapse;width:100%;font-size:13px}
td,th{text-align:left;padding:6px 10px 6px 0;vertical-align:top;overflow-wrap:anywhere}
th{color:var(--dim);font-weight:550;white-space:nowrap;width:1%}
tr+tr td,tr+tr th{border-top:1px solid var(--soft)}
.axes{display:flex;gap:6px;flex-wrap:wrap;margin-top:9px}
.axis{font-size:10px;border-radius:4px;padding:2px 6px;border:1px solid var(--line);background:#1d2833}
.axis.unstated{background:var(--unstatedbg);color:var(--unstated);border-style:dashed}
.axis b{font-weight:650}
.src{margin-top:12px;font-size:11px;color:var(--dim);border-top:1px dashed var(--line);padding-top:9px}
.src code,code{background:var(--soft);padding:1px 4px;border-radius:3px;font-size:.92em;
font-family:ui-monospace,SFMono-Regular,Menlo,monospace;overflow-wrap:anywhere}
pre{background:#0c131a;color:#e8eef5;padding:14px;border-radius:8px;overflow-x:auto;font-size:12px;
font-family:ui-monospace,SFMono-Regular,Menlo,monospace;margin:0}
.unknown{font-size:11px;color:var(--warn);background:var(--warnbg);border-radius:5px;padding:3px 7px;display:inline-block;margin-top:5px}
.num{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px}
.unmeasured{color:var(--unstated);background:var(--unstatedbg);border:1px dashed var(--line);border-radius:5px;padding:2px 6px;font-size:11px}
tr.metric td{border-top:1px solid var(--soft)}
.grp{font-size:11px;color:var(--dim);text-transform:uppercase;letter-spacing:.07em;margin:16px 0 5px}
form.mut{display:flex;gap:8px;align-items:flex-start;margin:8px 0;flex-wrap:wrap}
form.mut input[type=text]{font-family:ui-monospace,Menlo,monospace;font-size:12px;padding:9px 10px;
border:1px solid var(--line-strong);border-radius:6px;min-width:260px;min-height:38px;flex:1;background:#111a23;color:var(--fg)}
button{font-family:inherit;font-size:12px;font-weight:650;line-height:1.2;min-height:38px;padding:8px 13px;border-radius:6px;border:1px solid var(--line-strong);background:var(--card);color:var(--fg);cursor:pointer}
button:hover:not([disabled]){border-color:#627389;background:#202b36}
button[type=submit]{color:var(--on-accent);background:var(--accent);border-color:var(--accent)}
button[type=submit]:hover:not([disabled]){background:var(--accent-strong);border-color:var(--accent-strong)}
button[disabled]{cursor:not-allowed;opacity:.55;background:#202833;color:var(--dim)}
.disabled-reason{font-size:11px;color:var(--dim);flex-basis:100%;margin:-2px 0 4px}
.roadmap-group{padding:0;overflow:hidden}
.milestone-heading{padding:14px 16px;background:#141e27;border-bottom:1px solid var(--line);display:flex;justify-content:space-between;gap:12px;align-items:center}
.milestone-heading h2{font-size:14px;margin:0;letter-spacing:-.005em}.milestone-heading span{font-size:11px;color:var(--dim)}
.roadmap-row{display:grid;grid-template-columns:minmax(280px,2.3fr) minmax(155px,.9fr) minmax(170px,1fr) minmax(170px,1fr);gap:14px;padding:14px 16px;border-bottom:1px solid var(--soft);align-items:start}
.roadmap-row:last-child{border-bottom:0}.roadmap-row:hover{background:#1a2530}
.roadmap-ticket>a{color:var(--fg);font-weight:680;text-decoration:none;line-height:1.35;display:block;margin-bottom:5px}.roadmap-ticket>a:hover{color:var(--accent)}
.roadmap-ticket code{color:var(--dim);background:transparent;padding:0}
.roadmap-fact{min-width:0}.roadmap-fact dt{font-size:10px;text-transform:uppercase;letter-spacing:.075em;color:var(--faint);font-weight:700;margin-bottom:5px}
.roadmap-fact dd{margin:0;color:var(--fg);font-size:12px}.roadmap-fact dd+dt{margin-top:10px}
.roadmap-tags{display:flex;gap:5px;flex-wrap:wrap;margin-top:8px}
.tag.blocked{background:var(--warnbg);border-color:#61542e;color:var(--warn)}
.tag.open{background:var(--accent-soft);border-color:#3b5793;color:var(--accent-strong)}
.pagination{display:flex;justify-content:space-between;gap:20px;align-items:center}.pagination nav{display:flex;gap:8px;align-items:center}
.ticket-hero h2{font-size:22px;text-transform:none;letter-spacing:-.02em;color:var(--fg);margin-bottom:7px}
.ticket-identity{color:var(--dim);font-size:12px;margin-bottom:13px}.ticket-identity code{background:transparent;padding:0}
.ticket-badges{display:flex;gap:6px;flex-wrap:wrap;margin-bottom:14px}
.ticket-summary{display:grid;grid-template-columns:repeat(5,minmax(100px,1fr));gap:12px;padding-top:13px;border-top:1px solid var(--soft);margin:0}
.ticket-summary div{min-width:0}.ticket-summary dt{font-size:10px;text-transform:uppercase;letter-spacing:.075em;color:var(--faint);font-weight:700}.ticket-summary dd{margin:3px 0 0;font-weight:600;overflow-wrap:anywhere}
.ticket-layout{display:grid;grid-template-columns:minmax(0,1.6fr) minmax(320px,.8fr);gap:16px;align-items:start}.ticket-layout .panel{margin-bottom:16px}
.ticket-actions{position:sticky;top:126px}.ticket-actions .panel{box-shadow:none}.ticket-actions .panel h2{color:var(--fg)}
footer{color:var(--dim);font-size:11px;border-top:1px solid var(--line);background:var(--card)}
.footer-inner{max-width:1600px;margin:0 auto;padding:16px clamp(16px,3vw,32px)}
.ticket-form{max-width:52rem;display:grid;gap:.8rem}
.ticket-form label{display:grid;gap:.35rem;font-weight:650;font-size:12px}
.ticket-form input,.ticket-form textarea{width:100%;font:inherit;padding:.7rem;border:1px solid var(--line-strong);border-radius:6px;background:#111a23;color:var(--fg)}
.ticket-form textarea{min-height:7rem;resize:vertical}.ticket-form button{justify-self:start}
.panel,.card{overflow-wrap:anywhere}
@media (max-width:1000px){.roadmap-row{grid-template-columns:minmax(260px,1.5fr) 1fr 1fr}.roadmap-row>.roadmap-fact:last-child{grid-column:2/-1}.ticket-layout{grid-template-columns:1fr}.ticket-actions{position:static}.ticket-summary{grid-template-columns:repeat(3,1fr)}}
@media (max-width:720px){.header-inner{grid-template-columns:1fr;align-items:start}.brand-lockup{white-space:normal}.header-inner nav{justify-content:flex-start;overflow-x:auto;flex-wrap:nowrap;margin:0 -6px;padding:0 6px}.repo-meta{grid-column:1}.repo-meta span:last-child{display:none}main{padding-top:18px}.page-intro{display:block}.page-intro .button-link{margin-top:12px}.historical{display:block}.board{margin-right:-16px}.col{flex-basis:min(86vw,320px)}.roadmap-row{grid-template-columns:1fr 1fr;gap:12px}.roadmap-ticket{grid-column:1/-1}.roadmap-row>.roadmap-fact:last-child{grid-column:auto}.pagination{align-items:flex-start;flex-direction:column}.ticket-summary{grid-template-columns:repeat(2,1fr)}.summary-hint{display:none}td,th{display:block;width:100%;padding:5px 0}tr{display:block;padding:7px 0}tr+tr td,tr+tr th{border-top:0}}
@media (prefers-reduced-motion:reduce){*{scroll-behavior:auto!important;transition:none!important}}
</style></head><body>
<header>
<div class="header-inner">
<div class="brand-lockup"><span class="brand">Corvint Console</span><h1>{{.Title}}</h1></div>
<nav aria-label="Primary"><a href="/"{{if eq .Title "Board"}} aria-current="page"{{end}}>Board</a><a href="/roadmap"{{if eq .Title "Roadmap"}} aria-current="page"{{end}}>Roadmap</a><a href="/specs"{{if eq .Title "Specs"}} aria-current="page"{{end}}>Specs</a><a href="/evidence"{{if eq .Title "Evidence"}} aria-current="page"{{end}}>Evidence</a><a href="/dogfood"{{if eq .Title "Dogfood"}} aria-current="page"{{end}}>Dogfood</a><a href="/benchmarks"{{if eq .Title "Benchmarks"}} aria-current="page"{{end}}>Benchmarks</a><a href="/backlogs"{{if eq .Title "Backlogs"}} aria-current="page"{{end}}>Backlogs</a></nav>
<div class="repo-meta"><span>repo <code>{{.Repo}}</code></span>
<span>revision <code>{{if .Revision.Commit}}{{slice .Revision.Commit 0 12}}{{else}}unknown{{end}}</code></span>
{{if .Revision.Dirty}}<span>worktree dirty · {{len .Revision.DirtyPaths}} paths</span>{{end}}
<span>compiled {{.CompiledAt}}</span></div>
</div>
</header>
<main>

{{if .Boundary}}<div class="note historical"><b>This page is historical.</b><span>It was compiled at {{.CompiledAt}} against revision <code>{{or_dash .Revision.Commit}}</code>. Reload to observe again; nothing on this page is guaranteed to still hold.</span></div>{{end}}

{{template "body" .}}

</main>
<footer>
<div class="footer-inner">
Corvint Console · loopback only, no account, no database, no outbound connection.
It renders what <code>corvint-tasks</code>, <code>corvint-dashboard-snapshot</code> and <code>git</code>
report and performs a change only by invoking the owning tool's own verb. It is never a gate: it cannot accept a specification,
advance a delivery stage, or qualify anything.
</div>
</footer>
</body></html>`))

// mustView builds one page variant. Each view gets its own clone of the base
// set, so the "body" each defines cannot collide with another view's.
func mustView(body string) *template.Template {
	clone := template.Must(base().Clone())
	return template.Must(clone.Parse(ticketFormTemplate + body))
}

// renderValue is the text form of one decoded JSON value.
func renderValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "—"
	case string:
		if strings.TrimSpace(typed) == "" {
			return "—"
		}
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "—"
	}
	return string(encoded)
}

// axesTemplate renders the six axes of one value. An axis its source did not
// state is visibly distinct from one that was measured, which is the whole
// point of LAC-V0-008.
var axesTemplate = template.Must(page.New("axes").Parse(`
<div class="axes">
{{range .Pairs}}<span class="axis{{if not .Stated}} unstated{{end}}"><b>{{.Axis}}</b> {{.Value}}</span>{{end}}
</div>`))

// sourceTemplate renders one value's attribution: the exact invocation, the
// tool, and the revision it answered about (LAC-V0-006).
var sourceTemplate = template.Must(page.New("source").Parse(`
<div class="src">
<div>read by <code>{{.Line}}</code></div>
{{if .Tool}}<div>tool {{.Tool}}</div>{{end}}
{{if .Revision}}<div>revision <code>{{.Revision}}</code></div>{{end}}
<div>observed at {{.ObservedAt.Format "2006-01-02T15:04:05Z"}}</div>
{{if .Outcome}}<div>envelope outcome {{.Outcome}}{{if .Codes}} · codes {{range .Codes}}{{.}} {{end}}{{end}}</div>{{end}}
{{template "axes" .Axes}}
</div>`))

// refusalTemplate renders a refusal as itself, with the tool's exact codes
// and warning text. A refusal is never an empty board (LAC-V0-009).
var refusalTemplate = template.Must(page.New("refusal").Parse(`
<div class="refusal">
<h2>{{.Heading}}</h2>
{{if .Outcome}}<div>outcome <b>{{.Outcome}}</b>{{if .Codes}} · codes {{range .Codes}}<code>{{.}}</code> {{end}}{{end}}</div>{{end}}
{{range .Warnings}}<div>{{.}}</div>{{end}}
{{if .Err}}<div>{{.Err}}</div>{{end}}
<div style="margin-top:8px;font-size:12px">This is the tool's own refusal, shown as a refusal.
It is not an empty result, and no count on this page should be read as zero.</div>
{{template "source" .Source}}
</div>`))

// Refusal is what refusalTemplate renders.
type Refusal struct {
	Heading  string
	Outcome  string
	Codes    []string
	Warnings []string
	Err      string
	Source   Source
}

// base is the page shell plus the three shared partials, ready to be cloned.
func base() *template.Template {
	return sharedTemplates
}

var sharedTemplates = func() *template.Template {
	_ = axesTemplate
	_ = sourceTemplate
	_ = refusalTemplate
	return page
}()
