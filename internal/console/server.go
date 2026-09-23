package console

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Options configure one console process.
type Options struct {
	Addr   string // loopback host:port
	Repo   string // the repository whose ticket store is administered
	Binary string // the `corvint-tasks` executable
	// Specs is the repository holding `docs/specs/REQUIREMENTS.tsv` and
	// `docs/specs/INDEX.json`, and the tree the code pane reads. It is
	// separate from Repo because a ticket store and the spec corpus that
	// governs it need not be the same repository (decision 0081).
	Specs string
	// Snapshot is the `corvint-dashboard-snapshot` executable. The evidence pane
	// compiles no snapshot of its own: it runs this tool and renders what the
	// tool wrote, the same delegation the board makes to `corvint-tasks` (LAC-V0-004).
	Snapshot string
	Timeout  time.Duration
}

// specsRoot is where the spec and code panes read from.
func (o Options) specsRoot() string {
	if o.Specs != "" {
		return o.Specs
	}
	return o.Repo
}

// Server keeps only its listener binding and per-start form secret. Domain
// values are read from their owning tools at request time (LAC-V0-004).
type Server struct {
	options Options
	mux     *http.ServeMux
	host    string
	token   string
}

// view is everything one page renders.
type view struct {
	Token          string
	Form           *ticketForm
	CreateEnabled  bool
	EditEnabled    bool
	Title          string
	Repo           string
	Revision       Revision
	CompiledAt     string
	Boundary       bool
	Caps           *Capabilities
	Board          *Board
	Detail         *Detail
	Clauses        []*Clause
	Controls       []Control
	Roadmap        *Roadmap
	DepsForm       *ticketForm
	PrioritizeForm *ticketForm
	RequestID      string
	IssuedAt       string
	CodePath       string
	Blob           *Blob
	Pin            string
	Links          *RequirementLinks
	Backlinks      *CodeBacklinks
	Specs          []SpecEntry
	SpecErr        string
	Result         *MutationResult
	RefreshSeconds int

	// S3 panes: the Corvint evidence snapshot, the dogfood loop's own report,
	// and one committed directory (the benchmark results, the agent-memory
	// backlogs) rendered under the same axes as everything else.
	Evidence     *Evidence
	Dogfood      *Dogfood
	Listing      *Listing
	ListingTitle string
	ListingNote  string
	ListingRoute string

	// The chain pane: the sealed changes this commit holds and, for the one
	// selected, its hunk-to-verification chain (LAC-V0-033).
	Changes []string
	Chain   *Chain
}

// New builds a console bound to one repository. It refuses a non-loopback
// address: the console has no authentication because it is never reachable
// from another host, and those two facts must not come apart (LAC-V0-002).
func New(options Options) (*Server, error) {
	host, port, err := net.SplitHostPort(options.Addr)
	if err != nil {
		return nil, fmt.Errorf("address %q is not host:port: %w", options.Addr, err)
	}
	if !loopback(host) {
		return nil, fmt.Errorf("address %q is not loopback: the console has no authentication and must not be reachable from another host", options.Addr)
	}
	if n, err := strconv.Atoi(port); err != nil || n < 0 || n > 65535 {
		return nil, fmt.Errorf("invalid listener port")
	}
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return nil, err
	}
	server := &Server{options: options, mux: http.NewServeMux(), host: options.Addr, token: hex.EncodeToString(secret[:])}
	server.mux.HandleFunc("/", server.handleBoard)
	server.mux.HandleFunc("/roadmap", server.handleRoadmap)
	server.mux.HandleFunc("/ticket", server.handleTicket)
	server.mux.HandleFunc("/specs", server.handleSpecs)
	server.mux.HandleFunc("/code", server.handleCode)
	server.mux.HandleFunc("/requirement", server.handleRequirement)
	server.mux.HandleFunc("/mutate", server.handleMutate)
	server.mux.HandleFunc("/evidence", server.handleEvidence)
	server.mux.HandleFunc("/dogfood", server.handleDogfood)
	server.mux.HandleFunc("/chain", server.handleChain)
	server.mux.HandleFunc("/benchmarks", server.handleBenchmarks)
	server.mux.HandleFunc("/backlogs", server.handleBacklogs)
	return server, nil
}

func loopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ServeHTTP answers one request. Every handler reads its sources fresh.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Host != s.host {
		http.Error(w, "unexpected Host", http.StatusForbidden)
		return
	}
	if origins, present := r.Header["Origin"]; present {
		if len(origins) != 1 {
			http.Error(w, "unexpected Origin", http.StatusForbidden)
			return
		}
		origin, err := url.Parse(origins[0])
		if err != nil || origin.Scheme != "http" || origin.Host != s.host || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
			http.Error(w, "unexpected Origin", http.StatusForbidden)
			return
		}
	}
	if r.URL.Path != "/mutate" && r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "read routes accept GET or HEAD", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path == "/mutate" {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "mutations are POST only", http.StatusMethodNotAllowed)
			return
		}
		// A maximal ATM ticket (64 KiB body plus 64 criteria of 4 KiB) is
		// about 320 KiB before form encoding; 1 MiB keeps it readable and
		// still bounds the body before ParseForm.
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "unreadable or oversized form", http.StatusBadRequest)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("token")), []byte(s.token)) != 1 {
			http.Error(w, "invalid form token; reload this page", http.StatusForbidden)
			return
		}
		if !reviewedMutation(r.PostForm.Get("verb")) {
			http.Error(w, "mutation is not offered by this console", http.StatusBadRequest)
			return
		}
	}
	s.mux.ServeHTTP(w, r)
}

// Serve runs the console in the foreground on ln until ctx is cancelled. It
// installs nothing and survives no invocation (LAC-V0-003).
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	address, ok := ln.Addr().(*net.TCPAddr)
	if !ok || !address.IP.IsLoopback() || address.Port < 1 {
		_ = ln.Close()
		return fmt.Errorf("listener must be a concrete loopback TCP address")
	}
	s.host = address.String()
	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	httpServer := &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 15 * time.Second,
		BaseContext: func(net.Listener) context.Context { return serveCtx }}
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-serveCtx.Done()
		shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			_ = httpServer.Close()
		}
	}()
	err := httpServer.Serve(ln)
	cancel()
	<-done
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) taskman(ctx context.Context) *Taskman {
	tool := &Taskman{Binary: s.options.Binary, Repo: s.options.Repo, Timeout: s.options.Timeout}
	if envelope, _ := tool.Run(ctx, "version"); envelope != nil && len(envelope.Items) > 0 {
		tool.Version = stringOf(envelope.Items[0]["version"])
	}
	return tool
}

// newView compiles the frame every page shares: the revision it was read at
// and the boundary it was compiled at (LAC-V0-011).
func (s *Server) newView(ctx context.Context, title string) *view {
	worktree := Worktree{Root: s.options.specsRoot(), Timeout: s.options.Timeout}
	revision := worktree.Head(ctx)
	return &view{
		Token:      s.token,
		Title:      title,
		Repo:       s.options.Repo,
		Revision:   revision,
		CompiledAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		Boundary:   true,
	}
}

func (s *Server) handleBoard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	tool := s.taskman(ctx)
	data := s.newView(ctx, "Board")
	data.Caps = tool.ReadCapabilities(ctx)
	data.Board = tool.ReadBoard(ctx, data.Caps)
	data.Form = newTicketForm("create")
	data.CreateEnabled = data.Caps.Implements("ticket create") && data.Board.Err == "" && data.Board.Envelope != nil && !data.Board.Envelope.Refused()
	render(w, boardView, data)
}

