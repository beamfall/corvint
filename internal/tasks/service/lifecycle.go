package service

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Public lifecycle result profiles.
const (
	InstallResultName   = "taskman-user-service-install/0"
	UninstallResultName = "taskman-user-service-uninstall/0"
	StopResultName      = "taskman-user-service-stop/0"
	ResumeResultName    = "taskman-user-service-resume/0"
)

const (
	maxServiceProfile = 64 * wire.KiB
	pulseStale        = 30
)

// InstallRequest is `service install`.
type InstallRequest struct {
	Program, Config, RequestID string
	Replace                    bool
}

func (h Host) begin(program, request string) (string, func(), error) {
	if !platformSupported {
		return "", nil, wire.Errorf(wire.CodeUnsupported, "/platform", "the user service supports only darwin launchd and linux systemd --user")
	}
	if _, _, err := h.managerDomain(); err != nil {
		return "", nil, err
	}
	if _, err := mutation.ParseRequestID("/requestId", request); err != nil {
		return "", nil, err
	}
	root, err := h.StateRoot(program)
	if err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
		return "", nil, err
	}
	if err := h.privateDir(root, true); err != nil {
		return "", nil, err
	}
	if err := h.privateDir(filepath.Join(root, operationDir), true); err != nil {
		return "", nil, err
	}
	unlock, err := h.lock(root)
	if err != nil {
		return "", nil, err
	}
	return root, unlock, nil
}

// uninstallReserve keeps journal headroom only an uninstall may use, so a
// full history never prevents removing the service.
const uninstallReserve = 8

// journal finds this request's record and refuses conflicts, other
// unfinished operations and a history of limit records before any effect.
// An unjournaled NO_CHANGE install is found in the request ledger.
func (h Host) journal(root, request string, sum wire.Digest, limit int) (*Operation, error) {
	ops, err := h.operations(root)
	if err != nil {
		return nil, err
	}
	for _, o := range ops {
		if o.RequestID == request {
			if o.RequestSha256 != sum {
				return nil, wire.Errorf(wire.CodeRequestIDConflict, "/requestId", "request id was already used for a different service operation")
			}
			return o, nil
		}
	}
	rs, err := h.controlRequests(root, nil)
	if err != nil {
		return nil, err
	}
	if prior, ok := lookupRequest(rs, request); ok && prior != sum {
		return nil, wire.Errorf(wire.CodeRequestIDConflict, "/requestId", "request id was already used for a different service request")
	}
	if err := h.pendingResumeConflict(root, request, sum); err != nil {
		return nil, err
	}
	for _, o := range ops {
		if !o.finished() {
			return nil, wire.Errorf(wire.CodeResourceCollision, "/operations", "operation %s is unfinished (%s); rerun it with its own request id first", o.RequestID, o.Phase)
		}
	}
	if len(ops) >= limit {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/operations", "operation history holds %d records; nothing was changed", len(ops))
	}
	return nil, nil
}

func requestSum(parts ...string) wire.Digest {
	var b bytes.Buffer
	for _, p := range parts {
		b.WriteString(p)
		b.WriteByte('\n')
	}
	return wire.Sum(b.Bytes())
}

func (h Host) readProfile(path string) (*Profile, []byte, error) {
	if err := servicePath(path); err != nil {
		return nil, nil, wire.Errorf(wire.CodeMalformed, "/config", "service config must be an absolute normalized path")
	}
	raw, err := h.readPrivate(path, maxServiceProfile)
	if err != nil {
		return nil, nil, err
	}
	p, err := DecodeProfile(raw)
	if err != nil {
		return nil, nil, wire.Errorf(wire.CodeMalformed, "/config", "%v", err)
	}
	if len(p.Helpers) != 0 && !HelperProfileSupported(h.GOOS) {
		return nil, nil, wire.Errorf(wire.CodeUnsupported, "/config/helpers", "the helper profile is unsupported on %s: detached helper descendants cannot be proved retired there", h.GOOS)
	}
	return p, raw, nil
}

// facts observes everything BuildManifest binds.
func (h Host) facts(program, root string, p *Profile, gen wire.Size, prev *wire.Digest) (InstallationFacts, error) {
	manager, domain, err := h.managerDomain()
	if err != nil {
		return InstallationFacts{}, err
	}
	cfgFacts, cfgRaw, err := h.observeFile(p.DispatchConfig, dispatch.MaxConfig)
	if err != nil {
		return InstallationFacts{}, err
	}
	c, err := dispatch.DecodeConfig(cfgRaw)
	if err != nil {
		return InstallationFacts{}, wire.Errorf(wire.CodeMalformed, "/dispatchConfig", "%v", err)
	}
	exeFacts, _, err := h.observeFile(p.Executable, 0)
	if err != nil {
		return InstallationFacts{}, err
	}
	if h.QueueID == nil {
		return InstallationFacts{}, wire.Errorf(wire.CodeCapabilityUnavailable, "/store", "no store resolver is configured")
	}
	queue, err := h.QueueID(p.WorkRoot)
	if err != nil {
		return InstallationFacts{}, err
	}
	unitRoot, err := h.unitRoot(manager)
	if err != nil {
		return InstallationFacts{}, err
	}
	if err := h.unitDir(unitRoot, true); err != nil {
		return InstallationFacts{}, err
	}
	helpers := map[string]FileFacts{}
	for _, x := range p.Helpers {
		facts, _, err := h.observeFile(x.Argv[0], 0)
		if err != nil {
			return InstallationFacts{}, err
		}
		helpers[x.ID] = facts
	}
	if !h.reachable(&Manifest{Manager: manager, Domain: domain}) {
		return InstallationFacts{}, wire.Errorf(wire.CodeCapabilityUnavailable, "/manager", "the %s user manager for %s is not reachable", manager, domain)
	}
	return InstallationFacts{State: "VERIFIED", Manager: manager, Domain: domain, Program: program, QueueID: queue, ConfigRoot: filepath.Dir(p.DispatchConfig), StateRoot: root, UnitRoot: unitRoot, ManifestPath: filepath.Join(root, manifestFile), DispatchStateRoot: c.StateDir, CanonicalStore: p.WorkRoot, UID: wire.SizeOf(uint64(h.UID)), Generation: gen, Previous: prev, Executable: exeFacts, DispatchConfig: cfgFacts, RootsSafe: true, ManagerReachable: true, ConfigWorkRoot: c.WorkRoot, ConfigStateRoot: c.StateDir, HelperExecutables: helpers}, nil
}

