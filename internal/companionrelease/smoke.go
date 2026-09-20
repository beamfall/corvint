package companionrelease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// atmCreatePayload is the exact §3.3 CREATE payload the taskman CLI test
// suite uses for a minimal valid FEATURE ticket
// (internal/cli/mutate_test.go, corvint-taskman). It is not test-only content
// shipped in the bundle; it is fixed input this smoke step feeds to the
// real, extracted `atm` binary.
const atmCreatePayload = `{"acceptanceCriteria":["it exists"],"body":null,"capabilities":[],` +
	`"dependencies":[],"dueDate":null,"effects":{"coverage":"QUALIFIED","externalUnbounded":false,` +
	`"resources":[],"touchPaths":[]},"estimateMinutes":null,"executionClass":"AUTONOMOUS",` +
	`"kind":"FEATURE","labels":[],"milestone":null,"order":"0","owner":null,"priority":"P2",` +
	`"requiredGates":[],"requirementRefs":[],"source":{"kind":"NATIVE","sourceItemId":null,` +
	`"sourceQueueId":"queue:acme:main","sourceRevisionSha256":null},"supersededBy":null,` +
	`"supersedes":null,"title":"Companion smoke ticket"}`

// SmokeStep is one recorded, pass/fail installed-artifact check.
type SmokeStep struct {
	Name            string `json:"name"`
	OK              bool   `json:"ok"`
	Detail          string `json:"detail"`
	BundleSHA256    string `json:"bundleSha256"`
	ComponentSHA256 string `json:"componentSha256,omitempty"`
	SourceCommit    string `json:"sourceCommit,omitempty"`
	SourceTree      string `json:"sourceTree,omitempty"`
	InvokedPath     string `json:"invokedPath,omitempty"`
}

func step(name string, err error, detail string) SmokeStep {
	if err != nil {
		return SmokeStep{Name: name, OK: false, Detail: fmt.Sprintf("%v", err)}
	}
	return SmokeStep{Name: name, OK: true, Detail: detail}
}

// runSmoke exercises the real extracted binaries: atm's version/help/init
// and a create+refine mutation, the console's board and detail pages over
// real loopback HTTP, and the standalone snapshot compiler — then stops the
// console and requires procgroup's own owned-process-group cleanup guarantee
// to hold. It never claims browser qualification; that is a separate,
// explicitly out-of-scope step (PUB-V0-004).
func runSmoke(ctx context.Context, extractedDir, scratch string, queuePolicy queuePolicyFiles, inventory bundleInventory) ([]SmokeStep, error) {
	atmBin := filepath.Join(extractedDir, "bin", inventory.name("atm"))
	consoleBin := filepath.Join(extractedDir, "bin", inventory.name("corvint-console"))
	snapshotBin := filepath.Join(extractedDir, "bin", inventory.name("corvint-dashboard-snapshot"))

	if err := mkdirScratchDir(scratch); err != nil {
		return nil, fmt.Errorf("create smoke scratch dir: %w", err)
	}
	smokeRepo := filepath.Join(scratch, "smoke-repo")
	var steps []SmokeStep

	steps = append(steps, checkAtmVersion(ctx, atmBin, scratch))
	steps = append(steps, checkAtmHelp(ctx, atmBin, scratch))
	commit, err := initSmokeRepo(ctx, smokeRepo, queuePolicy)
	if err != nil {
		steps = append(steps, step("atm-init-fixture", err, ""))
		return steps, fmt.Errorf("smoke fixture setup: %w", err)
	}
	workflowSteps, err := checkWorkflowTools(ctx, extractedDir, smokeRepo, scratch, inventory)
	steps = append(steps, workflowSteps...)
	if err != nil {
		return steps, err
	}
	coreSteps, err := checkCoreDiscoveryWorkflows(ctx, filepath.Join(extractedDir, "bin", inventory.name("corvint")), scratch)
	steps = append(steps, coreSteps...)
	if err != nil {
		return steps, err
	}

	initStep, _, err := runAtmJSON(ctx, atmBin, smokeRepo, "atm-init", "init")
	steps = append(steps, initStep)
	if err != nil {
		return steps, err
	}

	createStep, created, err := runAtmJSON(ctx, atmBin, smokeRepo, "atm-ticket-create", "ticket", "create",
		"--request-id", "smoke-create-1", "--payload", atmCreatePayload)
	steps = append(steps, createStep)
	if err != nil {
		return steps, err
	}
	ticketID, revision, err := extractTicketIDAndRevision(created)
	if err != nil {
		steps = append(steps, step("atm-ticket-create-parse", err, ""))
		return steps, err
	}

	refineStep, _, err := runAtmJSON(ctx, atmBin, smokeRepo, "atm-ticket-refine", "ticket", "refine",
		"--request-id", "smoke-refine-1", "--target", ticketID, "--expected-revision", revision,
		"--payload", `{"title":"Companion smoke ticket (refined)"}`)
	steps = append(steps, refineStep)
	if err != nil {
		return steps, err
	}

	snapshotStep := checkSnapshot(ctx, snapshotBin, smokeRepo, scratch)
	steps = append(steps, snapshotStep)

	consoleSteps, err := checkConsole(ctx, consoleBin, atmBin, snapshotBin, smokeRepo, commit, ticketID, scratch)
	steps = append(steps, consoleSteps...)
	if err != nil {
		return steps, err
	}

	return steps, nil
}

