package localcompletion

import (
	"reflect"
	"strings"
	"testing"
)

// TestLCPV0780OptionalTagAdmitsAnAbsentMember is V1-4053: strictJSON accepts an
// absent member only for a field tagged `localcompletion:"optional"`, so an
// optional field needs no edit in storage.go.
func TestLCPV0780OptionalTagAdmitsAnAbsentMember(t *testing.T) {
	type probe struct {
		Required string `json:"required"`
		Optional *Plan  `json:"optional,omitempty" localcompletion:"optional"`
	}
	var got probe
	if err := strictJSON([]byte(`{"required":"x"}`), &got, MaxPlanBytes); err != nil {
		t.Fatalf("a tagged absent member was refused: %v", err)
	}
	if err := strictJSON([]byte(`{"optional":null}`), &got, MaxPlanBytes); err == nil {
		t.Fatal("an untagged absent member was accepted")
	}
	var plan Plan
	if err := strictJSON([]byte(`{"base":"b","intents":[],"checks":[{"id":"c","argv":["true"],"timeoutSeconds":1}]}`), &plan, MaxPlanBytes); err != nil {
		t.Fatalf("a check without allowCemSidecarOnlyReuse was refused: %v", err)
	}
}

// TestLCPV0780OmitemptyFieldsAreTaggedOptional derives the rule from the
// types strictJSON reads: a field the encoder may omit (omitempty) must carry
// the optional tag, or a saved file would be refused on its next read.
func TestLCPV0780OmitemptyFieldsAreTaggedOptional(t *testing.T) {
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(kind reflect.Type) {
		for kind.Kind() == reflect.Pointer || kind.Kind() == reflect.Slice || kind.Kind() == reflect.Array || kind.Kind() == reflect.Map {
			kind = kind.Elem()
		}
		if kind.Kind() != reflect.Struct || seen[kind] {
			return
		}
		seen[kind] = true
		for index := 0; index < kind.NumField(); index++ {
			field := kind.Field(index)
			tag := strings.Split(field.Tag.Get("json"), ",")
			if strings.Contains(","+strings.Join(tag[1:], ",")+",", ",omitempty,") && field.Tag.Get(optionalTagKey) != optionalTagValue {
				t.Errorf("%s.%s is omitempty but not tagged %s:%q: strictJSON in internal/localcompletion/storage.go refuses a file that omits it; add the tag", kind.Name(), field.Name, optionalTagKey, optionalTagValue)
			}
			walk(field.Type)
		}
	}
	walk(reflect.TypeFor[Plan]())
	walk(reflect.TypeFor[state]())
}