func build(p *Profile, f InstallationFacts) (*Manifest, []byte, error) {
	m, err := BuildManifest(*p, f)
	if err != nil {
		return nil, nil, wire.Errorf(wire.CodeResourceCollision, "/manifest", "%v", err)
	}
	raw, err := m.Encode()
	return m, raw, err
}

func (h Host) currentManifest(root string) (*Manifest, []byte, error) {
	m, raw, err := h.readManifest(root)
	if absent(err) {
		return nil, nil, nil
	}
	return m, raw, err
}

func (h Host) operationFacts(cur *Manifest, next *Manifest, replace bool) (OperationFacts, error) {
	f := OperationFacts{State: "VERIFIED", Current: cur, Replace: replace, ManagerReachable: h.reachable(next), Units: []ExistingUnit{}}
	if cur != nil {
		for _, u := range cur.Units {
			e := h.existing(cur, u)
			if e.Registration == regUnknown {
				f.State = "UNKNOWN"
			}
			f.Units = append(f.Units, e)
		}
	}
	// A new label or path that already exists is foreign: refuse it rather
	// than adopt or overwrite it.
	if next != cur {
		known := map[string]bool{}
		if cur != nil {
			for _, u := range cur.Units {
				known[u.Label] = true
			}
		}
		for _, u := range next.Units {
			if known[u.Label] {
				continue
			}
			e := h.existing(next, u)
			if e.Registration != regAbsent || e.Ownership != "ABSENT" || !e.NoDropIns {
				return f, wire.Errorf(wire.CodeResourceCollision, u.Path, "unit %s already exists or is registered; it is not adopted", u.Label)
			}
		}
	}
	if !f.ManagerReachable {
		return f, wire.Errorf(wire.CodeCapabilityUnavailable, "/manager", "the %s user manager for %s is not reachable", next.Manager, next.Domain)
	}
	if f.State != "VERIFIED" {
		return f, wire.Errorf(wire.CodeUncertainEffect, "/manager", "a current unit registration is UNKNOWN; nothing was changed")
	}
	return f, nil
}

func plain(actions []Action) []Action {
	out := make([]Action, len(actions))
	for i, a := range actions {
		out[i] = Action{Kind: a.Kind, Label: a.Label, Path: a.Path}
	}
	return out
}

// Install binds the profile, plans against observed ownership, journals the
// original request before any effect and executes or reconciles it.
func (h Host) Install(req InstallRequest) (*wire.Object, error) {
	// The profile is validated before the registry is created.
	p, raw, err := h.readProfile(req.Config)
	if err != nil {
		return nil, err
	}
	root, unlock, err := h.begin(req.Program, req.RequestID)
	if err != nil {
		return nil, err
	}
	defer unlock()
	sum := requestSum("INSTALL", req.Program, string(wire.Sum(raw)), strconv.FormatBool(req.Replace))
	op, err := h.journal(root, req.RequestID, sum, maxOperations-uninstallReserve)
	if err != nil {
		return nil, err
	}
	if op != nil {
		return h.continueOperation(root, op, op.finished())
	}
	cur, curRaw, err := h.currentManifest(root)
	if err != nil {
		return nil, err
	}
	gen, prev := wire.SizeOf(1), (*wire.Digest)(nil)
	if cur != nil {
		gen, prev = cur.Generation, cur.Previous
	}
	f, err := h.facts(req.Program, root, p, gen, prev)
	if err != nil {
		return nil, err
	}
	next, nextRaw, err := build(p, f)
	if err != nil {
		return nil, err
	}
	if cur != nil && !bytes.Equal(nextRaw, curRaw) {
		if !req.Replace {
			return nil, wire.Errorf(wire.CodeResourceCollision, "/manifest", "an installation with different bound facts exists; pass --replace to replace generation %s", cur.Generation)
		}
		d := wire.Sum(curRaw)
		f.Generation, f.Previous = wire.SizeOf(cur.Generation.Uint64()+1), &d
		if next, _, err = build(p, f); err != nil {
			return nil, err
		}
	}
	of, err := h.operationFacts(cur, next, req.Replace)
	if err != nil {
		return nil, err
	}
	actions, err := PlanOperation("INSTALL", *next, of)
	if err != nil {
		return nil, wire.Errorf(wire.CodeResourceCollision, "/plan", "%v", err)
	}
	op = &Operation{Kind: "INSTALL", RequestID: req.RequestID, RequestSha256: sum, Program: req.Program, Phase: "INSTALLING", Previous: cur, Next: next, Actions: plain(actions), Published: []string{}}
	if len(actions) == 1 && actions[0].Kind == "NO_CHANGE_PRESERVE_CONTROL_DEBT_WORKERS" {
		// A NO_CHANGE install has no effect to reconcile, so it takes no
		// journal slot; the request ledger keeps its id bound.
		op.Phase, op.Previous, op.Completed = "NO_CHANGE", nil, 1
		rs, err := h.controlRequests(root, nil)
		if err != nil {
			return nil, err
		}
		if _, err := h.rememberRequest(root, rs, req.RequestID, sum); err != nil {
			return nil, err
		}
		return h.operationResult(root, op, false), nil
	}
	if c, err := h.readControl(root); err == nil {
		op.PriorDesired = c.Desired
	} else if !absent(err) {
		return nil, err
	}
	if err := h.saveOperation(root, op); err != nil {
		return nil, err
	}
	return h.continueOperation(root, op, false)
}

