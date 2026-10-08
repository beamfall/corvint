package store

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// workerAttemptUnadmitted reports a WORKER KNOWHOW_ADD that the committed
// intent tree's policy does not opt in to (KHN-V0-021). It runs before the
// lock, the store checks and §5.2 recovery, so a disabled grant is refused
// exactly where ActorAdmitted refused it before the key existed, with nothing
// read from the journal or written. An unreadable policy counts as disabled.
// An opted-in tree is checked again under the lock against the canonical
// policy record.
func workerAttemptUnadmitted(repo *intent.Repository, r transaction.Request) bool {
	if !transaction.WorkerAttemptMutation(r) {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(repo.IntentRoot(), intent.Dir, intent.PolicyFile))
	if err != nil {
		return true
	}
	p, err := intent.DecodePolicy(raw)
	return err != nil || !transaction.WorkerAttemptAdmitted(r, p)
}

// Know-how freshness states (KHN-V0-005). They are computed at read time
// against the reader's committed HEAD and never stored.
const (
	KnowHowCurrent = "CURRENT"
	KnowHowStale   = "STALE"
	KnowHowUnknown = "UNKNOWN"
)

// KnowHowDeliveryMaxBytes caps the canonical encoding of the notes a claim
// delivers (KHN-V0-006).
const KnowHowDeliveryMaxBytes = 2048

// KnowHowNote is one active ADD entry of a home ticket with its read-time
// freshness: Anchors holds one state per entry anchor, in anchor order, and
// Freshness the note state. The text is agent-authored, untrusted data.
//
// RepositoryHead is the HEAD commit of the root a repository note's alias
// was mapped to, or "" when the alias was not mapped or that HEAD is
// unavailable (KHN-V0-027); a note without a repository leaves it "".
type KnowHowNote struct {
	TicketID       string
	Entry          ticket.KnowHowEntry
	Anchors        []string
	Freshness      string
	RepositoryHead string
}

// KnowHowUnresolved prefixes the detail of a pin refused because an anchor
// does not resolve at the writer's commit: its path is not a file there, or
// its symbol is not exactly one declaration the index's extractor names in a
// blob of at most 1 MiB (KHN-V0-016, KHN-V0-019).
const KnowHowUnresolved = "KNOWHOW_UNRESOLVED"

// KnowHowPinAnchors pins each anchor at rev with one `git cat-file
// --batch-check` and, when any anchor names a symbol, one `git cat-file
// --batch` over the blobs those anchors name: Blob is set for every anchor and
// SymbolSha256 for every symbol anchor (KHN-V0-001, KHN-V0-016). It returns
// rev's commit. An anchor that does not resolve is refused MALFORMED with the
// KnowHowUnresolved prefix; a note is never pinned to something the writer's
// commit does not hold. The anchors keep their order.
func KnowHowPinAnchors(root, rev string, anchors []ticket.KnowHowAnchor) (string, []ticket.KnowHowAnchor, error) {
	var paths []string
	index := map[string]int{}
	for _, a := range anchors {
		if _, ok := index[a.Path]; !ok {
			index[a.Path] = len(paths)
			paths = append(paths, a.Path)
		}
	}
	commit, objs, err := catFileAtCommit(root, rev, paths)
	if err != nil {
		return "", nil, err
	}
	if commit == "" {
		return "", nil, wire.Errorf(wire.CodeMalformed, "/payload/commit", "%s does not name a commit in this checkout", rev)
	}
	var symbolBlobs []string
	for _, a := range anchors {
		o := objs[index[a.Path]]
		if o.kind != "blob" {
			return "", nil, wire.Errorf(wire.CodeMalformed, "/payload/anchors", "%s: anchor %q is not a file in commit %s", KnowHowUnresolved, a.Path, commit)
		}
		if a.Symbol != "" {
			symbolBlobs = append(symbolBlobs, o.oid)
		}
	}
	contents, err := readKnowHowBlobs(root, symbolBlobs)
	if err != nil {
		return "", nil, err
	}
	tables := map[string]knowHowSymbols{}
	out := make([]ticket.KnowHowAnchor, len(anchors))
	for i, a := range anchors {
		oid := objs[index[a.Path]].oid
		out[i] = ticket.KnowHowAnchor{Path: a.Path, Blob: oid, Symbol: a.Symbol}
		if a.Symbol == "" {
			continue
		}
		data, ok := contents[oid]
		if !ok {
			return "", nil, wire.Errorf(wire.CodeMalformed, "/payload/anchors", "%s: anchor %q is larger than 1 MiB in commit %s", KnowHowUnresolved, a.Path, commit)
		}
		t, ok := tables[a.Path]
		if !ok {
			t = knowHowSymbolTable(a.Path, data)
			tables[a.Path] = t
		}
		digest, why := t.digest(a.Symbol)
		if why != "" {
			return "", nil, wire.Errorf(wire.CodeMalformed, "/payload/anchors", "%s: %s in commit %s", KnowHowUnresolved, why, commit)
		}
		out[i].SymbolSha256 = digest
	}
	return commit, out, nil
}