type queuePolicyFiles struct {
	Queue  []byte
	Policy []byte
}

// smokeLinkFixture is the committed spec corpus the requirement/code link
// check reads: SMK-V0-001 cites a committed file, SMK-V0-002 has no
// Traceability row, and SMK-V0-003 cites a path that is not committed. The
// last two must render as explicit gaps (PUB-V0-004).
var smokeLinkFixture = map[string]string{
	"docs/specs/smoke-links-v0.md": "# Smoke Links V0\n\n## Requirements\n\n" +
		"- `SMK-V0-001`: The linked file is committed.\n" +
		"- `SMK-V0-002`: This clause cites no code.\n" +
		"- `SMK-V0-003`: This clause cites a path that is not committed.\n\n" +
		"## Traceability\n\n| Requirement | Implementation | Test |\n|---|---|---|\n" +
		"| SMK-V0-001 | `src/linked.go` | none |\n| SMK-V0-003 | `src/absent.go` | none |\n",
	"docs/specs/REQUIREMENTS.tsv": "id\tfile\tline\ttitle\n" +
		"SMK-V0-001\tdocs/specs/smoke-links-v0.md\t5\tThe linked file is committed.\n" +
		"SMK-V0-002\tdocs/specs/smoke-links-v0.md\t6\tThis clause cites no code.\n" +
		"SMK-V0-003\tdocs/specs/smoke-links-v0.md\t7\tThis clause cites a path that is not committed.\n",
	"docs/specs/INDEX.json": `[{"path":"docs/specs/smoke-links-v0.md","title":"Smoke Links V0",` +
		`"reqPrefix":"SMK-V0","intent":"smoke fixture","delivery":"smoke fixture"}]`,
	"src/linked.go": "package linked\n",
}

