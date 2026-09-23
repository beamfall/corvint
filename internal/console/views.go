package console

// boardView is the Jira-style board. Columns come from the tool's own status
// enumeration; a status it did not enumerate gets its own visibly unmapped
// column so a card can never be dropped (LAC-V0-016).
var boardView = mustView(`{{define "body"}}
{{if .Board.Err}}{{template "refusal" refusalOf "The ticket store could not be read" .Board.Source .Board.Err}}{{end}}
{{if and .Board.Envelope .Board.Envelope.Refused}}
  {{template "refusal" refusalOfEnvelope "The ticket store refused this read" .Board.Source .Board.Envelope}}
{{end}}

{{if and .Board.Envelope .Board.Envelope.Refused}}<div class="note">If this repository has no ticket store, initialize a repository already configured for Corvint Tasks explicitly from its directory with <code>corvint-tasks init</code>. Reload this page afterward. Reading this page never initializes a store.</div>{{end}}
<section class="page-intro">
  <div><p class="eyebrow">Work queue</p><h2>Plan and track delivery</h2>
  <p>A Jira-style view of every ticket the owning tool reports. Status, eligibility, and blockers remain tool-owned observations.</p></div>
  <a class="button-link" href="/roadmap">View roadmap</a>
</section>
<details class="panel">
<summary><span>New ticket</span><span class="summary-hint">Capture work without leaving the board</span></summary>
<div class="details-body">
{{if .CreateEnabled}}{{template "ticket-form" .}}{{else}}<p>Ticket creation is unavailable until the store can be read and <code>corvint-tasks help</code> offers <code>ticket create</code>.</p>{{end}}
<p class="panel-subtitle">New tickets are manual features with priority P2. Creating one does not authorize autonomous execution.</p>
</div></details>
{{if .Caps.Err}}<div class="note"><b>Board columns are unavailable.</b> {{.Caps.Err}}
The console does not carry its own column list, so no board is drawn rather than one invented here.</div>{{end}}

{{if and (not .Board.Err) (not .Caps.Err) (not (and .Board.Envelope .Board.Envelope.Refused))}}
<div class="board-summary"><b>{{.Board.Total}} tickets</b><span>across {{len .Board.Columns}} tool-reported statuses</span></div>
<div class="board">
{{range .Board.Columns}}
  <div class="col{{if .Unmapped}} unmapped{{end}}">
    <h2>{{.Status}}{{if .Unmapped}} · not enumerated by the tool{{end}} ({{len .Cards}})</h2>
    {{range .Cards}}
    <div class="card">
      <a href="/ticket?id={{.TicketID}}">{{.Title}}</a>
      <div class="meta">
        <span class="pill">{{.TicketID}}</span>
        <span class="pill priority">{{.Priority}}</span>
        <span class="pill">{{.Kind}}</span>
        <span class="pill">rev {{.Revision}}</span>
        {{if .Owner}}<span class="pill">{{.Owner}}</span>{{end}}
        {{if .Blockers}}<span class="pill">{{.Blockers}} blockers</span>{{end}}
      </div>
      <div class="meta">eligibility {{.Eligibility}} · next {{or_dash .NextAction}}</div>
      {{range .Unknowns}}<div class="unknown">{{.Code}}: {{.Detail}}</div>{{end}}
    </div>
    {{else}}<div style="font-size:12px;color:#6b7280">no tickets</div>{{end}}
  </div>
{{end}}
</div>
<details class="panel"><summary><span>How this board was read</span><span class="summary-hint">Source, authority, and uncertainty</span></summary><div class="details-body">
<div>{{.Board.Total}} tickets in {{len .Board.Columns}} columns.
Columns are the statuses <code>corvint-tasks help</code> enumerated at request time.</div>
{{if .Board.Untrusted}}<div style="margin-top:6px;font-size:12px">The tool marked these fields
untrusted; they are rendered as inert text: {{range .Board.Untrusted}}<code>{{.}}</code> {{end}}</div>{{end}}
{{template "source" .Board.Source}}
</div></details>
{{end}}
{{end}}`)

