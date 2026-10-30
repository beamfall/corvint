package dogfoodflow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// finishLaneTimings is the file, under the Git directory's corvint evidence
// directory, that keeps one row per command a finish-lane run executed
// (DCW-V0-115).
const finishLaneTimings = "dogfood-finish-lane.timings.tsv"

// laneClock times a finish-lane run: every command its runners execute, by
// the step that ran it, so a caller can bound the run and name its dominant
// stage (DCW-V0-115).
type laneClock struct {
	mu       sync.Mutex
	start    time.Time
	step     string
	rows     []laneCommand
	commands map[string]int
}

type laneCommand struct {
	step, label string
	exit        int
	seconds     float64
}

func newLaneClock() *laneClock {
	return &laneClock{start: time.Now(), commands: map[string]int{}}
}

// wrap returns runner with every command it runs recorded under the step
// running at the time; a runner without a command is returned unchanged.
func (c *laneClock) wrap(runner Runner) Runner {
	if runner.Run == nil {
		return runner
	}
	run := runner.Run
	runner.Run = func(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
		began := time.Now()
		status := run(ctx, root, args, stdout, stderr)
		c.mu.Lock()
		c.rows = append(c.rows, laneCommand{c.step, commandLabel(args), status, time.Since(began).Seconds()})
		c.commands[c.step]++
		c.mu.Unlock()
		return status
	}
	return runner
}

// begin names the step later commands are recorded under and returns the
// function that ends it.
func (c *laneClock) begin(step string) func() (float64, int) {
	c.mu.Lock()
	c.step = step
	c.mu.Unlock()
	began := time.Now()
	return func() (float64, int) {
		c.mu.Lock()
		defer c.mu.Unlock()
		return time.Since(began).Seconds(), c.commands[step]
	}
}

// tsv renders the per-command rows: step, command, exit, seconds.
func (c *laneClock) tsv() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out bytes.Buffer
	out.WriteString("step\tcommand\texit\tseconds\n")
	for _, row := range c.rows {
		fmt.Fprintf(&out, "%s\t%s\t%d\t%.2f\n", row.step, row.label, row.exit, row.seconds)
	}
	return out.Bytes()
}

func (c *laneClock) started() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.step != ""
}

func (c *laneClock) total() (float64, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Since(c.start).Seconds(), len(c.rows)
}

// commandLabel is a command's leading words before its first flag, such as
// "cem cite" or "dogfood-record"; arguments are never recorded.
func commandLabel(args []string) string {
	var words []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") || len(words) == 2 {
			break
		}
		words = append(words, arg)
	}
	if len(words) == 0 {
		return "-"
	}
	return strings.Join(words, " ")
}

// timed starts step on the run's clock; the returned function, deferred by
// the step, writes its TIMING line to stderr however the step ends.
func (l *finishLane) timed(step string) func() {
	end := l.clock.begin(step)
	return func() {
		seconds, commands := end()
		l.say("%s: TIMING %s seconds=%.2f commands=%d\n", l.prefix, step, seconds, commands)
	}
}

// writeTimings keeps the per-command rows and writes the run's total, also
// after a stop or an interruption, so a slow run names its dominant stage.
// A run refused or skipped before its first step has nothing to time.
func (l *finishLane) writeTimings() {
	if !l.clock.started() {
		return
	}
	seconds, commands := l.clock.total()
	detail := "not-written"
	if l.gitDir != "" {
		name := l.gitDir + "/corvint/" + finishLaneTimings
		if os.MkdirAll(l.gitDir+"/corvint", 0o777) == nil && writePrivate(name, l.clock.tsv()) == nil {
			detail = name
		}
	}
	l.say("%s: TIMING total seconds=%.2f commands=%d detail=%s\n", l.prefix, seconds, commands, detail)
}