// SelectKnowHow returns the active notes of every ticket in inv, or of home
// alone when home is set, whose anchors intersect paths (all notes when paths
// is empty). A path ending in "/" matches every anchor below it; any other
// path matches one anchor exactly (KHN-V0-006). Each note carries its
// effective pins: those of its latest RECONFIRM, else its own (KHN-V0-018).
func SelectKnowHow(inv *ticket.Inventory, paths []string, home string) []KnowHowNote {
	var out []KnowHowNote
	for _, id := range inv.IDs() {
		if home != "" && id != home {
			continue
		}
		rec, ok := inv.Get(id)
		if !ok || len(rec.KnowHow) == 0 {
			continue
		}
		for _, k := range ticket.EffectiveKnowHow(rec.KnowHow) {
			if len(paths) == 0 || knowHowIntersects(k.Anchors, paths) {
				out = append(out, KnowHowNote{TicketID: id, Entry: k})
			}
		}
	}
	return out
}

func knowHowIntersects(anchors []ticket.KnowHowAnchor, paths []string) bool {
	for _, a := range anchors {
		for _, t := range paths {
			if a.Path == t || (strings.HasSuffix(t, "/") && strings.HasPrefix(a.Path, t)) {
				return true
			}
		}
	}
	return false
}

// ResolveKnowHowFreshness sets every note's anchor and note freshness from
// root's committed HEAD with one batched Git call and returns that HEAD
// commit, or "" when it is unavailable (KHN-V0-005). An anchor whose blob
// equals its pin is CURRENT and one that differs is STALE; a missing path, a
// non-blob or any Git failure is UNKNOWN, never CURRENT. A note is STALE when
// any anchor is, else UNKNOWN when any anchor is, else CURRENT. It writes
// nothing: the Git environment disables optional locks.
//
// A symbol anchor (KHN-V0-017) whose file blob equals its pin is CURRENT
// without reading content. Otherwise one more `git cat-file --batch` reads
// the changed blobs, each at most once: the anchor is CURRENT when its
// declaration's digest equals the pin, STALE when it differs, and UNKNOWN
// when the declaration no longer resolves to exactly one (deleted, renamed,
// duplicated), the file is no longer admitted by an extractor or is larger
// than 1 MiB, or the read fails.
func ResolveKnowHowFreshness(root string, notes []KnowHowNote) string {
	return ResolveKnowHowRepositories(root, nil, notes)
}

// ResolveKnowHowRepositories is ResolveKnowHowFreshness for notes that may
// name a repository (KHN-V0-027). A note without a repository resolves
// against root exactly as before, and root's HEAD is returned. A repository
// note resolves, per alias with one batched Git call, against the root repos
// maps that alias to, at that root's HEAD, asking each anchor path with the
// "<alias>/" prefix removed; its RepositoryHead is that HEAD. An alias repos
// does not map, or a root whose HEAD is unavailable, leaves every anchor of
// its notes UNKNOWN, never CURRENT: a repository note is never resolved
// against root.
func ResolveKnowHowRepositories(root string, repos map[string]string, notes []KnowHowNote) string {
	groups := map[string][]int{}
	var aliases []string
	var local []KnowHowNote
	var localAt []int
	for i, n := range notes {
		alias := n.Entry.Repository
		if alias == "" {
			local = append(local, n)
			localAt = append(localAt, i)
			continue
		}
		if _, ok := groups[alias]; !ok {
			aliases = append(aliases, alias)
		}
		groups[alias] = append(groups[alias], i)
	}
	head := resolveKnowHowAt(root, local)
	for j, i := range localAt {
		notes[i] = local[j]
	}
	sort.Strings(aliases)
	for _, alias := range aliases {
		group := make([]KnowHowNote, len(groups[alias]))
		for j, i := range groups[alias] {
			group[j] = notes[i]
			anchors := make([]ticket.KnowHowAnchor, len(notes[i].Entry.Anchors))
			for a, anchor := range notes[i].Entry.Anchors {
				anchor.Path = strings.TrimPrefix(anchor.Path, alias+"/")
				anchors[a] = anchor
			}
			group[j].Entry.Anchors = anchors
		}
		repoHead := resolveKnowHowAt(repos[alias], group)
		for j, i := range groups[alias] {
			notes[i].Anchors = group[j].Anchors
			notes[i].Freshness = group[j].Freshness
			notes[i].RepositoryHead = repoHead
		}
	}
	return head
}