// roadmapView is `atm roadmap`, grouped by the milestone the tool assigned.
// Gate evidence renders exactly as the tool stated it: NOT_OBSERVED is shown
// as "not observed", never upgraded to a pass or fail the tool did not
// report (IPR-02).
var roadmapView = mustView(`{{define "body"}}
{{if .Roadmap.Err}}{{template "refusal" refusalOf "The roadmap could not be read" .Roadmap.Source .Roadmap.Err}}{{end}}
{{if and .Roadmap.Envelope .Roadmap.Envelope.Refused}}
  {{template "refusal" refusalOfEnvelope "The ticket store refused this read" .Roadmap.Source .Roadmap.Envelope}}
{{end}}

<div class="note"><b>Safe auto-recheck {{if .RefreshSeconds}}is active{{else}}is paused{{end}}.</b><span>
{{if .RefreshSeconds}}This page repeats its read-only check every 30 seconds. <a href="/roadmap?page={{.Roadmap.Page}}&amp;refresh=off">Pause auto-recheck</a>.
{{else}}This page is not refreshing automatically. <a href="/roadmap?page={{.Roadmap.Page}}">Resume auto-recheck</a>.{{end}}
A blocker clears only when <code>corvint-tasks</code> reports it cleared. Manual and external work,
approval-required decisions, unknown evidence, admission, release candidacy, attestation, and promotion remain hard stops.</span></div>

{{if and (not .Roadmap.Err) (not (and .Roadmap.Envelope .Roadmap.Envelope.Refused))}}
<section class="page-intro">
  <div><p class="eyebrow">Release plan</p><h2>Roadmap by milestone</h2>
  <p>Sequence, gates, and blocker closure from <code>corvint-tasks roadmap</code>, without inferred progress.</p></div>
  <a class="button-link" href="/">Back to board</a>
</section>
{{range .Roadmap.Groups}}
<section class="roadmap-group">
<header class="milestone-heading"><h2>{{.Milestone}}{{if .Unmapped}} · not assigned{{end}}</h2><span>{{len .Rows}} tickets</span></header>
<div class="roadmap-list">
{{range .Rows}}
<article class="roadmap-row">
<div class="roadmap-ticket"><a href="/ticket?id={{.TicketID}}">{{.Title}}</a><code>{{.TicketID}}</code>
<div class="roadmap-tags"><span class="tag">#{{or_dash .Order}}</span><span class="tag">{{.Priority}}</span><span class="tag open">{{.Status}}</span><span class="tag{{if eq .Eligibility "BLOCKED"}} blocked{{end}}">{{.Eligibility}}</span></div></div>
<dl class="roadmap-fact"><dt>Owner</dt><dd>{{or_dash .Owner}}</dd><dt>Next action</dt><dd>{{or_dash .NextAction}}</dd></dl>
<dl class="roadmap-fact"><dt>Required gates</dt><dd>{{range .RequiredGates}}<code>{{.}}</code> {{else}}none{{end}}</dd><dt>Gate results</dt><dd>{{if eq .GateResults "NOT_OBSERVED"}}not observed{{else}}{{or_dash .GateResults}}{{end}}</dd></dl>
<dl class="roadmap-fact"><dt>Blockers</dt><dd>
{{if .Blockers}}
  {{if .Blockers.Refused}}<span class="unknown">blocker read refused</span>
  {{else if .Blockers.Items}}{{range .Blockers.Items}}<code>{{index . "ticketId"}}</code> {{end}}
  {{else}}<span>none reported</span>{{end}}
{{else if .BlockerSource.Err}}<span class="unknown">blocker read failed: {{.BlockerSource.Err}}</span>
{{else if eq .Eligibility "BLOCKED"}}<span class="unknown">blocker read not attempted</span>
{{else}}<span>not blocked</span>{{end}}
</dd></dl>
</article>
{{end}}
</div>
</section>
{{else}}<div class="panel"><div>The tool reported no ticket on this page.</div></div>
{{end}}

<div class="panel pagination">
<div><b>Page {{.Roadmap.Page}}</b><div class="panel-subtitle">{{.Roadmap.Total}} tickets here{{if .Roadmap.TotalKnown}} of {{.Roadmap.TotalCount}} total{{end}}.</div></div>
<nav aria-label="Roadmap pages">
{{if .Roadmap.HasPrev}}<a href="/roadmap?page={{.Roadmap.PrevPage}}">Previous page</a>{{else}}<span>Previous page</span>{{end}}
<span aria-hidden="true">·</span>
{{if .Roadmap.HasNext}}<a href="/roadmap?page={{.Roadmap.NextPage}}">Next page</a>{{else}}<span>Next page</span>{{end}}
</nav>
</div>
<details class="panel"><summary><span>How this roadmap was read</span><span class="summary-hint">Source, authority, and uncertainty</span></summary><div class="details-body">
{{if .Roadmap.Untrusted}}<div style="margin-top:6px;font-size:12px">The tool marked these fields
untrusted; they are rendered as inert text: {{range .Roadmap.Untrusted}}<code>{{.}}</code> {{end}}</div>{{end}}
{{template "source" .Roadmap.Source}}
</div></details>
{{end}}
{{end}}`)

