package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	nativeSocketRelativePath  = ".codex/app-server-control/app-server-control.sock"
	nativeEnginePath          = "/Applications/Codex.app/Contents/Resources/codex"
	nativeEngineCDHash        = "864aa1693ffed7034fd3d1a723386b250aa1627d"
	notificationInventoryHash = "fdda196d47ea428d026348a04ba59330c10395531ae5d5b4e7be6b485209de1d"
)

//go:embed notification-inventory.json
var nativeObserverAssets embed.FS

type peerIdentity struct {
	pid   uint32
	birth []byte
	code  []byte
}

func (p peerIdentity) equal(other peerIdentity) bool {
	return p.pid == other.pid && bytes.Equal(p.birth, other.birth) && bytes.Equal(p.code, other.code)
}

type observerConnection struct {
	conn         net.Conn
	reader       *bufio.Reader
	deadline     time.Time
	frames       int
	targetThread string
	observations []map[string]any
	prearming    bool
	comparison   *comparison
}

func newObserverConnection(conn net.Conn, deadline time.Time) *observerConnection {
	return &observerConnection{conn: conn, reader: bufio.NewReader(conn), deadline: deadline}
}

func (c *observerConnection) setDeadline() error {
	if time.Now().After(c.deadline) {
		return errors.New("connection-deadline")
	}
	return c.conn.SetDeadline(c.deadline)
}

func (c *observerConnection) handshake() error {
	if err := c.setDeadline(); err != nil {
		return err
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	key := base64.StdEncoding.EncodeToString(nonce)
	request := "GET /rpc HTTP/1.1\r\nHost: localhost\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: " + key + "\r\nSec-WebSocket-Version: 13\r\n\r\n"
	if _, err := io.WriteString(c.conn, request); err != nil {
		return err
	}
	header := make([]byte, 0, 8192)
	for !bytes.HasSuffix(header, []byte("\r\n\r\n")) && len(header) < 8192 {
		value, err := c.reader.ReadByte()
		if err != nil {
			return err
		}
		header = append(header, value)
	}
	if !bytes.HasSuffix(header, []byte("\r\n\r\n")) {
		return errors.New("upgrade-header-limit")
	}
	lines := strings.Split(string(header), "\r\n")
	if len(lines) < 3 || lines[0] != "HTTP/1.1 101 Switching Protocols" {
		return errors.New("upgrade-rejected")
	}
	fields := map[string]string{}
	for _, line := range lines[1 : len(lines)-2] {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return errors.New("invalid-upgrade")
		}
		name := strings.ToLower(parts[0])
		if _, duplicate := fields[name]; duplicate {
			return errors.New("duplicate-upgrade-header")
		}
		fields[name] = strings.TrimSpace(parts[1])
	}
	expectedRaw := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	expected := base64.StdEncoding.EncodeToString(expectedRaw[:])
	connections := strings.Split(strings.ToLower(fields["connection"]), ",")
	upgrade := false
	for _, value := range connections {
		upgrade = upgrade || strings.TrimSpace(value) == "upgrade"
	}
	if fields["sec-websocket-accept"] != expected || strings.ToLower(fields["upgrade"]) != "websocket" || !upgrade {
		return errors.New("invalid-upgrade")
	}
	return nil
}

func (c *observerConnection) send(value any) error {
	raw, err := canonical(value)
	if err != nil {
		return err
	}
	return c.frame(1, raw)
}

func (c *observerConnection) frame(opcode byte, data []byte) error {
	if len(data) > 65535 {
		return errors.New("outbound-frame-limit")
	}
	if err := c.setDeadline(); err != nil {
		return err
	}
	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return err
	}
	header := []byte{0x80 | opcode}
	if len(data) < 126 {
		header = append(header, 0x80|byte(len(data)))
	} else {
		header = append(header, 0xfe, byte(len(data)>>8), byte(len(data)))
	}
	masked := make([]byte, len(data))
	for index := range data {
		masked[index] = data[index] ^ mask[index%4]
	}
	_, err := c.conn.Write(append(append(header, mask...), masked...))
	return err
}

