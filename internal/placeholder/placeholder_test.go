package placeholder

import (
	"strings"
	"testing"
)

func TestExactPlaceholderKeepsNativeType(t *testing.T) {
	e := New(42)
	v, err := e.Resolve("{{int:1:100}}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n, ok := v.(int)
	if !ok {
		t.Fatalf("expected int, got %T (%v)", v, v)
	}
	if n < 1 || n > 100 {
		t.Fatalf("int out of range: %d", n)
	}

	b, err := e.Resolve("{{bool}}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := b.(bool); !ok {
		t.Fatalf("expected bool, got %T", b)
	}
}

func TestEmbeddedPlaceholderRendersAsString(t *testing.T) {
	e := New(42)
	v, err := e.Resolve("user-{{uuid}}-suffix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("expected string, got %T", v)
	}
	if !strings.HasPrefix(s, "user-") || !strings.HasSuffix(s, "-suffix") {
		t.Fatalf("unexpected value: %q", s)
	}
	if strings.Contains(s, "{{") {
		t.Fatalf("unresolved placeholder in %q", s)
	}
}

func TestPlainStringUntouched(t *testing.T) {
	e := New(42)
	v, err := e.Resolve("INSERT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "INSERT" {
		t.Fatalf("got %v, want INSERT", v)
	}
}

func TestUnknownGeneratorErrors(t *testing.T) {
	e := New(42)
	if _, err := e.Resolve("{{nosuchthing}}"); err == nil {
		t.Fatal("expected error for unknown generator")
	}
	if _, err := e.Resolve("prefix-{{nosuchthing}}"); err == nil {
		t.Fatal("expected error for embedded unknown generator")
	}
}

func TestBadArgumentsError(t *testing.T) {
	e := New(42)
	for _, s := range []string{"{{int:1}}", "{{int:a:b}}", "{{int:100:1}}", "{{sentence:0}}", "{{float:x:y}}"} {
		if _, err := e.Resolve(s); err == nil {
			t.Errorf("Resolve(%q): expected error, got nil", s)
		}
	}
}

func TestDeterministicWithSeed(t *testing.T) {
	e1, e2 := New(7), New(7)
	for i := 0; i < 5; i++ {
		v1, err := e1.Resolve("{{name}}")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		v2, err := e2.Resolve("{{name}}")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v1 != v2 {
			t.Fatalf("same seed produced different values: %v vs %v", v1, v2)
		}
	}
}

func TestResolveDoc(t *testing.T) {
	e := New(42)
	doc := map[string]any{
		"id":        "{{uuid}}",
		"age":       "{{int:18:65}}",
		"tags":      []any{"{{word}}", "literal"},
		"nested":    map[string]any{"email": "{{email}}"},
		"untouched": "plain",
	}
	if err := e.ResolveDoc(doc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := doc["age"].(int); !ok {
		t.Fatalf("age should be int, got %T", doc["age"])
	}
	tags := doc["tags"].([]any)
	if _, ok := tags[0].(string); !ok || tags[0] == "{{word}}" {
		t.Fatalf("tag not resolved: %v", tags[0])
	}
	if tags[1] != "literal" {
		t.Fatalf("literal changed: %v", tags[1])
	}
	if doc["untouched"] != "plain" {
		t.Fatalf("plain string changed: %v", doc["untouched"])
	}
	nested := doc["nested"].(map[string]any)
	if !strings.Contains(nested["email"].(string), "@") {
		t.Fatalf("email looks wrong: %v", nested["email"])
	}
}
