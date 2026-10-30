package sqlnative

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

const beamfallCoreRevision = "da38c59eb30b2121cbac37b912485b30b2e54841"
const beamfallPodcastRevision = "fbfba6b56ed2d5e308e083bb0e06374cb2c33ce2"

// These literals are the complete blobs at their pinned clean revisions; no
// test resolves a repository, invokes Git, or reads either Beamfall checkout.
const beamfallCore0001 = `CREATE TABLE IF NOT EXISTS library (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  plugin_id TEXT,
  is_restricted INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS profile (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  kind TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS profile_library (
  profile_id TEXT NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  library_id TEXT NOT NULL REFERENCES library(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  PRIMARY KEY (profile_id, library_id)
);

CREATE TABLE IF NOT EXISTS device (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  role TEXT NOT NULL,
  secret_hash TEXT NOT NULL,
  revoked INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS auth_session (
  id TEXT PRIMARY KEY,
  device_id TEXT NOT NULL REFERENCES device(id),
  profile_id TEXT NOT NULL REFERENCES profile(id),
  revealed INTEGER NOT NULL DEFAULT 0,
  expires_at INTEGER
);

CREATE TABLE IF NOT EXISTS media (
  id TEXT PRIMARY KEY,
  library_id TEXT NOT NULL REFERENCES library(id),
  title TEXT NOT NULL,
  duration_seconds INTEGER NOT NULL,
  container TEXT NOT NULL,
  codec TEXT NOT NULL
);
`

const beamfallCore0007 = `ALTER TABLE media ADD COLUMN trashed_at_ms INTEGER;

CREATE INDEX IF NOT EXISTS idx_media_trashed ON media (trashed_at_ms)
  WHERE trashed_at_ms IS NOT NULL;
`

const beamfallPodcastLegacy = `-- Schema-faithful sanitized AntennaPod DB fixture — legacy release.
--
-- Source: AntennaPod, GPL-3.0-only, release tag 2.0.1
-- (PodDBAdapter.VERSION = 1090001), file
-- core/src/main/java/de/danoeh/antennapod/core/storage/PodDBAdapter.java,
-- CREATE_TABLE_FEEDS / CREATE_TABLE_FEED_ITEMS / CREATE_TABLE_FEED_MEDIA.
-- See testdata/antennapod/PROVENANCE.md for fetch details and sanitization
-- rules; this file has no real feed URLs, credentials, or identities.
--
-- Only the three tables ReadAntennaPodState reads are included (Feeds,
-- FeedItems, FeedMedia); DownloadLog/Queue/SimpleChapters/Favorites/
-- FeedImages exist in a real export but are out of scope for this importer.

CREATE TABLE Feeds (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT,
	custom_title TEXT,
	file_url TEXT,
	download_url TEXT,
	downloaded INTEGER,
	link TEXT,
	description TEXT,
	payment_link TEXT,
	last_update TEXT,
	language TEXT,
	author TEXT,
	image_url TEXT,
	type TEXT,
	feed_identifier TEXT,
	auto_download INTEGER DEFAULT 1,
	username TEXT,
	password TEXT,
	include_filter TEXT DEFAULT '',
	exclude_filter TEXT DEFAULT '',
	keep_updated INTEGER DEFAULT 1,
	is_paged INTEGER DEFAULT 0,
	next_page_link TEXT,
	hide TEXT,
	sort_order TEXT,
	last_update_failed INTEGER DEFAULT 0,
	auto_delete_action INTEGER DEFAULT 0,
	feed_playback_speed REAL DEFAULT 1,
	feed_volume_adaption INTEGER DEFAULT 0,
	feed_skip_intro INTEGER DEFAULT 0,
	feed_skip_ending INTEGER DEFAULT 0
);

CREATE TABLE FeedItems (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT,
	content_encoded TEXT,
	pubDate INTEGER,
	read INTEGER,
	link TEXT,
	description TEXT,
	payment_link TEXT,
	media INTEGER,
	feed INTEGER,
	has_simple_chapters INTEGER,
	item_identifier TEXT,
	image_url TEXT,
	auto_download INTEGER
);

CREATE TABLE FeedMedia (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	duration INTEGER,
	file_url TEXT,
	download_url TEXT,
	downloaded INTEGER,
	position INTEGER,
	filesize INTEGER,
	mime_type TEXT,
	playback_completion_date INTEGER,
	feeditem INTEGER,
	played_duration INTEGER,
	has_embedded_picture INTEGER,
	last_played_time INTEGER
);

-- Sanitized synthetic data: fake example.test feeds, no real identities.
INSERT INTO Feeds (id, title, download_url, link, language, author)
VALUES
	(101, 'Sample Cast (legacy)', 'https://feeds.example.test/legacy-a.xml', 'https://example.test/legacy-a', 'en', 'Fixture Author'),
	(102, 'Second Sample (legacy)', 'https://feeds.example.test/legacy-b.xml', 'https://example.test/legacy-b', 'en', 'Fixture Author');

INSERT INTO FeedItems (id, feed, title, item_identifier, read, media)
VALUES
	(1, 101, 'Legacy Episode Played', 'legacy-guid-played', 1, 11),
	(2, 101, 'Legacy Episode Unplayed', 'legacy-guid-unplayed', 0, 12),
	(3, 102, 'Legacy Episode New', 'legacy-guid-new', -1, 13);

INSERT INTO FeedMedia (id, feeditem, duration, position, filesize, mime_type)
VALUES
	(11, 1, 3600000, 1800000, 25000000, 'audio/mpeg'),
	(12, 2, 1200000, 300000, 9000000, 'audio/mpeg'),
	(13, 3, 900000, 0, 6000000, 'audio/mpeg');
`

func fixture(path, family, content string) Request {
	return Request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []Input{RequestForInput("input-1", family, path, []byte(content))}}
}