// handleRoadmap renders `atm roadmap`, grouped by the milestone the tool
// assigned, one page of at most roadmapPageSize tickets at a time.
func (s *Server) handleRoadmap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tool := s.taskman(ctx)
	data := s.newView(ctx, "Roadmap")
	if r.URL.Query().Get("refresh") != "off" {
		data.RefreshSeconds = roadmapRefreshSeconds
	}
	page := 1
	if raw := r.URL.Query().Get("page"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	data.Roadmap = tool.ReadRoadmap(ctx, page)
	render(w, roadmapView, data)
}

func (s *Server) handleTicket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "no ticket id", http.StatusBadRequest)
		return
	}
	tool := s.taskman(ctx)
	data := s.newView(ctx, "Ticket")
	data.Caps = tool.ReadCapabilities(ctx)
	data.Detail = tool.ReadDetail(ctx, id)
	data.Controls = data.Caps.Controls()
	data.EditEnabled = data.Caps.Implements("ticket refine")
	data.Form = newTicketForm("refine")
	data.Form.TicketID, data.Form.Expected, data.Form.Title = data.Detail.Card.TicketID, data.Detail.Card.Revision, data.Detail.Card.Title
	data.Form.Body = stringOf(data.Detail.Record["body"])
	data.Form.Milestone = data.Detail.Card.Milestone
	// The idempotency key and timestamp are minted with the form, so
	// resubmitting it replays instead of committing a second change.
	data.RequestID = NewRequestID()
	data.IssuedAt = IssuedNow()

	data.DepsForm = newTicketForm("set-dependencies")
	data.DepsForm.TicketID, data.DepsForm.Expected = data.Detail.Card.TicketID, data.Detail.Card.Revision
	data.DepsForm.Dependencies = dependenciesText(data.Detail.Record["dependencies"])

	data.PrioritizeForm = newTicketForm("prioritize")
	data.PrioritizeForm.TicketID, data.PrioritizeForm.Expected = data.Detail.Card.TicketID, data.Detail.Card.Revision
	data.PrioritizeForm.Order = stringOf(data.Detail.Record["order"])
	data.PrioritizeForm.Priority = data.Detail.Card.Priority

	lookup := SpecLookup{Root: s.options.specsRoot()}
	for _, id := range data.Detail.Requirements {
		data.Clauses = append(data.Clauses, lookup.Resolve(id))
	}
	if path := r.URL.Query().Get("path"); path != "" {
		data.CodePath = path
		data.Blob = s.readBlob(ctx, data.Revision, path)
	}
	render(w, ticketView, data)
}

func (s *Server) handleSpecs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := s.newView(ctx, "Specs")
	specs, err := SpecLookup{Root: s.options.specsRoot()}.specs()
	if err != nil {
		data.SpecErr = err.Error()
	} else {
		data.Specs = specs
	}
	render(w, specView, data)
}

func (s *Server) handleCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := s.newView(ctx, "Code")
	data.CodePath = r.URL.Query().Get("path")
	data.Pin = stalePin(r.URL.Query().Get("at"), data.Revision)
	if data.CodePath != "" && data.Pin == "" {
		data.Blob = s.readBlob(ctx, data.Revision, data.CodePath)
	}
	if data.Blob != nil && data.Blob.Err == "" {
		worktree := Worktree{Root: s.options.specsRoot(), Timeout: s.options.Timeout}
		data.Backlinks = worktree.CodeBacklinks(ctx, SpecLookup{Root: s.options.specsRoot()}, data.Revision.Commit, data.Blob.Path)
	}
	render(w, codeView, data)
}

// handleRequirement renders one requirement's clause and the code its owning
// spec's Traceability table cites at the page's commit (LAC-V0-030).
func (s *Server) handleRequirement(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "no requirement id", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	data := s.newView(ctx, "Requirement")
	data.Pin = stalePin(r.URL.Query().Get("at"), data.Revision)
	if data.Pin == "" {
		worktree := Worktree{Root: s.options.specsRoot(), Timeout: s.options.Timeout}
		data.Links = worktree.RequirementLinks(ctx, SpecLookup{Root: s.options.specsRoot()}, data.Revision.Commit, id)
	}
	render(w, requirementView, data)
}