// ticketView is one ticket: its record, its origin, its blockers, its
// governing clause, and the controls that delegate to the owning tool.
var ticketView = mustView(`{{define "body"}}
{{if .Detail.Err}}{{template "refusal" refusalOf "This ticket could not be read" .Detail.Show .Detail.Err}}{{end}}
{{if and .Detail.Envelope .Detail.Envelope.Refused}}
  {{template "refusal" refusalOfEnvelope "The ticket store refused this read" .Detail.Show .Detail.Envelope}}
{{end}}

{{if .Detail.Card.TicketID}}
<div class="panel ticket-hero">
<div class="eyebrow">Ticket</div>
<h2>{{.Detail.Card.Title}}</h2>
<div class="ticket-identity"><code>{{.Detail.Card.TicketID}}</code> · revision {{.Detail.Card.Revision}}</div>
<div class="ticket-badges"><span class="tag open">{{.Detail.Card.Status}}</span><span class="tag{{if eq .Detail.Card.Eligibility "BLOCKED"}} blocked{{end}}">{{.Detail.Card.Eligibility}}</span><span class="tag">{{.Detail.Card.Kind}}</span><span class="tag">{{.Detail.Card.Priority}}</span></div>
<dl class="ticket-summary">
<div><dt>Owner</dt><dd>{{or_dash .Detail.Card.Owner}}</dd></div>
<div><dt>Milestone</dt><dd>{{or_dash .Detail.Card.Milestone}}</dd></div>
<div><dt>Next action</dt><dd>{{or_dash .Detail.Card.NextAction}}</dd></div>
<div><dt>Status</dt><dd>{{.Detail.Card.Status}}</dd></div>
<div><dt>Eligibility</dt><dd>{{.Detail.Card.Eligibility}}</dd></div>
</dl>
{{range .Detail.Card.Unknowns}}<div class="unknown">{{.Code}}: {{.Detail}}</div>{{end}}
{{template "source" .Detail.Show}}
</div>

<div class="ticket-layout">
<div class="ticket-content">
<div class="panel">
<h2>Where it comes from</h2>
{{if .Detail.Record}}
<table>
{{range $k, $v := .Detail.Record}}{{if originField $k}}<tr><th>{{$k}}</th><td>{{render $v}}</td></tr>{{end}}{{end}}
</table>
{{else}}<div>The tool returned no intent record for this ticket, so its origin is
<b>NOT_OBSERVED</b> rather than empty.</div>{{end}}
</div>

<div class="panel">
<h2>Blockers</h2>
{{if .Detail.Blockers}}
  {{if .Detail.Blockers.Refused}}
    {{template "refusal" refusalOfEnvelope "The blocker closure was refused" .Detail.BlockerSource .Detail.Blockers}}
  {{else}}
    {{if .Detail.Blockers.Items}}<table>
    {{range .Detail.Blockers.Items}}<tr><th><code>{{index . "ticketId"}}</code></th>
    <td>{{index . "title"}} · {{index . "status"}}</td></tr>{{end}}</table>
    {{else}}<div>The tool reported no blockers.</div>{{end}}
    {{template "source" .Detail.BlockerSource}}
  {{end}}
{{else}}<div>The blocker closure was not read.</div>{{end}}
</div>

<div class="panel">
<h2>Governing requirement</h2>
{{if .Clauses}}
  {{range .Clauses}}
    {{if .Err}}<div class="unknown">{{.Requirement.ID}}: {{.Err}}</div>
    {{else}}
    <table>
    <tr><th>requirement</th><td>{{if $.Revision.Commit}}<a href="/requirement?id={{.Requirement.ID}}&at={{$.Revision.Commit}}"><code>{{.Requirement.ID}}</code></a>{{else}}<code>{{.Requirement.ID}}</code>{{end}} — {{.Requirement.Title}}</td></tr>
    <tr><th>clause</th><td>{{.Text}}</td></tr>
    {{if .Spec}}
    <tr><th>spec</th><td>{{.Spec.Title}} (<code>{{.Spec.Path}}</code>)</td></tr>
    <tr><th>declared intent</th><td>{{.Spec.Intent}}</td></tr>
    <tr><th>declared delivery</th><td>{{.Spec.Delivery}}</td></tr>
    {{end}}
    </table>
    {{template "source" .Source}}
    {{end}}
  {{end}}
{{else}}<div>This ticket names no <code>requirementRefs</code>, so no governing clause is
reachable from it. That is an absent link, not a passing one.</div>{{end}}
</div>

</div>
<aside class="ticket-actions" aria-label="Ticket actions">

<div class="panel">
<h2>Priority and order</h2>
<div class="panel-subtitle">
Saving replaces the shown order and priority. The console writes nothing itself and sends the
revision shown above.
</div>
{{if .Caps.Implements "ticket prioritize"}}{{template "prioritize-form" .}}
{{else}}<p>Priority and order editing is unavailable: {{.Caps.Reason "ticket prioritize"}}</p>{{end}}
</div>

<div class="panel">
<h2>Dependencies</h2>
<div class="panel-subtitle">
Saving replaces the whole dependency list with the entries below; it does not add to what the tool
already has. The console writes nothing itself and sends the revision shown above.
</div>
{{if .Caps.Implements "ticket set-dependencies"}}{{template "deps-form" .}}
{{else}}<p>Dependency editing is unavailable: {{.Caps.Reason "ticket set-dependencies"}}</p>{{end}}
</div>

<div class="panel">
<h2>Change this ticket</h2>
<div class="panel-subtitle">
Every control below runs the owning tool's own verb. The console writes nothing itself, sends the
revision shown above, and does not retry on your behalf: a conflict comes back for you to re-read.
</div>
{{if .EditEnabled}}{{template "ticket-form" .}}{{else}}<p>Title and body editing is unavailable: {{.Caps.Reason "ticket refine"}}</p>{{end}}
{{range .Controls}}{{if and (ne .Verb "refine") (ne .Verb "set-dependencies") (ne .Verb "prioritize")}}
<form class="mut" method="post" action="/mutate">
  <input type="hidden" name="token" value="{{$.Token}}">
  <input type="hidden" name="verb" value="{{.Verb}}">
  <input type="hidden" name="ticket" value="{{$.Detail.Card.TicketID}}">
  <input type="hidden" name="expected" value="{{$.Detail.Card.Revision}}">
  <input type="hidden" name="requestId" value="{{$.RequestID}}{{.Verb}}">
  <input type="hidden" name="issuedAt" value="{{$.IssuedAt}}">
  <input type="text" name="payload" value="{{.PayloadTemplate}}"{{if not .Enabled}} disabled{{end}}>
  <button type="submit"{{if not .Enabled}} disabled{{end}}>{{.Label}}</button>
  {{if not .Enabled}}<div class="disabled-reason">disabled: {{.Reason}}</div>{{end}}
</form>
{{end}}{{end}}
</div>
</aside>
</div>

<div class="panel">
<h2>Code at revision</h2>
<form class="mut" method="get" action="/code">
  <input type="text" name="path" placeholder="path/in/repo.go" value="{{.CodePath}}">
  <button type="submit">Read at {{if .Revision.Commit}}{{slice .Revision.Commit 0 12}}{{else}}HEAD{{end}}</button>
</form>
{{if .Blob}}
  {{if .Blob.Err}}<div class="unknown">{{.Blob.Err}}</div>
  {{else}}
  <div style="font-size:12px;margin-bottom:6px">
  <code>{{.Blob.Path}}</code> at object <code>{{.Blob.ObjectID}}</code>
  {{if .Blob.Truncated}} · truncated{{end}}</div>
  {{if .Blob.Dirty}}<div class="note">{{.Blob.DirtyNote}}</div>{{end}}
  <pre>{{.Blob.Text}}</pre>
  {{template "source" .Blob.Source}}
  {{end}}
{{end}}
</div>
{{end}}
{{end}}`)

