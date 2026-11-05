// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// collectorOverlay initializes runtime/coverage before test windows and observes
// every os/exec launch, including a prebuilt or GOFLAGS-cleared child. Unknown
// launchers fail closed: absence of child counters alone never proves completeness
// (LPCV-V0-954). The patches are scratch copies of this toolchain's sources.
func collectorOverlay(work, overlay, module string) (string, error) {
	var o struct{ Replace map[string]string }
	o.Replace = map[string]string{}
	collectorPath := filepath.Join(work, "collector-overlay.json")
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
		{"os/exec/exec.go", "func (c *Cmd) Start() error {", "func (c *Cmd) Start() error {\ncorvintLinecovAudit(c)", strings.NewReplacer("CORVINT_MODULE_PATTERN", strconv.Quote(module+"/..."), "CORVINT_COLLECTOR_OVERLAY", strconv.Quote(collectorPath)).Replace(auditHelper)},
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
			s = strings.Replace(s, "import (", "import (\ncorvintLCBuild \"debug/buildinfo\"\ncorvintLCFlag \"flag\"\ncorvintLCHash \"crypto/sha256\"\ncorvintLCHex \"encoding/hex\"", 1)
			wait := "func (c *Cmd) Wait() error {"
			if strings.Count(s, wait) != 1 {
				return "", fmt.Errorf("linecov: unsupported-collector-toolchain: %s", p)
			}
			s = strings.Replace(s, wait, wait+"\ndefer corvintLinecovBuilt(c)", 1)
			decl := "type Cmd struct {"
			if strings.Count(s, decl) != 1 {
				return "", fmt.Errorf("linecov: unsupported-collector-toolchain: %s", p)
			}
			s = strings.Replace(s, decl, decl+"\ncorvintLinecovProof func()", 1)
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
 c.corvintLinecovProof = corvintLinecovBuildPlan(c)
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
  for _,s := range info.Settings {
   if s.Key == "-cover" && s.Value == "true" { covered = true }
  }
  env := c.Env
  if env == nil { env = os.Environ() }
  cov := ""
  for _,kv := range env { if strings.HasPrefix(kv,"GOCOVERDIR=") { cov = strings.TrimPrefix(kv,"GOCOVERDIR=") } }
  digest := corvintLinecovDigest(c.Path)
  _, proofErr := os.Stat(filepath.Join(p+".module-builds", digest))
  if covered && digest != "" && proofErr == nil && cov == os.Getenv("GOCOVERDIR") && cov != "" {
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

// BuildInfo omits -coverpkg. A successful, observed build must prove both the
// module-wide selector and the collector overlay; a digest binds that proof to
// the bytes later launched. Unknown build flags conservatively produce no proof.
func corvintLinecovBuilt(c *Cmd) {
 if c.ProcessState!=nil && c.ProcessState.Success() && c.corvintLinecovProof!=nil {c.corvintLinecovProof()}
}

func corvintLinecovBuildPlan(c *Cmd) func() {
 p := os.Getenv("CORVINT_LINECOV_AUDIT")
 if p=="" || len(c.Args)<3 || c.Args[1]!="build" { return nil }
 info,err:=corvintLCBuild.ReadFile(c.Path)
 if err!=nil || info.Path!="cmd/go" { return nil }
 env:=c.Env; if env==nil {env=os.Environ()}
 goflags:=""
 for _,kv:=range env {if strings.HasPrefix(kv,"GOFLAGS="){goflags=strings.TrimPrefix(kv,"GOFLAGS=")}}
 // Go's quoted GOFLAGS grammar is deliberately outside this bounded proof.
 if strings.ContainsAny(goflags,"\"'"){return nil}
 flags:=corvintLCFlag.NewFlagSet("linecov-build",corvintLCFlag.ContinueOnError)
 flags.SetOutput(io.Discard)
 cover:=flags.Bool("cover",false,"")
 pkg:=flags.String("coverpkg","","")
 overlay:=flags.String("overlay","","")
 output:=flags.String("o","","")
 for _,name:=range []string{"covermode","p","tags","buildvcs"}{flags.String(name,"","")}
 for _,name:=range []string{"a","n","x","race","trimpath","v","work"}{flags.Bool(name,false,"")}
 args:=append(strings.Fields(goflags),c.Args[2:]...)
 if flags.Parse(args)!=nil || !*cover || *pkg!=CORVINT_MODULE_PATTERN || *overlay!=CORVINT_COLLECTOR_OVERLAY || *output=="" {return nil}
 // -n prints commands without building; no dry run can certify existing bytes.
 if flags.Lookup("n").Value.String()=="true" {return nil}
 path:=*output; if !filepath.IsAbs(path){path=filepath.Join(c.Dir,path)}
 path,err=filepath.Abs(path);if err!=nil{return nil}
 // Freeze argv, environment and output identity before Start: callers may
 // mutate Cmd fields while the build is running, but cannot rewrite its proof.
 return func(){
 digest:=corvintLinecovDigest(path);if digest==""{return}
 dir:=p+".module-builds"
 if err:=os.MkdirAll(dir,0700);err!=nil{panic(err)}
 if err:=os.WriteFile(filepath.Join(dir,digest),nil,0600);err!=nil{panic(err)}
 }
}

func corvintLinecovDigest(path string) string {
 f,err:=os.Open(path);if err!=nil{return ""};defer f.Close()
 h:=corvintLCHash.New();if _,err:=io.Copy(h,f);err!=nil{return ""}
 return corvintLCHex.EncodeToString(h.Sum(nil))
}
`