// stalePin reports a link pinned to a commit other than the one this page
// reads at. Such a page renders nothing for the link rather than content from
// a commit the link did not name.
func stalePin(at string, revision Revision) string {
	if at == "" || at == revision.Commit {
		return ""
	}
	return "This link is pinned to commit " + at + ", but the repository now reads at commit " +
		strconv.Quote(revision.Commit) + ". Nothing is rendered for it; reopen it from a page compiled at the current commit."
}

// handleEvidence renders the Corvint evidence snapshot. It is the pane whose
// source states all six axes for every value, which is what makes it the
// counterpart to a board where most axes are NOT_STATED (LAC-V0-007).
func (s *Server) handleEvidence(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := s.newView(ctx, "Evidence")
	data.Evidence = Dashboard{
		Binary: s.snapshotBinary(), Root: s.options.specsRoot(), Timeout: s.options.Timeout,
	}.Read(ctx)
	render(w, evidenceView, data)
}

// handleDogfood renders the last dogfood run of this repository, including
// every step that did not produce and the reason it gave.
func (s *Server) handleDogfood(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := s.newView(ctx, "Dogfood")
	data.Dogfood = ReadDogfood(s.options.specsRoot())
	render(w, dogfoodView, data)
}

// handleChain renders the chain of one sealed change: hunk, cited evidence,
// governing requirement, recorded verification (LAC-V0-033). Only a change the
// listing of .corvint/changes at this commit named is read, so the pane cannot
// be turned into a reader for an arbitrary path or object.
func (s *Server) handleChain(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := s.newView(ctx, "Chain")
	data.Pin = stalePin(r.URL.Query().Get("at"), data.Revision)
	if data.Pin != "" || data.Revision.Commit == "" {
		render(w, chainView, data)
		return
	}
	worktree := Worktree{Root: s.options.specsRoot(), Timeout: s.options.Timeout}
	data.Listing = worktree.List(ctx, data.Revision.Commit, changesDir)
	data.Changes = ChangeIDs(data.Listing)
	change := r.URL.Query().Get("change")
	if change == "" || !data.Listing.names(changesDir+"/"+change+".cem.json") || !objectIDPattern.MatchString(change) {
		render(w, chainView, data)
		return
	}
	data.Chain = worktree.ReadChain(ctx, data.Revision.Commit, change)
	if hunk := r.URL.Query().Get("hunk"); hunk != "" && data.Chain.Err == "" {
		data.Chain.Detail = worktree.HunkDetail(ctx, data.Chain, hunk)
	}
	render(w, chainView, data)
}

// handleBenchmarks lists the committed benchmark results.
func (s *Server) handleBenchmarks(w http.ResponseWriter, r *http.Request) {
	s.serveListing(w, r, "Benchmarks", "/benchmarks", benchmarkResultsDir,
		"Every benchmark result committed at this revision. A result file states no evidence class "+
			"of its own, so its axes are those of the read: content at an immutable Git object id, "+
			"observed by Git. What a run measured is the file's content, not a claim this page makes.")
}

// handleBacklogs lists the agent-memory backlogs. Their entries are written by
// agents and are untrusted input: they render as inert text (LAC-V0-020).
func (s *Server) handleBacklogs(w http.ResponseWriter, r *http.Request) {
	s.serveListing(w, r, "Backlogs", "/backlogs", agentMemoryDir,
		"The cross-session backlogs of pending work. Every entry is agent-written untrusted text and "+
			"renders inert: no markup, no script, no link is activated from it. A backlog is a list of "+
			"work still to do, so nothing here is evidence that anything passed.")
}