// specView lists every spec with its declared intent and delivery.
var specView = mustView(`{{define "body"}}
<div class="panel">
<h2>Specifications</h2>
{{if .SpecErr}}<div class="unknown">{{.SpecErr}}</div>
{{else}}
<table>
<tr><th>spec</th><th>prefix</th><th>intent</th><th>delivery</th></tr>
{{range .Specs}}
<tr><th style="width:auto"><code>{{.Path}}</code><br>{{.Title}}</th>
<td>{{or_dash .ReqPrefix}}</td><td>{{.Intent}}</td><td>{{.Delivery}}</td></tr>
{{end}}
</table>
<div style="margin-top:10px;font-size:12px;color:var(--dim)">
Intent and delivery are each spec's own declaration, rendered as declared. The console does not
judge them and cannot advance one.</div>
{{end}}
</div>
{{end}}`)

// mutationView reports what the owning tool answered to one delegated change.
var mutationView = mustView(`{{define "body"}}
<div class="panel">
<h2>{{if .Result.Conflict}}Conflict{{else if .Result.Envelope}}{{.Result.Envelope.Outcome}}{{else}}Failed{{end}}</h2>
<table>
<tr><th>verb</th><td><code>corvint-tasks ticket {{.Result.Request.Verb}}</code></td></tr>
<tr><th>ticket</th><td><code>{{.Result.Request.TicketID}}</code></td></tr>
<tr><th>expected revision</th><td>{{.Result.Request.Expected}}</td></tr>
<tr><th>request id</th><td><code>{{.Result.Request.RequestID}}</code></td></tr>
</table>
{{if .Result.Conflict}}
<div class="note" style="margin-top:10px"><b>Nothing was written.</b>
The record moved since it was rendered, so the change was refused rather than applied to a
revision you did not see. Re-read the ticket and decide again; the console will not retry for you.</div>
{{end}}
{{if .Result.Err}}<div class="refusal" style="margin-top:10px">{{.Result.Err}}</div>{{end}}
{{if .Result.Envelope}}
  {{if .Result.Envelope.Refused}}
    {{template "refusal" refusalOfEnvelope "The owning tool refused this change" .Result.Source .Result.Envelope}}
  {{else}}
    <div style="margin-top:10px">The owning tool applied this change.</div>
    {{range .Result.Envelope.Items}}<table>
    {{range $k, $v := .}}<tr><th>{{$k}}</th><td>{{render $v}}</td></tr>{{end}}</table>{{end}}
    {{range .Result.Envelope.Warnings}}<div class="unknown" style="margin-top:6px">{{.}}</div>{{end}}
    {{template "source" .Result.Source}}
  {{end}}
{{end}}
{{if .Form}}{{if or (eq .Form.Verb "create") (eq .Form.Verb "refine")}}<h3>Your submitted entries</h3>{{template "ticket-form" .}}{{end}}{{end}}
{{if .DepsForm}}<h3>Your submitted entries</h3>{{template "deps-form" .}}{{end}}
{{if .PrioritizeForm}}<h3>Your submitted entries</h3>{{template "prioritize-form" .}}{{end}}
<div style="margin-top:12px">{{if .Result.Request.TicketID}}<a href="/ticket?id={{.Result.Request.TicketID}}">Open ticket</a> · {{end}}<a href="/">Back to board</a></div>
</div>
{{end}}`)

// codeView renders one file at an immutable object id on its own page.
var codeView = mustView(`{{define "body"}}
<div class="panel">
<h2>Code</h2>
<form class="mut" method="get" action="/code">
  <input type="text" name="path" value="{{.CodePath}}">
  <button type="submit">Read</button>
</form>
{{if .Blob}}
  {{if .Blob.Err}}<div class="unknown">{{.Blob.Err}}</div>
  {{else}}
  <div style="font-size:12px;margin-bottom:6px"><code>{{.Blob.Path}}</code> at object
  <code>{{.Blob.ObjectID}}</code>{{if .Blob.Truncated}} · truncated{{end}}</div>
  {{if .Blob.Dirty}}<div class="note">{{.Blob.DirtyNote}}</div>{{end}}
  <pre>{{.Blob.Text}}</pre>
  {{template "source" .Blob.Source}}
  {{end}}
{{end}}
{{if .Pin}}<div class="unknown">gap: {{.Pin}}</div>{{end}}
</div>
{{if .Backlinks}}
<div class="panel">
<h2>Requirements citing this path</h2>
<div style="font-size:12px;color:var(--dim);margin-bottom:8px">
A requirement is listed when a Traceability table's Implementation cell at commit
<code>{{.Backlinks.Commit}}</code> cites exactly this path. Each link is pinned to that commit.</div>
{{if .Backlinks.Gap}}<div class="unknown">gap: {{.Backlinks.Gap}}</div>{{end}}
{{if .Backlinks.Requirements}}<table>
{{range .Backlinks.Requirements}}<tr><th>{{if .Gap}}<code>{{or_dash .ID}}</code>{{else}}<a href="/requirement?id={{.ID}}&at={{$.Backlinks.Commit}}"><code>{{.ID}}</code></a>{{end}}</th>
<td><code>{{.Spec}}</code>{{if .Gap}} <span class="unknown">gap: {{.Gap}}</span>{{end}}</td></tr>{{end}}
</table>{{end}}
{{if .Backlinks.Truncated}}<div class="note">Only the first citing specs were read; this list is partial.</div>{{end}}
</div>
{{end}}
{{end}}`)

