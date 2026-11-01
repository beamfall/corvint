package modelgateway

import (
	"bufio"
	"encoding/json"
	"io"
	"time"
)

// minBreakTokens is the shortest cacheable prefix; a smaller shortfall is noise.
const minBreakTokens = 1024

// cacheTTL is the default prompt-cache lifetime.
const cacheTTL = 5 * time.Minute

// Break causes (LMG-V0-012). Each names the likely cause, not a proven one.
const (
	CauseIdle          = "over 5 minutes idle: expiry under the default TTL, or changed earlier content"
	CausePrefixChanged = "earlier prefix content changed (an edited message or hook output)"
	CausePrefixShrank  = "prefix shorter than before: compaction, or another conversation with the same system prompt and tools"
	CausePrefixSwitch  = "tool list or system prompt changed, or a subagent started"
)

// CacheBreak is one request whose cache read fell short of the prefix its
// thread had already cached.
type CacheBreak struct {
	At         string
	SessionKey string
	Model      string
	Cause      string
	Expected   int64         // tokens the predecessor had cached
	Read       int64         // tokens this request read from cache
	Created    int64         // tokens this request wrote to cache
	Gap        time.Duration // predecessor start to this request's start
}

// cacheThread is the last cached request of one thread.
type cacheThread struct {
	cached     int64
	start, end time.Time
}

// CacheBreaks reads ledger lines and reports each prompt-cache break
// (LMG-V0-012). A thread is one session, model, system prompt and tool list;
// its expected read is its previous read plus creation. Requests are timed
// from their start (completion time less duration), and a request that started
// before its predecessor finished is not compared with it, since the
// predecessor's cache entry may not have existed yet. A request with no cache
// activity, a failure, or no session key or usage counts is skipped.
func CacheBreaks(r io.Reader) ([]CacheBreak, error) {
	threads := map[string]*cacheThread{}
	last := map[string]string{} // session|model -> last thread key
	var breaks []CacheBreak
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		var u Usage
		if json.Unmarshal(scanner.Bytes(), &u) != nil || u.Profile != UsageProfile || u.SessionKey == "" ||
			u.Status >= 400 || u.Error != "" || u.CacheReadTokens == nil || u.CacheCreationTokens == nil {
			continue
		}
		read, created := *u.CacheReadTokens, *u.CacheCreationTokens
		if read+created == 0 {
			continue
		}
		end, err := time.Parse(time.RFC3339Nano, u.At)
		if err != nil {
			continue
		}
		current := &cacheThread{cached: read + created, start: end.Add(-time.Duration(u.DurationMs) * time.Millisecond), end: end}
		session := u.SessionKey + "|" + u.Model
		key := session + "|" + u.SystemKey + "|" + u.ToolsKey
		previous, seen := threads[key]
		if !seen && last[session] != "" {
			previous = threads[last[session]]
		}
		if previous != nil && !current.start.Before(previous.end) && previous.cached-read >= minBreakTokens && created >= minBreakTokens {
			gap := current.start.Sub(previous.start)
			cause := CausePrefixSwitch
			switch {
			case !seen:
			case gap > cacheTTL:
				cause = CauseIdle
			case current.cached+minBreakTokens <= previous.cached:
				cause = CausePrefixShrank
			default:
				cause = CausePrefixChanged
			}
			breaks = append(breaks, CacheBreak{At: u.At, SessionKey: u.SessionKey, Model: u.Model, Cause: cause,
				Expected: previous.cached, Read: read, Created: created, Gap: gap})
		}
		if seen && current.start.Before(previous.end) && previous.cached > current.cached {
			current.cached = previous.cached
		}
		threads[key] = current
		last[session] = key
	}
	return breaks, scanner.Err()
}
