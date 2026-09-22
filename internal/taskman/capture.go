package taskman

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/worksource"
)

type limitedBuffer struct {
	raw      []byte
	limit    int
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if len(p) > b.limit-len(b.raw) {
		b.overflow = true
		p = p[:b.limit-len(b.raw)]
	}
	b.raw = append(b.raw, p...)
	return n, nil
}
func regular(path string, max int) ([]byte, error) {
	before, e := os.Lstat(path)
	if e != nil || !before.Mode().IsRegular() || before.Size() > int64(max) {
		return nil, errors.New("bounded regular input required")
	}
	f, e := openInput(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil || !os.SameFile(before, opened) {
		return nil, errors.New("input changed")
	}
	raw, e := io.ReadAll(io.LimitReader(f, int64(max)+1))
	if e != nil || len(raw) > max {
		return nil, errors.New("input bound")
	}
	after, e := os.Lstat(path)
	if e != nil || !os.SameFile(opened, after) {
		return nil, errors.New("input changed")
	}
	return raw, nil
}
func reply(ctx context.Context, executor, root string, args []string, snapshot *wire.Value) (wire.Value, error) {
	raw, e := runRead(ctx, executor, root, args)
	if e != nil {
		return wire.Value{}, e
	}
	v, e := document(raw, 16<<20)
	if e != nil {
		return v, e
	}
	if e = object(v, "profile command outcome codes snapshot mutation items page untrusted warnings"); e != nil {
		return v, e
	}
	if stringAt(v, "profile") != "taskman-command-result/0" || stringAt(v, "outcome") != "OK" || value(v, "mutation").Kind != wire.KindNull {
		return v, errors.New("native read refused")
	}
	command, e := stringsAt(v, "command", 2)
	if e != nil || len(command) != 2 || len(args) < 2 || command[0] != args[0] || command[1] != args[1] {
		return v, errors.New("native command mismatch")
	}
	s := value(v, "snapshot")
	if e = object(s, "headSeq headReceiptSha256 intentTreeSha256 primaryWorktreeSha256 pendingRedo barrier"); e != nil {
		return v, e
	}
	if _, e = number(value(s, "headSeq"), ^uint64(0)); e != nil {
		return v, e
	}
	for _, k := range []string{"headReceiptSha256", "intentTreeSha256", "primaryWorktreeSha256"} {
		if !digest(stringAt(s, k)) {
			return v, errors.New("incomplete native snapshot")
		}
	}
	if e = boolField(s, "pendingRedo"); e != nil || boolAt(s, "pendingRedo") {
		return v, errors.New("pending native redo")
	}
	if value(s, "barrier").Kind != wire.KindNull {
		if e = object(value(s, "barrier"), "scope reason"); e != nil {
			return v, e
		}
		if !oneOf(stringAt(value(s, "barrier"), "scope"), "ADMISSION ALL") {
			return v, errors.New("barrier scope")
		}
	}
	if snapshot.Kind == wire.KindNull {
		*snapshot = s
	} else if !bytes.Equal(canonical(*snapshot), canonical(s)) {
		return v, errors.New("native snapshot moved")
	}
	return v, nil
}
func audit(ctx context.Context, executor, root string, snapshot *wire.Value) error {
	v, e := reply(ctx, executor, root, []string{"receipt", "audit"}, snapshot)
	if e != nil {
		return e
	}
	items, e := array(value(v, "items"), 1)
	if e != nil || len(items) != 1 {
		return errors.New("audit item")
	}
	a := items[0]
	if e = object(a, "headSeq lastReceiptSha256 structuralConsistency projectionAgreement semanticCoverage historicalAcceptance actorAuthentication liveness runtimeQualification stagingPresent"); e != nil {
		return e
	}
	if stringAt(a, "headSeq") != stringAt(*snapshot, "headSeq") || stringAt(a, "lastReceiptSha256") != stringAt(*snapshot, "headReceiptSha256") {
		return errors.New("audit binding")
	}
	if stringAt(a, "structuralConsistency") != "CONSISTENT" || stringAt(a, "projectionAgreement") != "AGREES" || stringAt(a, "semanticCoverage") != "KNOWN_CODECS" {
		return fmt.Errorf("unqualified audit: %s/%s", stringAt(a, "structuralConsistency"), stringAt(a, "projectionAgreement"))
	}
	if e = boolField(a, "stagingPresent"); e != nil || boolAt(a, "stagingPresent") {
		return errors.New("active staging")
	}
	return nil
}
func captureTickets(ctx context.Context, executor, root string, c *captured) error {
	offset := uint64(0)
	total := -1
	aggregate := 0
	seen := map[string]bool{}
	for {
		args := []string{"ticket", "export", "--offset", count(offset), "--limit", "100"}
		v, e := reply(ctx, executor, root, args, &c.snapshot)
		if e != nil {
			return e
		}
		page := value(v, "page")
		if e = object(page, "offset limit total truncated"); e != nil {
			return e
		}
		off, e := number(value(page, "offset"), 2147483647)
		if e != nil || off != offset {
			return errors.New("export offset")
		}
		limit, e := number(value(page, "limit"), 2147483647)
		if e != nil || limit != 100 {
			return errors.New("export limit")
		}
		n, e := number(value(page, "total"), 1000)
		if e != nil {
			return e
		}
		if total >= 0 && total != int(n) {
			return errors.New("export total changed")
		}
		total = int(n)
		if e = boolField(page, "truncated"); e != nil {
			return e
		}
		items, e := array(value(v, "items"), 100)
		if e != nil {
			return e
		}
		for _, item := range items {
			if e = object(item, "ticketId path sha256 bytes record"); e != nil {
				return e
			}
			raw := append(canonical(value(item, "record")), '\n')
			aggregate += len(raw)
			if aggregate > 32<<20 {
				return errors.New("ticket aggregate limit")
			}
			size, e := number(value(item, "bytes"), 128<<10)
			if e != nil || size != uint64(len(raw)) || sum(raw) != stringAt(item, "sha256") {
				return errors.New("export digest or length")
			}
			t, e := decodeTicket(value(item, "record"))
			if e != nil {
				return e
			}
			if t.id != stringAt(item, "ticketId") || seen[strings.ToLower(t.id)] || !nativeID(t.id, "ticket", stringAt(c.queue, "queueId")) {
				return errors.New("export ticket identity")
			}
			local := t.id[strings.LastIndex(t.id, ":")+1:]
			if stringAt(item, "path") != "tickets/"+local+".json" {
				return errors.New("export path")
			}
			seen[strings.ToLower(t.id)] = true
			if len(c.tickets) > 0 && ticketLess(t, c.tickets[len(c.tickets)-1]) {
				return errors.New("export ticket order")
			}
			c.tickets = append(c.tickets, t)
			c.digests[t.id] = sum(raw)
		}
		offset += uint64(len(items))
		if offset > n {
			return errors.New("export over count")
		}
		if !boolAt(page, "truncated") {
			if offset != n {
				return errors.New("incomplete final page")
			}
			return nil
		}
		if len(items) == 0 || offset >= n {
			return errors.New("export made no progress")
		}
	}
}

// Preview reads a native fixture queue; it never accepts production admission responsibility.
func Preview(ctx context.Context, root, executor, observations string) ([]byte, error) {
	if !filepath.IsAbs(executor) {
		return nil, errors.New("absolute trusted-local executor required")
	}
	binary, e := regular(executor, 256<<20)
	if e != nil {
		return nil, e
	}
	binaryDigest := sum(binary)
	source, e := worksource.Acquire(ctx, root)
	if e != nil {
		return nil, e
	}
	defer source.Close()
	c := captured{digests: map[string]string{}}
	if e = audit(ctx, executor, source.Root, &c.snapshot); e != nil {
		return nil, e
	}
	q, e := reply(ctx, executor, source.Root, []string{"queue", "status"}, &c.snapshot)
	if e != nil {
		return nil, e
	}
	items, e := array(value(q, "items"), 1)
	if e != nil || len(items) != 1 {
		return nil, errors.New("queue status")
	}
	c.queue = items[0]
	if stringAt(c.snapshot, "primaryWorktreeSha256") != sum([]byte(source.Root)) {
		return nil, errors.New("primary fixture source required")
	}
	if e = object(c.queue, "queueId canonicalWriter fixture intentBranch executionCutover writeBarrier policySha256 tickets byStatus intentChecksPassed blocked headSeq generation barrier attempts publication"); e != nil {
		return nil, e
	}
	if !queueID(stringAt(c.queue, "queueId")) || value(c.queue, "fixture").Kind != wire.KindBool || !boolAt(c.queue, "fixture") || stringAt(c.queue, "canonicalWriter") != "NATIVE" || !digest(stringAt(c.queue, "policySha256")) || stringAt(c.queue, "headSeq") != stringAt(c.snapshot, "headSeq") {
		return nil, errors.New("native fixture queue required")
	}
	if !oneOf(stringAt(c.queue, "writeBarrier"), "NONE CUTOVER EMERGENCY") || !bytes.Equal(canonical(value(c.queue, "barrier")), canonical(value(c.snapshot, "barrier"))) {
		return nil, errors.New("queue barrier binding")
	}
	var policyRaw []byte
	for _, entry := range source.Entries {
		if entry.Path == ".taskman/policy.json" {
			policyRaw = entry.Raw
		}
	}
	if len(policyRaw) == 0 {
		return nil, errors.New("canonical policy must be source-pinned")
	}
	c.policy, e = document(policyRaw, 1<<20)
	if e != nil {
		return nil, e
	}
	if e = object(c.policy, "profile policyVersion roles capacity budgets retries retention gates serialFallback integrationRequiredKinds allowEmptyObligationsKinds reviewLane docsLane cemRequired ocmRequired runtimes environment"); e != nil {
		return nil, e
	}
	if e = object(value(c.policy, "capacity"), "maxActiveAttempts maxWorkersTotal classes"); e != nil {
		return nil, e
	}
	if _, e = array(value(value(c.policy, "capacity"), "classes"), 128); e != nil {
		return nil, e
	}
	if stringAt(c.policy, "profile") != "taskman-policy/0" || sum(append([]byte("policy\x00taskman-policy/0\x00"), canonical(c.policy)...)) != stringAt(c.queue, "policySha256") {
		return nil, errors.New("policy identity")
	}
	if !oneOf(stringAt(c.policy, "serialFallback"), "BLOCK WHOLE_REPOSITORY") {
		return nil, errors.New("fallback policy")
	}
	if e = captureTickets(ctx, executor, source.Root, &c); e != nil {
		return nil, e
	}
	expected, e := number(value(c.queue, "tickets"), 1000)
	if e != nil || int(expected) != len(c.tickets) {
		return nil, errors.New("queue count")
	}
	raw, e := regular(observations, 16<<20)
	if e != nil {
		return nil, e
	}
	c.observed, e = decodeObservations(raw, c, source.Identity.Commit, source.Identity.Tree)
	if e != nil {
		return nil, e
	}
	c.index, e = contextindex.BuildWithGitExecution(ctx, source.Root, source.GitPath, source.GitEnvironment)
	if e != nil {
		return nil, e
	}
	if c.index.CommitRevision != source.Identity.Commit || c.index.Revision != source.Identity.Tree {
		return nil, errors.New("index source drift")
	}
	if e = audit(ctx, executor, source.Root, &c.snapshot); e != nil {
		return nil, e
	}
	if e = source.VerifyMaterialization(ctx, source.Root); e != nil {
		return nil, e
	}
	end, e := worksource.Acquire(ctx, source.Root)
	if e != nil {
		return nil, e
	}
	defer end.Close()
	if end.Identity != source.Identity {
		return nil, errors.New("source changed")
	}
	check, e := regular(executor, 256<<20)
	if e != nil || sum(check) != binaryDigest {
		return nil, errors.New("executor changed")
	}
	p, e := plan(c)
	if e != nil {
		return nil, e
	}
	snap, _ := jsonValue(c.snapshot)
	sourceValue := map[string]string{"commit": source.Identity.Commit, "tree": source.Identity.Tree, "id": source.Identity.ID, "materializationSha256": source.Identity.MaterializationSHA256, "statusSha256": source.Identity.StatusSHA256, "objectFormat": source.Identity.ObjectFormat}
	return encode(map[string]any{"profile": "corvint-taskman-fixture-plan/0", "provenance": "FIXTURE_OBSERVATIONS_TRUSTED_LOCAL_EXECUTOR", "source": sourceValue, "executorSha256": binaryDigest, "snapshot": snap, "observationsSha256": sum(raw), "ticketDigests": c.digests, "plan": p, "unknowns": []string{"ACTOR_AUTHENTICATION_NOT_OBSERVED", "CONFIG_PIN_NOT_IMPLEMENTED", "GP_NOT_RUN", "LIVE_RUNTIME_NOT_OBSERVED", "FIXTURE_HISTORY_ONLY", "FIXTURE_RESERVATIONS_ONLY"}})
}
func jsonValue(v wire.Value) (any, error) {
	raw := canonical(v)
	var a any
	e := json.Unmarshal(raw, &a)
	return a, e
}