// serveListing renders one committed directory and, when a path is given, that
// file's committed bytes.
func (s *Server) serveListing(w http.ResponseWriter, r *http.Request, title, route, dir, note string) {
	ctx := r.Context()
	data := s.newView(ctx, title)
	data.ListingTitle = title
	data.ListingRoute = route
	data.ListingNote = note
	at := data.Revision.Commit
	if at == "" {
		at = "HEAD"
	}
	worktree := Worktree{Root: s.options.specsRoot(), Timeout: s.options.Timeout}
	data.Listing = worktree.List(ctx, at, dir)
	// Only a path this listing named is readable here, so the pane cannot be
	// turned into a reader for an arbitrary path in the repository.
	if path := r.URL.Query().Get("path"); path != "" && data.Listing.names(path) {
		data.Blob = s.readBlob(ctx, data.Revision, path)
	}
	render(w, listingView, data)
}

// snapshotBinary is the snapshot compiler to invoke.
func (s *Server) snapshotBinary() string {
	if s.options.Snapshot != "" {
		return s.options.Snapshot
	}
	return "corvint-dashboard-snapshot"
}

// readBlob reads one path at the commit the page was compiled at, so the
// bytes shown are the bytes that revision names.
func (s *Server) readBlob(ctx context.Context, revision Revision, path string) *Blob {
	worktree := Worktree{Root: s.options.specsRoot(), Timeout: s.options.Timeout}
	at := revision.Commit
	if at == "" {
		at = "HEAD"
	}
	return worktree.Read(ctx, at, strings.TrimPrefix(path, "/"))
}

func (s *Server) handleMutate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "mutations are POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "unreadable form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	request := MutationRequest{
		Verb:      r.PostFormValue("verb"),
		TicketID:  r.PostFormValue("ticket"),
		Expected:  r.PostFormValue("expected"),
		Payload:   r.PostFormValue("payload"),
		RequestID: r.PostFormValue("requestId"),
		IssuedAt:  r.PostFormValue("issuedAt"),
	}
	form := readTicketForm(r)
	structuredForm := request.Verb == "create" || request.Verb == "refine" || request.Verb == "set-dependencies" || request.Verb == "prioritize"
	assignForm := func(data *view) {
		switch request.Verb {
		case "set-dependencies":
			data.DepsForm = form
		case "prioritize":
			data.PrioritizeForm = form
		default:
			data.Form = form
		}
	}
	if structuredForm {
		if err := form.validate(); err != nil {
			data := &view{Title: "Change", Repo: s.options.Repo, Token: s.token, Result: &MutationResult{Request: request, Err: err.Error()}}
			assignForm(data)
			render(w, mutationView, data)
			return
		}
	}
	tool := s.taskman(ctx)
	caps := tool.ReadCapabilities(ctx)
	data := s.newView(ctx, "Change")
	data.Caps = caps
	assignForm(data)
	// A verb the tool does not implement is never invoked, even if a form
	// asking for it arrives (LAC-V0-013).
	if !caps.Implements("ticket " + request.Verb) {
		data.Result = &MutationResult{Request: request, Err: caps.Reason("ticket " + request.Verb)}
		render(w, mutationView, data)
		return
	}
	if structuredForm {
		payload, err := tool.formPayload(ctx, form)
		if err != nil {
			data.Result = &MutationResult{Request: request, Err: err.Error()}
			render(w, mutationView, data)
			return
		}
		request.Payload = payload
	}
	data.Result = tool.Mutate(ctx, request)
	if data.Result.Envelope != nil && !data.Result.Envelope.Refused() {
		data.Form, data.DepsForm, data.PrioritizeForm = nil, nil, nil
	}
	if request.Verb == "create" && data.Result.Envelope != nil && !data.Result.Envelope.Refused() && len(data.Result.Envelope.Items) > 0 {
		data.Result.Request.TicketID = stringOf(data.Result.Envelope.Items[0]["ticketId"])
	}
	render(w, mutationView, data)
}

// render writes one page. It buffers first so a template failure cannot leave
// a half-written page that looks like a complete answer.
func render(w http.ResponseWriter, view *template.Template, data any) {
	var buffer bytes.Buffer
	if err := view.Execute(&buffer, data); err != nil {
		http.Error(w, "the console could not render this page: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// The page loads nothing from anywhere: no script, no external asset.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(buffer.Bytes())
}