func (c *observerConnection) receive() (map[string]any, error) {
	for c.frames < maxEvents {
		c.frames++
		if err := c.setDeadline(); err != nil {
			return nil, err
		}
		first, err := c.reader.ReadByte()
		if err != nil {
			return nil, err
		}
		second, err := c.reader.ReadByte()
		if err != nil {
			return nil, partialFrame(err)
		}
		if first&0x70 != 0 || first&0x80 == 0 || second&0x80 != 0 {
			return nil, errors.New("unsupported-websocket-frame")
		}
		size := uint64(second & 127)
		if size == 126 {
			var value uint16
			if err := binary.Read(c.reader, binary.BigEndian, &value); err != nil {
				return nil, partialFrame(err)
			}
			size = uint64(value)
		} else if size == 127 {
			if err := binary.Read(c.reader, binary.BigEndian, &size); err != nil {
				return nil, partialFrame(err)
			}
		}
		if size > maxFrameBytes {
			return nil, errors.New("inbound-frame-limit")
		}
		opcode := first & 15
		if (opcode == 8 || opcode == 9 || opcode == 10) && size > 125 {
			return nil, errors.New("control-frame-limit")
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(c.reader, data); err != nil {
			return nil, partialFrame(err)
		}
		if opcode == 9 {
			if err := c.frame(10, data); err != nil {
				return nil, err
			}
			continue
		}
		if opcode == 10 {
			continue
		}
		if opcode != 1 {
			return nil, errors.New("unsupported-message")
		}
		decoded, err := decodeClosedJSON(data)
		if err != nil {
			return nil, err
		}
		return asObject(decoded, "invalid-message")
	}
	return nil, errors.New("frame-count-limit")
}

// partialFrame makes a deadline hit after a frame's first byte a hard failure:
// the buffered stream is mid-frame and cannot be resumed (NPO-V0-005).
func partialFrame(err error) error {
	if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		return errors.New("partial-frame-deadline")
	}
	return err
}