// Uninstall removes exactly the current owned installation after saving
// STOPPED, keeping control, journal, dispatcher and worker state.
func (h Host) Uninstall(program, request string) (*wire.Object, error) {
	root, unlock, err := h.begin(program, request)
	if err != nil {
		return nil, err
	}
	defer unlock()
	cur, curRaw, err := h.currentManifest(root)
	if err != nil {
		return nil, err
	}
	ident := ""
	if cur != nil {
		ident = string(wire.Sum(curRaw))
	}
	ops, err := h.operations(root)
	if err != nil {
		return nil, err
	}
	// A replayed uninstall is found by request id even after the manifest
	// is gone; its hash binds the manifest it removed.
	for _, o := range ops {
		if o.RequestID == request && o.Kind == "UNINSTALL" && o.Previous != nil {
			if pr, err := o.Previous.Encode(); err == nil && ident == "" {
				ident = string(wire.Sum(pr))
			}
		}
	}
	if ident == "" {
		return nil, wire.Errorf(wire.CodeResourceCollision, "/manifest", "no service is installed for program %s", program)
	}
	sum := requestSum("UNINSTALL", program, ident)
	op, err := h.journal(root, request, sum, maxOperations)
	if err != nil {
		return nil, err
	}
	if op != nil {
		return h.continueOperation(root, op, op.finished())
	}
	if cur == nil {
		return nil, wire.Errorf(wire.CodeResourceCollision, "/manifest", "no service is installed for program %s", program)
	}
	of, err := h.operationFacts(cur, cur, false)
	if err != nil {
		return nil, err
	}
	actions, err := PlanOperation("UNINSTALL", *cur, of)
	if err != nil {
		return nil, wire.Errorf(wire.CodeResourceCollision, "/plan", "%v", err)
	}
	op = &Operation{Kind: "UNINSTALL", RequestID: request, RequestSha256: sum, Program: program, Phase: "REMOVING", Previous: cur, Actions: plain(actions), Published: []string{}}
	if c, err := h.readControl(root); err == nil {
		op.PriorDesired = c.Desired
	} else if !absent(err) {
		return nil, err
	}
	if err := h.saveOperation(root, op); err != nil {
		return nil, err
	}
	return h.continueOperation(root, op, false)
}

// continueOperation replays a finished record without effects, or executes
// the remaining journaled steps. Each step rechecks before acting, so a
// step interrupted after its effect is idempotent on continuation.
func (h Host) continueOperation(root string, op *Operation, replay bool) (*wire.Object, error) {
	if !op.finished() {
		var err error
		switch op.Phase {
		case "ROLLBACK_REQUIRED":
			err = h.rollback(root, op, nil)
		default:
			err = h.execute(root, op)
		}
		if err != nil {
			return nil, err
		}
	}
	return h.operationResult(root, op, replay), nil
}

func (h Host) execute(root string, op *Operation) error {
	for op.Completed < len(op.Actions) {
		if err := h.step(root, op, op.Actions[op.Completed]); err != nil {
			if op.Kind == "INSTALL" {
				return h.rollback(root, op, err)
			}
			return wire.Errorf(wire.CodeUncertainEffect, "/operation", "uninstall stopped at %s and stays REMOVING; rerun with the same request id: %v", op.Actions[op.Completed].Kind, err)
		}
		op.Completed++
		if err := h.saveOperation(root, op); err != nil {
			return err
		}
	}
	if op.Kind == "INSTALL" {
		if err := h.bindControl(root, op); err != nil {
			return err
		}
		op.Phase = "INSTALLED"
	} else {
		op.Phase = "REMOVED"
	}
	return h.saveOperation(root, op)
}

func (op *Operation) unit(label, path string) (*Manifest, Unit, bool) {
	for _, m := range []*Manifest{op.Next, op.Previous} {
		if m == nil {
			continue
		}
		for _, u := range m.Units {
			if (label != "" && u.Label == label) || (path != "" && u.Path == path) {
				return m, u, true
			}
		}
	}
	return nil, Unit{}, false
}