// requirementView renders one requirement's clause and the code its owning
// spec's Traceability table cites at one commit. A citation that does not
// resolve to a file at that commit is a gap, never a link (LAC-V0-030).
var requirementView = mustView(`{{define "body"}}
{{if .Pin}}<div class="panel"><div class="unknown">gap: {{.Pin}}</div></div>
{{else}}
<div class="panel">
<h2>Requirement</h2>
{{with .Links.Clause}}
  {{if .Err}}<div class="unknown">gap: {{.Err}}</div>
  {{else}}
  <table>
  <tr><th>requirement</th><td><code>{{.Requirement.ID}}</code> — {{.Requirement.Title}}</td></tr>
  <tr><th>clause</th><td>{{.Text}}</td></tr>
  {{if .Spec}}
  <tr><th>spec</th><td>{{.Spec.Title}} (<code>{{.Spec.Path}}</code>)</td></tr>
  <tr><th>declared intent</th><td>{{.Spec.Intent}}</td></tr>
  <tr><th>declared delivery</th><td>{{.Spec.Delivery}}</td></tr>
  {{end}}
  </table>
  {{template "source" .Source}}
  {{end}}
{{end}}
</div>
<div class="panel">
<h2>Cited code</h2>
{{if .Links.Spec}}{{if not .Links.Spec.Err}}<div style="font-size:12px;color:var(--dim);margin-bottom:8px">
Read from the Traceability table of <code>{{.Links.Spec.Path}}</code> at object
<code>{{.Links.Spec.ObjectID}}</code>, commit <code>{{.Links.Commit}}</code>. Each link is pinned to that commit.</div>{{end}}{{end}}
{{if .Links.Gap}}<div class="unknown">gap: {{.Links.Gap}}</div>{{end}}
{{if .Links.Code}}<table>
{{range .Links.Code}}<tr><th><code>{{.Path}}</code></th>
<td>{{if .Gap}}<span class="unknown">gap: {{.Gap}}</span>{{else}}<a href="/code?path={{.Path}}&at={{$.Links.Commit}}">blob <code>{{.ObjectID}}</code></a>{{end}}</td></tr>{{end}}
</table>{{end}}
{{if .Links.Spec}}{{if not .Links.Spec.Err}}{{template "source" .Links.Spec.Source}}{{end}}{{end}}
</div>
{{end}}
{{end}}`)

// evidenceView is the Corvint evidence pane. It is the one surface whose source
// states all six axes for every value, so a metric here carries measured axes
// where a board card carries NOT_STATED. Every axis shown is the snapshot's
// own; a value outside an axis's closed set stays unstated rather than being
// renamed into one (LAC-V0-007).
var evidenceView = mustView(`{{define "body"}}
{{if .Evidence.Err}}{{template "refusal" refusalOf "The evidence snapshot could not be compiled" .Evidence.Source .Evidence.Err}}{{end}}
{{if .Evidence.Refused}}{{template "refusal" refusalOf "The snapshot compiler refused" .Evidence.Source .Evidence.Code}}{{end}}

{{if and (not .Evidence.Err) (not .Evidence.Refused)}}
<div class="panel">
<h2>Observation boundary</h2>
<table>
<tr><th>schema</th><td><code>{{.Evidence.Schema}}</code></td></tr>
<tr><th>generated at</th><td>{{.Evidence.GeneratedAt}}</td></tr>
<tr><th>snapshot digest</th><td><code>{{.Evidence.Digest}}</code></td></tr>
<tr><th>scan state</th><td>{{or_dash .Evidence.ScanState}}</td></tr>
<tr><th>head revision</th><td><code>{{or_dash .Evidence.HeadRev}}</code></td></tr>
<tr><th>tree revision</th><td><code>{{or_dash .Evidence.TreeRev}}</code></td></tr>
<tr><th>worktree</th><td>{{or_dash .Evidence.Worktree}}{{if .Evidence.DirtyCount}} · {{.Evidence.DirtyCount}} dirty paths{{end}}</td></tr>
</table>
{{if .Evidence.Privacy}}<div class="grp">privacy, as the snapshot declared it</div>
<table>{{range .Evidence.Privacy}}<tr><th>{{.Key}}</th><td>{{.Value}}</td></tr>{{end}}</table>{{end}}
{{template "source" .Evidence.Source}}
</div>

{{if .Evidence.Issues}}
<div class="panel">
<h2>Issues the snapshot raised about its own inputs</h2>
<table>
<tr><th>code</th><th>severity</th><th>source</th></tr>
{{range .Evidence.Issues}}<tr class="metric"><th style="width:auto"><code>{{.Code}}</code></th>
<td>{{.Severity}}</td><td><code>{{or_dash .SourceID}}</code></td></tr>{{end}}
</table>
</div>
{{end}}

<div class="panel">
<h2>Metrics ({{.Evidence.Total}})</h2>
{{if .Evidence.Families}}
{{range .Evidence.Families}}
<div class="grp">{{.Name}} ({{len .Metrics}})</div>
<table>
{{range .Metrics}}
<tr class="metric"><th style="width:auto">{{.Name}}
{{if .Dimensions}}<div style="font-size:11px;color:var(--dim);font-weight:400">{{.Dimensions}}</div>{{end}}</th>
<td>{{if .Unmeasured}}<span class="unmeasured">no value measured</span>
{{else}}<span class="num">{{.Value}}</span> <span style="font-size:11px;color:var(--dim)">{{.Unit}}</span>{{end}}
<div style="font-size:11px;color:var(--dim)">scope {{.Scope}}{{if .SourceIDs}} · from {{len .SourceIDs}} sources{{end}}
{{if .Exclusions}} · {{len .Exclusions}} exclusions{{end}}</div>
{{template "axes" .Axes}}
{{range .SourceIDs}}<div style="font-size:11px;color:var(--dim)"><code>{{.}}</code></div>{{end}}
</td></tr>
{{end}}
</table>
{{end}}
<div style="margin-top:12px;font-size:12px;color:var(--dim)">
A metric with no measured value is marked as such and is not printed as a zero. Every axis above is
the snapshot's own statement about that value; the console states none of its own.</div>
{{else}}
<div>The snapshot reported no metric family the console renders. That is what the tool returned,
not an empty measurement.</div>
{{end}}
</div>

<div class="panel">
<h2>Contributing sources ({{len .Evidence.Sources}})</h2>
{{if .Evidence.Sources}}
{{range .Evidence.Sources}}
<table>
<tr><th>source</th><td>{{.Label}} · <code>{{.ID}}</code></td></tr>
<tr><th>adapter</th><td>{{.AdapterID}} · profile <code>{{.Profile}}</code></td></tr>
<tr><th>verifier</th><td>{{or_dash .VerifierID}}</td></tr>
<tr><th>content</th><td><code>{{or_dash .Digest}}</code>{{if .Bytes}} · {{.Bytes}} bytes{{end}}</td></tr>
{{if .Exclusions}}<tr><th>exclusions</th><td>{{range .Exclusions}}<code>{{.}}</code> {{end}}</td></tr>{{end}}
</table>
{{template "axes" .Axes}}
{{end}}
{{else}}<div>The snapshot named no contributing source.</div>{{end}}
</div>
{{end}}
{{end}}`)

