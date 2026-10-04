package service

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"math"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

func xmlElement(e *xml.Encoder, name string, value any) error {
	return e.EncodeElement(value, xml.StartElement{Name: xml.Name{Local: name}})
}
func plist(m Manifest, label, helper string) ([]byte, error) {
	var b bytes.Buffer
	e := xml.NewEncoder(&b)
	b.WriteString(xml.Header)
	start := xml.StartElement{Name: xml.Name{Local: "plist"}, Attr: []xml.Attr{{Name: xml.Name{Local: "version"}, Value: "1.0"}}}
	if err := e.EncodeToken(start); err != nil {
		return nil, err
	}
	dict := xml.StartElement{Name: xml.Name{Local: "dict"}}
	e.EncodeToken(dict)
	args := unitArguments(m, helper)
	for _, kv := range [][2]string{{"Label", label}, {"WorkingDirectory", m.CanonicalStore}, {"StandardOutPath", "/dev/null"}, {"StandardErrorPath", "/dev/null"}} {
		xmlElement(e, "key", kv[0])
		xmlElement(e, "string", kv[1])
	}
	xmlElement(e, "key", "ProgramArguments")
	array := xml.StartElement{Name: xml.Name{Local: "array"}}
	e.EncodeToken(array)
	for _, a := range args {
		xmlElement(e, "string", a)
	}
	e.EncodeToken(array.End())
	for _, kv := range []struct {
		k string
		v bool
	}{{"KeepAlive", true}, {"RunAtLoad", true}, {"AbandonProcessGroup", helper == ""}} {
		xmlElement(e, "key", kv.k)
		tag := "false"
		if kv.v {
			tag = "true"
		}
		e.EncodeToken(xml.StartElement{Name: xml.Name{Local: tag}})
		e.EncodeToken(xml.EndElement{Name: xml.Name{Local: tag}})
	}
	for _, kv := range [][2]string{{"ThrottleInterval", "30"}, {"ExitTimeOut", "30"}, {"Umask", "63"}} {
		xmlElement(e, "key", kv[0])
		xmlElement(e, "integer", kv[1])
	}
	e.EncodeToken(dict.End())
	e.EncodeToken(start.End())
	if err := e.Flush(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func unitArguments(m Manifest, helper string) []string {
	verb := "run"
	if helper != "" {
		verb = "run-helper"
	}
	a := []string{m.Executable, "service", verb, "--program", m.Program, "--manifest", m.Path}
	if helper != "" {
		a = append(a, "--helper", helper)
	}
	return a
}

// systemdExecAtom quotes one ExecStart word. ExecStart expands both percent
// and dollar. Words are limited to printable ASCII, where strconv.Quote and
// systemd's C-style unquoting agree (only backslash and double quote are
// escaped); other bytes and semicolons are refused rather than re-encoded.
func systemdExecAtom(s string) (string, error) {
	if err := serviceText(s); err != nil {
		return "", err
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return "", fmt.Errorf("systemd ExecStart word must be printable ASCII")
		}
	}
	if strings.Contains(s, ";") {
		return "", fmt.Errorf("semicolon unsupported")
	}
	s = strings.ReplaceAll(s, "%", "%%")
	s = strings.ReplaceAll(s, "$", "$$")
	return strconv.Quote(s), nil
}

// systemdPathValue renders an unquoted path directive value. systemd does not
// unquote WorkingDirectory and expands percent only, so whitespace, quotes,
// backslashes and control characters are refused instead of escaped.
func systemdPathValue(s string) (string, error) {
	if err := servicePath(s); err != nil {
		return "", err
	}
	for _, c := range s {
		if unicode.IsSpace(c) || unicode.IsControl(c) || c == '"' || c == '\'' || c == '\\' {
			return "", fmt.Errorf("systemd path value must not contain whitespace, quotes, backslashes or control characters")
		}
	}
	return strings.ReplaceAll(s, "%", "%%"), nil
}
func systemdUnit(m Manifest, helper string) ([]byte, error) {
	args := unitArguments(m, helper)
	quoted := []string{}
	for _, a := range args {
		x, err := systemdExecAtom(a)
		if err != nil {
			return nil, err
		}
		quoted = append(quoted, x)
	}
	cwd, err := systemdPathValue(m.CanonicalStore)
	if err != nil {
		return nil, err
	}
	kill := "process"
	if helper != "" {
		kill = "control-group"
	}
	return []byte("[Unit]\nDescription=Corvint optional user service\nStartLimitIntervalSec=0\n\n[Service]\nType=simple\nExecStart=" + strings.Join(quoted, " ") + "\nWorkingDirectory=" + cwd + "\nRestart=always\nRestartSec=30s\nKillMode=" + kill + "\nKillSignal=SIGTERM\nTimeoutStopSec=30s\nSendSIGKILL=yes\nUMask=0077\nStandardOutput=null\nStandardError=null\n\n[Install]\nWantedBy=default.target\n"), nil
}

// RenderUnits returns bytes only. Helpers' arbitrary argv/env are deliberately
// not executable unit text; a future foreground wrapper consumes the manifest.
func RenderUnits(m Manifest, p Profile) ([]Unit, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if !serviceName.MatchString(m.Program) {
		return nil, fmt.Errorf("program")
	}
	if _, err := wire.ParseDigest("/namespace", string(m.Namespace)); err != nil {
		return nil, err
	}
	for _, path := range []string{m.Executable, m.Path, m.UnitRoot, m.CanonicalStore} {
		if err := servicePath(path); err != nil {
			return nil, err
		}
	}
	units := []Unit{}
	ids := []string{""}
	for _, h := range p.Helpers {
		ids = append(ids, h.ID)
	}
	for _, helper := range ids {
		label := "org.corvint." + string(m.Namespace[:16]) + "." + m.Program
		if helper != "" {
			label += ".helper-" + helper
		}
		var raw []byte
		var err error
		ext := ".plist"
		if m.Manager == "launchd" {
			raw, err = plist(m, label, helper)
		} else if m.Manager == "systemd-user" {
			raw, err = systemdUnit(m, helper)
			ext = ".service"
		} else {
			return nil, fmt.Errorf("unsupported manager")
		}
		if err != nil {
			return nil, err
		}
		if len(raw) > 64*wire.KiB {
			return nil, fmt.Errorf("unit bound")
		}
		units = append(units, Unit{Label: label, HelperID: helper, Path: filepath.Join(m.UnitRoot, label+ext), Raw: raw})
	}
	return units, nil
}

type ExistingUnit struct {
	Label, Path, Ownership, Registration string
	Sha256                               wire.Digest
	NoSymlink, NoDropIns                 bool
}
type OperationFacts struct {
	State                                  string
	Current                                *Manifest
	Units                                  []ExistingUnit
	ManagerReachable, Replace, RestoreSafe bool
	PublishedNewLabels                     []string
}
type Action struct {
	Kind, Path, Label string
	Argv              []string
}

func managerAction(m Manifest, u Unit, remove bool) Action {
	a := Action{Label: u.Label, Path: u.Path}
	if m.Manager == "launchd" {
		if remove {
			a.Kind = "UNREGISTER"
			a.Argv = []string{"launchctl", "bootout", m.Domain + "/" + u.Label}
		} else {
			a.Kind = "REGISTER"
			a.Argv = []string{"launchctl", "bootstrap", m.Domain, u.Path}
		}
	} else {
		a.Kind = "REGISTER"
		verb := "enable"
		if remove {
			a.Kind = "UNREGISTER"
			verb = "disable"
		}
		a.Argv = []string{"systemctl", "--user", verb, "--now", u.Label + ".service"}
	}
	return a
}
func ownedUnits(m Manifest, observed []ExistingUnit) error {
	if len(observed) != len(m.Units) {
		return fmt.Errorf("unit observation incomplete")
	}
	seen := map[string]bool{}
	for _, u := range m.Units {
		found := false
		for _, o := range observed {
			if o.Label == u.Label {
				if seen[o.Label] || o.Path != u.Path || o.Sha256 != wire.Sum(u.Raw) || o.Ownership != "OWNED" || !o.NoSymlink || !o.NoDropIns || (o.Registration != "PRESENT" && o.Registration != "ABSENT") {
					return fmt.Errorf("foreign/unknown unit")
				}
				found = true
				seen[o.Label] = true
			}
		}
		if !found {
			return fmt.Errorf("unit missing")
		}
	}
	return nil
}

// PlanOperation is a hypothetical bounded operation journal, never a manager call.
func PlanOperation(kind string, next Manifest, f OperationFacts) ([]Action, error) {
	if f.State != "VERIFIED" || !f.ManagerReachable {
		return nil, fmt.Errorf("manager/ownership unknown")
	}
	if _, err := next.Encode(); err != nil {
		return nil, err
	}
	actions := []Action{}
	if f.Current == nil {
		if len(f.Units) != 0 || kind != "INSTALL" {
			return nil, fmt.Errorf("foreign registration or manifest absent")
		}
	} else {
		old := f.Current
		if _, err := old.Encode(); err != nil {
			return nil, err
		}
		if old.Program != next.Program || old.UID != next.UID || old.QueueID != next.QueueID || old.CanonicalStore != next.CanonicalStore || old.Manager != next.Manager || old.Domain != next.Domain {
			return nil, fmt.Errorf("program registry cannot be repurposed")
		}
		if err := ownedUnits(*old, f.Units); err != nil {
			return nil, err
		}
		a, _ := old.Encode()
		b, _ := next.Encode()
		if kind == "UNINSTALL" && !bytes.Equal(a, b) {
			return nil, fmt.Errorf("uninstall requires exact current manifest")
		}
		if kind == "INSTALL" && bytes.Equal(a, b) {
			return []Action{{Kind: "NO_CHANGE_PRESERVE_CONTROL_DEBT_WORKERS"}}, nil
		}
		if kind == "INSTALL" {
			if !f.Replace || next.Previous == nil || *next.Previous != wire.Sum(a) || next.Generation.Uint64() != old.Generation.Uint64()+1 {
				return nil, fmt.Errorf("replace generation required")
			}
		}
		actions = append(actions, Action{Kind: "JOURNAL_OLD_OWNED_BYTES"}, Action{Kind: "SAVE_STOPPED_UNDER_FENCE"}, Action{Kind: "PRESERVE_WORKERS"})
		for _, u := range old.Units {
			actions = append(actions, managerAction(*old, u, true), Action{Kind: "VERIFY_MANAGER_ABSENCE", Label: u.Label}, Action{Kind: "REMOVE_IF_UNCHANGED", Path: u.Path})
		}
	}
	if kind == "UNINSTALL" {
		return append(actions, Action{Kind: "REMOVE_IF_UNCHANGED", Path: next.Path}, Action{Kind: "RETAIN_CONTROL_WORKER_DEBT_LOG_STATE"}), nil
	}
	if kind != "INSTALL" {
		return nil, fmt.Errorf("unsupported operation")
	}
	actions = append(actions, Action{Kind: "JOURNAL_INSTALLING"})
	for _, u := range next.Units {
		actions = append(actions, Action{Kind: "RECHECK_OWNER_STAGE_FSYNC_RENAME", Path: u.Path}, managerAction(next, u, false), Action{Kind: "VERIFY_OWNED_REGISTRATION", Label: u.Label})
	}
	actions = append(actions, Action{Kind: "COMMIT_MANIFEST_AFTER_READBACK", Path: next.Path})
	return actions, nil
}
func PlanRollback(next Manifest, f OperationFacts) ([]Action, error) {
	if f.State != "VERIFIED" || !f.ManagerReachable {
		return nil, fmt.Errorf("rollback observation unknown")
	}
	if _, err := next.Encode(); err != nil {
		return nil, err
	}
	// Only operation-published units grant rollback authority. Other observed
	// units may be absent or foreign and are never cleanup targets.
	subset := next
	subset.Units = nil
	selected := map[string]bool{}
	for _, label := range f.PublishedNewLabels {
		if selected[label] {
			return nil, fmt.Errorf("duplicate rollback label")
		}
		selected[label] = true
		for _, u := range next.Units {
			if u.Label == label {
				subset.Units = append(subset.Units, u)
			}
		}
	}
	if len(subset.Units) != len(selected) {
		return nil, fmt.Errorf("foreign rollback label")
	}
	observed := []ExistingUnit{}
	for _, o := range f.Units {
		if selected[o.Label] {
			observed = append(observed, o)
		}
	}
	if err := ownedUnits(subset, observed); err != nil {
		return nil, err
	}
	a := []Action{{Kind: "ROLLBACK_REQUIRED"}, {Kind: "PRESERVE_WORKERS"}}
	seen := map[string]bool{}
	for _, label := range f.PublishedNewLabels {
		if seen[label] {
			return nil, fmt.Errorf("duplicate rollback label")
		}
		seen[label] = true
		found := false
		for _, u := range next.Units {
			if u.Label == label {
				a = append(a, managerAction(next, u, true), Action{Kind: "VERIFY_MANAGER_ABSENCE", Label: u.Label}, Action{Kind: "REMOVE_IF_UNCHANGED", Path: u.Path})
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("foreign rollback label")
		}
	}
	if f.Current != nil {
		if !f.RestoreSafe {
			return nil, fmt.Errorf("prior restore ownership unknown")
		}
		old, err := f.Current.Encode()
		if err != nil {
			return nil, err
		}
		c := f.Current
		if next.Previous == nil || *next.Previous != wire.Sum(old) || next.Generation.Uint64() != c.Generation.Uint64()+1 || next.Program != c.Program || next.UID != c.UID || next.QueueID != c.QueueID || next.CanonicalStore != c.CanonicalStore || next.Manager != c.Manager || next.Domain != c.Domain {
			return nil, fmt.Errorf("prior restore is not this operation lineage")
		}
		for _, u := range f.Current.Units {
			a = append(a, Action{Kind: "RESTORE_SNAPSHOTTED_OWNED_BYTES", Path: u.Path}, managerAction(*f.Current, u, false))
		}
	}
	return append(a, Action{Kind: "RETAIN_ROLLBACK_RECEIPT"}), nil
}

// Lifecycle models below consume explicit observations, never elapsed wall time.
type Debt struct {
	Failures                  uint8
	Delay, EligibleAfter      uint64
	BootID                    string
	Fences                    map[string]string
	HealthySince, LastHealthy *uint64
	HealthyGeneration         string
}
type FailureObservation struct {
	Generation, Outcome, BootID, Termination string
	Now                                      uint64
	TimeKnown, IdentityKnown                 bool
}

func cloneDebt(d Debt) Debt {
	copy := d
	copy.Fences = map[string]string{}
	for k, v := range d.Fences {
		copy.Fences[k] = v
	}
	if d.HealthySince != nil {
		n := *d.HealthySince
		copy.HealthySince = &n
	}
	if d.LastHealthy != nil {
		n := *d.LastHealthy
		copy.LastHealthy = &n
	}
	return copy
}
func ChargeFailure(d Debt, o FailureObservation) (Debt, string, error) {
	d = cloneDebt(d)
	prior := cloneDebt(d)
	if d.Failures > 6 || len(d.Fences) > 128 {
		return d, "HOLD", fmt.Errorf("debt record bound")
	}
	if o.Generation == "" || o.Outcome == "" {
		return d, "HOLD", fmt.Errorf("generation outcome required")
	}
	if old, ok := d.Fences[o.Generation]; ok {
		if old != o.Outcome {
			return d, "HOLD", fmt.Errorf("generation outcome conflict")
		}
		return d, "REPLAY", nil
	}
	if !o.TimeKnown || !o.IdentityKnown || o.BootID == "" || (o.Termination != "PROVED_TERMINATED" && o.Termination != "PROVED_NO_LIVE_UNEXECUTED") {
		return d, "HOLD", fmt.Errorf("termination/identity/time unknown")
	}
	if d.Failures == 6 || len(d.Fences) == 128 {
		return d, "SERVICE_RESTART_HOLD", nil
	}
	if o.Outcome == "INTENTIONAL_STOP" || o.Outcome == "IDLE_WRAPPER_RESTART" {
		d.Fences[o.Generation] = o.Outcome
		return d, "NO_CHARGE", nil
	}
	if o.Outcome != "FAILED" && o.Outcome != "UNPROVED_NO_LIVE" {
		return d, "HOLD", fmt.Errorf("outcome unknown")
	}
	d.Fences[o.Generation] = o.Outcome
	d.Failures++
	d.BootID = o.BootID
	d.HealthySince = nil
	d.LastHealthy = nil
	d.HealthyGeneration = ""
	if d.Failures == 6 {
		d.Delay = 0
		d.EligibleAfter = 0
		return d, "SERVICE_RESTART_HOLD", nil
	}
	delays := []uint64{30, 60, 120, 240, 300}
	d.Delay = delays[d.Failures-1]
	if o.Now > math.MaxUint64-d.Delay {
		return prior, "HOLD", fmt.Errorf("monotonic overflow")
	}
	d.EligibleAfter = o.Now + d.Delay
	return d, "BACKOFF", nil
}
func Eligibility(d Debt, boot string, now uint64, known bool) (Debt, string) {
	d = cloneDebt(d)
	if d.Failures > 6 || !known || boot == "" {
		return d, "HOLD"
	}
	if d.Failures == 6 {
		return d, "SERVICE_RESTART_HOLD"
	}
	if d.BootID != boot {
		if now > math.MaxUint64-d.Delay {
			return d, "HOLD"
		}
		d.BootID = boot
		d.EligibleAfter = now + d.Delay
		d.HealthySince = nil
		d.LastHealthy = nil
		d.HealthyGeneration = ""
	}
	if now < d.EligibleAfter {
		return d, "BACKOFF"
	}
	return d, "ELIGIBLE"
}

type HealthObservation struct {
	BootID, Generation, Desired, State                                 string
	Now                                                                uint64
	TimeKnown, PinnedValid, PulseTimely, TickHealthy, OwnershipSettled bool
}

func ObserveHealth(d Debt, o HealthObservation) Debt {
	d = cloneDebt(d)
	valid := d.Failures < 6 && o.TimeKnown && o.BootID != "" && o.BootID == d.BootID && o.Generation != "" && o.Desired == "RUNNING" && o.State == "HEALTHY" && o.PinnedValid && o.PulseTimely && o.TickHealthy && o.OwnershipSettled
	// A recorded window that starts after now or after its last healthy
	// observation is corrupt; it is cleared and cannot certify a reset.
	corrupt := d.HealthySince != nil && (*d.HealthySince > o.Now || (d.LastHealthy != nil && *d.HealthySince > *d.LastHealthy))
	if !valid || corrupt {
		d.HealthySince = nil
		d.LastHealthy = nil
		d.HealthyGeneration = ""
		return d
	}
	if d.LastHealthy == nil || d.HealthyGeneration != o.Generation || o.Now < *d.LastHealthy || o.Now-*d.LastHealthy > 30 {
		n := o.Now
		d.HealthySince = &n
	}
	n := o.Now
	d.LastHealthy = &n
	d.HealthyGeneration = o.Generation
	if d.HealthySince != nil && o.Now-*d.HealthySince >= 600 {
		d.Failures = 0
		d.Delay = 0
		d.EligibleAfter = 0
	}
	return d
}

type Control struct {
	Program              string
	ManifestIdentity     wire.Digest
	Revision             wire.Count
	Desired, LastRequest string
	LastRequestSha256    wire.Digest
}
type ControlFacts struct {
	FenceHeld, Fresh, PinnedValid, Published bool
	ObservedRevision                         wire.Count
	Legacy                                   string
	IntentStates                             []string
	CommittedWorkers                         []string
}
type ControlProposal struct {
	Control          Control
	State            string
	Actions          []Action
	PreservedWorkers []string
	Launch           bool
}

func validControl(c Control) bool {
	_, e := wire.ParseCount("/revision", string(c.Revision))
	_, d := wire.ParseDigest("/manifest", string(c.ManifestIdentity))
	return e == nil && d == nil && serviceName.MatchString(c.Program) && (c.Desired == "RUNNING" || c.Desired == "DRAINING" || c.Desired == "STOPPED")
}
func PlanLaunch(c Control, f ControlFacts) ControlProposal {
	p := ControlProposal{Control: c, State: "HOLD"}
	if !validControl(c) || !f.FenceHeld || !f.Fresh || !f.PinnedValid || f.ObservedRevision != c.Revision || c.Desired != "RUNNING" || f.Legacy != "ABSENT" {
		return p
	}
	if len(f.IntentStates) != 1 || f.IntentStates[0] != "CHECKED_SAVED" {
		return p
	}
	p.State = "PROPOSED_LAUNCH"
	p.Launch = true
	p.Actions = []Action{{Kind: "SPAWN_UNDER_FENCE"}, {Kind: "CHECKED_SAVE_OUTCOME_BEFORE_UNLOCK"}}
	return p
}
func Suppress(c Control, request, desired string, f ControlFacts) (ControlProposal, error) {
	p := ControlProposal{Control: c, State: "SUPPRESSION_PENDING", PreservedWorkers: append([]string(nil), f.CommittedWorkers...)}
	if !validControl(c) || request == "" || !f.FenceHeld || !f.Fresh || f.ObservedRevision != c.Revision || (desired != "DRAINING" && desired != "STOPPED") {
		return p, fmt.Errorf("control fence/CAS")
	}
	hash := wire.Sum([]byte(request + "\n" + desired + "\n" + string(c.ManifestIdentity)))
	if c.LastRequest == request {
		if c.LastRequestSha256 != hash {
			return p, fmt.Errorf("control request conflict")
		}
	} else {
		if c.Revision.Int() == wire.MaxCountValue {
			return p, fmt.Errorf("revision overflow")
		}
		p.Control.Revision = wire.CountOf(c.Revision.Int() + 1)
		p.Control.Desired = desired
		p.Control.LastRequest = request
		p.Control.LastRequestSha256 = hash
	}
	p.Actions = []Action{{Kind: "CHECKED_SAVE_SUPPRESSION"}, {Kind: "PRESERVE_WORKERS"}}
	if !f.Published {
		return p, nil
	}
	for _, state := range f.IntentStates {
		if state != "COMMITTED" && state != "PROVED_NO_EFFECT" {
			return p, nil
		}
	}
	p.State = "SUPPRESSION_ACKNOWLEDGED"
	return p, nil
}

type LegacyObservation struct {
	ParentVerified bool
	FileState      string
}

func LegacyPresence(o LegacyObservation) string {
	if !o.ParentVerified {
		return "UNKNOWN"
	}
	switch o.FileState {
	case "ENOENT":
		return "ABSENT"
	case "OWNED_REGULAR":
		return "PRESENT"
	}
	return "UNKNOWN"
}
func LegacyLatch(c Control, role, request string, f ControlFacts) ControlProposal {
	p := ControlProposal{Control: c, State: "HOLD"}
	if role != "MAIN" {
		return p
	}
	if !validControl(c) || !f.FenceHeld || !f.Fresh || f.ObservedRevision != c.Revision || f.Legacy == "UNKNOWN" {
		return p
	}
	p.State = "NO_CHANGE"
	if f.Legacy != "PRESENT" || c.Desired != "RUNNING" {
		return p
	}
	next, err := Suppress(c, request, "DRAINING", f)
	if err != nil {
		return ControlProposal{Control: c, State: "HOLD"}
	}
	return next
}

type OwnedTree struct{ Identity, Kind, State, Ownership string }

// ResumeOperation is retained supplied journal evidence, not durable storage.
// Completed replay never republishes resets over subsequently charged debt.
type ResumeOperation struct {
	Request                 string
	RequestSha256           wire.Digest
	Before, After           Control
	BeforeDebts, AfterDebts map[string]Debt
	PublishedUnits          map[string]bool
	Complete                bool
	PreservedWorkers        []string
}

func resetDebt(d Debt) Debt {
	d = cloneDebt(d)
	d.Failures, d.Delay, d.EligibleAfter = 0, 0, 0
	d.HealthySince, d.LastHealthy, d.HealthyGeneration = nil, nil, ""
	return d
}
func resumeAfter(c Control, request string, hash wire.Digest) (Control, error) {
	if c.Revision.Int() == wire.MaxCountValue {
		return c, fmt.Errorf("revision overflow")
	}
	c.Revision = wire.CountOf(c.Revision.Int() + 1)
	c.Desired, c.LastRequest, c.LastRequestSha256 = "RUNNING", request, hash
	return c, nil
}
func cloneResumeOperation(o *ResumeOperation) *ResumeOperation {
	n := *o
	n.PreservedWorkers = append([]string(nil), o.PreservedWorkers...)
	n.BeforeDebts, n.AfterDebts, n.PublishedUnits = map[string]Debt{}, map[string]Debt{}, map[string]bool{}
	for k, v := range o.BeforeDebts {
		n.BeforeDebts[k] = cloneDebt(v)
	}
	for k, v := range o.AfterDebts {
		n.AfterDebts[k] = cloneDebt(v)
	}
	for k, v := range o.PublishedUnits {
		n.PublishedUnits[k] = v
	}
	return &n
}

type ResumeFacts struct {
	Operation                                   *ResumeOperation
	Control                                     ControlFacts
	Trees                                       []OwnedTree
	LaunchIntentsReconciled, AllResetsPublished bool
	NativeSentinels                             map[string]string
}
type ResumeProposal struct {
	Operation *ResumeOperation
	ControlProposal
	Debts           map[string]Debt
	NativeSentinels map[string]string
}

func Resume(c Control, request string, debts map[string]Debt, f ResumeFacts) (ResumeProposal, error) {
	p := ResumeProposal{ControlProposal: ControlProposal{Control: c, State: "RESUME_PENDING"}, Debts: map[string]Debt{}, NativeSentinels: map[string]string{}}
	for k, v := range debts {
		p.Debts[k] = cloneDebt(v)
	}
	for k, v := range f.NativeSentinels {
		p.NativeSentinels[k] = v
	}
	cf := f.Control
	if !validControl(c) || request == "" || len(debts) < 1 || len(debts) > 9 || !cf.FenceHeld || !cf.Fresh || !cf.PinnedValid || cf.ObservedRevision != c.Revision || cf.Legacy != "ABSENT" {
		return p, fmt.Errorf("resume fence/CAS/legacy/config")
	}
	hash := wire.Sum([]byte(request + "\nRESUME\n" + string(c.ManifestIdentity)))
	if c.LastRequest == request && c.LastRequestSha256 != hash {
		return p, fmt.Errorf("resume request conflict")
	}
	var op *ResumeOperation
	if f.Operation == nil {
		if c.LastRequest == request {
			return p, fmt.Errorf("resume operation evidence missing")
		}
		after, err := resumeAfter(c, request, hash)
		if err != nil {
			return p, err
		}
		op = &ResumeOperation{Request: request, RequestSha256: hash, Before: c, After: after, BeforeDebts: map[string]Debt{}, AfterDebts: map[string]Debt{}, PublishedUnits: map[string]bool{}}
		for k, d := range debts {
			op.BeforeDebts[k], op.AfterDebts[k] = cloneDebt(d), resetDebt(d)
		}
	} else {
		op = cloneResumeOperation(f.Operation)
		after, err := resumeAfter(op.Before, request, hash)
		if err != nil || !validControl(op.Before) || op.Request != request || op.RequestSha256 != hash || op.Before.ManifestIdentity != c.ManifestIdentity || !reflect.DeepEqual(after, op.After) || (c != op.Before && c != op.After) || len(op.BeforeDebts) != len(debts) || len(op.AfterDebts) != len(debts) {
			return p, fmt.Errorf("resume operation binding")
		}
		for k, d := range op.BeforeDebts {
			post, ok := op.AfterDebts[k]
			if !ok || !reflect.DeepEqual(resetDebt(d), post) {
				return p, fmt.Errorf("resume post identity")
			}
			if _, ok := debts[k]; !ok {
				return p, fmt.Errorf("resume unit set")
			}
		}
		for k := range op.PublishedUnits {
			if _, ok := op.BeforeDebts[k]; !ok {
				return p, fmt.Errorf("resume publication unit")
			}
		}
		if op.Complete {
			if c != op.After {
				return p, fmt.Errorf("completed resume control")
			}
			for k := range op.BeforeDebts {
				if !op.PublishedUnits[k] {
					return p, fmt.Errorf("completed resume publication missing")
				}
			}
			p.PreservedWorkers = append([]string(nil), op.PreservedWorkers...)
			p.Operation, p.State = op, "RESUMED_REPLAY"
			return p, nil
		}
		for k, d := range debts {
			expected := op.BeforeDebts[k]
			if op.PublishedUnits[k] {
				expected = op.AfterDebts[k]
			}
			if !reflect.DeepEqual(d, expected) {
				return p, fmt.Errorf("resume debt reconciliation conflict")
			}
		}
	}
	p.Operation = op
	for _, tree := range f.Trees {
		if tree.Ownership != "VERIFIED" || tree.Identity == "" || (tree.State != "LIVE" && tree.State != "RETIRED") {
			return p, nil
		}
		switch tree.Kind {
		case "WORKER":
			if tree.State == "LIVE" {
				p.PreservedWorkers = append(p.PreservedWorkers, tree.Identity)
			}
		case "TRANSIENT", "HELPER":
			if tree.State != "RETIRED" {
				p.Actions = append(p.Actions, Action{Kind: "RETIRE_VERIFIED_OWNED_TRANSIENT_TREE", Label: tree.Identity})
				return p, nil
			}
		default:
			return p, fmt.Errorf("tree kind")
		}
	}
	if !f.LaunchIntentsReconciled {
		return p, nil
	}
	op.PreservedWorkers = append([]string(nil), p.PreservedWorkers...)
	keys := make([]string, 0, len(op.BeforeDebts))
	for k := range op.BeforeDebts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !op.PublishedUnits[k] {
			p.Actions = append(p.Actions, Action{Kind: "CHECKED_PUBLISH_SERVICE_DEBT_RESET", Label: k})
		}
	}
	p.Actions = append(p.Actions, Action{Kind: "PRESERVE_WORKERS_NATIVE_DEBT_AND_ATTEMPTS"})
	if !f.AllResetsPublished {
		return p, nil
	}
	for k, d := range op.AfterDebts {
		op.PublishedUnits[k] = true
		p.Debts[k] = cloneDebt(d)
	}
	if !cf.Published {
		return p, nil
	}
	p.Control = op.After
	op.Complete = true
	p.State = "RESUMED"
	return p, nil
}