func (h Host) step(root string, op *Operation, a Action) error {
	switch a.Kind {
	case "RECHECK_OWNER_STAGE_FSYNC_RENAME":
		m, u, ok := op.unit("", a.Path)
		if !ok {
			return wire.Errorf(wire.CodeMalformed, a.Path, "unit is not in the operation")
		}
		if err := h.publishUnit(m, u); err != nil {
			return err
		}
		for _, l := range op.Published {
			if l == u.Label {
				return nil
			}
		}
		op.Published = append(op.Published, u.Label)
		return nil
	case "REGISTER", "UNREGISTER":
		m, u, ok := op.unit(a.Label, "")
		if !ok {
			return wire.Errorf(wire.CodeMalformed, a.Label, "label is not in the operation")
		}
		remove := a.Kind == "UNREGISTER"
		want := regPresent
		if remove {
			want = regAbsent
		}
		if h.query(m, u) == want {
			return nil
		}
		return h.effect(m, u, remove)
	case "VERIFY_OWNED_REGISTRATION", "VERIFY_MANAGER_ABSENCE":
		m, u, ok := op.unit(a.Label, "")
		if !ok {
			return wire.Errorf(wire.CodeMalformed, a.Label, "label is not in the operation")
		}
		want := regPresent
		if a.Kind == "VERIFY_MANAGER_ABSENCE" {
			want = regAbsent
		}
		if !h.await(m, u, want) {
			return wire.Errorf(wire.CodeUncertainEffect, a.Label, "registration of %s was not observed %s", u.Label, want)
		}
		return nil
	case "REMOVE_IF_UNCHANGED":
		if m := op.manifestAt(a.Path); m != nil {
			raw, err := m.Encode()
			if err != nil {
				return err
			}
			return h.removeExact(a.Path, wire.Sum(raw))
		}
		// A replacement may keep a unit path; either generation's exact
		// bytes are owned by this registry.
		sums := []wire.Digest{}
		for _, m := range []*Manifest{op.Next, op.Previous} {
			if m == nil {
				continue
			}
			for _, u := range m.Units {
				if u.Path == a.Path {
					sums = append(sums, wire.Sum(u.Raw))
				}
			}
		}
		if len(sums) == 0 {
			return wire.Errorf(wire.CodeMalformed, a.Path, "path is not in the operation")
		}
		return h.removeExact(a.Path, sums...)
	case "RESTORE_SNAPSHOTTED_OWNED_BYTES":
		if op.Previous == nil {
			return wire.Errorf(wire.CodeMalformed, a.Path, "no prior installation to restore")
		}
		for _, u := range op.Previous.Units {
			if u.Path == a.Path {
				return h.publishUnit(op.Previous, u)
			}
		}
		return wire.Errorf(wire.CodeMalformed, a.Path, "path is not a prior unit")
	case "COMMIT_MANIFEST_AFTER_READBACK":
		raw, err := op.Next.Encode()
		if err != nil {
			return err
		}
		if err := writeAtomic(root, manifestFile, raw); err != nil {
			return err
		}
		back, err := h.readPrivate(filepath.Join(root, manifestFile), maxManifest)
		if err != nil {
			return err
		}
		if !bytes.Equal(back, raw) {
			return wire.Errorf(wire.CodeUncertainEffect, "/manifest", "manifest readback differs")
		}
		return nil
	case "SAVE_STOPPED_UNDER_FENCE":
		return h.suppressFor(root, op)
	}
	// Journal/retention markers: the record itself is the evidence.
	return nil
}

func (op *Operation) manifestAt(path string) *Manifest {
	for _, m := range []*Manifest{op.Next, op.Previous} {
		if m != nil && m.Path == path {
			return m
		}
	}
	return nil
}

// publishUnit writes u when absent, accepts an identical owned file and
// refuses anything else.
func (h Host) publishUnit(m *Manifest, u Unit) error {
	switch h.unitFile(u) {
	case fileOwned:
		return nil
	case fileAbsent:
		if err := h.unitDir(m.UnitRoot, false); err != nil {
			return err
		}
		return writeAtomic(filepath.Dir(u.Path), filepath.Base(u.Path), u.Raw)
	}
	return wire.Errorf(wire.CodeResourceCollision, u.Path, "unit file exists with other bytes or ownership; it is left in place")
}

// suppressFor saves STOPPED for the operation's current installation and
// waits a bounded time for its controller to report it is not running.
func (h Host) suppressFor(root string, op *Operation) error {
	if err := h.suppressControl(root, op); err != nil {
		return err
	}
	// F is released before the wait.
	// Helper wrappers retire their trees on STOPPED; every helper tree must
	// be proved retired (its intent removed) before units change.
	for i := 0; i < 15; i++ {
		if st, _ := h.pulseState(root, ""); st != "RUNNING" && h.helperTreesSettled(root) {
			return nil
		}
		h.sleep(time.Second)
	}
	if !h.helperTreesSettled(root) {
		return wire.Errorf(wire.CodeUncertainEffect, "/helpers", "a helper tree is not proved retired after STOPPED was saved")
	}
	return wire.Errorf(wire.CodeUncertainEffect, "/control", "the controller still reports RUNNING after STOPPED was saved")
}