func resolveKnowHowAt(root string, notes []KnowHowNote) string {
	index := map[string]int{}
	var paths []string
	for _, n := range notes {
		for _, a := range n.Entry.Anchors {
			if _, ok := index[a.Path]; !ok {
				index[a.Path] = len(paths)
				paths = append(paths, a.Path)
			}
		}
	}
	head, objs, err := catFileAtCommit(root, "HEAD", paths)
	if err != nil {
		head = ""
	}
	symbols := knowHowChangedSymbols(root, head, notes, objs, index)
	for i := range notes {
		n := &notes[i]
		n.Anchors = make([]string, len(n.Entry.Anchors))
		stale, unknown := false, false
		for j, a := range n.Entry.Anchors {
			state := KnowHowUnknown
			if head != "" {
				if o := objs[index[a.Path]]; o.kind == "blob" {
					switch {
					case o.oid == a.Blob:
						state = KnowHowCurrent
					case a.Symbol == "":
						state = KnowHowStale
					default:
						if t, ok := symbols[a.Path]; ok {
							if d, why := t.digest(a.Symbol); why == "" {
								state = KnowHowStale
								if d == a.SymbolSha256 {
									state = KnowHowCurrent
								}
							}
						}
					}
				}
			}
			n.Anchors[j] = state
			stale = stale || state == KnowHowStale
			unknown = unknown || state == KnowHowUnknown
		}
		n.Freshness = KnowHowCurrent
		if stale {
			n.Freshness = KnowHowStale
		} else if unknown {
			n.Freshness = KnowHowUnknown
		}
	}
	return head
}

// SortKnowHow orders notes CURRENT, then UNKNOWN, then STALE, newest first
// within a state, then by ticket id and note seq for a deterministic order.
func SortKnowHow(notes []KnowHowNote) {
	rank := map[string]int{KnowHowCurrent: 0, KnowHowUnknown: 1, KnowHowStale: 2}
	sort.SliceStable(notes, func(i, j int) bool {
		a, b := notes[i], notes[j]
		if rank[a.Freshness] != rank[b.Freshness] {
			return rank[a.Freshness] < rank[b.Freshness]
		}
		if a.Entry.RecordedAt != b.Entry.RecordedAt {
			return a.Entry.RecordedAt > b.Entry.RecordedAt
		}
		if a.TicketID != b.TicketID {
			return a.TicketID < b.TicketID
		}
		return a.Entry.Seq.Int() > b.Entry.Seq.Int()
	})
}