// initSmokeRepo commits the link fixture, writes the ticket-store intent
// files, and returns the fixture commit.
func initSmokeRepo(ctx context.Context, root string, qp queuePolicyFiles) (string, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	gitPath, err := lookGit()
	if err != nil {
		return "", err
	}
	if _, _, err := runCaptured(ctx, root, closedGitEnv(root), subprocessTimeout, gitPath, "init", "-q"); err != nil {
		return "", fmt.Errorf("git init smoke repo: %w", err)
	}
	if err := writeFiles(root, smokeLinkFixture, 0o600); err != nil {
		return "", err
	}
	if _, _, err := runCaptured(ctx, root, closedGitEnv(root), subprocessTimeout, gitPath, "add", "--", "docs", "src"); err != nil {
		return "", fmt.Errorf("git add smoke fixture: %w", err)
	}
	// corvint-dashboard-snapshot reads HEAD and refuses a repository with no
	// commits (DASHBOARD_REPOSITORY_UNAVAILABLE), so the fixture needs one.
	commitEnv := append(closedGitEnv(root),
		"GIT_AUTHOR_NAME=Corvint Companion Smoke", "GIT_AUTHOR_EMAIL=smoke@corvint.invalid",
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_NAME=Corvint Companion Smoke",
		"GIT_COMMITTER_EMAIL=smoke@corvint.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	if _, _, err := runCaptured(ctx, root, commitEnv, subprocessTimeout, gitPath,
		"commit", "-q", "--no-gpg-sign", "-m", "companion smoke fixture"); err != nil {
		return "", fmt.Errorf("git commit smoke repo: %w", err)
	}
	head, _, err := runCaptured(ctx, root, closedGitEnv(root), subprocessTimeout, gitPath, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse smoke repo: %w", err)
	}
	intent := map[string]string{".taskman/queue.json": string(qp.Queue), ".taskman/policy.json": string(qp.Policy)}
	return strings.TrimSpace(string(head)), writeFiles(root, intent, 0o600)
}

func writeFiles(root string, files map[string]string, mode os.FileMode) error {
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			return err
		}
	}
	return nil
}

func checkAtmVersion(ctx context.Context, atmBin, scratch string) SmokeStep {
	result, _, _ := runAtmJSON(ctx, atmBin, scratch, "atm-version", "version")
	return result
}

func checkAtmHelp(ctx context.Context, atmBin, scratch string) SmokeStep {
	result, _, _ := runAtmJSON(ctx, atmBin, scratch, "atm-help", "help")
	return result
}

// runAtmJSON runs `atm <args...>` in dir, requires exit 0 and a decodable
// taskman-command-result/0 envelope, and returns the decoded top-level JSON
// object alongside the recorded step.
func runAtmJSON(ctx context.Context, atmBin, dir, name string, args ...string) (SmokeStep, map[string]any, error) {
	stdout, stderr, err := runCaptured(ctx, dir, minimalRunEnv(dir), subprocessTimeout, append([]string{atmBin}, args...)...)
	if err != nil {
		s := step(name, err, trimForError(stderr))
		return s, nil, fmt.Errorf("%s: %w", name, err)
	}
	var decoded map[string]any
	if jsonErr := json.Unmarshal(stdout, &decoded); jsonErr != nil {
		s := step(name, jsonErr, string(stdout))
		return s, nil, fmt.Errorf("%s: decode result: %w", name, jsonErr)
	}
	outcome, _ := decoded["outcome"].(string)
	if outcome != "OK" {
		err := fmt.Errorf("outcome=%q", outcome)
		return step(name, err, string(stdout)), decoded, err
	}
	return step(name, nil, outcome), decoded, nil
}

func extractTicketIDAndRevision(decoded map[string]any) (id, revision string, err error) {
	items, _ := decoded["items"].([]any)
	if len(items) == 0 {
		return "", "", fmt.Errorf("ticket create returned no items")
	}
	first, _ := items[0].(map[string]any)
	id, _ = first["ticketId"].(string)
	revision, _ = first["revision"].(string)
	if id == "" {
		return "", "", fmt.Errorf("ticket create result had no ticketId")
	}
	if revision == "" {
		revision = "1"
	}
	return id, revision, nil
}

func checkSnapshot(ctx context.Context, snapshotBin, repo, scratch string) SmokeStep {
	stdout, stderr, err := runCaptured(ctx, scratch, minimalRunEnv(scratch), subprocessTimeout,
		snapshotBin, "snapshot", "--root", repo)
	if err != nil {
		return step("corvint-dashboard-snapshot", err, trimForError(stderr))
	}
	return step("corvint-dashboard-snapshot", nil, fmt.Sprintf("%d bytes", len(stdout)))
}