// suppressControl is suppressFor's control write under F.
func (h Host) suppressControl(root string, op *Operation) error {
	unlock, err := h.fence(root)
	if err != nil {
		return err
	}
	defer unlock()
	c, err := h.readControl(root)
	if absent(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if c.Desired != "STOPPED" {
		next := *c
		next.Revision = wire.CountOf(c.Revision.Int() + 1)
		next.Desired = "STOPPED"
		next.LastRequest = op.RequestID
		next.LastRequestSha256 = requestSum("SUPPRESS", op.RequestID, string(c.ManifestIdentity))
		return h.writeControl(root, next)
	}
	return nil
}

// bindControl binds control to the committed manifest identity. A new
// control starts RUNNING; an existing one keeps its desired state (a
// replacement restores the state it suppressed).
func (h Host) bindControl(root string, op *Operation) error {
	raw, err := op.Next.Encode()
	if err != nil {
		return err
	}
	ident := wire.Sum(raw)
	unlock, err := h.fence(root)
	if err != nil {
		return err
	}
	defer unlock()
	c, err := h.readControl(root)
	if absent(err) {
		return h.writeControl(root, Control{Program: op.Program, ManifestIdentity: ident, Revision: wire.CountOf(1), Desired: "RUNNING"})
	}
	if err != nil {
		return err
	}
	if c.ManifestIdentity == ident && (op.PriorDesired == "" || op.Previous == nil || c.Desired == op.PriorDesired) {
		return nil
	}
	next := *c
	next.ManifestIdentity = ident
	next.Revision = wire.CountOf(c.Revision.Int() + 1)
	if op.Previous != nil && op.PriorDesired != "" {
		next.Desired = op.PriorDesired
	}
	return h.writeControl(root, next)
}

// rollback cleans only the operation-published owned subset and restores
// the exact prior installation; uncertainty stays ROLLBACK_REQUIRED.
func (h Host) rollback(root string, op *Operation, cause error) error {
	op.Phase = "ROLLBACK_REQUIRED"
	if err := h.saveOperation(root, op); err != nil {
		return err
	}
	f := OperationFacts{State: "VERIFIED", Current: op.Previous, ManagerReachable: h.reachable(op.Next), RestoreSafe: true, PublishedNewLabels: append([]string(nil), op.Published...)}
	for _, l := range op.Published {
		_, u, _ := op.unit(l, "")
		e := h.existing(op.Next, u)
		if e.Ownership == "ABSENT" {
			// Already removed by an interrupted rollback; ownership is
			// proved by the record and absence by observation.
			e.Ownership, e.Sha256 = "OWNED", wire.Sum(u.Raw)
		}
		f.Units = append(f.Units, e)
	}
	if op.Previous != nil {
		for _, u := range op.Previous.Units {
			if st := h.unitFile(u); st != fileOwned && st != fileAbsent {
				f.RestoreSafe = false
			}
		}
	}
	held := func(err error) error {
		if cause != nil {
			return wire.Errorf(wire.CodeUncertainEffect, "/operation", "install failed (%v) and rollback stays ROLLBACK_REQUIRED: %v", cause, err)
		}
		return wire.Errorf(wire.CodeUncertainEffect, "/operation", "rollback stays ROLLBACK_REQUIRED: %v", err)
	}
	actions, err := PlanRollback(*op.Next, f)
	if err != nil {
		return held(err)
	}
	for _, a := range actions {
		if err := h.step(root, op, a); err != nil {
			return held(err)
		}
	}
	if err := h.restoreManifest(root, op); err != nil {
		return held(err)
	}
	if err := h.restoreDesired(root, op.PriorDesired); err != nil {
		return held(err)
	}
	op.Phase = "ROLLED_BACK"
	if err := h.saveOperation(root, op); err != nil {
		return err
	}
	return rolledBack(cause)
}

// restoreDesired republishes the desired state a rollback's operation
// suppressed, under F.
func (h Host) restoreDesired(root, prior string) error {
	unlock, err := h.fence(root)
	if err != nil {
		return err
	}
	defer unlock()
	if c, err := h.readControl(root); err == nil && prior != "" && c.Desired != prior {
		next := *c
		next.Revision = wire.CountOf(c.Revision.Int() + 1)
		next.Desired = prior
		return h.writeControl(root, next)
	}
	return nil
}

// rolledBack is the one answer for a ROLLED_BACK install: RESTORED, with
// the failure that caused the rollback kept as the diagnostic.
func rolledBack(cause error) error {
	if cause == nil {
		return wire.Errorf(wire.CodeRestored, "/operation", "install rolled back after a previous attempt of this request failed; the prior installation is restored")
	}
	return wire.Errorf(wire.CodeRestored, "/operation", "install rolled back; the prior installation is restored (cause %s: %v)", wire.CodeOf(cause), cause)
}

// restoreManifest undoes a manifest commit that renamed before its
// readback failed: the previous manifest is restored exactly, or a fresh
// install's manifest is removed. A manifest that is neither is foreign.
func (h Host) restoreManifest(root string, op *Operation) error {
	nextRaw, err := op.Next.Encode()
	if err != nil {
		return err
	}
	var prevRaw []byte
	if op.Previous != nil {
		if prevRaw, err = op.Previous.Encode(); err != nil {
			return err
		}
	}
	path := filepath.Join(root, manifestFile)
	cur, err := h.readPrivate(path, maxManifest)
	switch {
	case absent(err):
		if prevRaw == nil {
			return nil
		}
	case err != nil:
		return err
	case prevRaw != nil && bytes.Equal(cur, prevRaw):
		return nil
	case !bytes.Equal(cur, nextRaw):
		return wire.Errorf(wire.CodeResourceCollision, "/manifest", "manifest is neither this operation's previous nor next installation")
	case prevRaw == nil:
		return h.removeExact(path, wire.Sum(nextRaw))
	}
	if err := writeAtomic(root, manifestFile, prevRaw); err != nil {
		return err
	}
	back, err := h.readPrivate(path, maxManifest)
	if err != nil {
		return err
	}
	if !bytes.Equal(back, prevRaw) {
		return wire.Errorf(wire.CodeUncertainEffect, "/manifest", "restored manifest readback differs")
	}
	return nil
}

func (h Host) operationResult(root string, op *Operation, replay bool) *wire.Object {
	name := InstallResultName
	if op.Kind == "UNINSTALL" {
		name = UninstallResultName
	}
	o := wire.NewObject().Set("profile", wire.String(name)).Set("program", wire.String(op.Program)).Set("requestId", wire.String(op.RequestID)).Set("phase", wire.String(op.Phase)).Set("replayed", wire.Bool(replay)).Set("stateRoot", wire.String(root))
	m := op.Next
	if m == nil {
		m = op.Previous
	}
	if m != nil {
		labels := []string{}
		for _, u := range m.Units {
			labels = append(labels, u.Label)
		}
		o.Set("manager", wire.String(m.Manager)).Set("domain", wire.String(m.Domain)).Set("generation", wire.String(string(m.Generation))).Set("labels", wire.Strings(labels))
	}
	desired := wire.Null()
	if c, err := h.readControl(root); err == nil {
		desired = wire.String(c.Desired)
	}
	o.Set("desired", desired)
	o.Set("preserved", wire.Strings([]string{"control", "dispatcherState", "operationJournal", "workers"}))
	return o
}

// controlled loads the bound manifest and control for stop/resume.
func (h Host) controlled(root string) (*Manifest, wire.Digest, *Control, error) {
	m, raw, err := h.readManifest(root)
	if absent(err) {
		return nil, "", nil, wire.Errorf(wire.CodeResourceCollision, "/manifest", "no service is installed for this program")
	}
	if err != nil {
		return nil, "", nil, err
	}
	ident := wire.Sum(raw)
	c, err := h.readControl(root)
	if err != nil {
		return nil, "", nil, wire.Errorf(wire.CodeUncertainEffect, "/control", "control is UNKNOWN: %v", err)
	}
	if c.ManifestIdentity != ident || c.Program != m.Program {
		return nil, "", nil, wire.Errorf(wire.CodeUncertainEffect, "/control", "control is not bound to the installed manifest; HOLD")
	}
	return m, ident, c, nil
}

func dispatchDir(m *Manifest) string {
	return dispatch.ProgramDir(&dispatch.Config{StateDir: m.DispatchStateRoot}, m.Program)
}

// Stop saves STOPPED, or with drain DRAINING, under O and F. ACKNOWLEDGED
// means the suppression is durable and no launch can be admitted after it:
// the dispatcher owner is absent or is the fenced managed main, whose
// admitted launch completes its worker record before F is released.
// PENDING means an unfenced or unobservable owner. close is OBSERVED only
// when no dispatcher owner, no RUNNING pulse and no unretired helper tree
// remain after the save; a
// drain's close stays PENDING until the managed main settles it STOPPED.
// Workers are always preserved.
func (h Host) Stop(program, request string, drain bool) (*wire.Object, error) {
	root, unlock, err := h.begin(program, request)
	if err != nil {
		return nil, err
	}
	defer unlock()
	desired := "STOPPED"
	if drain {
		desired = "DRAINING"
	}
	m, ident, next, replay, acked, err := h.suppress(root, request, desired)
	if err != nil {
		return nil, err
	}
	// The owner and pulse are read after the suppression is durable.
	owner, fenced := h.fencedOwner(root, m, ident)
	state := "ACKNOWLEDGED"
	switch {
	case drain && next.Desired == "RUNNING", !drain && next.Desired != "STOPPED":
		state = "SUPERSEDED"
	case owner != "NOT_RUNNING" && !fenced, !acked:
		state = "PENDING"
	}
	closed := "PENDING"
	if pulse, _ := h.pulseState(root, ident); next.Desired == "STOPPED" && owner == "NOT_RUNNING" && pulse != "RUNNING" && h.helperTreesSettled(root) {
		closed = "OBSERVED"
	}
	return wire.NewObject().Set("profile", wire.String(StopResultName)).Set("program", wire.String(program)).Set("requestId", wire.String(request)).Set("drain", wire.Bool(drain)).Set("desired", wire.String(next.Desired)).Set("revision", wire.String(string(next.Revision))).Set("replayed", wire.Bool(replay)).Set("state", wire.String(state)).Set("close", wire.String(closed)).Set("workers", wire.String("PRESERVED")), nil
}

// suppress is Stop's checked control change under F: STOPPED from any
// state, DRAINING only from RUNNING. A replay changes nothing. acked
// reports that no launch intent, the dispatcher's or a helper's, was
// unresolved under the same F, so every admitted effect is durable
// (SUPPRESSION_ACKNOWLEDGED).
func (h Host) suppress(root, request, desired string) (m *Manifest, ident wire.Digest, next Control, replay, acked bool, err error) {
	unfence, err := h.fence(root)
	if err != nil {
		return nil, "", Control{}, false, false, err
	}
	defer unfence()
	m, ident, c, err := h.controlled(root)
	if err != nil {
		return nil, "", Control{}, false, false, err
	}
	intents := []string{}
	if st := h.intentState(root); st != "ABSENT" {
		intents = append(intents, st)
	}
	intents = append(intents, h.helperIntentStates(root)...)
	hash := wire.Sum([]byte(request + "\n" + desired + "\n" + string(ident)))
	rs, replay, err := h.controlReplay(root, c, request, hash)
	if err != nil || replay {
		return m, ident, *c, replay, launchesResolved(intents), err
	}
	if desired == "DRAINING" && c.Desired != "RUNNING" {
		return nil, "", Control{}, false, false, wire.Errorf(wire.CodeResourceCollision, "/control", "desired state is %s; drain applies only to RUNNING", c.Desired)
	}
	p, err := Suppress(*c, request, desired, ControlFacts{FenceHeld: true, Fresh: true, Published: true, ObservedRevision: c.Revision, IntentStates: intents})
	if err != nil {
		return nil, "", Control{}, false, false, wire.Errorf(wire.CodeResourceCollision, "/control", "%v", err)
	}
	if err := h.writeControl(root, p.Control); err != nil {
		return nil, "", Control{}, false, false, err
	}
	if _, err := h.rememberRequest(root, rs, request, hash); err != nil {
		return nil, "", Control{}, false, false, err
	}
	return m, ident, p.Control, false, p.State == "SUPPRESSION_ACKNOWLEDGED", nil
}

// launchesResolved reports whether every launch claim seen under F is a
// committed effect: none is unresolved or unknown.
func launchesResolved(states []string) bool {
	for _, s := range states {
		if s != "COMMITTED" && s != "PROVED_NO_EFFECT" {
			return false
		}
	}
	return true
}

// controlReplay finds request in the control request ledger: the same hash
// is a replay, any other binding (including a journaled install or
// uninstall) is a conflict.
func (h Host) controlReplay(root string, c *Control, request string, hash wire.Digest) ([]controlRequest, bool, error) {
	rs, err := h.controlRequests(root, c)
	if err != nil {
		return nil, false, err
	}
	if sum, ok := lookupRequest(rs, request); ok {
		if sum != hash {
			return nil, false, wire.Errorf(wire.CodeRequestIDConflict, "/requestId", "request id was already used for a different control change")
		}
		return rs, true, nil
	}
	if err := h.pendingResumeConflict(root, request, hash); err != nil {
		return nil, false, err
	}
	ops, err := h.operations(root)
	if err != nil {
		return nil, false, err
	}
	for _, o := range ops {
		if o.RequestID == request {
			return nil, false, wire.Errorf(wire.CodeRequestIDConflict, "/requestId", "request id was already used for a service operation")
		}
	}
	return rs, false, nil
}

// Resume publishes RUNNING after STOPPED once the dispatcher is proved not
// running, or over DRAINING (resume wins; the dispatcher keeps
// supervising), when the pinned executable and dispatch config are
// unchanged and any legacy stop file is ABSENT. Pins are observed before F;
// the change is a CAS under F against the control observed then. A replay
// reports the current control without changing it.
func (h Host) Resume(program, request string) (*wire.Object, error) {
	root, unlock, err := h.begin(program, request)
	if err != nil {
		return nil, err
	}
	defer unlock()
	m, ident, c, err := h.controlled(root)
	if err != nil {
		return nil, err
	}
	hash := wire.Sum([]byte(request + "\nRESUME\n" + string(ident)))
	rs, replay, err := h.controlReplay(root, c, request, hash)
	if err != nil {
		return nil, err
	}
	next, reset := *c, []string{}
	if !replay {
		switch c.Desired {
		case "STOPPED":
			if st := dispatch.OwnerState(dispatchDir(m)); st != "NOT_RUNNING" {
				return nil, wire.Errorf(wire.CodeResourceCollision, "/dispatcher", "dispatcher state is %s; resume waits until it is proved NOT_RUNNING", st)
			}
		case "DRAINING":
		default:
			return nil, wire.Errorf(wire.CodeResourceCollision, "/control", "desired state is %s; resume applies only to STOPPED or DRAINING", c.Desired)
		}
		if err := h.pinsValid(m); err != nil {
			return nil, err
		}
		if next, reset, err = h.publishResume(root, m, *c, rs, request, hash); err != nil {
			return nil, err
		}
	}
	return wire.NewObject().Set("profile", wire.String(ResumeResultName)).Set("program", wire.String(program)).Set("requestId", wire.String(request)).Set("desired", wire.String(next.Desired)).Set("revision", wire.String(string(next.Revision))).Set("replayed", wire.Bool(replay)).Set("restartDebt", wire.String("NOT_OBSERVED")).Set("helperDebtReset", wire.Strings(reset)), nil
}

// publishResume re-reads control under F, requires it unchanged since
// observed, a fresh ABSENT legacy stop file and every helper tree retired,
// resets helper restart debt, then publishes RUNNING.
// The request ledger rs is stable under O: the managed main never writes
// it.
func (h Host) publishResume(root string, m *Manifest, observed Control, rs []controlRequest, request string, hash wire.Digest) (Control, []string, error) {
	unfence, err := h.fence(root)
	if err != nil {
		return observed, nil, err
	}
	defer unfence()
	c, err := h.readControl(root)
	if err != nil {
		return observed, nil, wire.Errorf(wire.CodeUncertainEffect, "/control", "control is UNKNOWN: %v", err)
	}
	if *c != observed {
		return observed, nil, wire.Errorf(wire.CodeResourceCollision, "/control", "control changed while resume observed its pins; retry")
	}
	legacy, err := legacyPath(m)
	if err != nil {
		return observed, nil, err
	}
	if legacy != nil {
		switch h.legacyPresence(*legacy) {
		case "PRESENT":
			return observed, nil, wire.Errorf(wire.CodeResourceCollision, "/legacyStopFile", "legacy stop file %s is present; remove it before resume", *legacy)
		case "UNKNOWN":
			return observed, nil, wire.Errorf(wire.CodeUncertainEffect, "/legacyStopFile", "legacy stop file %s presence is UNKNOWN", *legacy)
		}
	}
	next, err := resumeAfter(*c, request, hash)
	if err != nil {
		return observed, nil, wire.Errorf(wire.CodeLimitExceeded, "/control", "%v", err)
	}
	// Resume requires every helper tree proved retired and resets helper
	// restart debt through its durable operation journal: a retry
	// reconciles a partial reset, and a superseded or changed request is
	// refused rather than erasing later debt.
	rs, reset, journal, err := h.resumeHelpers(root, m, *c, rs, request, hash)
	if err != nil {
		return observed, nil, err
	}
	if err := h.writeControl(root, next); err != nil {
		return observed, nil, err
	}
	if _, err := h.rememberRequest(root, rs, request, hash); err != nil {
		return observed, nil, err
	}
	if err := h.removeExact(filepath.Join(root, resumeOperationFile), journal); err != nil {
		return observed, nil, err
	}
	return next, reset, nil
}

// pinsValid requires the executable and dispatch config bytes bound by the
// manifest.
func (h Host) pinsValid(m *Manifest) error {
	p, err := DecodeProfile(m.ProfileRaw)
	if err != nil {
		return err
	}
	exe, _, err := h.observeFile(p.Executable, 0)
	if err != nil {
		return err
	}
	cfg, _, err := h.observeFile(p.DispatchConfig, dispatch.MaxConfig)
	if err != nil {
		return err
	}
	if exe.Sha256 != m.ExecutableSha256 || exe.Path != m.Executable || cfg.Sha256 != m.DispatchConfigSha256 {
		return wire.Errorf(wire.CodeResourceCollision, "/pins", "executable or dispatch config changed since install; install --replace first")
	}
	return nil
}

// pulseState reads the controller pulse: RUNNING/IDLE/HOLD when fresh and
// bound to ident (any identity when ident is empty) and its process still
// has the recorded identity; STALE, ABSENT or UNKNOWN otherwise.
func (h Host) pulseState(root string, ident wire.Digest) (string, *Pulse) {
	return h.pulseStateAt(root, pulseFile, ident)
}

// pulseStateAt is pulseState for the pulse file name (the main's or a
// helper wrapper's).
func (h Host) pulseStateAt(root, name string, ident wire.Digest) (string, *Pulse) {
	raw, err := h.readPrivate(filepath.Join(root, name), maxPulse)
	if absent(err) {
		return "ABSENT", nil
	}
	if err != nil {
		return "UNKNOWN", nil
	}
	p, err := DecodePulse(raw)
	if err != nil {
		return "UNKNOWN", nil
	}
	age := h.now().Unix() - p.At
	if (ident != "" && p.ManifestSha256 != ident) || age < 0 || age > pulseStale {
		return "STALE", &p
	}
	if live, err := supervisor.ProcessIdentity(p.PID); err != nil || live == "" || live != p.Identity {
		return "STALE", &p
	}
	return p.State, &p
}

// Status is a pure read: it creates, locks and writes nothing.
func (h Host) Status(program string) (*wire.Object, error) {
	root, err := h.StateRoot(program)
	if err != nil {
		return nil, err
	}
	o := wire.NewObject().Set("profile", wire.String(StatusName)).Set("program", wire.String(program)).Set("stateRoot", wire.String(root))
	m, raw, err := h.readManifest(root)
	switch {
	case absent(err):
		o.Set("installed", wire.String("ABSENT"))
	case err != nil:
		o.Set("installed", wire.String("UNKNOWN")).Set("diagnostic", wire.String(describe(err)))
	default:
		o.Set("installed", wire.String("PRESENT")).Set("manager", wire.String(m.Manager)).Set("domain", wire.String(m.Domain)).Set("generation", wire.String(string(m.Generation))).Set("manifestSha256", wire.String(string(wire.Sum(raw))))
		units := []wire.Value{}
		for _, u := range m.Units {
			units = append(units, wire.ObjectValue(wire.NewObject().Set("label", wire.String(u.Label)).Set("path", wire.String(u.Path)).Set("file", wire.String(h.unitFile(u))).Set("registration", wire.String(h.query(m, u)))))
		}
		o.Set("units", wire.Array(units...))
		o.Set("dispatcher", wire.String(dispatch.OwnerState(dispatchDir(m))))
		if legacy, err := legacyPath(m); err == nil && legacy != nil {
			o.Set("legacyStopFile", wire.String(h.legacyPresence(*legacy)))
		}
	}
	ident := wire.Digest("")
	if m != nil {
		ident = wire.Sum(raw)
	}
	switch c, err := h.readControl(root); {
	case absent(err):
		o.Set("desired", wire.String("ABSENT"))
	case err != nil:
		o.Set("desired", wire.String("UNKNOWN"))
	default:
		o.Set("desired", wire.String(c.Desired)).Set("revision", wire.String(string(c.Revision))).Set("controlBound", wire.Bool(m != nil && c.ManifestIdentity == ident))
	}
	state, p := h.pulseState(root, ident)
	pulse := wire.NewObject().Set("state", wire.String(state))
	if p != nil {
		pulse.Set("pid", wire.String(strconv.Itoa(p.PID))).Set("at", wire.String(strconv.FormatInt(p.At, 10))).Set("hold", optionalString(p.Hold))
	}
	o.Set("pulse", wire.ObjectValue(pulse))
	ops, err := h.operations(root)
	if err != nil {
		o.Set("operations", wire.String("UNKNOWN"))
	} else {
		unfinished := []string{}
		for _, op := range ops {
			if !op.finished() {
				unfinished = append(unfinished, op.RequestID+" "+op.Phase)
			}
		}
		sort.Strings(unfinished)
		o.Set("operations", wire.String(strconv.Itoa(len(ops)))).Set("unfinished", wire.Strings(unfinished))
	}
	o.Set("launchIntent", wire.String(h.intentState(root)))
	o.Set("logs", h.logStatus(root, "main", "stderr"))
	if ids, err := h.helperIntentIDs(root); err != nil {
		o.Set("helperTrees", wire.String("UNKNOWN"))
	} else {
		o.Set("helperTrees", wire.Strings(ids))
	}
	if m != nil {
		if p, err := DecodeProfile(m.ProfileRaw); err == nil {
			helpers := []wire.Value{}
			for _, x := range p.Helpers {
				helpers = append(helpers, wire.ObjectValue(h.helperStatus(root, program, x.ID, ident)))
			}
			o.Set("helpers", wire.Array(helpers...))
		} else {
			o.Set("helpers", wire.String("UNKNOWN"))
		}
	}
	o.Set("notObserved", wire.Strings([]string{"bootLoginScope", "completedTick", "restartDebt"}))
	return o, nil
}

// DispatchService is the additive namespaced object for `dispatch status`:
// present only when this program's installed manifest binds stateDir. It
// performs no manager query.
func (h Host) DispatchService(program, stateDir string) (*wire.Object, bool) {
	root, err := h.StateRoot(program)
	if err != nil {
		return nil, false
	}
	m, raw, err := h.readManifest(root)
	if err != nil || m.DispatchStateRoot != stateDir {
		return nil, false
	}
	ident := wire.Sum(raw)
	o := wire.NewObject().Set("installed", wire.Bool(true)).Set("generation", wire.String(string(m.Generation))).Set("manager", wire.String(m.Manager))
	desired := "UNKNOWN"
	if c, err := h.readControl(root); err == nil && c.ManifestIdentity == ident {
		desired = c.Desired
	}
	state, _ := h.pulseState(root, ident)
	o.Set("desired", wire.String(desired)).Set("controller", wire.String(state))
	return o, true
}