// KnowHowNoteValue encodes one note. The compact form a claim delivers keeps
// the text, anchor paths with their states, routes and time; the full form
// adds the pins, provenance and supersession a list reader audits. A symbol
// anchor adds its symbol, and the full form its digest pin (KHN-V0-016); a
// re-confirmed note's full form adds the latest RECONFIRM's provenance and
// shows its pins (KHN-V0-018). A note without either encodes as before.
func KnowHowNoteValue(n KnowHowNote, compact bool) wire.Value {
	k := n.Entry
	anchors := make([]wire.Value, 0, len(k.Anchors))
	for j, a := range k.Anchors {
		o := wire.NewObject().Set("path", wire.String(a.Path)).Set("freshness", wire.String(n.Anchors[j]))
		if a.Symbol != "" {
			o.Set("symbol", wire.String(a.Symbol))
		}
		if !compact {
			o.Set("blob", wire.String(a.Blob))
			if a.Symbol != "" {
				o.Set("symbolSha256", wire.String(a.SymbolSha256))
			}
		}
		anchors = append(anchors, wire.ObjectValue(o))
	}
	o := wire.NewObject().Set("ticketId", wire.String(n.TicketID)).Set("note", wire.String(string(k.Seq)))
	o.Set("freshness", wire.String(n.Freshness)).Set("text", wire.String(k.Text))
	o.Set("anchors", wire.Array(anchors...)).Set("routes", wire.Strings(append([]string{}, k.Routes...)))
	o.Set("recordedAt", wire.String(string(k.RecordedAt)))
	if k.Repository != "" {
		r := wire.NewObject().Set("alias", wire.String(k.Repository)).Set("head", stringOrNullValue(n.RepositoryHead))
		o.Set("repository", wire.ObjectValue(r))
	}
	if !compact {
		o.Set("commit", wire.String(k.Commit))
		o.Set("supersedes", countOrNullValue(k.Supersedes)).Set("reason", strOrNull(k.Reason))
		o.Set("attempt", strOrNull(k.Attempt)).Set("evidencePath", strOrNull(k.EvidencePath))
		gen := wire.Null()
		if k.Generation != nil {
			gen = wire.String(string(*k.Generation))
		}
		o.Set("generation", gen)
		o.Set("actor", wire.ObjectValue(wire.NewObject().Set("id", wire.String(k.ActorID)).Set("role", wire.String(k.ActorRole))))
		if r := k.Reconfirmed; r != nil {
			c := wire.NewObject().Set("seq", wire.String(string(r.Seq)))
			c.Set("actor", wire.ObjectValue(wire.NewObject().Set("id", wire.String(r.ActorID)).Set("role", wire.String(r.ActorRole))))
			c.Set("recordedAt", wire.String(string(r.RecordedAt))).Set("attempt", strOrNull(r.Attempt))
			gen := wire.Null()
			if r.Generation != nil {
				gen = wire.String(string(*r.Generation))
			}
			o.Set("reconfirmed", wire.ObjectValue(c.Set("generation", gen)))
		}
	}
	return wire.ObjectValue(o)
}

func countOrNullValue(c *wire.Count) wire.Value {
	if c == nil {
		return wire.Null()
	}
	return wire.String(string(*c))
}

func stringOrNullValue(s string) wire.Value {
	if s == "" {
		return wire.Null()
	}
	return wire.String(s)
}

func strOrNull(s *string) wire.Value {
	if s == nil {
		return wire.Null()
	}
	return wire.String(*s)
}

// ProjectKnowHow keeps the longest prefix of the ordered notes for which
// envelope(omitted) plus the canonical encoding of the notes array fits
// maxBytes, and returns it with the number left out (KHN-V0-006). envelope
// is the size the enclosing member adds around the array for a given omitted
// count; nil means a bare array. Every prefix is a candidate, so the order the
// reader sees is never reshuffled to fill the cap.
func ProjectKnowHow(notes []KnowHowNote, compact bool, maxBytes int, envelope func(omitted int) int) ([]wire.Value, int) {
	if envelope == nil {
		envelope = func(int) int { return 0 }
	}
	items := []wire.Value{}
	best := 0
	array := len("[]")
	for i, n := range notes {
		v := KnowHowNoteValue(n, compact)
		array += len(wire.Encode(v))
		if i > 0 {
			array++ // the separating comma
		}
		if array > maxBytes {
			break // no longer prefix fits either
		}
		items = append(items, v)
		if array+envelope(len(notes)-len(items)) <= maxBytes {
			best = len(items)
		}
	}
	return items[:best], len(notes) - best
}

// ClaimedKnowHow is what a claim or claim-next delivers from the know-how
// ledgers (KHN-V0-006): the active notes whose anchors intersect the claimed
// ticket's touchPaths, with freshness against the claimant's HEAD. It is
// computed when the response is built, never pinned at admission, so a replay
// shows current freshness. Err keeps an unreadable inventory visible; it never
// fails the committed claim.
//
// Warnings names each repository alias of the delivered notes that the claim
// did not map with --repo (KHN-V0-027).
type ClaimedKnowHow struct {
	Head     string
	Notes    []KnowHowNote
	Warnings []string
	Err      error
}