func checkConsole(ctx context.Context, consoleBin, atmBin, snapshotBin, repo, commit, ticketID, scratch string) ([]SmokeStep, error) {
	addr, listener, err := reserveLoopbackAddr()
	if err != nil {
		return []SmokeStep{step("console-reserve-port", err, "")}, err
	}
	_ = listener.Close()

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan procgroup.Observation, 1)
	go func() {
		done <- procgroup.Run(runCtx, procgroup.Spec{
			Argv: []string{consoleBin, "-addr", addr, "-repo", repo, "-atm", atmBin, "-snapshot", snapshotBin,
				"-timeout", "20s"},
			Dir:             scratch,
			Env:             minimalRunEnv(scratch),
			Timeout:         30 * time.Second,
			ShutdownTimeout: 5 * time.Second,
			OutputLimit:     4 << 20,
		})
	}()

	var steps []SmokeStep
	ready := waitForHTTP(addr, 5*time.Second)
	steps = append(steps, step("console-listen", boolErr(ready, "console did not accept connections"), addr))

	if ready {
		boardStatus, boardBody, boardErr := httpGet("http://" + addr + "/")
		steps = append(steps, step("console-board", boardErr, boardStatus))
		detailStatus, detailBody, detailErr := httpGet("http://" + addr + "/ticket?id=" + ticketID)
		steps = append(steps, step("console-detail", detailErr, detailStatus))
		steps = append(steps, checkMutationRefusals("http://"+addr, boardBody, filepath.Join(repo, ".taskman"))...)
		steps = append(steps, checkConsolePages("http://"+addr, ticketID, detailBody)...)
		steps = append(steps, checkConsoleLinks("http://"+addr, commit)...)
	}

	cancel()
	observation := <-done
	cleanupOK := observation.OwnedProcessGroupCleanup
	steps = append(steps, step("console-stop-no-descendants", boolErr(cleanupOK, "owned process group cleanup did not confirm"),
		fmt.Sprintf("status=%s", observation.DescendantCleanupStatus)))

	if !ready {
		return steps, fmt.Errorf("console never became reachable at %s", addr)
	}
	return steps, nil
}

func boolErr(ok bool, msg string) error {
	if ok {
		return nil
	}
	return fmt.Errorf("%s", msg)
}

func waitForHTTP(addr string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func httpGet(url string) (string, string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return resp.Status, string(body), fmt.Errorf("unexpected status %s", resp.Status)
	}
	return resp.Status, string(body), nil
}

var hiddenInputPattern = regexp.MustCompile(`<input type="hidden" name="([A-Za-z]+)" value="([^"]*)">`)

// checkMutationRefusals is the installed-path half of PUB-V0-005: it replays
// the board's own create form (real token, request id and issue time) from a
// foreign Origin, and again same-origin without the token, and requires each
// to be refused with 403 for that reason while the ticket store stays
// byte-identical.
func checkMutationRefusals(base, boardBody, storeDir string) []SmokeStep {
	withToken := url.Values{"title": {"refused smoke ticket"}, "criteria": {"never created"}}
	fillHiddenInputs(withToken, boardBody)
	steps := []SmokeStep{step("console-create-form", boolErr(withToken.Get("token") != "" && withToken.Get("verb") == "create",
		"board rendered no tokened create form"), "")}
	before, beforeErr := treeDigest(storeDir)
	if beforeErr != nil {
		return append(steps, step("console-refusal-no-store-effect", beforeErr, ""))
	}
	withoutToken := url.Values{}
	for key, values := range withToken {
		withoutToken[key] = values
	}
	withoutToken.Del("token")
	steps = append(steps,
		checkRefused("console-refuse-cross-origin", base, "http://attacker.invalid", withToken, "unexpected Origin"),
		checkRefused("console-refuse-missing-token", base, base, withoutToken, "invalid form token"))
	after, afterErr := treeDigest(storeDir)
	if afterErr == nil && after != before {
		afterErr = fmt.Errorf("ticket store changed after refused mutations")
	}
	return append(steps, step("console-refusal-no-store-effect", afterErr, ""))
}

