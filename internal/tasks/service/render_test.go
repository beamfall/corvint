package service

import (
	"bytes"
	"encoding/xml"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func helperFixture(t *testing.T, manager string) (Profile, InstallationFacts, *Manifest) {
	t.Helper()
	p, f, _ := serviceFixture(t, manager)
	h := Helper{ID: "health", Cwd: p.WorkRoot, Argv: []string{"/Applications/Corvint/bin/helper", "-leading", "literal % $ ; --flag"}, Env: map[string]string{"SYNTHETIC": "manifest-only"}, Foreground: true, StopWithProgram: true}
	p.Helpers = append(p.Helpers, h)
	hf := f.Executable
	hf.Path = h.Argv[0]
	hf.DeclaredPath = hf.Path
	hf.Sha256 = wire.Sum([]byte("helper"))
	f.HelperExecutables[h.ID] = hf
	m, err := BuildManifest(p, f)
	if err != nil {
		t.Fatal("initialize helper", err)
	}
	return p, f, m
}
func TestIssue500_UnitRenderingAndArgumentEscaping(t *testing.T) {
	for _, manager := range []string{"launchd", "systemd-user"} {
		t.Run(manager, func(t *testing.T) {
			p, f, _ := helperFixture(t, manager)
			p.WorkRoot = "/Users/alice/Unicode-é space \\\"%$"
			f.CanonicalStore = p.WorkRoot
			f.ConfigWorkRoot = p.WorkRoot
			f.ManifestPath = "/Users/alice/state/manifest \\\"%$.json"
			m, err := BuildManifest(p, f)
			if err != nil {
				t.Fatal("adversarial path fixture", err)
			}
			if len(m.Units) != 2 {
				t.Fatal("helper unit missing")
			}
			for _, u := range m.Units {
				if bytes.Contains(u.Raw, []byte("manifest-only")) || bytes.Contains(u.Raw, []byte("literal %")) {
					t.Fatal("helper command/env became unit text")
				}
				expected := unitArguments(*m, u.HelperID)
				var actual []string
				if manager == "launchd" {
					d := xml.NewDecoder(bytes.NewReader(u.Raw))
					last := ""
					for {
						tok, e := d.Token()
						if e == io.EOF {
							break
						}
						if e != nil {
							t.Fatal(e)
						}
						if start, ok := tok.(xml.StartElement); ok {
							if start.Name.Local == "key" {
								if e = d.DecodeElement(&last, &start); e != nil {
									t.Fatal(e)
								}
							} else if start.Name.Local == "array" && last == "ProgramArguments" {
								var a struct {
									Strings []string `xml:"string"`
								}
								if e = d.DecodeElement(&a, &start); e != nil {
									t.Fatal(e)
								}
								actual = a.Strings
							}
						}
					}
					group := "<key>AbandonProcessGroup</key><true>"
					if u.HelperID != "" {
						group = "<key>AbandonProcessGroup</key><false>"
					}
					if !strings.Contains(string(u.Raw), group) || bytes.Contains(u.Raw, []byte("<!DOCTYPE")) {
						t.Fatal("wrong plist containment or DTD")
					}
				} else {
					var line string
					for _, l := range strings.Split(string(u.Raw), "\n") {
						if strings.HasPrefix(l, "ExecStart=") {
							line = strings.TrimPrefix(l, "ExecStart=")
						}
					}
					re := regexp.MustCompile(`"(?:\\.|[^"\\])*"`)
					for _, atom := range re.FindAllString(line, -1) {
						value, e := strconv.Unquote(atom)
						if e != nil {
							t.Fatal(e)
						}
						value = strings.ReplaceAll(strings.ReplaceAll(value, "%%", "%"), "$$", "$")
						actual = append(actual, value)
					}
					kill := "KillMode=process"
					if u.HelperID != "" {
						kill = "KillMode=control-group"
					}
					if !strings.Contains(string(u.Raw), kill) || !strings.Contains(string(u.Raw), "StartLimitIntervalSec=0") || !strings.Contains(string(u.Raw), "StandardOutput=null") {
						t.Fatal("systemd policy")
					}
				}
				if strings.Join(actual, "\x00") != strings.Join(expected, "\x00") {
					t.Fatalf("argv mismatch\ngot%q\nwant%q", actual, expected)
				}
			}
			a, err := m.Encode()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(a, []byte("manifest-only")) || !bytes.Contains(a, []byte("helperExecutables")) {
				t.Fatal("manifest omitted helper snapshot")
			}
		})
	}
	if _, err := systemdAtom(";", true); err == nil {
		t.Fatal("unsupported delimiter accepted")
	}
}
func TestIssue500_OwnedInstallAndForeignRefusal(t *testing.T) {
	for _, manager := range []string{"launchd", "systemd-user"} {
		t.Run(manager, func(t *testing.T) {
			p, f, m := helperFixture(t, manager)
			actions, err := PlanOperation("INSTALL", *m, OperationFacts{State: "VERIFIED", ManagerReachable: true})
			if err != nil || len(actions) == 0 {
				t.Fatal("new plan", err)
			}
			for _, a := range actions {
				if a.Kind == "REGISTER" && manager == "launchd" {
					if len(a.Argv) != 4 || a.Argv[1] != "bootstrap" || a.Argv[2] != m.Domain || !strings.HasPrefix(a.Argv[3], m.UnitRoot+"/") {
						t.Fatal("bootstrap exact domain/path")
					}
				}
			}
			same, err := PlanOperation("INSTALL", *m, OperationFacts{State: "VERIFIED", ManagerReachable: true, Current: m, Units: serviceOwned(*m)})
			if err != nil || len(same) != 1 || same[0].Kind != "NO_CHANGE_PRESERVE_CONTROL_DEBT_WORKERS" {
				t.Fatal("owned idempotence", err)
			}
			for name, spoil := range map[string]func(*OperationFacts){"foreign": func(f *OperationFacts) { f.Units[0].Ownership = "FOREIGN" }, "dropin": func(f *OperationFacts) { f.Units[0].NoDropIns = false }, "symlink": func(f *OperationFacts) { f.Units[0].NoSymlink = false }, "modified": func(f *OperationFacts) { f.Units[0].Sha256 = wire.Sum([]byte("changed")) }, "manager": func(f *OperationFacts) { f.ManagerReachable = false }} {
				t.Run(name, func(t *testing.T) {
					obs := OperationFacts{State: "VERIFIED", ManagerReachable: true, Current: m, Units: serviceOwned(*m)}
					spoil(&obs)
					if _, err := PlanOperation("INSTALL", *m, obs); err == nil {
						t.Fatal("foreign operation allowed")
					}
				})
			}
			oldBytes, _ := m.Encode()
			f.Generation = "2"
			oldHash := wire.Sum(oldBytes)
			f.Previous = &oldHash
			p.LegacyStopFile = new(string)
			*p.LegacyStopFile = "/Users/alice/state/legacy-stop"
			next, err := BuildManifest(p, f)
			if err != nil {
				t.Fatal(err)
			}
			obs := OperationFacts{State: "VERIFIED", ManagerReachable: true, Current: m, Units: serviceOwned(*m)}
			if _, err = PlanOperation("INSTALL", *next, obs); err == nil {
				t.Fatal("implicit replacement")
			}
			obs.Replace = true
			if _, err = PlanOperation("INSTALL", *next, obs); err != nil {
				t.Fatal("exact replacement", err)
			}
			forged := *next
			forged.Units = append([]Unit(nil), next.Units...)
			forged.Units[0].Label = "foreign"
			if _, err = forged.Encode(); err == nil {
				t.Fatal("forged generated-unit identity")
			}
		})
	}
}
func TestIssue500_UninstallAndRollbackPlan(t *testing.T) {
	profile, install, m := helperFixture(t, "launchd")
	f := OperationFacts{State: "VERIFIED", ManagerReachable: true, Current: m, Units: serviceOwned(*m)}
	actions, err := PlanOperation("UNINSTALL", *m, f)
	if err != nil {
		t.Fatal(err)
	}
	saved, preserved := false, false
	for _, a := range actions {
		if a.Kind == "SAVE_STOPPED_UNDER_FENCE" {
			saved = true
		}
		if a.Kind == "PRESERVE_WORKERS" {
			preserved = true
		}
		if a.Kind == "UNREGISTER" {
			if !saved || !preserved || len(a.Argv) != 3 || a.Argv[2] != m.Domain+"/"+a.Label {
				t.Fatal("uninstall domain-wide/worker-loss ordering")
			}
		}
		if strings.Contains(a.Kind, "RECURSIVE") {
			t.Fatal("state erased")
		}
	}
	if actions[len(actions)-1].Kind != "RETAIN_CONTROL_WORKER_DEBT_LOG_STATE" {
		t.Fatal("state retention")
	}
	old, _ := m.Encode()
	hash := wire.Sum(old)
	install.Generation, install.Previous = "2", &hash
	next, err := BuildManifest(profile, install)
	if err != nil {
		t.Fatal("rollback replacement fixture", err)
	}
	f.Units = serviceOwned(*next)
	f.PublishedNewLabels = []string{next.Units[1].Label}
	f.RestoreSafe = true
	rollback, err := PlanRollback(*next, f)
	if err != nil {
		t.Fatal(err)
	}
	removed := 0
	for _, a := range rollback {
		if a.Kind == "UNREGISTER" {
			removed++
			if a.Label != m.Units[1].Label {
				t.Fatal("unregistered unproved operation unit")
			}
		}
	}
	if removed != 1 {
		t.Fatal("rollback exact published set")
	}
	f.PublishedNewLabels = []string{"foreign"}
	if _, err = PlanRollback(*next, f); err == nil {
		t.Fatal("foreign rollback")
	}
	f.PublishedNewLabels = []string{m.Units[1].Label}
	f.RestoreSafe = false
	if _, err = PlanRollback(*next, f); err == nil {
		t.Fatal("unknown prior restoration")
	}
}

func TestIssue500_ExactUninstallAndPartialRollback(t *testing.T) {
	for _, manager := range []string{"launchd", "systemd-user"} {
		t.Run(manager, func(t *testing.T) {
			p, f, current := helperFixture(t, manager)
			old, err := current.Encode()
			if err != nil {
				t.Fatal("current fixture", err)
			}
			hash := wire.Sum(old)
			f.Generation = "2"
			f.Previous = &hash
			f.ManifestPath = current.StateRoot + "/other-manifest.json"
			next, err := BuildManifest(p, f)
			if err != nil {
				t.Fatal("distinct valid next", err)
			}
			owned := OperationFacts{State: "VERIFIED", ManagerReachable: true, Current: current, Units: serviceOwned(*current)}
			if _, err = PlanOperation("UNINSTALL", *next, owned); err == nil {
				t.Fatal("uninstall supplied next instead of current")
			}
			units := serviceOwned(*next)
			partial := OperationFacts{State: "VERIFIED", ManagerReachable: true, PublishedNewLabels: []string{next.Units[0].Label}, Units: units[:1]}
			actions, err := PlanRollback(*next, partial)
			if err != nil {
				t.Fatal("honest partial publication", err)
			}
			for _, a := range actions {
				if a.Kind == "UNREGISTER" && a.Label != next.Units[0].Label {
					t.Fatal("unpublished removal")
				}
			}
			foreign := units[1]
			foreign.Ownership = "FOREIGN"
			partial.Units = append(partial.Units, foreign)
			if _, err = PlanRollback(*next, partial); err != nil {
				t.Fatal("unpublished foreign unit blocked safe rollback", err)
			}
			partial.Current = current
			partial.RestoreSafe = true
			if _, err = PlanRollback(*next, partial); err != nil {
				t.Fatal("same operation prior restore", err)
			}
			unrelated := *current
			unrelated.Generation = "3"
			if _, err = unrelated.Encode(); err != nil {
				t.Fatal("valid unrelated prior", err)
			}
			partial.Current = &unrelated
			if _, err = PlanRollback(*next, partial); err == nil {
				t.Fatal("unrelated restoration")
			}
			malformed := *next
			malformed.Units = append([]Unit(nil), next.Units...)
			malformed.Units[0].Label = "foreign"
			if _, err = PlanRollback(malformed, partial); err == nil {
				t.Fatal("malformed next rollback")
			}
			partial.Current = nil
			partial.Units[0].Ownership = "UNKNOWN"
			if _, err = PlanRollback(*next, partial); err == nil {
				t.Fatal("unknown published unit cleanup")
			}
		})
	}
}