func claimKnowHow(repo *intent.Repository, root string, repos map[string]string, ticketID string) ClaimedKnowHow {
	st, err := intent.Load(repo.IntentRoot())
	if err != nil {
		return ClaimedKnowHow{Err: err}
	}
	rec, ok := st.Inventory.Get(ticketID)
	if !ok {
		return ClaimedKnowHow{Err: wire.Errorf(wire.CodeMissingEvidence, "claim", "claimed ticket %s is not in the inventory", ticketID)}
	}
	var c ClaimedKnowHow
	if len(rec.Effects.TouchPaths) == 0 {
		return c
	}
	c.Notes = SelectKnowHow(st.Inventory, rec.Effects.TouchPaths, "")
	if len(c.Notes) > 0 {
		c.Head = ResolveKnowHowRepositories(root, repos, c.Notes)
		c.Warnings = KnowHowRepositoryWarnings(repos, c.Notes)
		SortKnowHow(c.Notes)
	}
	return c
}

type catFileObject struct{ oid, kind string }

// catFileAtCommit answers rev's commit and the object each path names in
// that commit with one `git cat-file --batch-check` in root. It resolves
// rev^{commit} first and asks every path as <commit oid>:<path>, so a
// concurrent commit cannot mix two commits into one answer (KHN-V0-001,
// KHN-V0-005). The commit is "" when rev names no commit; a path Git cannot
// resolve has kind "", so callers read it as absent; a Git failure or a
// short answer is an error.
func catFileAtCommit(root, rev string, paths []string) (string, []catFileObject, error) {
	if root == "" {
		return "", nil, wire.Errorf(wire.CodeUnsupported, "git", "no checkout to observe")
	}
	inR, inW, err := os.Pipe()
	if err != nil {
		return "", nil, gitObservationFailed(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		_, _ = inR.Close(), inW.Close()
		return "", nil, gitObservationFailed(err)
	}
	defer outR.Close()
	c := exec.Command("git", "-c", "credential.helper=", "cat-file", "--batch-check=%(objectname) %(objecttype)")
	c.Dir = root
	c.Env = gitEnvironment()
	c.Stdin, c.Stdout = inR, outW
	err = c.Start()
	// The child holds its own copies; closing ours lets EOF reach both sides.
	_, _ = inR.Close(), outW.Close()
	if err != nil {
		_ = inW.Close()
		return "", nil, gitObservationFailed(err)
	}
	out := bufio.NewReader(outR)
	commit, objs, err := askAtCommit(inW, out, rev, paths)
	_ = inW.Close()
	_, _ = io.Copy(io.Discard, out)
	if werr := c.Wait(); err == nil && werr != nil {
		err = gitObservationFailed(werr)
	}
	if err != nil {
		return "", nil, err
	}
	return commit, objs, nil
}

func askAtCommit(stdin io.WriteCloser, out *bufio.Reader, rev string, paths []string) (string, []catFileObject, error) {
	if _, err := io.WriteString(stdin, rev+"^{commit}\n"); err != nil {
		return "", nil, gitObservationFailed(err)
	}
	first, err := readCatFileObject(out)
	if err != nil || first.kind != "commit" {
		return "", nil, err
	}
	// Git flushes each answer, so the paths are written from a goroutine
	// while the answers are read: neither pipe can fill and block the other.
	written := make(chan error, 1)
	go func() {
		var b strings.Builder
		for _, p := range paths {
			b.WriteString(first.oid + ":" + p + "\n")
		}
		_, err := io.WriteString(stdin, b.String())
		written <- errors.Join(err, stdin.Close())
	}()
	objs := make([]catFileObject, len(paths))
	for i := range paths {
		if objs[i], err = readCatFileObject(out); err != nil {
			_ = stdin.Close()
			<-written
			return "", nil, err
		}
	}
	if err := <-written; err != nil {
		return "", nil, gitObservationFailed(err)
	}
	return first.oid, objs, nil
}

// readCatFileObject reads one answer line. A line that is not "<oid> <type>"
// (for example "<name> missing") is an absent object, never an error.
func readCatFileObject(out *bufio.Reader) (catFileObject, error) {
	line, err := out.ReadString('\n')
	if err != nil {
		return catFileObject{}, wire.Errorf(wire.CodeUnsupported, "git", "git answered short: %v", err)
	}
	oid, kind, ok := strings.Cut(strings.TrimSuffix(line, "\n"), " ")
	if !ok || strings.Contains(kind, " ") {
		return catFileObject{}, nil
	}
	if _, err := wire.ParseOID("oid", oid); err != nil {
		return catFileObject{}, nil
	}
	return catFileObject{oid: oid, kind: kind}, nil
}

func gitObservationFailed(err error) error {
	return wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", err)
}

// FilesAtCommit reads paths at rev with the same two Git calls the know-how
// verifier uses, for the TOL-V0-012 obligation source binding. commit is ""
// when rev names no commit. present holds every path that is a blob there;
// content holds its bytes when the blob is small enough to read.
func FilesAtCommit(root, rev string, paths []string) (commit string, present map[string]bool, content map[string][]byte, err error) {
	commit, objs, err := catFileAtCommit(root, rev, paths)
	if err != nil || commit == "" {
		return commit, nil, nil, err
	}
	present, content = map[string]bool{}, map[string][]byte{}
	var oids []string
	for i, o := range objs {
		if o.kind == "blob" {
			present[paths[i]] = true
			oids = append(oids, o.oid)
		}
	}
	blobs, err := readKnowHowBlobs(root, oids)
	if err != nil {
		return "", nil, nil, err
	}
	for i, o := range objs {
		if b, ok := blobs[o.oid]; ok && o.kind == "blob" {
			content[paths[i]] = b
		}
	}
	return commit, present, content, nil
}

// KnowHowRepositoryArg parses one `--repo ALIAS=ROOT` value (KHN-V0-024,
// KHN-V0-027): ALIAS is a token of at most 64 bytes and ROOT, relative to
// cwd when not absolute, must be the top level of a Git work tree, so an
// anchor path is never resolved against a parent or nested checkout by
// accident. It returns the alias and the root with symbolic links resolved.
// Anything else is refused MALFORMED with the KNOWHOW_REPOSITORY prefix.
func KnowHowRepositoryArg(cwd, arg string) (string, string, error) {
	alias, root, ok := strings.Cut(arg, "=")
	if !ok || root == "" {
		return "", "", wire.Errorf(wire.CodeMalformed, "--repo", "%s: --repo takes ALIAS=ROOT", wire.KnowHowRepositoryDetail)
	}
	if _, err := wire.ParseKnowHowRepository("--repo", alias); err != nil {
		return "", "", wire.Errorf(wire.CodeMalformed, "--repo", "%s: repository alias must be a token of at most %d bytes", wire.KnowHowRepositoryDetail, wire.KnowHowMaxRepositoryBytes)
	}
	if !filepath.IsAbs(root) {
		root = filepath.Join(cwd, root)
	}
	refuse := func() (string, string, error) {
		return "", "", wire.Errorf(wire.CodeMalformed, "--repo", "%s: root for %s is not the top level of a Git work tree", wire.KnowHowRepositoryDetail, alias)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return refuse()
	}
	out, err := gitOutput(resolved, "rev-parse", "--show-toplevel")
	if err != nil {
		return refuse()
	}
	top, err := filepath.EvalSymlinks(strings.TrimRight(string(out), "\r\n"))
	if err != nil || top != resolved {
		return refuse()
	}
	return alias, resolved, nil
}

// KnowHowRepositoryArgs parses repeated `--repo` values into an alias map;
// an alias given twice is refused, since it would make the repository a
// note names ambiguous (KHN-V0-027).
func KnowHowRepositoryArgs(cwd string, args []string) (map[string]string, error) {
	if len(args) == 0 {
		return nil, nil
	}
	repos := map[string]string{}
	for _, arg := range args {
		alias, root, err := KnowHowRepositoryArg(cwd, arg)
		if err != nil {
			return nil, err
		}
		if _, dup := repos[alias]; dup {
			return nil, wire.Errorf(wire.CodeMalformed, "--repo", "%s: repository alias %s is given more than once", wire.KnowHowRepositoryDetail, alias)
		}
		repos[alias] = root
	}
	return repos, nil
}

// KnowHowRepositoryWarnings names each repository alias of notes that repos
// does not map, once, in alias order: its notes are UNKNOWN (KHN-V0-027).
func KnowHowRepositoryWarnings(repos map[string]string, notes []KnowHowNote) []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range notes {
		alias := n.Entry.Repository
		if _, mapped := repos[alias]; alias == "" || mapped || seen[alias] {
			continue
		}
		seen[alias] = true
		out = append(out, wire.KnowHowRepositoryDetail+": repository "+alias+" is not mapped with --repo; its notes are UNKNOWN")
	}
	sort.Strings(out)
	return out
}