func (c *observerConnection) request(id int, method string, params map[string]any) (any, error) {
	if err := c.send(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	for {
		value, err := c.receive()
		if err != nil {
			return nil, err
		}
		if _, present := value["id"]; present {
			responseID, _, parseErr := integer(value["id"], false)
			if parseErr != nil || int(responseID) != id || len(value) > 3 || value["result"] == nil {
				return nil, errors.New("unexpected-response")
			}
			for key := range value {
				if key != "jsonrpc" && key != "id" && key != "result" {
					return nil, errors.New("unexpected-response")
				}
			}
			return value["result"], nil
		}
		if err := c.notification(value); err != nil {
			return nil, err
		}
	}
}

func (c *observerConnection) notification(value map[string]any) error {
	method, _ := value["method"].(string)
	if c.comparison != nil && c.comparison.continuation != nil && activityMethods[method] {
		params, _ := value["params"].(map[string]any)
		if c.prearming || c.targetThread == "" || params["threadId"] != c.targetThread || len(c.observations) >= 32 {
			return errors.New("unexpected-activity")
		}
		projection, err := c.comparison.observeActivity(value)
		if err != nil {
			return err
		}
		c.observations = append(c.observations, projection)
		return nil
	}
	if !hookMethods[method] {
		return errors.New("unexpected-notification")
	}
	params, _ := value["params"].(map[string]any)
	if c.targetThread == "" || params["threadId"] != c.targetThread {
		return errors.New("unexpected-thread-notification")
	}
	raw, _ := canonical(value)
	normalized, err := normalizeHook(raw)
	if err != nil {
		return err
	}
	if len(c.observations) >= 32 {
		return errors.New("observation-count-limit")
	}
	if c.prearming {
		c.observations = append(c.observations, normalized)
		return errors.New("hook-before-ready")
	}
	if c.comparison != nil {
		compared, err := c.comparison.observe(value)
		if err != nil {
			return err
		}
		normalized["profile"], normalized["comparison"], normalized["sequence"] = "corvint-native-hook-observation/1-experimental", compared, c.comparison.sequence
	}
	c.observations = append(c.observations, normalized)
	return nil
}

func hooksProjection(result any, cwd string, expanded bool) (map[string]any, error) {
	root, err := asObject(result, "invalid-hook-inventory")
	if err != nil || exactFields(root, "data") != nil {
		return nil, errors.New("invalid-hook-inventory")
	}
	data, err := asArray(root["data"], "invalid-hook-inventory")
	if err != nil || len(data) != 1 {
		return nil, errors.New("invalid-hook-inventory")
	}
	entry, err := asObject(data[0], "hook-inventory-unavailable")
	if err != nil || exactFields(entry, "cwd", "hooks", "warnings", "errors") != nil || entry["cwd"] != cwd {
		return nil, errors.New("hook-inventory-unavailable")
	}
	warnings, werr := asArray(entry["warnings"], "hook-inventory-unavailable")
	errorsList, eerr := asArray(entry["errors"], "hook-inventory-unavailable")
	hooks, herr := asArray(entry["hooks"], "hook-inventory-unavailable")
	if werr != nil || eerr != nil || herr != nil || len(warnings) > 0 || len(errorsList) > 0 || len(hooks) > 32 {
		return nil, errors.New("hook-inventory-unavailable")
	}
	projected := []any{}
	for _, rawHook := range hooks {
		hook, err := asObject(rawHook, "invalid-hook")
		if err != nil {
			return nil, err
		}
		if !expanded && hook["eventName"] != "stop" {
			continue
		}
		if hook["handlerType"] != "command" {
			return nil, errors.New("unsupported-stop-hook")
		}
		enabled, enabledOK := hook["enabled"].(bool)
		async, asyncOK := hook["async"].(bool)
		if expanded && hook["async"] == nil {
			async, asyncOK = false, true
		}
		if !enabledOK || !asyncOK {
			return nil, errors.New("unsupported-stop-hook")
		}
		command, err := textDigest(hook["command"], 8192)
		if err != nil {
			return nil, err
		}
		sourcePath, err := textDigest(hook["sourcePath"], 4096)
		if err != nil {
			return nil, err
		}
		currentHash, err := textDigest(hook["currentHash"], 4096)
		if err != nil {
			return nil, err
		}
		trust, err := enum(hook["trustStatus"], stringSet("managed", "trusted", "untrusted", "modified"))
		if err != nil {
			return nil, err
		}
		row := map[string]any{"commandSHA256": command, "sourcePathSHA256": sourcePath, "currentHashSHA256": currentHash, "enabled": enabled, "async": async, "trustStatus": trust}
		if expanded {
			required := stringSet("currentHash", "displayOrder", "enabled", "eventName", "isManaged", "key", "source", "sourcePath", "timeoutSec", "trustStatus", "command", "handlerType")
			optional := stringSet("additionalContextLimit", "matcher", "pluginId", "statusMessage", "async")
			for key := range required {
				if _, ok := hook[key]; !ok {
					return nil, errors.New("unsupported-hook-fields")
				}
			}
			for key := range hook {
				if !required[key] && !optional[key] {
					return nil, errors.New("unsupported-hook-fields")
				}
			}
			display, _, err := integer(hook["displayOrder"], false)
			managed, managedOK := hook["isManaged"].(bool)
			timeout, _, timeoutErr := unsignedInteger(hook["timeoutSec"], false)
			if err != nil || !managedOK || timeoutErr != nil {
				return nil, errors.New("invalid-hook-metadata")
			}
			var contextLimit any
			if hook["additionalContextLimit"] != nil {
				limit, _, err := unsignedInteger(hook["additionalContextLimit"], false)
				if err != nil {
					return nil, errors.New("invalid-context-limit")
				}
				contextLimit = limit
			}
			matcherHash, err := nullableDigest(hook["matcher"])
			if err != nil {
				return nil, err
			}
			pluginHash, err := nullableDigest(hook["pluginId"])
			if err != nil {
				return nil, err
			}
			statusHash, err := nullableDigest(hook["statusMessage"])
			if err != nil {
				return nil, err
			}
			keyHash, err := textDigest(hook["key"], 4096)
			if err != nil {
				return nil, err
			}
			event, err := enum(hook["eventName"], eventNames)
			if err != nil {
				return nil, err
			}
			source, err := enum(hook["source"], sources)
			if err != nil {
				return nil, err
			}
			row["eventName"], row["matcherSHA256"], row["timeoutSec"], row["source"], row["displayOrder"], row["isManaged"], row["additionalContextLimit"], row["keySHA256"], row["pluginIdSHA256"], row["statusMessageSHA256"] = event, matcherHash, timeout, source, display, managed, contextLimit, keyHash, pluginHash, statusHash
		}
		projected = append(projected, row)
	}
	profile := "corvint-native-hook-inventory/0-experimental"
	if expanded {
		profile = "corvint-native-hook-inventory/1-experimental"
	}
	return map[string]any{"profile": profile, "qualification": "UNQUALIFIED", "hooks": projected}, nil
}

func nullableDigest(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	text, ok := value.(string)
	if !ok || len([]byte(text)) > 4096 {
		return nil, errors.New("invalid-nullable-text")
	}
	return digestBytes([]byte(text)), nil
}

func loadRegularJSON(path string, limit int) (any, error) {
	fd, err := openRegularNoFollow(path)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("input-not-regular")
	}
	raw, err := io.ReadAll(io.LimitReader(file, int64(limit+1)))
	if err != nil || len(raw) > limit {
		return nil, errors.New("input-limit")
	}
	return decodeClosedJSON(raw)
}