// dogfoodView is the dogfood loop's own account of the last run. Its point is
// the NOT_PRODUCED reasons: a step that could not run is what an operator has
// to see, so no step is summarised away and no count is presented as success.
var dogfoodView = mustView(`{{define "body"}}
{{if .Dogfood.Absent}}
<div class="panel">
<h2>Dogfood</h2>
<div>No dogfood report exists in this repository, so the loop's state here is
<b>NOT_OBSERVED</b>. That is an absent report, not a run that produced nothing and not a pass.
Run <code>make dogfood-change BASE=&lt;sha&gt;</code> to produce one.</div>
{{template "source" .Dogfood.Source}}
</div>
{{else if .Dogfood.Err}}
{{template "refusal" refusalOf "The dogfood report could not be read" .Dogfood.Source .Dogfood.Err}}
{{else}}
<div class="panel">
<h2>Last-written dogfood report</h2>
<p>This shared report file is replaced by each dogfood run in this worktree.
It may belong to another session. Session ownership is NOT_OBSERVED.</p>
<table>
<tr><th>report path</th><td><code>{{.Dogfood.Path}}</code></td></tr>
<tr><th>profile</th><td><code>{{.Dogfood.Profile}}</code></td></tr>
<tr><th>base</th><td><code>{{or_dash .Dogfood.Base}}</code></td></tr>
<tr><th>target</th><td><code>{{or_dash .Dogfood.Target}}</code></td></tr>
<tr><th>complete</th><td>{{if .Dogfood.Complete}}yes{{else}}<b>no</b> — {{.Dogfood.NotProduced}} of {{len .Dogfood.Steps}} steps did not produce{{end}}</td></tr>
<tr><th>anchor</th><td>{{or_dash .Dogfood.Anchor.State}}</td></tr>
<tr><th>file modified at</th><td>{{.Dogfood.ModifiedAt.Format "2006-01-02T15:04:05Z"}}</td></tr>
</table>
<div class="note" style="margin-top:10px">This report is untracked local derived state. It carries no
content digest and no owning verifier, so the console states none of the six axes for it rather than
supplying one. It is the loop's own account of itself, not independent evidence.</div>
{{template "source" .Dogfood.Source}}
</div>

<div class="panel">
<h2>Steps</h2>
{{if .Dogfood.Steps}}
<table>
<tr><th>step</th><th>status</th><th>reason</th></tr>
{{range .Dogfood.Steps}}
<tr class="metric"><th style="width:auto">{{.Name}}</th>
<td>{{if .Produced}}{{.Status}}{{else}}<span class="unknown">{{.Status}}</span>{{end}}</td>
<td>{{or_dash .Reason}}</td></tr>
{{end}}
</table>
<div style="margin-top:10px;font-size:12px;color:var(--dim)">
Every NOT_PRODUCED reason is shown as the loop wrote it. A step that did not produce is not a step
that passed, and the console does not aggregate these into a verdict: it is never a gate.</div>
{{else}}<div>The report lists no step, which is the report's content and not a completed loop.</div>{{end}}
</div>
{{end}}
{{end}}`)

