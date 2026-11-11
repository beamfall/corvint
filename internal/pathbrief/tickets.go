package pathbrief

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// Ticket is one open ticket as the brief reads it.
type Ticket struct {
	ID            string
	Title         string
	DeclaredScope []string
	TouchPaths    []string
}

// TasksExecutable is the companion the tickets section reads through.
const TasksExecutable = "corvint-tasks"

// errTasks is the one failure of the Tasks source: not installed, no store, or an answer this
// reader does not understand. The brief reports it as a missing source.
var errTasks = errors.New("corvint-tasks source unavailable")

const ticketPage = 1000

// ticketPages bounds the read at 64,000 open tickets.
const ticketPages = 64

// ExecTickets returns a source that reads every open ticket of the store at root with
// `corvint-tasks ticket search`. It reads only: the search verb writes nothing.
func ExecTickets(root string) func(context.Context) ([]Ticket, error) {
	return func(ctx context.Context) ([]Ticket, error) {
		executable, err := exec.LookPath(TasksExecutable)
		if err != nil {
			return nil, errTasks
		}
		var tickets []Ticket
		for page := 0; page < ticketPages; page++ {
			result := runProcess(ctx, procgroup.Spec{
				Argv: []string{executable, "--root", root, "ticket", "search", "--status", "OPEN",
					"--limit", strconv.Itoa(ticketPage), "--offset", strconv.Itoa(page * ticketPage),
					"--fields", "ticketId,title,declaredScope,record.effects"},
				Dir: root, Env: tasksEnvironment(), Timeout: 60 * time.Second, InputLimit: 1, OutputLimit: 64 << 20,
			})
			if result.Err != nil || result.ExitStatus != 0 {
				return nil, errTasks
			}
			items, more, err := parseTicketPage(result.Stdout)
			if err != nil {
				return nil, errTasks
			}
			tickets = append(tickets, items...)
			if !more || len(items) == 0 {
				return tickets, nil
			}
		}
		return nil, errTasks
	}
}

// tasksEnvironment passes the companion only what it needs to start.
func tasksEnvironment() []string {
	env := []string{}
	for _, name := range []string{"PATH", "HOME", "TMPDIR", "USERPROFILE", "SystemRoot", "TEMP", "TMP"} {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return env
}

func parseTicketPage(data []byte) ([]Ticket, bool, error) {
	page := struct {
		Outcome string `json:"outcome"`
		Page    struct {
			Truncated bool `json:"truncated"`
		} `json:"page"`
		Items []struct {
			TicketID      string   `json:"ticketId"`
			Title         string   `json:"title"`
			DeclaredScope []string `json:"declaredScope"`
			Record        struct {
				Effects struct {
					TouchPaths []string `json:"touchPaths"`
				} `json:"effects"`
			} `json:"record"`
		} `json:"items"`
	}{}
	if err := json.Unmarshal(data, &page); err != nil || page.Outcome != "OK" {
		return nil, false, errTasks
	}
	tickets := make([]Ticket, 0, len(page.Items))
	for _, item := range page.Items {
		if item.TicketID == "" {
			return nil, false, errTasks
		}
		tickets = append(tickets, Ticket{ID: item.TicketID, Title: item.Title, DeclaredScope: item.DeclaredScope, TouchPaths: item.Record.Effects.TouchPaths})
	}
	return tickets, page.Page.Truncated, nil
}

// scopeCovers reports whether one scope entry of a ticket covers the path. An entry covers a path
// it equals or contains as a directory. A directory path is also covered by an entry beneath it.
// Entries are read literally: no pattern is expanded.
func scopeCovers(entry, path string, directory bool) bool {
	entry = strings.TrimSuffix(strings.TrimPrefix(entry, "./"), "/")
	if entry == "" || entry == "." || path == "" {
		return entry != ""
	}
	if entry == path || strings.HasPrefix(path, entry+"/") {
		return true
	}
	return directory && strings.HasPrefix(entry, path+"/")
}