func expectedInventory(path string) (map[string]any, error) {
	value, err := loadRegularJSON(path, maxFrameBytes)
	if err != nil {
		return nil, err
	}
	inventory, err := asObject(value, "invalid-expected-inventory")
	if err != nil || exactFields(inventory, "profile", "qualification", "hooks") != nil || inventory["profile"] != "corvint-native-hook-inventory/1-experimental" || inventory["qualification"] != "UNQUALIFIED" {
		return nil, errors.New("invalid-expected-inventory")
	}
	hooks, err := asArray(inventory["hooks"], "invalid-expected-inventory")
	if err != nil || len(hooks) > 32 {
		return nil, errors.New("invalid-expected-inventory")
	}
	for _, rawHook := range hooks {
		hook, err := asObject(rawHook, "invalid-expected-hook")
		if err != nil || exactFields(hook, "commandSHA256", "sourcePathSHA256", "currentHashSHA256", "enabled", "async", "trustStatus", "eventName", "matcherSHA256", "timeoutSec", "source", "displayOrder", "isManaged", "additionalContextLimit", "keySHA256", "pluginIdSHA256", "statusMessageSHA256") != nil {
			return nil, errors.New("invalid-expected-hook")
		}
		for _, field := range []string{"commandSHA256", "sourcePathSHA256", "currentHashSHA256", "keySHA256"} {
			if !validSHA(hook[field]) {
				return nil, errors.New("invalid-expected-digest")
			}
		}
		for _, field := range []string{"matcherSHA256", "pluginIdSHA256", "statusMessageSHA256"} {
			if hook[field] != nil && !validSHA(hook[field]) {
				return nil, errors.New("invalid-expected-digest")
			}
		}
		if _, ok := hook["enabled"].(bool); !ok {
			return nil, errors.New("invalid-expected-hook-value")
		}
		if _, ok := hook["async"].(bool); !ok {
			return nil, errors.New("invalid-expected-hook-value")
		}
		if _, _, err := unsignedInteger(hook["timeoutSec"], false); err != nil {
			return nil, errors.New("invalid-expected-hook-value")
		}
		if _, _, err := integer(hook["displayOrder"], false); err != nil {
			return nil, errors.New("invalid-expected-metadata")
		}
		if _, ok := hook["isManaged"].(bool); !ok {
			return nil, errors.New("invalid-expected-metadata")
		}
		if hook["additionalContextLimit"] != nil {
			if _, _, err := unsignedInteger(hook["additionalContextLimit"], false); err != nil {
				return nil, errors.New("invalid-expected-context-limit")
			}
		}
		if _, err := enum(hook["source"], sources); err != nil {
			return nil, err
		}
		if _, err := enum(hook["eventName"], eventNames); err != nil {
			return nil, err
		}
		if _, err := enum(hook["trustStatus"], stringSet("managed", "trusted", "untrusted", "modified")); err != nil {
			return nil, err
		}
	}
	if _, err := projectionBytes(inventory); err != nil {
		return nil, err
	}
	return inventory, nil
}

