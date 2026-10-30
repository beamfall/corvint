package main

import (
	_ "embed"
	"errors"
)

//go:embed activity-schema.json
var activitySchemaBytes []byte

const activitySchemaSHA256 = "1f249a61648e7ec3a75f3fb4b11c048f1b33189908fbdd1ebaab43fe99422281"

var activityMethods = stringSet("turn/started", "turn/completed", "item/started", "item/completed")

type activityDefinition struct {
	fields, required map[string]bool
	statuses         map[string]bool
}

func loadActivityDefinitions() (map[string]activityDefinition, error) {
	if len(activitySchemaBytes) > maxFrameBytes || digestBytes(activitySchemaBytes) != activitySchemaSHA256 {
		return nil, errors.New("activity-schema-drift")
	}
	decoded, err := decodeClosedJSON(activitySchemaBytes)
	if err != nil {
		return nil, err
	}
	root, _ := asObject(decoded, "activity-schema")
	items, err := asObject(root["items"], "activity-schema")
	if err != nil {
		return nil, err
	}
	result := map[string]activityDefinition{}
	for name, rawDefinition := range items {
		definition, err := asObject(rawDefinition, "activity-schema")
		if err != nil {
			return nil, err
		}
		fields, err := stringArraySet(definition["fields"])
		if err != nil {
			return nil, err
		}
		required, err := stringArraySet(definition["required"])
		if err != nil {
			return nil, err
		}
		statuses := map[string]bool{}
		if definition["statuses"] != nil {
			statuses, err = stringArraySet(definition["statuses"])
			if err != nil {
				return nil, err
			}
		}
		result[name] = activityDefinition{fields: fields, required: required, statuses: statuses}
	}
	return result, nil
}

func stringArraySet(value any) (map[string]bool, error) {
	array, err := asArray(value, "string-array")
	if err != nil {
		return nil, err
	}
	result := map[string]bool{}
	for _, item := range array {
		text, ok := item.(string)
		if !ok || result[text] {
			return nil, errors.New("string-array")
		}
		result[text] = true
	}
	return result, nil
}

func normalizeActivity(value map[string]any, definitions map[string]activityDefinition, sequence int) (map[string]any, string, error) {
	for key := range value {
		if key != "jsonrpc" && key != "method" && key != "params" {
			return nil, "", errors.New("activity-notification")
		}
	}
	if value["jsonrpc"] != nil && value["jsonrpc"] != "2.0" {
		return nil, "", errors.New("activity-notification")
	}
	method, _ := value["method"].(string)
	if !activityMethods[method] {
		return nil, "", errors.New("activity-notification")
	}
	params, err := asObject(value["params"], "activity-params")
	if err != nil {
		return nil, "", err
	}
	thread, err := textDigest(params["threadId"], 4096)
	if err != nil {
		return nil, "", err
	}
	result := map[string]any{"profile": "corvint-native-activity/0-experimental", "qualification": "UNQUALIFIED", "method": method, "sequence": sequence, "threadSHA256": thread}
	if method == "turn/started" || method == "turn/completed" {
		if exactFields(params, "threadId", "turn") != nil {
			return nil, "", errors.New("activity-turn-fields")
		}
		turn, err := asObject(params["turn"], "activity-turn")
		if err != nil {
			return nil, "", err
		}
		for key := range turn {
			if !stringSet("id", "items", "status", "itemsView", "error", "startedAt", "completedAt", "durationMs")[key] {
				return nil, "", errors.New("activity-turn")
			}
		}
		for _, key := range []string{"id", "items", "status"} {
			if _, ok := turn[key]; !ok {
				return nil, "", errors.New("activity-turn")
			}
		}
		items, err := asArray(turn["items"], "activity-turn-items")
		if err != nil || len(items) > 256 {
			return nil, "", errors.New("activity-turn-items")
		}
		if view := turn["itemsView"]; view != nil && view != "full" && view != "summary" && view != "notLoaded" {
			return nil, "", errors.New("activity-turn-items")
		}
		for _, field := range []string{"startedAt", "completedAt", "durationMs"} {
			if turn[field] != nil {
				if _, _, err := integer(turn[field], false); err != nil {
					return nil, "", errors.New("activity-integer")
				}
			}
		}
		status, err := enum(turn["status"], stringSet("inProgress", "completed", "failed", "interrupted"))
		if err != nil || (method == "turn/started") != (status == "inProgress") || status != "failed" && turn["error"] != nil {
			return nil, "", errors.New("activity-turn-status")
		}
		turnDigest, err := textDigest(turn["id"], 4096)
		if err != nil {
			return nil, "", err
		}
		result["turnSHA256"], result["status"] = turnDigest, status
		return result, "", nil
	}

	timing := "startedAtMs"
	if method == "item/completed" {
		timing = "completedAtMs"
	}
	if exactFields(params, "threadId", "turnId", "item", timing) != nil {
		return nil, "", errors.New("activity-item-fields")
	}
	if _, _, err := integer(params[timing], false); err != nil {
		return nil, "", errors.New("activity-integer")
	}
	item, err := asObject(params["item"], "activity-item-type")
	if err != nil {
		return nil, "", err
	}
	typeName, _ := item["type"].(string)
	definition, ok := definitions[typeName]
	if !ok {
		return nil, "", errors.New("activity-item-type")
	}
	for field := range definition.required {
		if _, ok := item[field]; !ok {
			return nil, "", errors.New("activity-item-shape")
		}
	}
	for field := range item {
		if !definition.fields[field] {
			return nil, "", errors.New("activity-item-shape")
		}
	}
	var status any
	if len(definition.statuses) > 0 {
		status, err = enum(item["status"], definition.statuses)
		if err != nil {
			return nil, "", err
		}
	}
	textHash := ""
	if typeName == "agentMessage" {
		text, ok := item["text"].(string)
		if !ok || len([]byte(text)) > 8192 {
			return nil, "", errors.New("activity-agent-result-size")
		}
		if method == "item/completed" {
			textHash = digestBytes([]byte(text))
		}
	}
	turnDigest, err := textDigest(params["turnId"], 4096)
	if err != nil {
		return nil, "", err
	}
	itemDigest, err := textDigest(item["id"], 4096)
	if err != nil {
		return nil, "", err
	}
	result["turnSHA256"], result["itemSHA256"], result["itemType"], result["status"] = turnDigest, itemDigest, typeName, status
	return result, textHash, nil
}