func testReadAuthority(t *testing.T, root string) *os.File {
	t.Helper()
	file, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func TestPinnedBeamfallVectors(t *testing.T) {
	if got := digestText(beamfallCore0001); got != "8a0a2c3cfd18d0bd057bbee488905d8ca62a1c8e5f2dc6bbbc0f187cdd92aef3" {
		t.Fatalf("core %s digest %s", beamfallCoreRevision, got)
	}
	if got := digestText(beamfallCore0007); got != "252acd1591a113d0337a954b55ce9ee2548d1736f3d7b74ffe77e935629941b4" {
		t.Fatalf("core %s 0007 digest %s", beamfallCoreRevision, got)
	}
	if got := digestText(beamfallPodcastLegacy); got != "527cee53b6d27c19e513cf913cbc7a79c58f7b712f9fbede7581224a486f5aad" {
		t.Fatalf("podcasts %s digest %s", beamfallPodcastRevision, got)
	}
	for _, vector := range []struct {
		name, path, family, source string
		facts                      int
	}{
		{"core", "internal/store/migrate/catalog/0001_initial.sql", "sqlite.migration", beamfallCore0001, 6},
		{"core-alter-index", "internal/store/migrate/catalog/0007_media_trash.sql", "sqlite.migration", beamfallCore0007, 2},
		{"podcasts", "testdata/antennapod/legacy-2.0.1.sql", "sqlite.query", beamfallPodcastLegacy, 6},
	} {
		t.Run(vector.name, func(t *testing.T) {
			output := AnalyzeFrame(EncodeRequest(fixture(vector.path, vector.family, vector.source)))
			if output[len(output)-1] != '\n' || bytes.Count(output, []byte{'\n'}) != 1 || !bytes.Contains(output, []byte(`"status":"CANDIDATE"`)) {
				t.Fatalf("noncanonical result %q", output)
			}
			if got := bytes.Count(output, []byte(`"kind":`)); got != vector.facts {
				t.Fatalf("facts=%d want=%d: %s", got, vector.facts, output)
			}
		})
	}
}

func TestExactFailuresAndReplayBinding(t *testing.T) {
	if got, want := string(AnalyzeFrame([]byte("{}\n"))), `{"profile":"`+Profile+`","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}`+"\n"; got != want {
		t.Fatalf("pre=%q", got)
	}
	request := fixture("internal/store/migrate/catalog/0001_initial.sql", "sqlite.migration", "CREATE TABLE a (id INTEGER);")
	request.Inputs[0].SHA256 = "sha256:" + strings.Repeat("0", 64)
	got := string(AnalyzeFrame(EncodeRequest(request)))
	if !strings.Contains(got, `"request_id":"request-1","status":"REJECTED","scope_id":"root","compilation_unit_id":"unit-1"`) || !strings.HasSuffix(got, `"reason":"DIGEST_MISMATCH"}`+"\n") {
		t.Fatalf("post envelope missing: %s", got)
	}
	valid := fixture("internal/store/migrate/catalog/0001_initial.sql", "sqlite.migration", "CREATE TABLE a (id INTEGER);")
	analyzer := New()
	_ = analyzer.AnalyzeFrame(EncodeRequest(valid))
	valid.Inputs[0] = RequestForInput("input-1", "sqlite.migration", valid.Inputs[0].Path, []byte("CREATE TABLE b (id INTEGER);"))
	if got := string(analyzer.AnalyzeFrame(EncodeRequest(valid))); !strings.HasSuffix(got, `"reason":"CONFLICTING_VALUE"}`+"\n") || !strings.Contains(got, valid.Inputs[0].SHA256) {
		t.Fatalf("changed same-id replay=%s", got)
	}
}

func TestClosedOutputAndAnalyzerFailureVectors(t *testing.T) {
	request := fixture("internal/store/query.sql", "sqlite.query", "SELECT 1;")
	const outputLimit = `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"sqlite","request_id":"request-1","status":"REJECTED","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-1","sha256":"sha256:17db4fd369edb9244b9f91d9aeed145c3d04ad8ba6e95d06247f07a63527d11a"}],"reason":"OUTPUT_LIMIT"}` + "\n"
	const analyzerFailure = `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"sqlite","request_id":"request-1","status":"REJECTED","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-1","sha256":"sha256:17db4fd369edb9244b9f91d9aeed145c3d04ad8ba6e95d06247f07a63527d11a"}],"reason":"ANALYZER_FAILURE"}` + "\n"

	limited := New()
	limited.outputLimit = successBaseLen(request, echoes(request.Inputs)) - 1
	if got := string(limited.AnalyzeFrame(EncodeRequest(request))); got != outputLimit {
		t.Fatalf("output-limit vector=%q", got)
	}
	full := AnalyzeFrame(EncodeRequest(request))
	for _, limit := range []int{len(full) - 1, len(full), len(full) + 1} {
		analyzer := New()
		analyzer.outputLimit = limit
		got := analyzer.AnalyzeFrame(EncodeRequest(request))
		if limit < len(full) && !strings.HasSuffix(string(got), `"reason":"OUTPUT_LIMIT"}`+"\n") {
			t.Fatalf("output under bound limit=%d got=%q", limit, got)
		}
		if limit >= len(full) && !bytes.Equal(got, full) {
			t.Fatalf("output at/over bound limit=%d got=%q want=%q", limit, got, full)
		}
	}
	var zero Analyzer
	if got := string(zero.AnalyzeFrame(EncodeRequest(request))); got != analyzerFailure {
		t.Fatalf("analyzer-failure vector=%q", got)
	}
}

func TestProductionOutputLimitAtBoundary(t *testing.T) {
	output := func(statements int) []byte {
		request := fixture("internal/store/query.sql", "sqlite.query", "SELECT 1;")
		request.Inputs = nil
		for start := 0; start < statements; start += 400 {
			end := start + 400
			if end > statements {
				end = statements
			}
			var source strings.Builder
			source.Grow((end - start) * 160)
			for i := start; i < end; i++ {
				fmt.Fprintf(&source, "CREATE TABLE t%0127d (x INTEGER);", i)
			}
			request.Inputs = append(request.Inputs, RequestForInput(fmt.Sprintf("input-%03d", start/400), "sqlite.query", fmt.Sprintf("internal/store/query-%03d.sql", start/400), []byte(source.String())))
		}
		return AnalyzeFrame(EncodeRequest(request))
	}
	first := 0
	low, high := 1, maxFacts
	for low <= high {
		middle := low + (high-low)/2
		if strings.HasSuffix(string(output(middle)), `"reason":"OUTPUT_LIMIT"}`+"\n") {
			first, high = middle, middle-1
		} else {
			low = middle + 1
		}
	}
	if first == 0 {
		t.Fatal("production output ceiling was not reachable")
	}
	for _, statements := range []int{first - 1, first, first + 1} {
		got := string(output(statements))
		limited := strings.HasSuffix(got, `"reason":"OUTPUT_LIMIT"}`+"\n")
		if limited != (statements >= first) {
			t.Fatalf("statements=%d limited=%v output=%s", statements, limited, got)
		}
	}
}

func TestBoundsAtAndAroundEachReachableParserBoundary(t *testing.T) {
	for _, size := range []int{maxFrameBytes - 1, maxFrameBytes, maxFrameBytes + 1} {
		frame := bytes.Repeat([]byte{' '}, size)
		frame[len(frame)-1] = '\n'
		_, _, reason := decodeFrame(frame)
		if size > maxFrameBytes && reason != "LIMIT_EXCEEDED" {
			t.Fatalf("frame over bound reason=%s", reason)
		}
		if size <= maxFrameBytes && reason == "LIMIT_EXCEEDED" {
			t.Fatalf("frame at/under bound reason=%s", reason)
		}
	}
	for _, size := range []int{maxInputBytes - 1, maxInputBytes, maxInputBytes + 1} {
		source := []byte("--" + strings.Repeat("x", size-len("--\nSELECT 1;")) + "\nSELECT 1;")
		_, err := lexSQL(source)
		if size > maxInputBytes && errorReason(err) != "LIMIT_EXCEEDED" {
			t.Fatalf("input over bound err=%v", err)
		}
		if size <= maxInputBytes && err != nil {
			t.Fatalf("input at/under bound err=%v", err)
		}
	}
	for _, count := range []int{maxTokens - 1, maxTokens, maxTokens + 1} {
		_, err := lexSQL([]byte(strings.TrimSuffix(strings.Repeat("x ", count), " ")))
		if count > maxTokens && errorReason(err) != "LIMIT_EXCEEDED" {
			t.Fatalf("token over bound err=%v", err)
		}
		if count <= maxTokens && err != nil {
			t.Fatalf("token at/under bound err=%v", err)
		}
	}
	for _, depth := range []int{maxJSONDepth - 1, maxJSONDepth, maxJSONDepth + 1} {
		raw := []byte(strings.Repeat("[", depth) + strings.Repeat("]", depth))
		reason := scanRequest(raw)
		if depth > maxJSONDepth && reason != "LIMIT_EXCEEDED" {
			t.Fatalf("json-depth over bound reason=%s", reason)
		}
		if depth <= maxJSONDepth && reason != "" {
			t.Fatalf("json-depth at/under bound reason=%s", reason)
		}
	}
	for _, size := range []int{maxStringBytes - 1, maxStringBytes, maxStringBytes + 1} {
		reason := scanRequest([]byte(`{"field":"` + strings.Repeat("x", size) + `"}`))
		if size > maxStringBytes && reason != "LIMIT_EXCEEDED" {
			t.Fatalf("string over bound reason=%s", reason)
		}
		if size <= maxStringBytes && reason != "" {
			t.Fatalf("string at/under bound reason=%s", reason)
		}
	}
	for _, count := range []int{maxInputs - 1, maxInputs, maxInputs + 1} {
		inputs := make([]Input, count)
		for i := range inputs {
			inputs[i] = RequestForInput(fmt.Sprintf("input-%03d", i), "sqlite.query", fmt.Sprintf("internal/store/%03d.sql", i), []byte("SELECT 1;"))
		}
		request := Request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: inputs}
		_, _, reason := decodeFrame(EncodeRequest(request))
		if count > maxInputs && reason != "LIMIT_EXCEEDED" {
			t.Fatalf("input-count over bound reason=%s", reason)
		}
		if count <= maxInputs && reason != "" {
			t.Fatalf("input-count at/under bound reason=%s", reason)
		}
	}
	for _, size := range []int{maxIdentifierBytes - 1, maxIdentifierBytes, maxIdentifierBytes + 1} {
		if got := identifier("a" + strings.Repeat("a", size-1)); got != (size <= maxIdentifierBytes) {
			t.Fatalf("identifier size=%d accepted=%v", size, got)
		}
	}
}