func validSHA(value any) bool {
	text, ok := value.(string)
	if !ok || len(text) != 64 {
		return false
	}
	_, err := hex.DecodeString(text)
	return err == nil && strings.ToLower(text) == text
}

func loadedMember(result any, threadID string) error {
	object, err := asObject(result, "invalid-loaded-membership")
	if err != nil || exactFields(object, "data", "nextCursor") != nil {
		return errors.New("invalid-loaded-membership")
	}
	data, err := asArray(object["data"], "invalid-loaded-membership")
	if err != nil || len(data) > 256 {
		return errors.New("invalid-loaded-membership")
	}
	found := false
	for _, item := range data {
		text, ok := item.(string)
		if !ok || len(text) > 128 {
			return errors.New("invalid-loaded-id")
		}
		found = found || text == threadID
	}
	if !found {
		return errors.New("task-not-loaded")
	}
	return nil
}

func idleMetadata(result any, threadID, cwd string, resume bool) error {
	object, err := asObject(result, "invalid-idle-metadata")
	if err != nil {
		return err
	}
	thread, err := asObject(object["thread"], "unexpected-idle-metadata")
	if err != nil || thread["id"] != threadID || thread["cwd"] != cwd {
		return errors.New("unexpected-idle-metadata")
	}
	turns, err := asArray(thread["turns"], "unexpected-idle-metadata")
	status, statusErr := asObject(thread["status"], "task-not-idle")
	if err != nil || len(turns) != 0 || statusErr != nil || len(status) != 1 || status["type"] != "idle" {
		return errors.New("task-not-idle")
	}
	if resume && (object["initialTurnsPage"] != nil || object["cwd"] != cwd) {
		return errors.New("unexpected-idle-resume")
	}
	return nil
}

type requester interface {
	request(int, string, map[string]any) (any, error)
}

func prearmRequests(connection requester, threadID, cwd string) error {
	for _, call := range []struct {
		id     int
		method string
		params map[string]any
		resume bool
	}{
		{3, "thread/loaded/list", map[string]any{"limit": 256}, false},
		{7, "thread/read", map[string]any{"threadId": threadID, "includeTurns": false}, false},
		{4, "thread/resume", map[string]any{"threadId": threadID, "excludeTurns": true}, true},
		{8, "thread/read", map[string]any{"threadId": threadID, "includeTurns": false}, false},
		{5, "thread/loaded/list", map[string]any{"limit": 256}, false},
	} {
		result, err := connection.request(call.id, call.method, call.params)
		if err != nil {
			return err
		}
		if call.method == "thread/loaded/list" {
			if err := loadedMember(result, threadID); err != nil {
				return err
			}
		} else if err := idleMetadata(result, threadID, cwd, call.resume); err != nil {
			return err
		}
	}
	return nil
}

func subscribeRequests(connection requester, threadID, cwd string) error {
	result, err := connection.request(3, "thread/loaded/list", map[string]any{"limit": 256})
	if err != nil || loadedMember(result, threadID) != nil {
		return errors.New("task-not-loaded")
	}
	result, err = connection.request(4, "thread/resume", map[string]any{"threadId": threadID, "excludeTurns": true})
	if err != nil {
		return err
	}
	object, err := asObject(result, "unexpected-resume-metadata")
	if err != nil {
		return err
	}
	thread, err := asObject(object["thread"], "unexpected-resume-metadata")
	if err != nil {
		return err
	}
	status, err := asObject(thread["status"], "task-not-active")
	if err != nil {
		return err
	}
	turns, err := asArray(thread["turns"], "unexpected-resume-metadata")
	if err != nil || object["initialTurnsPage"] != nil || object["cwd"] != cwd || thread["id"] != threadID || thread["cwd"] != cwd || len(turns) != 0 {
		return errors.New("unexpected-resume-metadata")
	}
	if status["type"] != "active" {
		return errors.New("task-not-active")
	}
	result, err = connection.request(5, "thread/loaded/list", map[string]any{"limit": 256})
	if err != nil {
		return err
	}
	return loadedMember(result, threadID)
}

