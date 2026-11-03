// SPDX-License-Identifier: Apache-2.0

package browsercov

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// startServer starts the collector's server command with every process
// instrumented and waits until ServerURL answers. stop asks each process
// under test to write its coverage, then stops the server; it returns a
// reason when a process did not answer (LPCV-V0-974).
func (c *collector) startServer(ctx context.Context, dir string) (stop func() string, err error) {
	cmd := exec.Command("sh", "-c", c.Server)
	cmd.Dir, cmd.Env = c.Root, c.env(dir)
	log, err := os.Create(filepath.Join(dir, "server.log"))
	if err != nil {
		return nil, err
	}
	cmd.Stdout, cmd.Stderr = log, log
	setGroup(cmd)
	if err := cmd.Start(); err != nil {
		log.Close()
		return nil, fmt.Errorf("browsercov: server: %v", err)
	}
	exited := make(chan struct{})
	go func() { cmd.Wait(); log.Close(); close(exited) }()
	stop = func() string {
		reason := flush(dir)
		_ = signalGroup(cmd.Process.Pid, sigTerm)
		select {
		case <-exited:
		case <-time.After(10 * time.Second):
			_ = signalGroup(cmd.Process.Pid, sigKill)
			<-exited
		}
		return reason
	}
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		if c.ServerURL == "" {
			return stop, nil
		}
		if resp, err := client.Get(c.ServerURL); err == nil {
			resp.Body.Close()
			return stop, nil
		}
		select {
		case <-exited:
			out, _ := os.ReadFile(filepath.Join(dir, "server.log"))
			return nil, fmt.Errorf("browsercov: server exited before %s answered: %s", c.ServerURL, tail(out))
		case <-ctx.Done():
			stop()
			return nil, fmt.Errorf("browsercov: %s did not answer: %v", c.ServerURL, ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// flush sends SIGUSR2 to every live process under test and waits for each
// one's taken marker, so its coverage is on disk before it is stopped.
func flush(dir string) string {
	entries, _ := os.ReadDir(dir)
	count := func(pid int) int {
		n := 0
		es, _ := os.ReadDir(dir)
		for _, e := range es {
			if strings.HasPrefix(e.Name(), fmt.Sprintf("taken-%d-", pid)) {
				n++
			}
		}
		return n
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "role-") {
			continue
		}
		var r roleFile
		body, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		if json.Unmarshal(body, &r) != nil || r.Role != "process" || !alive(r.Pid) {
			continue
		}
		before := count(r.Pid)
		if signalUsr2(r.Pid) != nil {
			continue
		}
		deadline := time.Now().Add(10 * time.Second)
		for count(r.Pid) == before {
			if time.Now().After(deadline) {
				if alive(r.Pid) {
					return fmt.Sprintf("process %d under test did not write its coverage", r.Pid)
				}
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	return ""
}

func tail(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 2000 {
		s = "…" + s[len(s)-2000:]
	}
	return s
}