// chainView renders one sealed change as hunk, cited evidence, governing
// requirement and recorded verification (LAC-V0-033). Every edge row names the
// artifact and field that justify it; an edge the artifacts do not establish
// is a gap row with its class and reason, never an inferred link (LAC-V0-034,
// LAC-V0-035). All artifact text is agent- or tool-written and renders inert.
var chainView = mustView(`{{define "edge"}}
<tr{{if .Gap}} class="unknown"{{end}}>
<td>{{if .Gap}}<b>gap: {{.Gap}}</b>{{else}}linked{{end}}</td>
<td>{{if .Anchor}}<a href="#{{.Anchor}}"><code>{{.Target}}</code></a>{{else}}<code>{{or_dash .Target}}</code>{{end}}</td>
<td><code>{{.Artifact}}</code> · <code>{{.Field}}</code></td>
<td>{{if .Gap}}{{.Reason}}{{else}}{{if .Pin}}pinned <code>{{.Pin}}</code>{{end}}{{if .Detail}}<div>{{.Detail}}</div>{{end}}{{end}}
{{template "axes" .Axes}}</td>
</tr>
{{end}}
{{define "edges"}}<table>
<tr><th>state</th><th>target</th><th>justified by (artifact · field)</th><th>pin, detail or gap reason</th></tr>
{{range .}}{{template "edge" .}}{{end}}
</table>{{end}}
{{define "body"}}
{{if .Pin}}<div class="panel"><div class="unknown">gap: {{.Pin}}</div></div>
{{else}}
<div class="panel">
<h2>Sealed changes</h2>
<div style="font-size:12px;color:var(--dim);margin-bottom:10px">Each sealed change map committed under
<code>.corvint/changes</code> at this commit, named by the commit that bound it. A chain links only what
an artifact field names by identifier or digest: nothing is linked by position, text, shared path or
proximity. A linked edge is structural. It is never a claim that evidence supports a change semantically
or that a test passed.</div>
{{if .Listing}}{{if .Listing.Err}}{{template "refusal" refusalOf "The sealed changes could not be listed" .Listing.Source .Listing.Err}}
{{else}}{{if .Changes}}<ul>{{range .Changes}}<li><a href="/chain?change={{.}}&amp;at={{$.Revision.Commit}}"{{if $.Chain}}{{if eq . $.Chain.Change}} aria-current="true"{{end}}{{end}}><code>{{.}}</code></a></li>{{end}}</ul>
{{else}}<div>The tree at this revision holds no sealed change map, so there is no chain to show.</div>{{end}}
{{if .Listing.Truncated}}<div class="unknown">this listing was truncated; it is PARTIAL</div>{{end}}
{{template "source" .Listing.Source}}{{end}}{{end}}
</div>
{{with .Chain}}
{{if .Err}}{{template "refusal" refusalOf "This sealed change map cannot be drawn as a chain" .Sealed.Source .Err}}
{{else}}
<div class="panel">
<h2>Change <code>{{.Change}}</code></h2>
<table>
<tr><th>sealed map</th><td><code>{{.SealedPath}}</code> at object <code>{{.Sealed.ObjectID}}</code>, commit <code>{{.Commit}}</code></td></tr>
<tr><th>profile</th><td><code>{{.Profile}}</code></td></tr>
<tr><th>base revision</th><td><code>{{or_dash .Base}}</code></td></tr>
<tr><th>hunks</th><td>{{len .Hunks}}</td></tr>
</table>
<h3>Change binding</h3>
{{template "edges" .Binding}}
{{template "source" .Sealed.Source}}
</div>

<div class="panel">
<h2>Local artifacts read</h2>
<div style="font-size:12px;color:var(--dim);margin-bottom:8px">Untracked files the CLI wrote in this
worktree. An OCM map is bound only when its <code>targetRevision</code> names this change and its
<code>cem.mapSha256</code> is the SHA-256 of the sealed map. These files carry no owning verifier, so
their axes are NOT_STATED.</div>
<table>
<tr><th>path</th><th>state</th><th>reason</th></tr>
{{range .Artifacts}}<tr{{if and (ne .State "bound") (ne .State "read") (ne .State "unrelated")}} class="unknown"{{end}}><td><code>{{.Path}}</code></td><td>{{.State}}</td><td>{{.Reason}}{{template "axes" .Source.Axes}}</td></tr>{{end}}
</table>
</div>

<div class="panel">
<h2>Recorded verification</h2>
<div style="font-size:12px;color:var(--dim);margin-bottom:8px">A row of the local trace whose
<code>revision</code> field names this change. It is the outcome an operator recorded for the whole
revision, not a test result and not a result for any one requirement.</div>
{{template "edges" .Verification}}
</div>

<div class="panel">
<h2>Hunks</h2>
{{range .Hunks}}
<h3><a href="/chain?change={{$.Chain.Change}}&amp;hunk={{.ID}}&amp;at={{$.Revision.Commit}}#hunk-detail"><code>{{.Path}}</code> −{{.Old.Start}},{{.Old.Count}} +{{.New.Start}},{{.New.Count}}</a></h3>
<div style="font-size:12px"><code>{{.ID}}</code> · <code>{{$.Chain.SealedPath}}</code> · <code>{{.Field}}</code> · disposition <code>{{.Disposition}}</code>{{if .Reason}} · reason <code>{{.Reason}}</code>{{end}}</div>
<h4>Cited evidence</h4>
{{template "edges" .Evidence}}
<h4>Governing requirement</h4>
{{template "edges" .Requirements}}
{{end}}
</div>

{{with .Detail}}
<div class="panel" id="hunk-detail">
<h2>Hunk detail</h2>
{{if .Err}}<div class="unknown">gap: {{.Err}}</div>
{{else}}
<div style="font-size:12px"><code>{{.Hunk.ID}}</code> · {{.Range}} lines of <code>{{.Hunk.Path}}</code></div>
{{if .Lines.Err}}<div class="unknown">gap: missing: {{.Lines.Err}}</div>
{{else}}
<div style="font-size:12px">at object <code>{{.Lines.ObjectID}}</code>, revision <code>{{.Lines.Revision}}</code></div>
{{if .Text}}<pre>{{.Text}}</pre>{{else}}<div class="unknown">gap: stale: the map's line range does not lie inside this object</div>{{end}}
{{template "source" .Lines.Source}}
{{end}}
<h3>Cited spans</h3>
{{range .Spans}}
<div><code>{{or_dash .Edge.Target}}</code> · <code>{{.Edge.Field}}</code></div>
{{if .Edge.Gap}}<div class="unknown">gap: {{.Edge.Gap}}: {{.Edge.Reason}}</div>
{{else}}<div style="font-size:12px">pinned <code>{{.Edge.Pin}}</code></div><pre>{{.Text}}</pre>{{end}}
{{end}}
{{end}}
</div>
{{end}}

<div class="panel">
<h2>Requirements</h2>
{{range .Requirements}}
<h3{{if .Anchor}} id="{{.Anchor}}"{{end}}><code>{{.ID}}</code> · disposition <code>{{or_dash .Disposition}}</code>{{if .Reason}} · reason <code>{{.Reason}}</code>{{end}}</h3>
<h4>Clause at the pinned intent scope</h4>
{{template "edges" .Clause}}
<h4>Hunks the obligation lists</h4>
{{template "edges" .Hunks}}
<h4>Test claims the obligation lists</h4>
{{template "edges" .Claims}}
<h4>Recorded verification of the change</h4>
{{template "edges" .Verification}}
{{else}}
<div class="unknown">gap: missing: no OCM map bound to this sealed map lists an obligation, so no requirement is shown.</div>
{{end}}
</div>
{{end}}
{{end}}
{{end}}
{{end}}`)