func readyProjection(threadID, cwd string, peer peerIdentity, inventory map[string]any) (map[string]any, error) {
	inventoryBytes, err := projectionBytes(inventory)
	if err != nil {
		return nil, err
	}
	thread, err := textDigest(threadID, 4096)
	if err != nil {
		return nil, err
	}
	cwdHash, err := textDigest(cwd, 4096)
	if err != nil {
		return nil, err
	}
	basis := append([]byte(strconv.FormatUint(uint64(peer.pid), 10)+"\x00"), peer.birth...)
	basis = append(basis, peer.code...)
	return map[string]any{"profile": "corvint-native-observer-ready/experimental", "qualification": "UNQUALIFIED", "setup": "READY", "mode": "idle", "threadSHA256": thread, "cwdSHA256": cwdHash, "peerSHA256": digestBytes(basis), "inventorySHA256": digestBytes(inventoryBytes), "maximumConnectionSeconds": 30}, nil
}

func completedProjection(ready map[string]any, projections []map[string]any) (map[string]any, error) {
	all := append([]map[string]any{ready}, projections...)
	var basis bytes.Buffer
	for _, projection := range all {
		raw, err := projectionBytes(projection)
		if err != nil {
			return nil, err
		}
		basis.Write(raw)
	}
	return map[string]any{"profile": "corvint-native-observer-complete/experimental", "qualification": "UNQUALIFIED", "completion": "COMPLETE", "projectionCount": len(all), "projectionsSHA256": digestBytes(basis.Bytes())}, nil
}