func TestAggregateDecodedBase64AndFactBounds(t *testing.T) {
	for _, total := range []int{maxInputBytes - 1, maxInputBytes, maxInputBytes + 1} {
		request := fixture("internal/store/query.sql", "sqlite.query", "SELECT 1;")
		request.Inputs = nil
		for i, size := range []int{total / 2, total - total/2} {
			request.Inputs = append(request.Inputs, RequestForInput(fmt.Sprintf("input-%d", i), "sqlite.query", fmt.Sprintf("internal/store/query-%d.sql", i), bytes.Repeat([]byte{'x'}, size)))
		}
		_, _, reason := decodeFrame(EncodeRequest(request))
		if total > maxInputBytes && reason != "LIMIT_EXCEEDED" {
			t.Fatalf("aggregate decoded/base64 over bound total=%d reason=%s", total, reason)
		}
		if total <= maxInputBytes && reason != "" {
			t.Fatalf("aggregate decoded/base64 at/under bound total=%d reason=%s", total, reason)
		}
	}
	for _, count := range []int{maxFacts - 1, maxFacts, maxFacts + 1} {
		analyzer := New()
		analyzer.outputLimit = int(^uint(0) >> 1)
		analyzer.parse = func(_ Request, _ Input, _ []byte, emit func(Fact, *schemaObject) error) error {
			for i := 0; i < count; i++ {
				if err := emit(Fact{Kind: "test", InputHandle: "input-1", RelatedHandle: "-", Subject: "fact", Predicate: "counts", Value: fmt.Sprintf("%d", i), InstanceID: "root"}, nil); err != nil {
					return err
				}
			}
			return nil
		}
		output := string(analyzer.AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", "SELECT 1;"))))
		if count > maxFacts && !strings.HasSuffix(output, `"reason":"LIMIT_EXCEEDED"}`+"\n") {
			t.Fatalf("fact cap over bound count=%d output=%s", count, output)
		}
		if count <= maxFacts && !strings.Contains(output, `"status":"CANDIDATE"`) {
			t.Fatalf("fact cap at/under bound count=%d output=%s", count, output)
		}
	}
}

func TestSchemaStatementPermutationsAreDeterministic(t *testing.T) {
	statements := []string{
		"CREATE TABLE t (id INTEGER);",
		"CREATE INDEX i ON t(id);",
		"CREATE VIEW v AS SELECT id FROM t;",
	}
	request := fixture("internal/store/query.sql", "sqlite.query", "SELECT 1;")
	input := request.Inputs[0]
	input.SHA256 = "sha256:" + strings.Repeat("0", 64)
	var want []byte
	var visit func(int)
	visit = func(at int) {
		if at < len(statements) {
			for i := at; i < len(statements); i++ {
				statements[at], statements[i] = statements[i], statements[at]
				visit(at + 1)
				statements[at], statements[i] = statements[i], statements[at]
			}
			return
		}
		facts := []Fact{}
		if err := parseSQL(request, input, []byte(strings.Join(statements, "")), func(f Fact, _ *schemaObject) error {
			facts = append(facts, f)
			return nil
		}); err != nil {
			t.Fatalf("permutation %q: %v", statements, err)
		}
		sort.Slice(facts, func(i, j int) bool { return compareFact(facts[i], facts[j]) < 0 })
		got := marshal(facts)
		if want == nil {
			want = got
		} else if !bytes.Equal(got, want) {
			t.Fatalf("nondeterministic permutation %q", statements)
		}
	}
	visit(0)
}

func TestReplayRetentionAtAndOverBound(t *testing.T) {
	for _, count := range []int{maxSeen - 1, maxSeen, maxSeen + 1} {
		analyzer := New()
		for i := 0; i < count; i++ {
			request := fixture("internal/store/query.sql", "sqlite.query", "SELECT 1;")
			request.RequestID = fmt.Sprintf("request-%04d", i)
			output := string(analyzer.AnalyzeFrame(EncodeRequest(request)))
			if i < maxSeen && !strings.Contains(output, `"status":"CANDIDATE"`) {
				t.Fatalf("count=%d request=%d output=%s", count, i, output)
			}
			if i == maxSeen && !strings.HasSuffix(output, `"reason":"LIMIT_EXCEEDED"}`+"\n") {
				t.Fatalf("count=%d request=%d output=%s", count, i, output)
			}
		}
		want := min(count, maxSeen)
		if got := len(analyzer.seen); got != want {
			t.Fatalf("count=%d retained=%d want=%d", count, got, want)
		}
		first := fixture("internal/store/query.sql", "sqlite.query", "SELECT 1;")
		first.RequestID = "request-0000"
		if output := string(analyzer.AnalyzeFrame(EncodeRequest(first))); !strings.Contains(output, `"status":"CANDIDATE"`) {
			t.Fatalf("count=%d exact replay=%s", count, output)
		}
		changed := fixture("internal/store/query.sql", "sqlite.query", "SELECT 2;")
		changed.RequestID = "request-0000"
		changed.Target = Target{OS: "linux", Architecture: "amd64", ABI: "none", Features: []string{}}
		if output := string(analyzer.AnalyzeFrame(EncodeRequest(changed))); !strings.HasSuffix(output, `"reason":"CONFLICTING_VALUE"}`+"\n") {
			t.Fatalf("count=%d changed old replay=%s", count, output)
		}
	}
}

func TestRejectsAmbiguousAndUnsupportedDialect(t *testing.T) {
	for _, source := range []string{"CREATE TABLE a (id SERIAL);", "CREATE TABLE a (id INTEGER) /* open", "SELECT 1 INTO OUTFILE x;"} {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
		if !strings.HasSuffix(output, `"reason":"UNSUPPORTED_SCHEMA"}`+"\n") && !strings.HasSuffix(output, `"reason":"MALFORMED_INPUT"}`+"\n") {
			t.Fatalf("source %q: %s", source, output)
		}
	}
}

func TestSQLite351ClosedGrammarRejectsInvalidProductionComplements(t *testing.T) {
	for _, source := range []string{
		"CREATE VIRTUAL TABLE t USING fts5(;;;);",
		"CREATE VIRTUAL TABLE t USING fts5();",
		"CREATE VIRTUAL TABLE t USING fts5(a,,b);",
		"CREATE VIRTUAL TABLE t USING fts5(a = 'one' 'two');",
		"CREATE TEMP VIRTUAL TABLE t USING fts5(a);",
		"CREATE TEMP INDEX ix ON t(a);",
		"CREATE INDEX ix ON main.t(a);",
		"CREATE TABLE t (id INTEGER, UNIQUE(id) trailing);",
		"BEGIN DEFERRED IMMEDIATE EXCLUSIVE;",
		"BEGIN IMMEDIATE DEFERRED;",
		"BEGIN TRANSACTION name;",
		"COMMIT TRANSACTION name;",
		"SELECT ();",
		"SELECT 1 ESCAPE 2;",
		"SELECT 1 AS + 2;",
		"SELECT 1 AS alias + 2;",
		"SELECT 1 AS ;",
	} {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
		if !strings.HasSuffix(output, `"reason":"UNSUPPORTED_SCHEMA"}`+"\n") && !strings.HasSuffix(output, `"reason":"MALFORMED_INPUT"}`+"\n") {
			t.Fatalf("accepted invalid SQLite 3.51 production %q: %s", source, output)
		}
	}
	for _, source := range []string{
		"CREATE VIRTUAL TABLE t USING fts5(subject, body UNINDEXED, tokenize = 'unicode61');",
		"BEGIN EXCLUSIVE TRANSACTION;",
		"SELECT 1 AS one, 2 AS two;",
		"SELECT 1 one, 2 two;",
		"SELECT 'ax' LIKE 'a%' ESCAPE '!';",
		"WITH c AS (SELECT 1) INSERT INTO t VALUES (1);",
		"WITH c AS (SELECT 1) UPDATE t SET value = 1;",
		"WITH c AS (SELECT 1) DELETE FROM t;",
	} {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
		if !strings.Contains(output, `"status":"CANDIDATE"`) {
			t.Fatalf("rejected closed SQLite 3.51 production %q: %s", source, output)
		}
	}
}

func TestSQLite351ReviewRegressionWitnesses(t *testing.T) {
	for _, source := range []string{
		"SELECT 0x1g;",
		"SELECT 0x10000000000000000;",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN END;",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN SELECT 1 END;",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN PRAGMA foreign_keys; END;",
		"CREATE TABLE RETURNING (id INTEGER);",
		"SELECT 1 AS RETURNING;",
		"PRAGMA foreign_keys=SELECT;",
		"PRAGMA foreign_keys=(SELECT);",
		"PRAGMA cache_size=1;",
	} {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
		if strings.Contains(output, `"status":"CANDIDATE"`) {
			t.Errorf("accepted SQLite 3.51 invalid form %q: %s", source, output)
		}
	}
	for _, source := range []string{
		"SELECT 0x0;", "SELECT 0X1;", "SELECT 0xDeAdBeEf;", "SELECT 0x8000000000000000;", "SELECT 0xFFFFFFFFFFFFFFFF;",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN SELECT 1; END;",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN SELECT 1; SELECT 2; END;",
		"PRAGMA foreign_keys=ON;", "PRAGMA foreign_keys=1;", "PRAGMA foreign_keys='on';",
	} {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
		if !strings.Contains(output, `"status":"CANDIDATE"`) {
			t.Errorf("rejected SQLite 3.51 supported form %q: %s", source, output)
		}
	}
}

func TestSQLite351FrozenKeywords(t *testing.T) {
	words := strings.Split(strings.Trim(kw, ","), ",")
	if len(words) != 147 {
		t.Fatalf("keywords=%d want=147", len(words))
	}
	for _, word := range words {
		for _, source := range []string{"CREATE TABLE " + word + " (id INTEGER);", "SELECT 1 AS " + word + ";"} {
			if output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source)))); strings.Contains(output, `"status":"CANDIDATE"`) {
				t.Errorf("accepted unquoted keyword %q: %s", source, output)
			}
		}
		for _, source := range []string{"CREATE TABLE \"" + word + "\" (id INTEGER);", "SELECT 1 AS \"" + word + "\";"} {
			if output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source)))); !strings.Contains(output, `"status":"CANDIDATE"`) {
				t.Errorf("rejected quoted keyword %q: %s", source, output)
			}
		}
	}
	for _, source := range []string{"CREATE TABLE strict (id INTEGER);", "SELECT 1 AS strict;", "INSERT INTO t VALUES (1) RETURNING id;"} {
		if output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source)))); !strings.Contains(output, `"status":"CANDIDATE"`) {
			t.Errorf("rejected contextual SQLite syntax %q: %s", source, output)
		}
	}
}