// fillHiddenInputs copies the first value of each hidden input in fragment
// into form, leaving fields already set untouched.
func fillHiddenInputs(form url.Values, fragment string) {
	for _, match := range hiddenInputPattern.FindAllStringSubmatch(fragment, -1) {
		if form.Get(match[1]) == "" {
			form.Set(match[1], html.UnescapeString(match[2]))
		}
	}
}

var ticketFormPattern = regexp.MustCompile(`(?s)<form class="ticket-form" method="post" action="/mutate">.*?</form>`)

// consoleControlVerbs are the per-ticket verbs the detail page must render as
// a control, enabled or visibly disabled (PUB-V0-004, LAC-V0-013).
var consoleControlVerbs = []string{"hold", "release-hold", "archive", "restore", "reopen"}

// checkConsolePages covers the PUB-V0-004 console pages that need no browser:
// every per-ticket control rendered, each disabled one with its reason; the
// detail page's own title/body form applied through /mutate and visible on
// re-read; and the evidence dashboard compiled by the real snapshot binary.
func checkConsolePages(base, ticketID, detailBody string) []SmokeStep {
	missing := []string{}
	for _, verb := range consoleControlVerbs {
		if !strings.Contains(detailBody, `<input type="hidden" name="verb" value="`+verb+`">`) {
			missing = append(missing, verb)
		}
	}
	disabled := strings.Count(detailBody, `<button type="submit" disabled>`)
	reasons := strings.Count(detailBody, `<div class="disabled-reason">disabled: `)
	controlsErr := boolErr(len(missing) == 0, fmt.Sprintf("detail page renders no control for %v", missing))
	if controlsErr == nil {
		controlsErr = boolErr(disabled == reasons, fmt.Sprintf("%d disabled controls but %d reasons", disabled, reasons))
	}
	steps := []SmokeStep{step("console-detail-controls", controlsErr, fmt.Sprintf("%d disabled, each with a reason", disabled))}

	const editedTitle = "Companion smoke ticket (edited in console)"
	edit := url.Values{"title": {editedTitle}, "body": {""}, "milestone": {""}}
	for _, form := range ticketFormPattern.FindAllString(detailBody, -1) {
		if strings.Contains(form, `name="verb" value="refine"`) {
			fillHiddenInputs(edit, form)
		}
	}
	status, body, err := postForm(base, base, edit)
	if err == nil {
		err = boolErr(status == "200 OK" && strings.Contains(body, "The owning tool applied this change."),
			fmt.Sprintf("edit form was not applied (%s)", status))
	}
	if err == nil {
		_, reread, rereadErr := httpGet(base + "/ticket?id=" + url.QueryEscape(ticketID))
		err = rereadErr
		if err == nil {
			err = boolErr(strings.Contains(reread, editedTitle), "edited title absent from the re-read detail page")
		}
	}
	steps = append(steps, step("console-edit", err, status))

	evidenceStatus, evidenceBody, evidenceErr := httpGet(base + "/evidence")
	if evidenceErr == nil {
		evidenceErr = boolErr(strings.Contains(evidenceBody, "<h2>Observation boundary</h2>"),
			"evidence page rendered no compiled snapshot")
	}
	return append(steps, step("console-evidence", evidenceErr, evidenceStatus))
}

var consoleLinkPattern = regexp.MustCompile(`href="(/(?:code|requirement)\?[^"]*)"`)

const consoleGapMarker = `class="unknown">gap: `