var uuid7Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\n$`)

func readThreadID(input io.Reader) (string, error) {
	raw, err := io.ReadAll(io.LimitReader(input, 39))
	if err != nil || !uuid7Pattern.Match(raw) {
		return "", errors.New("invalid-existing-task-id")
	}
	return string(raw[:len(raw)-1]), nil
}

// nativeSocketPath resolves the Codex app-server control socket under the current user's home
// directory; the socket location is per-user, never a build-time constant.
func nativeSocketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("native socket: %w", err)
	}
	return filepath.Join(home, nativeSocketRelativePath), nil
}

func runNativeClient(ctx context.Context, arguments []string, input io.Reader, output io.Writer) int {
	if len(arguments) < 1 || !filepath.IsAbs(arguments[0]) {
		return 1
	}
	cwd := arguments[0]
	mode := "inventory"
	var inventoryPath, goldPath string
	if len(arguments) == 2 && arguments[1] == "--subscribe" {
		mode = "active"
	} else if len(arguments) == 3 && arguments[1] == "--prearm-idle" {
		mode, inventoryPath = "idle", arguments[2]
	} else if len(arguments) == 4 && arguments[1] == "--prearm-idle" {
		mode, inventoryPath, goldPath = "idle", arguments[2], arguments[3]
	} else if len(arguments) != 1 {
		return 1
	}
	deadline := time.Now().Add(maxConnection)
	var expected map[string]any
	var compare *comparison
	var err error
	if mode == "idle" {
		expected, err = expectedInventory(inventoryPath)
		if err != nil {
			return 1
		}
	}
	if goldPath != "" {
		compare, err = loadComparison(goldPath, cwd)
		if err != nil {
			return 1
		}
	}
	threadID := ""
	if mode != "inventory" {
		threadID, err = readThreadID(input)
		if err != nil {
			return 1
		}
	}
	inventoryRaw, err := nativeObserverAssets.ReadFile("notification-inventory.json")
	if err != nil || len(inventoryRaw) > maxFrameBytes || digestBytes(inventoryRaw) != notificationInventoryHash {
		return 1
	}
	inventoryValue, err := decodeClosedJSON(inventoryRaw)
	if err != nil {
		return 1
	}
	inventoryObject, _ := inventoryValue.(map[string]any)
	optOut, err := asArray(inventoryObject["optOutNotificationMethods"], "notification-schema-drift")
	if err != nil {
		return 1
	}
	if compare != nil && compare.continuation != nil {
		present := map[string]bool{}
		for _, item := range optOut {
			text, ok := item.(string)
			if !ok {
				return 1
			}
			present[text] = true
		}
		for method := range activityMethods {
			if !present[method] {
				return 1
			}
		}
		filtered := []any{}
		for _, item := range optOut {
			if !activityMethods[item.(string)] {
				filtered = append(filtered, item)
			}
		}
		optOut = filtered
	}
	dialer := net.Dialer{Deadline: deadline}
	socketPath, err := nativeSocketPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	conn, err := dialer.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return 1
	}
	defer conn.Close()
	before, err := inspectPeer(conn)
	if err != nil {
		return 1
	}
	connection := newObserverConnection(conn, deadline.Add(-time.Second))
	connection.comparison = compare
	if connection.handshake() != nil {
		return 1
	}
	if _, err = connection.request(1, "initialize", map[string]any{"clientInfo": map[string]any{"name": "corvint_native_hook_observer", "version": "0-experimental", "title": nil}, "capabilities": map[string]any{"experimentalApi": true, "requestAttestation": false, "optOutNotificationMethods": optOut}}); err != nil {
		return 1
	}
	if err = connection.send(map[string]any{"method": "initialized"}); err != nil {
		return 1
	}
	rawHooks, err := connection.request(2, "hooks/list", map[string]any{"cwds": []any{cwd}})
	if err != nil {
		return 1
	}
	inventory, err := hooksProjection(rawHooks, cwd, mode == "idle")
	if err != nil {
		return 1
	}
	var ready map[string]any
	if mode != "inventory" {
		if mode == "idle" {
			expectedRaw, _ := canonical(expected)
			actualRaw, _ := canonical(inventory)
			if !bytes.Equal(expectedRaw, actualRaw) {
				return 1
			}
			if compare != nil {
				if err := compare.bindInventory(inventory); err != nil {
					return 1
				}
			}
			connection.prearming = true
			connection.targetThread = threadID
			if prearmRequests(connection, threadID, cwd) != nil {
				return 1
			}
			if len(connection.observations) > 0 {
				return 1
			}
			current, err := inspectPeer(conn)
			if err != nil || !before.equal(current) {
				return 1
			}
			ready, err = readyProjection(threadID, cwd, before, inventory)
			if err != nil {
				return 1
			}
			if compare != nil {
				ready["profile"] = "corvint-native-observer-ready/1-experimental"
				ready["comparisonGoldSHA256"] = compare.goldDigest
			}
			if emitProjection(output, ready, deadline) != nil {
				return 1
			}
			connection.prearming = false
		} else {
			connection.targetThread = threadID
			if subscribeRequests(connection, threadID, cwd) != nil {
				return 1
			}
		}
		for time.Now().Before(connection.deadline) {
			value, err := connection.receive()
			if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				break
			}
			if err != nil || connection.notification(value) != nil {
				return 1
			}
		}
		connection.deadline = deadline.Add(-500 * time.Millisecond)
		result, err := connection.request(6, "thread/loaded/list", map[string]any{"limit": 256})
		if err != nil || loadedMember(result, threadID) != nil {
			return 1
		}
		if mode == "idle" {
			result, err := connection.request(9, "hooks/list", map[string]any{"cwds": []any{cwd}})
			if err != nil {
				return 1
			}
			finalInventory, err := hooksProjection(result, cwd, true)
			if err != nil {
				return 1
			}
			a, _ := canonical(inventory)
			b, _ := canonical(finalInventory)
			if !bytes.Equal(a, b) {
				return 1
			}
		}
	}
	after, err := inspectPeer(conn)
	if err != nil || !before.equal(after) {
		return 1
	}
	_ = conn.Close()
	projections := []map[string]any{inventory}
	if mode != "inventory" {
		projections = append(projections, connection.observations...)
	}
	if compare != nil {
		witness, err := compare.finish()
		if err != nil {
			return 1
		}
		if witness != nil {
			projections = append(projections, witness)
		}
	}
	var completion map[string]any
	if mode == "idle" {
		completion, err = completedProjection(ready, projections)
		if err != nil {
			return 1
		}
		if _, err = projectionBytes(completion); err != nil {
			return 1
		}
	}
	for _, projection := range projections {
		if emitProjection(output, projection, deadline) != nil {
			return 1
		}
	}
	if mode == "idle" {
		if emitProjection(output, completion, deadline) != nil {
			return 1
		}
	}
	return 0
}

func inspectPeer(conn net.Conn) (peerIdentity, error) {
	return platformPeerIdentity(conn, nativeEnginePath, nativeEngineCDHash)
}
