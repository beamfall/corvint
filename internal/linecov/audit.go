// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// collectorOverlay initializes runtime/coverage before test windows and observes
// every os/exec launch, including a prebuilt or GOFLAGS-cleared child. Unknown
// launchers fail closed: absence of child counters alone never proves completeness
// (LPCV-V0-954). The patches are scratch copies of this toolchain's sources.
func collectorOverlay(work, overlay string) (string, error) {
	var o struct{ Replace map[string]string }
	o.Replace = map[string]string{}
	if overlay != "" {
		b, e := os.ReadFile(overlay)
		if e != nil {
			return "", e
		}
		if e = json.Unmarshal(b, &o); e != nil {
			return "", e
		}
	}
	edits := []struct{ path, old, new, tail string }{
		{"internal/coverage/cfile/hooks.go", "if istest {", "if istest && os.Getenv(\"CORVINT_LINECOV_WINDOWS\") == \"\" {", ""},
		{"os/exec/exec.go", "func (c *Cmd) Start() error {", "func (c *Cmd) Start() error {\ncorvintLinecovAudit(c)", auditHelper},
	}
	for i, e := range edits {
		p := filepath.Join(runtime.GOROOT(), "src", e.path)
		src, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		if strings.Count(string(src), e.old) != 1 {
			return "", fmt.Errorf("linecov: unsupported-collector-toolchain: %s", p)
		}
		s := strings.Replace(string(src), e.old, e.new, 1)
		if i == 0 {
			s = strings.Replace(s, `import "internal/runtime/exithook"`, `import "internal/runtime/exithook"`+"\nimport \"os\"", 1)
		} else {
			s = strings.Replace(s, "import (", "import (\ncorvintLCBuild \"debug/buildinfo\"", 1)
		}
		dst := filepath.Join(work, fmt.Sprintf("stdlib-%d.go", i))
		if err := os.WriteFile(dst, []byte(s+e.tail), 0600); err != nil {
			return "", err
		}
		o.Replace[p] = dst
	}
	b, _ := json.Marshal(o)
	p := filepath.Join(work, "collector-overlay.json")
	return p, os.WriteFile(p, b, 0600)
}

const auditHelper = `
// corvintLinecovAudit is present only in the collector's scratch overlay.
func corvintLinecovAudit(c *Cmd) {
 p := os.Getenv("CORVINT_LINECOV_AUDIT")
 if p == "" { return }
 reason := "uninstrumented-child"
 info, err := corvintLCBuild.ReadFile(c.Path)
 if err == nil {
  // Go's own build tools are launchers, not repository test subjects. Their
  // descendants inherit this overlay unless the caller explicitly clears it.
  if info.Path=="cmd/go" {
   for _,arg:=range c.Args[1:] {if strings.HasPrefix(arg,"-"){continue};if arg=="build" || arg=="list" || arg=="env" || arg=="version"{return};break}
  }
  switch info.Path {case "cmd/compile","cmd/link","cmd/asm","cmd/cgo","cmd/vet":return}
  covered := false
  for _,s := range info.Settings { if s.Key == "-cover" && s.Value == "true" { covered = true } }
  env := c.Env
  if env == nil { env = os.Environ() }
  cov := ""
  for _,kv := range env { if strings.HasPrefix(kv,"GOCOVERDIR=") { cov = strings.TrimPrefix(kv,"GOCOVERDIR=") } }
  if covered && cov == os.Getenv("GOCOVERDIR") && cov != "" {
   child,err:=os.MkdirTemp(cov,"child-")
   if err != nil { panic(err) }
   clean:=make([]string,0,len(env)+1)
   for _,kv:=range env{if !strings.HasPrefix(kv,"GOCOVERDIR="){clean=append(clean,kv)}}
   c.Env=append(clean,"GOCOVERDIR="+child)
   reason="covered-child"
   // A private directory per launch makes a second child's missing counters
   // visible even when the first completed normally.
   cpath:=child
   f,err:=os.OpenFile(p,os.O_CREATE|os.O_APPEND|os.O_WRONLY,0600)
   if err!=nil{panic(err)};_,err=f.WriteString(reason+"\t"+cpath+"\n");_ = f.Close();if err!=nil{panic(err)};return
  }
 }
 f,err:=os.OpenFile(p,os.O_CREATE|os.O_APPEND|os.O_WRONLY,0600)
 if err != nil { panic(err) }
 _,err=f.WriteString(reason+"\t"+c.Path+"\n");_ = f.Close()
 if err != nil { panic(err) }
}
`