// checkConsoleLinks covers the PUB-V0-004 requirement/code link pages over the
// committed link fixture: the requirement page links its cited file, the code
// page links back, every link is pinned to the fixture commit and resolves to
// a rendered page, and an uncited requirement and an uncommitted citation
// each render an explicit gap with no link.
func checkConsoleLinks(base, commit string) []SmokeStep {
	_, requirementBody, err := httpGet(base + "/requirement?id=SMK-V0-001")
	codeLinks, err := followLinks(base, commit, requirementBody, err, "/code?", "<h2>Requirements citing this path</h2>")
	steps := []SmokeStep{step("console-requirement-links", err, fmt.Sprintf("%d code links", len(codeLinks)))}

	var backLinks []string
	for _, body := range codeLinks {
		links, followErr := followLinks(base, commit, body, nil, "/requirement?id=SMK-V0-001&", "<th>clause</th>")
		err, backLinks = errors.Join(err, followErr), append(backLinks, links...)
	}
	steps = append(steps, step("console-code-links", err, fmt.Sprintf("%d requirement links", len(backLinks))))

	var gapErr error
	for _, id := range []string{"SMK-V0-002", "SMK-V0-003"} {
		_, body, getErr := httpGet(base + "/requirement?id=" + id)
		gapErr = errors.Join(gapErr, getErr, boolErr(getErr != nil || strings.Contains(body, consoleGapMarker), id+" rendered no explicit gap"),
			boolErr(len(consoleLinkPattern.FindAllString(body, -1)) == 0, id+" rendered a link for missing evidence"))
	}
	return append(steps, step("console-link-gaps", gapErr, "SMK-V0-002, SMK-V0-003"))
}

// followLinks requires body to hold at least one console link with the given
// prefix, every console link in body to be pinned to commit, and every link
// with the prefix to resolve to a page containing marker. It returns the
// resolved pages' bodies.
func followLinks(base, commit, body string, readErr error, prefix, marker string) ([]string, error) {
	if readErr != nil {
		return nil, readErr
	}
	var pages []string
	var err error
	for _, match := range consoleLinkPattern.FindAllStringSubmatch(body, -1) {
		link := html.UnescapeString(match[1])
		target, parseErr := url.Parse(link)
		err = errors.Join(err, parseErr, boolErr(parseErr != nil || target.Query().Get("at") == commit, link+" is not pinned to "+commit))
		if !strings.HasPrefix(link, prefix) || parseErr != nil {
			continue
		}
		_, page, getErr := httpGet(base + link)
		err = errors.Join(err, getErr, boolErr(getErr != nil || strings.Contains(page, marker), link+" is a dangling target"))
		pages = append(pages, page)
	}
	return pages, errors.Join(err, boolErr(len(pages) > 0, "no "+prefix+" link rendered"))
}

func checkRefused(name, base, origin string, form url.Values, reason string) SmokeStep {
	status, body, err := postForm(base, origin, form)
	if err != nil {
		return step(name, err, "")
	}
	refused := status == "403 Forbidden" && strings.Contains(body, reason)
	return step(name, boolErr(refused, fmt.Sprintf("want 403 %q, got %s", reason, status)), status)
}

// postForm sends form to base's /mutate with the given Origin header.
func postForm(base, origin string, form url.Values) (string, string, error) {
	request, err := http.NewRequest(http.MethodPost, base+"/mutate", strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", origin)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(request)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.Status, string(body), nil
}

// treeDigest hashes every path, mode and regular-file content under root.
func treeDigest(root string) (string, error) {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(hash, "%s\x00%o\x00", path, info.Mode())
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		hash.Write(data)
		return err
	})
	return hex.EncodeToString(hash.Sum(nil)), err
}

func reserveLoopbackAddr() (string, net.Listener, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	return l.Addr().String(), l, nil
}

func minimalRunEnv(home string) []string {
	return []string{
		"PATH=/usr/bin:/bin",
		"HOME=" + home,
		"LANG=C",
		"LC_ALL=C",
	}
}

func lookGit() (string, error) {
	if p := strings.TrimSpace(os.Getenv("CORVINT_COMPANION_GIT")); p != "" {
		return p, nil
	}
	return "/usr/bin/git", nil
}
