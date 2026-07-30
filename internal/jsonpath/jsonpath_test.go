package jsonpath

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	segs, err := Parse("Records.[0].eventName")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []Segment{
		{Key: "Records"},
		{Index: 0, IsIndex: true},
		{Key: "eventName"},
	}
	if !reflect.DeepEqual(segs, want) {
		t.Fatalf("got %+v, want %+v", segs, want)
	}
}

func TestParseErrors(t *testing.T) {
	for _, p := range []string{"", "a..b", ".a", "a.", "Records.[x].eventName", "Records.[-1].eventName", "Records.[0"} {
		if _, err := Parse(p); err == nil {
			t.Errorf("Parse(%q): expected error, got nil", p)
		}
	}
}

func TestSetNestedKey(t *testing.T) {
	doc := map[string]any{
		"Records": []any{
			map[string]any{"eventName": "INSERT"},
		},
	}
	segs, _ := Parse("Records.[0].eventName")
	if err := Set(doc, segs, "REMOVE"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := doc["Records"].([]any)[0].(map[string]any)["eventName"]
	if got != "REMOVE" {
		t.Fatalf("got %v, want REMOVE", got)
	}
}

func TestSetTypedValue(t *testing.T) {
	doc := map[string]any{"n": float64(1)}
	segs, _ := Parse("n")
	if err := Set(doc, segs, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc["n"] != true {
		t.Fatalf("got %v, want true", doc["n"])
	}
}

func TestSetErrors(t *testing.T) {
	doc := map[string]any{
		"Records": []any{
			map[string]any{"eventName": "INSERT"},
		},
	}
	cases := []string{
		"missing",                   // missing key
		"Records.missing.eventName", // missing intermediate key
		"Records.[5].eventName",     // index out of range
		"Records.eventName",         // key into array (type mismatch)
		"Records.[0].eventName.[0]", // index into string (type mismatch)
	}
	for _, p := range cases {
		segs, err := Parse(p)
		if err != nil {
			t.Fatalf("Parse(%q): %v", p, err)
		}
		if err := Set(doc, segs, "x"); err == nil {
			t.Errorf("Set(%q): expected error, got nil", p)
		}
	}
}
