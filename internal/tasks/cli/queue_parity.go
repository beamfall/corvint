package cli

import (
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Store parity for lane hosts (docs/specs/corvint-tasks-cli-v0.md,
// TCL-V0-742; V1-3931): a lane host reads its own store copy or a queue
// snapshot, which can fall behind the owning store. `queue parity --against
// FILE` compares the store this host reads with an authoritative `queue
// snapshot` file by intent file digest and names the tickets the host lacks,
// holds at different content, or holds that the authority does not.

// maxParityIDs bounds each ticket list in the item; the counts stay whole.
const maxParityIDs = 200

func queueParity(env Env, args []string) *wire.Result {
	cmd := []string{"queue", "parity"}
	values, _, bad := listFlags(cmd, args, []string{"against"}, nil, nil)
	if bad != nil {
		return bad
	}
	against := values["against"]
	if len(against) != 1 || against[0] == "" {
		return usage(cmd, "--against FILE names the authoritative queue snapshot; export it on the owning host with `corvint-tasks queue snapshot --out FILE`")
	}
	path := commandPath(env.Cwd, against[0])
	auth, authTree, err := readSnapshotBundle(path)
	if err != nil {
		return failure(cmd, nil, err)
	}
	want := map[string]string{}
	for _, f := range authTree.Files {
		want[f.Path] = string(f.Sha256)
	}
	var missing, stale, extra, other []string
	var hostTree string
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		missing, stale, extra, other = nil, nil, nil, nil
		hostTree = string(rc.store.Tree.Sha256)
		seen := map[string]bool{}
		for _, f := range rc.store.Tree.Files {
			seen[f.Path] = true
			sum, ok := want[f.Path]
			switch {
			case ok && sum == string(f.Sha256):
			case !isTicketFile(f.Path):
				other = append(other, f.Path)
			case ok:
				stale = append(stale, ticketOfFile(f.Path))
			default:
				extra = append(extra, ticketOfFile(f.Path))
			}
		}
		for _, f := range authTree.Files {
			if seen[f.Path] {
				continue
			}
			if isTicketFile(f.Path) {
				missing = append(missing, ticketOfFile(f.Path))
			} else {
				other = append(other, f.Path)
			}
		}
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	host := wire.NewObject().Set("treeSha256", wire.String(hostTree))
	if rc.bundle != nil {
		host.Set("source", wire.String("snapshot "+rc.bundlePath)).Set("producedAt", wire.String(rc.bundle.ProducedAt)).
			Set("age", wire.String(snapshotAge(rc.bundle.ProducedAt, time.Now()))).Set("journalSeq", snapshotSeqValue(rc.bundle.Journal))
	} else {
		host.Set("source", wire.String("worktree "+rc.repo.PrimaryWorktree))
	}
	authObj := wire.NewObject().Set("path", wire.String(path)).Set("sourceHost", wire.String(auth.SourceHost)).
		Set("primaryWorktree", wire.String(auth.PrimaryWorktree)).Set("treeSha256", wire.String(auth.TreeSha256)).
		Set("producedAt", wire.String(auth.ProducedAt)).Set("age", wire.String(snapshotAge(auth.ProducedAt, time.Now()))).
		Set("journalSeq", snapshotSeqValue(auth.Journal))
	inSync := hostTree == auth.TreeSha256
	o := wire.NewObject().Set("inSync", wire.Bool(inSync)).Set("host", wire.ObjectValue(host)).Set("authoritative", wire.ObjectValue(authObj))
	for _, l := range []struct {
		key string
		ids []string
	}{{"missing", missing}, {"stale", stale}, {"extra", extra}, {"otherFiles", other}} {
		o.Set(l.key+"Count", count(len(l.ids)))
		o.Set(l.key, wire.Strings(l.ids[:min(len(l.ids), maxParityIDs)]))
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{wire.ObjectValue(o)}
	if !inSync {
		res.Outcome = outcomeFor(wire.CodeStaleTicket)
		res.Codes = []string{wire.CodeStaleTicket}
		res.Warnings = append(res.Warnings, "this host's store (tree "+hostTree+") is not the authoritative snapshot (tree "+auth.TreeSha256+
			", produced "+auth.ProducedAt+" on "+auth.SourceHost+"): "+strings.Join([]string{
			strconv.Itoa(len(missing)) + " tickets missing", strconv.Itoa(len(stale)) + " stale", strconv.Itoa(len(extra)) + " extra", strconv.Itoa(len(other)) + " other files differ"}, ", ")+
			"; read tickets from the authoritative snapshot with "+SnapshotEnv+"="+path+" (TCL-V0-742)")
	}
	return res
}

func isTicketFile(p string) bool {
	return strings.HasPrefix(p, "tickets/") && strings.HasSuffix(p, ".json")
}

func ticketOfFile(p string) string {
	return strings.TrimSuffix(strings.TrimPrefix(p, "tickets/"), ".json")
}

// snapshotAge is the time since producedAt as whole minutes, or NOT_OBSERVED
// when producedAt does not parse (TCL-V0-742).
func snapshotAge(producedAt string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, producedAt)
	if err != nil {
		return "NOT_OBSERVED"
	}
	return now.Sub(t).Truncate(time.Minute).String()
}