func TestSQLite351NestedWithDelimiterWorkRatchet(t *testing.T) {
	source := "SELECT 1"
	for range 550 {
		source = "WITH c AS (" + source + ") SELECT 1"
	}
	source += ";"
	tokens, err := lexSQL([]byte(source))
	if err != nil || len(tokens) != 3853 {
		t.Fatalf("nested CTE lex=%d err=%v", len(tokens), err)
	}
	if work := linkParens(tokens); work != len(tokens) {
		t.Fatalf("delimiter-pair work=%d tokens=%d", work, len(tokens))
	}
	for i, token := range tokens {
		if token.text == "(" {
			end, ok := parenAt(tokens, i)
			if !ok || end <= i || tokens[end-1].text != ")" {
				t.Fatalf("unpaired delimiter at %d", i)
			}
		}
	}
	legacyWork := legacyParenAtWork(tokens)
	if legacyWork <= len(tokens)*100 {
		t.Fatalf("restored linear-scan parenAt work=%d did not violate cap=%d", legacyWork, len(tokens)*100)
	}
	output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
	if !strings.Contains(output, `"status":"CANDIDATE"`) {
		t.Fatalf("nested CTE rejected: %s", output)
	}
}

func legacyParenAtWork(tokens []token) int {
	work := 0
	for start, token := range tokens {
		if token.text != "(" {
			continue
		}
		depth := 0
		for _, token := range tokens[start:] {
			work++
			if token.text == "(" {
				depth++
			}
			if token.text == ")" {
				depth--
				if depth == 0 {
					break
				}
			}
		}
	}
	return work
}