// listingView renders one committed directory: the benchmark results and the
// agent-memory backlogs both reach the surface this way. Each entry is named
// by the immutable object id its bytes come from, and its content is read
// through the code pane, which renders it inert (LAC-V0-019, LAC-V0-020).
var listingView = mustView(`{{define "body"}}
<div class="panel">
<h2>{{.ListingTitle}}</h2>
<div style="font-size:12px;color:var(--dim);margin-bottom:10px">{{.ListingNote}}</div>
{{if .Listing.Err}}{{template "refusal" refusalOf "This directory could not be listed" .Listing.Source .Listing.Err}}
{{else}}
{{if .Listing.Entries}}
<table>
<tr><th>file</th><th>object</th><th>bytes</th></tr>
{{range .Listing.Entries}}
<tr class="metric"><th style="width:auto"><a href="{{$.ListingRoute}}?path={{.Path}}">{{.Name}}</a></th>
<td><code>{{slice .ObjectID 0 12}}</code></td><td class="num">{{.Bytes}}</td></tr>
{{end}}
</table>
{{if .Listing.Truncated}}<div class="unknown">this listing was truncated; it is PARTIAL, not the
whole directory</div>{{end}}
{{else}}
<div>The tree at this revision holds no file in <code>{{.Listing.Dir}}</code>. That is what the
revision names, not a directory the console failed to read.</div>
{{end}}
{{template "source" .Listing.Source}}
{{end}}
</div>

{{if .Blob}}
<div class="panel">
<h2>{{.Blob.Path}}</h2>
{{if .Blob.Err}}<div class="unknown">{{.Blob.Err}}</div>
{{else}}
<div style="font-size:12px;margin-bottom:6px">at object <code>{{.Blob.ObjectID}}</code>
{{if .Blob.Truncated}} · truncated{{end}}</div>
{{if .Blob.Dirty}}<div class="note">{{.Blob.DirtyNote}}</div>{{end}}
<pre>{{.Blob.Text}}</pre>
{{template "source" .Blob.Source}}
{{end}}
</div>
{{end}}
{{end}}`)

const ticketFormTemplate = `{{define "ticket-form"}}
<form class="ticket-form" method="post" action="/mutate">
<input type="hidden" name="token" value="{{.Token}}">
{{with .Form}}
<input type="hidden" name="verb" value="{{.Verb}}">
<input type="hidden" name="ticket" value="{{.TicketID}}">
<input type="hidden" name="expected" value="{{.Expected}}">
<input type="hidden" name="requestId" value="{{.RequestID}}">
<input type="hidden" name="issuedAt" value="{{.IssuedAt}}">
<label>Title (at most 512 bytes)<input name="title" required maxlength="512" value="{{.Title}}"></label>
<label>Body (at most 64 KiB)<textarea name="body" maxlength="65536">{{.Body}}</textarea></label>
{{if eq .Verb "create"}}
<label>Acceptance criteria (one per line, at most 64 of 4 KiB each)<textarea name="criteria" maxlength="262208">{{.Criteria}}</textarea></label>
<p>New tickets are manual features with priority P2. Effects are unknown; creating a ticket does not authorize execution.</p>
<button type="submit">Create ticket</button>
{{else}}<label>Milestone (blank leaves it unchanged)<input name="milestone" value="{{.Milestone}}"></label>
<p>Saving against shown revision {{.Expected}}. A conflict keeps your entries here; reload the ticket before deciding again.</p><button type="submit">Save title and body</button>{{end}}
{{end}}</form>{{end}}

{{define "deps-form"}}
<form class="ticket-form" method="post" action="/mutate">
<input type="hidden" name="token" value="{{.Token}}">
{{with .DepsForm}}
<input type="hidden" name="verb" value="{{.Verb}}">
<input type="hidden" name="ticket" value="{{.TicketID}}">
<input type="hidden" name="expected" value="{{.Expected}}">
<input type="hidden" name="requestId" value="{{.RequestID}}">
<input type="hidden" name="issuedAt" value="{{.IssuedAt}}">
<label>Dependencies (one per line: a ticket id is a COMPLETED obligation; a ticket id and a gate id is a GATE_PASSED obligation)<textarea name="dependencies" maxlength="65536">{{.Dependencies}}</textarea></label>
<p>Saving against shown revision {{.Expected}}. A conflict or cycle keeps your entries here; reload the ticket before deciding again.</p>
<button type="submit">Save dependencies</button>
{{end}}</form>{{end}}

{{define "prioritize-form"}}
<form class="ticket-form" method="post" action="/mutate">
<input type="hidden" name="token" value="{{.Token}}">
{{with .PrioritizeForm}}
<input type="hidden" name="verb" value="{{.Verb}}">
<input type="hidden" name="ticket" value="{{.TicketID}}">
<input type="hidden" name="expected" value="{{.Expected}}">
<input type="hidden" name="requestId" value="{{.RequestID}}">
<input type="hidden" name="issuedAt" value="{{.IssuedAt}}">
<label>Order<input name="order" required value="{{.Order}}"></label>
<label>Priority<input name="priority" required value="{{.Priority}}"></label>
<p>Saving against shown revision {{.Expected}}. A conflict keeps your entries here; reload the ticket before deciding again.</p>
<button type="submit">Save order and priority</button>
{{end}}</form>{{end}}`