type continuationWitness struct {
	blockedCase, releasedCase int
	expectedResult            any
	definitions               map[string]activityDefinition
	turn                      string
	blockedSequence           int
	released, activity        bool
	resultMatch, completed    bool
	items                     map[string]struct {
		sequence int
		itemType string
	}
	seen, turns map[string]bool
}

func newContinuation(config map[string]any) (*continuationWitness, error) {
	definitions, err := loadActivityDefinitions()
	if err != nil {
		return nil, err
	}
	blocked, _, err := integer(config["blockedCase"], false)
	if err != nil {
		return nil, err
	}
	released, _, err := integer(config["releasedCase"], false)
	if err != nil || blocked < 0 || released <= blocked {
		return nil, errors.New("comparison-continuation-order")
	}
	return &continuationWitness{blockedCase: int(blocked), releasedCase: int(released), expectedResult: config["resultTextSHA256"], definitions: definitions, items: map[string]struct {
		sequence int
		itemType string
	}{}, seen: map[string]bool{}, turns: map[string]bool{}}, nil
}

func (w *continuationWitness) hook(caseIndex int, params, comparison map[string]any, sequence int) error {
	if caseIndex != w.blockedCase && caseIndex != w.releasedCase {
		return nil
	}
	turn, err := textDigest(params["turnId"], 4096)
	if err != nil {
		return err
	}
	if caseIndex == w.blockedCase {
		if w.blockedSequence != 0 || comparison["decision"] != "block" {
			return errors.New("continuation-block")
		}
		w.turn, w.blockedSequence = turn, sequence
		return nil
	}
	if w.turn != turn || !w.activity || w.released || comparison["decision"] != "release" {
		return errors.New("continuation-release")
	}
	w.released = true
	return nil
}

func (w *continuationWitness) observe(value map[string]any, sequence int) (map[string]any, error) {
	result, textHash, err := normalizeActivity(value, w.definitions, sequence)
	if err != nil {
		return nil, err
	}
	method, turn := result["method"].(string), result["turnSHA256"].(string)
	if w.completed && turn == w.turn {
		return nil, errors.New("continuation-after-completion")
	}
	if method == "item/started" || method == "item/completed" {
		key := turn + "\x00" + result["itemSHA256"].(string)
		if method == "item/started" {
			if w.seen[key] || w.items[key].sequence != 0 || len(w.items) >= 32 {
				return nil, errors.New("continuation-duplicate-item")
			}
			w.items[key] = struct {
				sequence int
				itemType string
			}{sequence, result["itemType"].(string)}
		} else {
			if w.seen[key] {
				return nil, errors.New("continuation-duplicate-item")
			}
			w.seen[key] = true
			started, present := w.items[key]
			delete(w.items, key)
			if present && started.itemType != result["itemType"] {
				return nil, errors.New("continuation-item-mismatch")
			}
			if w.blockedSequence != 0 && turn == w.turn && !w.released && present && started.sequence > w.blockedSequence && result["itemType"] == "agentMessage" {
				w.activity = true
				expected, _ := w.expectedResult.(string)
				w.resultMatch = expected != "" && textHash == expected
			}
		}
	} else if method == "turn/started" {
		if w.turns[turn] || len(w.turns) >= 32 || w.blockedSequence != 0 && turn == w.turn {
			return nil, errors.New("continuation-turn-order")
		}
		w.turns[turn] = true
	} else if turn == w.turn {
		if !w.released || result["status"] != "completed" {
			return nil, errors.New("continuation-turn-incomplete")
		}
		w.completed = true
	}
	return result, nil
}

func (w *continuationWitness) finish() (map[string]any, error) {
	if !w.completed || !w.activity || len(w.items) > 0 {
		return nil, errors.New("continuation-incomplete")
	}
	expected, _ := w.expectedResult.(string)
	if expected != "" && !w.resultMatch {
		return nil, errors.New("continuation-result-mismatch")
	}
	semantic := "NOT_OBSERVED"
	if w.resultMatch {
		semantic = "MATCH"
	}
	return map[string]any{"profile": "corvint-native-continuation/0-experimental", "qualification": "UNQUALIFIED", "turnSHA256": w.turn, "activity": "OBSERVED", "recursiveRelease": "EXPECTED_RECEIPT_DIGEST_MATCH", "requestProvenance": "caller-asserted", "eventSurface": "unattributed", "turnCompletion": "SUCCEEDED", "semanticResult": semantic, "expectedResultSHA256": w.expectedResult}, nil
}