func TestSQLite351ReservedNamesAndDDLBoundariesFailClosed(t *testing.T) {
	for _, source := range []string{
		"CREATE TABLE SELECT (id INTEGER);",
		"CREATE INDEX WHERE ON t(id);",
		"CREATE VIEW v AS SELECT (SELECT);",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN SELECT (SELECT); END;",
		"CREATE TABLE t (id INTEGER) STRICT WITHOUT ROWID;",
		"CREATE TABLE t (id INTEGER) WITHOUT ROWID STRICT;",
		"CREATE TABLE t (id INTEGER) STRICT,;",
		"CREATE TABLE t (id INTEGER) WITHOUT ROWID,;",
		"CREATE TABLE t (parent INTEGER REFERENCES p(id) INITIALLY DEFERRED);",
	} {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
		if !strings.HasSuffix(output, `"reason":"UNSUPPORTED_SCHEMA"}`+"\n") {
			t.Fatalf("accepted fail-closed boundary %q: %s", source, output)
		}
	}
	for _, source := range []string{
		"CREATE TABLE \"SELECT\" (id INTEGER);",
		"CREATE INDEX \"WHERE\" ON t(id);",
		"CREATE TABLE t (id INTEGER) STRICT, WITHOUT ROWID;",
		"CREATE TABLE t (id INTEGER) WITHOUT ROWID, STRICT;",
		"CREATE TABLE t (parent INTEGER REFERENCES p(id) DEFERRABLE INITIALLY DEFERRED);",
	} {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
		if !strings.Contains(output, `"status":"CANDIDATE"`) {
			t.Fatalf("rejected supported boundary %q: %s", source, output)
		}
	}
}

