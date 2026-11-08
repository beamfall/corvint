// SPDX-License-Identifier: Apache-2.0
package linecov

// Added only to the checked syscall/env_unix.go implementation. Reads before
// os initializes are unproven: retain the whole-environment claim, because
// neither their caller nor the initial value can be reconstructed later.
const syscallEnvHelper = `
var corvintLCEnvAudit struct {
 sync.Mutex
 read func(string, string, bool)
 all func()
 pending bool
}

func Getenv(key string) (string, bool) {
 value, found := CorvintLinecovGetenvQuiet(key)
 corvintLCEnvAudit.Lock()
 read := corvintLCEnvAudit.read
 if read == nil {
  corvintLCEnvAudit.pending = true
 }
 corvintLCEnvAudit.Unlock()
 if read != nil { read(key, value, found) }
 return value, found
}

func Environ() []string {
 values := CorvintLinecovEnvironQuiet()
 corvintLCEnvAudit.Lock()
 all := corvintLCEnvAudit.all
 if all == nil {
  corvintLCEnvAudit.pending = true
 }
 corvintLCEnvAudit.Unlock()
 if all != nil { all() }
 return values
}

func CorvintLinecovEnvObserve(read func(string, string, bool), all func()) {
 corvintLCEnvAudit.Lock()
 pending := corvintLCEnvAudit.pending
 corvintLCEnvAudit.pending = false
 corvintLCEnvAudit.read, corvintLCEnvAudit.all = read, all
 corvintLCEnvAudit.Unlock()
 if pending { all() }
}
`