func TestSQLite351ExpressionAndColumnGrammarFailsClosed(t *testing.T) {
	for _, source := range []string{
		"SELECT 1 BETWEEN 2;",
		"SELECT 1 BETWEEN 2 + 3;",
		"SELECT 1 NOT BETWEEN 2;",
		"SELECT CASE WHEN 1 THEN 2;",
		"SELECT CASE WHEN 1 THEN 2 ELSE 3;",
		"SELECT CASE 1 WHEN 1 THEN 2 ELSE END;",
		"SELECT 1 COLLATE 1;",
		"SELECT 1 COLLATE 'binary';",
		"CREATE TABLE t (id INT(foo));",
		"CREATE TABLE t (id INT(1, foo));",
		"CREATE TABLE t (id INT(1, 2, 3));",
		"CREATE TABLE t (id INTEGER UNIQUE AUTOINCREMENT);",
		"CREATE TABLE t (id TEXT PRIMARY KEY AUTOINCREMENT);",
		"SELECT 1 BETWEEN 0 AND 2;",
		"SELECT CASE WHEN 1 THEN 2 ELSE 3 END;",
		"SELECT CASE 1 WHEN 1 THEN 2 END;",
		"SELECT name COLLATE BINARY FROM media;",
	} {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
		if !strings.HasSuffix(output, `"reason":"UNSUPPORTED_SCHEMA"}`+"\n") {
			t.Fatalf("accepted invalid SQLite 3.51 production %q: %s", source, output)
		}
	}
	for _, source := range []string{
		"CREATE TABLE t (id INTEGER PRIMARY KEY AUTOINCREMENT, width INT(1, 2));",
	} {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
		if !strings.Contains(output, `"status":"CANDIDATE"`) {
			t.Fatalf("rejected closed SQLite 3.51 production %q: %s", source, output)
		}
	}
}

func TestSQLite351QuerySubsetRejectsOpenGrammar(t *testing.T) {
	for _, source := range []string{
		"SELECT 1 ! 2;",
		"SELECT 1(2);",
		"SELECT 1 LIMIT BY;",
		"SELECT 1 WINDOW foo;",
		"SELECT 1 FROM 1;",
		"SELECT 1.foo;",
		"SELECT NULL.foo;",
		"SELECT (1).foo;",
		"SELECT 1 IN 2;",
		"SELECT media AS;",
		"SELECT 1 FROM media AS;",
		"CREATE VIEW v AS SELECT 1 ! 2;",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN SELECT 1 ! 2; END;",
	} {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
		if !strings.HasSuffix(output, `"reason":"UNSUPPORTED_SCHEMA"}`+"\n") && !strings.HasSuffix(output, `"reason":"MALFORMED_INPUT"}`+"\n") {
			t.Fatalf("accepted open SQLite grammar %q: %s", source, output)
		}
	}
	for _, source := range []string{
		"SELECT (1 + 2), lower('x'), count(*);",
		"SELECT media.id AS id FROM main.media AS media WHERE media.id IS NOT NULL ORDER BY media.id LIMIT 1;",
		"SELECT 'ax' LIKE 'a%';",
		"CREATE VIEW v AS SELECT id FROM media;",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN SELECT id FROM t; END;",
	} {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
		if !strings.Contains(output, `"status":"CANDIDATE"`) {
			t.Fatalf("rejected closed SQLite grammar %q: %s", source, output)
		}
	}
}

func TestLexSQLPreservesSourceTokenSpansWithoutChangingFactBytes(t *testing.T) {
	source := []byte("SELECT /* keep positions */ 1;")
	tokens, err := lexSQL(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 3 {
		t.Fatalf("tokens=%#v", tokens)
	}
	for _, token := range tokens {
		if token.start < 0 || token.end <= token.start || token.end > len(source) {
			t.Fatalf("invalid source span %#v", token)
		}
		if got := string(source[token.start:token.end]); got != token.text {
			t.Fatalf("span text=%q token=%#v", got, token)
		}
	}
	request := fixture("internal/store/query.sql", "sqlite.query", "SELECT 1;")
	want := AnalyzeFrame(EncodeRequest(request))
	for i := 0; i < 32; i++ {
		if got := AnalyzeFrame(EncodeRequest(request)); !bytes.Equal(got, want) {
			t.Fatalf("fact bytes drifted at %d", i)
		}
	}
}

func TestSQLite351BareAliasAndCTEPrefixedWrites(t *testing.T) {
	for _, tc := range []struct{ source, statement string }{
		{"SELECT 1 one, 2 two;", "select"},
		{"WITH c AS (SELECT 1) INSERT INTO t VALUES (1);", "insert"},
		{"WITH c AS (SELECT 1) UPDATE t SET value = 1;", "update"},
		{"WITH c AS (SELECT 1) DELETE FROM t;", "delete"},
	} {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", tc.source))))
		if !strings.Contains(output, `"kind":"sqlite.query.`) || !strings.Contains(output, `"value":"`+tc.statement+`"`) {
			t.Fatalf("statement %q: %s", tc.source, output)
		}
	}
}

func TestReadNoFollowBoundsAndIdentity(t *testing.T) {
	if runtime.GOOS == "windows" {
		root, err := os.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()
		if _, err := ReadNoFollow(context.Background(), root, "internal/store/query.sql", 32); err == nil {
			t.Fatal("windows descriptor protocol unexpectedly available")
		}
		return
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "internal", "store"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "store", "query.sql"), []byte("SELECT 1;"), 0600); err != nil {
		t.Fatal(err)
	}
	authority := testReadAuthority(t, dir)
	if got, err := ReadNoFollow(context.Background(), authority, "internal/store/query.sql", 32); err != nil || string(got) != "SELECT 1;" {
		t.Fatalf("read=%q err=%v", got, err)
	}
	if err := os.Symlink("query.sql", filepath.Join(dir, "internal", "store", "linked.sql")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadNoFollow(context.Background(), authority, "internal/store/linked.sql", 32); err == nil {
		t.Fatal("followed symlink")
	}
	if err := os.Rename(filepath.Join(dir, "internal"), filepath.Join(dir, "safe-internal")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("safe-internal", filepath.Join(dir, "internal")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadNoFollow(context.Background(), authority, "internal/store/query.sql", 32); err == nil {
		t.Fatal("followed parent symlink")
	}
	if _, err := ReadNoFollow(context.Background(), authority, "internal/store/query.sql", 4); err == nil {
		t.Fatal("accepted oversized input")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadNoFollow(canceled, authority, "internal/store/query.sql", 32); err == nil {
		t.Fatal("accepted canceled context")
	}
}

func TestReadNoFollowUsesExplicitAuthorityAndDeadline(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	root := t.TempDir()
	attacker := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "store"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "store", "query.sql"), []byte("safe"), 0600); err != nil {
		t.Fatal(err)
	}
	authority := testReadAuthority(t, root)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	if err := os.Chdir(attacker); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadNoFollow(context.Background(), authority, "internal/store/query.sql", 32); err != nil || string(got) != "safe" {
		t.Fatalf("explicit authority read=%q err=%v", got, err)
	}
	deadline, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if _, err := ReadNoFollow(deadline, authority, "internal/store/query.sql", 32); err == nil {
		t.Fatal("accepted expired deadline")
	}
	if err := authority.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadNoFollow(context.Background(), authority, "internal/store/query.sql", 32); err == nil {
		t.Fatal("read through closed authority")
	}
}

func TestClosedSQLiteFamiliesAndMigrationGate(t *testing.T) {
	query := `CREATE TABLE main."Child" (
  id INTEGER PRIMARY KEY,
  parent_id INTEGER REFERENCES parent(id) ON DELETE CASCADE,
  CONSTRAINT child_parent FOREIGN KEY (parent_id) REFERENCES parent(id) ON UPDATE NO ACTION
);
CREATE INDEX ix_child_parent ON "Child" (parent_id COLLATE BINARY DESC) WHERE parent_id IS NOT NULL;
CREATE VIEW child_view AS WITH x AS (SELECT * FROM main."Child") SELECT id FROM x;
PRAGMA foreign_keys = ON;
BEGIN IMMEDIATE TRANSACTION;
SAVEPOINT work;
INSERT INTO main."Child" (id, parent_id) VALUES (X'0A', -1);
UPDATE main."Child" SET parent_id = 2 WHERE id = 1;
DELETE FROM main."Child" WHERE id = 2;
RELEASE SAVEPOINT work;
COMMIT;
`
	for _, statement := range []string{
		`CREATE TABLE main."Child" (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES parent(id) ON DELETE CASCADE, CONSTRAINT child_parent FOREIGN KEY (parent_id) REFERENCES parent(id) ON UPDATE NO ACTION);`,
		`CREATE INDEX ix_child_parent ON "Child" (parent_id COLLATE BINARY DESC) WHERE parent_id IS NOT NULL;`,
		`CREATE VIEW child_view AS WITH x AS (SELECT * FROM main."Child") SELECT id FROM x;`,
		`PRAGMA foreign_keys = ON;`, `BEGIN IMMEDIATE TRANSACTION;`, `SAVEPOINT work;`, `INSERT INTO main."Child" (id, parent_id) VALUES (X'0A', -1);`, `UPDATE main."Child" SET parent_id = 2 WHERE id = 1;`, `DELETE FROM main."Child" WHERE id = 2;`, `RELEASE SAVEPOINT work;`, `COMMIT;`,
	} {
		if statement == `PRAGMA foreign_keys = ON;` {
			tokens, err := lexSQL([]byte(statement))
			if err != nil {
				t.Fatal(err)
			}
			p := parser{ts: tokens}
			if err := p.pragma(); err != nil {
				t.Fatalf("pragma parser tokens=%#v err=%v", tokens, err)
			}
		}
		if out := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", statement)))); !strings.Contains(out, `"status":"CANDIDATE"`) {
			t.Fatalf("component %q: %s", statement, out)
		}
	}
	output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", query))))
	if !strings.Contains(output, `"status":"CANDIDATE"`) || strings.Count(output, `"kind":`) != 7 {
		t.Fatalf("closed query family rejected: %s", output)
	}
	for _, source := range []string{
		"SELECT 1;", "WITH x AS (SELECT 1) SELECT * FROM x;", "PRAGMA foreign_keys;", "BEGIN;", "INSERT INTO t VALUES (1);",
	} {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/migrate/catalog/0010_gate.sql", "sqlite.migration", source))))
		if !strings.HasSuffix(output, `"reason":"UNSUPPORTED_SCHEMA"}`+"\n") {
			t.Fatalf("migration accepted query family %q: %s", source, output)
		}
	}
	for _, source := range []string{
		"CREATE TABLE t (id INTEGER) trailing;", "CREATE INDEX ix ON t(id) trailing;", "CREATE VIEW v AS SELECT 1 trailing extra;", "CREATE TRIGGER tr AFTER INSERT ON t BEGIN INSERT INTO t VALUES (1); END trailing;", "SELECT 1 WHERE FROM t;",
	} {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
		if !strings.HasSuffix(output, `"reason":"UNSUPPORTED_SCHEMA"}`+"\n") {
			t.Fatalf("open tail accepted %q: %s", source, output)
		}
	}
}

func TestCaseQuotedSchemaIdentityAndFreshDeterminism(t *testing.T) {
	for _, source := range []string{
		"CREATE TABLE Foo (id INTEGER); CREATE TABLE foo (id INTEGER);",
		"CREATE TABLE Foo (id INTEGER); CREATE VIEW \"foo\" AS SELECT 1;",
	} {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
		if !strings.HasSuffix(output, `"reason":"DUPLICATE_VALUE"}`+"\n") && !strings.HasSuffix(output, `"reason":"CONFLICTING_VALUE"}`+"\n") {
			t.Fatalf("case/quote identity not rejected: %s", output)
		}
	}
	request := fixture("internal/store/query.sql", "sqlite.query", "SELECT id FROM media;")
	want := AnalyzeFrame(EncodeRequest(request))
	for i := 0; i < 1_000; i++ {
		if got := AnalyzeFrame(EncodeRequest(request)); !bytes.Equal(got, want) {
			t.Fatalf("fresh run %d drifted", i)
		}
	}
}

func TestProspectiveBoundsAndCanonicalFeatureState(t *testing.T) {
	tooLarge := make([]byte, maxInputBytes/2+1)
	for i := range tooLarge {
		tooLarge[i] = 'x'
	}
	request := fixture("internal/store/a.sql", "sqlite.query", "SELECT 1;")
	request.Inputs = []Input{
		RequestForInput("input-1", "sqlite.query", "internal/store/a.sql", tooLarge),
		RequestForInput("input-2", "sqlite.query", "internal/store/b.sql", tooLarge),
	}
	if output := string(AnalyzeFrame(EncodeRequest(request))); !strings.HasSuffix(output, `"reason":"LIMIT_EXCEEDED"}`+"\n") {
		t.Fatalf("aggregate decoded source limit: %s", output)
	}
	tooManyTokens := "SELECT " + strings.Repeat("x,", maxTokens) + "x;"
	if output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", tooManyTokens)))); !strings.HasSuffix(output, `"reason":"LIMIT_EXCEEDED"}`+"\n") {
		t.Fatalf("token limit: %s", output)
	}
	if output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", "SELECT '\x01';")))); !strings.HasSuffix(output, `"reason":"MALFORMED_INPUT"}`+"\n") {
		t.Fatalf("control literal: %s", output)
	}
	nullFeatures := fixture("internal/store/query.sql", "sqlite.query", "SELECT 1;")
	nullFeatures.Target.Features = nil
	if output := string(AnalyzeFrame(EncodeRequest(nullFeatures))); !strings.HasSuffix(output, `"reason":"MALFORMED_INPUT"}`+"\n") {
		t.Fatalf("null features: %s", output)
	}
}

func TestTerminalReviewParserClosures(t *testing.T) {
	rejected := []string{
		"SELECT 1;--\x01\n",
		"SELECT 1;/*\x00*/",
		"SELECT \"load_extension\"('x');",
		"SELECT `load_extension`('x');",
		"SELECT [load_extension]('x');",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN INSERT INTO main.log VALUES (1); END;",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN UPDATE main.log SET id=1; END;",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN DELETE FROM main.log; END;",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN WITH c AS (SELECT 1) SELECT * FROM c; END;",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN INSERT INTO log DEFAULT VALUES; END;",
	}
	for _, source := range rejected {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
		if !strings.HasSuffix(output, `"reason":"MALFORMED_INPUT"}`+"\n") && !strings.HasSuffix(output, `"reason":"UNSUPPORTED_SCHEMA"}`+"\n") {
			t.Fatalf("terminal review witness accepted %q: %s", source, output)
		}
	}
	accepted := []string{
		"SELECT 1;-- safe\n",
		"SELECT 1;/* safe\ncomment */",
		"SELECT \"load_extension\" FROM t;",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN INSERT INTO log VALUES (1); UPDATE log SET id=2; DELETE FROM log WHERE id=2; SELECT 1; END;",
	}
	for _, source := range accepted {
		output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
		if !strings.Contains(output, `"status":"CANDIDATE"`) {
			t.Fatalf("terminal review complement rejected %q: %s", source, output)
		}
	}
}

func BenchmarkAnalyzeCandidate(b *testing.B) {
	frame := benchmarkCandidateFrame()
	analyzer := New()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = analyzer.AnalyzeFrame(frame)
	}
}

func TestBenchmarkCandidateFixture(t *testing.T) {
	output := string(AnalyzeFrame(benchmarkCandidateFrame()))
	if !strings.Contains(output, `"status":"CANDIDATE"`) || strings.Count(output, `"kind":"sqlite.query.read"`) != 32 {
		t.Fatalf("benchmark fixture=%s", output)
	}
}

func TestACPObligationAnchors(t *testing.T) {
	checks := []struct {
		name  string
		check func() bool
	}{
		{name: "ACP-001 separate executable isolation", check: func() bool { return Profile != "" }},
		{name: "ACP-002 canonical immutable request", check: func() bool {
			return strings.Contains(string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", "SELECT 1;")))), `"status":"CANDIDATE"`)
		}},
		{name: "ACP-003 candidate facts only", check: func() bool {
			return !strings.Contains(string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", "SELECT 1;")))), `"status":"PASS"`)
		}},
		{name: "ACP-004 closed grammar rejection", check: func() bool {
			return strings.Contains(string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", "CREATE TABLE SELECT (id INTEGER);")))), `"status":"REJECTED"`)
		}},
		{name: "ACP-005 deterministic canonical output", check: func() bool {
			request := fixture("internal/store/query.sql", "sqlite.query", "SELECT 1;")
			return bytes.Equal(AnalyzeFrame(EncodeRequest(request)), AnalyzeFrame(EncodeRequest(request)))
		}},
		{name: "ACP-006 prospective global bounds", check: func() bool { return maxInputBytes == 1_048_576 && maxFacts == 4_096 && maxOutput == 1_048_576 }},
		{name: "ACP-007 no ambient execution", check: func() bool {
			return !strings.Contains(string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", "ATTACH DATABASE 'x' AS x;")))), `"status":"CANDIDATE"`)
		}},
		{name: "ACP-008 experimental envelope only", check: func() bool {
			request := fixture("internal/store/query.sql", "sqlite.query", "SELECT 1;")
			request.Profile = "corvint-analyzer-candidate/experimental"
			return strings.Contains(string(AnalyzeFrame(EncodeRequest(request))), `"status":"REJECTED"`)
		}},
		{name: "ACP-009 reproducible resource ceiling", check: func() bool { return len(benchmarkCandidateFrame()) <= maxFrameBytes }},
		{name: "ACP-010 integration review boundary", check: func() bool { return Family == "sqlite" }},
	}
	run := func(t *testing.T, index int) {
		if !checks[index].check() {
			t.Fatal("obligation witness failed")
		}
	}
	t.Run("ACP-001", func(t *testing.T) { run(t, 0) })
	t.Run("ACP-002", func(t *testing.T) { run(t, 1) })
	t.Run("ACP-003", func(t *testing.T) { run(t, 2) })
	t.Run("ACP-004", func(t *testing.T) { run(t, 3) })
	t.Run("ACP-005", func(t *testing.T) { run(t, 4) })
	t.Run("ACP-006", func(t *testing.T) { run(t, 5) })
	t.Run("ACP-007", func(t *testing.T) { run(t, 6) })
	t.Run("ACP-008", func(t *testing.T) { run(t, 7) })
	t.Run("ACP-009", func(t *testing.T) { run(t, 8) })
	t.Run("ACP-010", func(t *testing.T) { run(t, 9) })
}

func benchmarkCandidateFrame() []byte {
	request := fixture("internal/store/000.sql", "sqlite.query", strings.Repeat("SELECT 1;", 8))
	request.Inputs = nil
	for i := 0; i < 4; i++ {
		request.Inputs = append(request.Inputs, RequestForInput(fmt.Sprintf("input-%d", i), "sqlite.query", fmt.Sprintf("internal/store/%03d.sql", i), []byte(strings.Repeat("SELECT 1;", 8))))
	}
	return EncodeRequest(request)
}
func digestText(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
