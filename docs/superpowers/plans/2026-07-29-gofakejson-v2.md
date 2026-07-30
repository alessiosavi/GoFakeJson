# GoFakeJson v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite GoFakeJson as a clean CLI that generates JSON test data via exhaustive permutations of configured substitutions and/or `{{generator}}` placeholders.

**Architecture:** Thin `cmd/gofakejson` entry over `internal/cli`, which orchestrates `internal/generate` (cartesian product, dedup, emission), `internal/jsonpath` (path parse/set), and `internal/placeholder` (gofakeit-backed generator engine). Spec: `docs/superpowers/specs/2026-07-29-gofakejson-v2-design.md`.

**Tech Stack:** Go 1.22+, stdlib only plus `github.com/brianvoe/gofakeit/v7`.

## Global Constraints

- Module path: `github.com/alessiosavi/GoFakeJson`; `go 1.22` directive in go.mod.
- Only third-party dependency: `github.com/brianvoe/gofakeit/v7`. Drop `GoGPUtils` and `jsonparser`.
- No `panic` in library or CLI code; all errors wrapped with `fmt.Errorf("...: %w", err)` and surfaced once in `main`.
- Exit codes: 0 success, 1 any failure (flag errors, unreadable files, invalid JSON, unknown placeholder, bad path). `-h` prints usage, exit 0.
- Stdout output is JSON Lines (one document per line, trailing newline). File output is `%04d.json` (1-based) inside `-o` directory.
- Placeholder generators supported (exact set): `{{name}} {{firstname}} {{lastname}} {{email}} {{username}} {{uuid}} {{int:min:max}} {{float:min:max}} {{bool}} {{word}} {{sentence:N}} {{ipv4}} {{date}} {{timestamp}}`.
- Typing rule: a string that is exactly one placeholder yields the generator's native JSON type; embedded placeholders render as text.
- Conf format: `path=value1,value2`; split on first `=`; `\,` escapes a comma; blank lines and `#` comments ignored; values that parse as JSON literals keep their JSON type.
- Paths: dot-separated keys, array elements as `[index]`; setting a missing key or out-of-range index is an error.
- Quality gates (run at the end of every task): `gofmt -l .` empty, `go vet ./...` clean, `go test ./...` green.

---

### Task 1: Module reset and scaffolding

**Files:**
- Modify: `go.mod`
- Delete: `main.go` (v1 implementation), `go.sum` (regenerated)
- Keep: `conf/data.json` (used as an example template later)

**Interfaces:**
- Consumes: nothing
- Produces: module `github.com/alessiosavi/GoFakeJson` with gofakeit available for import.

- [ ] **Step 1: Remove v1 code and reset the module**

```bash
git rm main.go
rm -f go.sum
```

Overwrite `go.mod` with exactly:

```
module github.com/alessiosavi/GoFakeJson

go 1.22
```

- [ ] **Step 2: Add the single dependency**

Run: `go get github.com/brianvoe/gofakeit/v7@latest`
Expected: go.mod gains a `require github.com/brianvoe/gofakeit/v7 v7.x.x` line; go.sum regenerated.

- [ ] **Step 3: Verify the module loads**

Run: `go build ./...`
Expected: success (no packages yet, or only conf/ which is not Go code — either way no errors).

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: reset module for v2 rewrite, add gofakeit dependency"
```

---

### Task 2: `jsonpath` package — parse and set

**Files:**
- Create: `internal/jsonpath/jsonpath.go`
- Test: `internal/jsonpath/jsonpath_test.go`

**Interfaces:**
- Consumes: nothing
- Produces (used by `generate` in Task 4):
  - `type Segment struct { Key string; Index int; IsIndex bool }`
  - `func Parse(path string) ([]Segment, error)`
  - `func Set(doc any, path []Segment, value any) error`

- [ ] **Step 1: Write the failing tests**

Create `internal/jsonpath/jsonpath_test.go`:

```go
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
		"missing",                  // missing key
		"Records.missing.eventName", // missing intermediate key
		"Records.[5].eventName",    // index out of range
		"Records.eventName",        // key into array (type mismatch)
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/jsonpath/`
Expected: FAIL — `package .../internal/jsonpath: no Go files` or undefined symbols.

- [ ] **Step 3: Implement the package**

Create `internal/jsonpath/jsonpath.go`:

```go
// Package jsonpath parses simple dot-separated JSON paths
// (e.g. "Records.[0].eventName") and sets values at those paths
// inside decoded JSON documents (map[string]any / []any trees).
package jsonpath

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Segment is one step of a parsed path: either an object key or an array index.
type Segment struct {
	Key     string
	Index   int
	IsIndex bool
}

// Parse converts "a.b.[0].c" into a slice of Segments.
func Parse(path string) ([]Segment, error) {
	if path == "" {
		return nil, errors.New("jsonpath: empty path")
	}
	parts := strings.Split(path, ".")
	segs := make([]Segment, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("jsonpath: empty segment in %q", path)
		}
		if strings.HasPrefix(p, "[") && strings.HasSuffix(p, "]") {
			n, err := strconv.Atoi(p[1 : len(p)-1])
			if err != nil || n < 0 {
				return nil, fmt.Errorf("jsonpath: invalid index %q in %q", p, path)
			}
			segs = append(segs, Segment{Index: n, IsIndex: true})
			continue
		}
		if strings.HasPrefix(p, "[") || strings.HasSuffix(p, "]") {
			return nil, fmt.Errorf("jsonpath: malformed segment %q in %q", p, path)
		}
		segs = append(segs, Segment{Key: p})
	}
	return segs, nil
}

// Set replaces the value at path inside doc. The path must already exist:
// missing keys, out-of-range indexes, and type mismatches are errors.
func Set(doc any, path []Segment, value any) error {
	if len(path) == 0 {
		return errors.New("jsonpath: empty path")
	}
	cur := doc
	for i, seg := range path {
		last := i == len(path)-1
		if seg.IsIndex {
			arr, ok := cur.([]any)
			if !ok {
				return fmt.Errorf("jsonpath: segment %d: expected array, got %T", i, cur)
			}
			if seg.Index >= len(arr) {
				return fmt.Errorf("jsonpath: index %d out of range (len %d)", seg.Index, len(arr))
			}
			if last {
				arr[seg.Index] = value
				return nil
			}
			cur = arr[seg.Index]
			continue
		}
		obj, ok := cur.(map[string]any)
		if !ok {
			return fmt.Errorf("jsonpath: segment %d: expected object, got %T", i, cur)
		}
		next, ok := obj[seg.Key]
		if !ok {
			return fmt.Errorf("jsonpath: key %q not found", seg.Key)
		}
		if last {
			obj[seg.Key] = value
			return nil
		}
		cur = next
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/jsonpath/ -v && gofmt -l . && go vet ./...`
Expected: all PASS; gofmt prints nothing; vet clean.

- [ ] **Step 5: Commit**

```bash
git add internal/jsonpath/
git commit -m "feat: add jsonpath package with Parse and Set"
```

---

### Task 3: `placeholder` package — generator engine

**Files:**
- Create: `internal/placeholder/placeholder.go`
- Test: `internal/placeholder/placeholder_test.go`

**Interfaces:**
- Consumes: `github.com/brianvoe/gofakeit/v7`
- Produces (used by `generate` in Task 4):
  - `type Engine struct` (wraps `*gofakeit.Faker`)
  - `func New(seed int64) *Engine` — seed 0 means random seed
  - `func (e *Engine) Resolve(s string) (any, error)` — resolve placeholders in one string; exact single placeholder yields native JSON type, embedded yields string
  - `func (e *Engine) ResolveDoc(doc any) error` — recursively resolve all string values in a `map[string]any` / `[]any` tree in place

- [ ] **Step 1: Write the failing tests**

Create `internal/placeholder/placeholder_test.go`:

```go
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
		"id":    "{{uuid}}",
		"age":   "{{int:18:65}}",
		"tags":  []any{"{{word}}", "literal"},
		"nested": map[string]any{"email": "{{email}}"},
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/placeholder/`
Expected: FAIL — no Go files / undefined `New`.

- [ ] **Step 3: Implement the package**

Create `internal/placeholder/placeholder.go`:

```go
// Package placeholder resolves {{generator}} placeholders in strings and
// JSON document trees using gofakeit. A string that is exactly one
// placeholder yields the generator's native Go type (int, bool, ...);
// placeholders embedded in a larger string render as text.
package placeholder

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

var placeholderRe = regexp.MustCompile(`\{\{([^{}]+)\}\}`)

// Engine resolves placeholders. It is NOT safe for concurrent use.
type Engine struct {
	f *gofakeit.Faker
}

// New returns an Engine. seed 0 means a cryptographically random seed;
// any other value makes output reproducible.
func New(seed int64) *Engine {
	return &Engine{f: gofakeit.New(seed)}
}

// Resolve replaces placeholders in s. If s is exactly one placeholder,
// the generator's native type is returned; otherwise a string.
func (e *Engine) Resolve(s string) (any, error) {
	if loc := placeholderRe.FindStringIndex(s); loc != nil && loc[0] == 0 && loc[1] == len(s) {
		return e.resolveOne(s[2 : len(s)-2])
	}
	var rerr error
	out := placeholderRe.ReplaceAllStringFunc(s, func(m string) string {
		if rerr != nil {
			return m
		}
		v, err := e.resolveOne(m[2 : len(m)-2])
		if err != nil {
			rerr = err
			return m
		}
		return fmt.Sprint(v)
	})
	if rerr != nil {
		return nil, rerr
	}
	return out, nil
}

func (e *Engine) resolveOne(expr string) (any, error) {
	parts := strings.Split(expr, ":")
	switch parts[0] {
	case "name":
		return e.f.Name(), nil
	case "firstname":
		return e.f.FirstName(), nil
	case "lastname":
		return e.f.LastName(), nil
	case "email":
		return e.f.Email(), nil
	case "username":
		return e.f.Username(), nil
	case "uuid":
		return e.f.UUID(), nil
	case "bool":
		return e.f.Bool(), nil
	case "word":
		return e.f.Word(), nil
	case "ipv4":
		return e.f.IPv4Address(), nil
	case "date":
		return e.f.Date().Format(time.RFC3339), nil
	case "timestamp":
		return e.f.Date().Unix(), nil
	case "int":
		if len(parts) != 3 {
			return nil, fmt.Errorf("placeholder: {{int:min:max}} takes 2 arguments, got %q", expr)
		}
		min, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("placeholder: invalid int min %q", parts[1])
		}
		max, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("placeholder: invalid int max %q", parts[2])
		}
		if min > max {
			return nil, fmt.Errorf("placeholder: int min %d greater than max %d", min, max)
		}
		return e.f.IntRange(min, max), nil
	case "float":
		if len(parts) != 3 {
			return nil, fmt.Errorf("placeholder: {{float:min:max}} takes 2 arguments, got %q", expr)
		}
		min, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return nil, fmt.Errorf("placeholder: invalid float min %q", parts[1])
		}
		max, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return nil, fmt.Errorf("placeholder: invalid float max %q", parts[2])
		}
		if min > max {
			return nil, fmt.Errorf("placeholder: float min %v greater than max %v", min, max)
		}
		return min + e.f.Float64()*(max-min), nil
	case "sentence":
		if len(parts) != 2 {
			return nil, fmt.Errorf("placeholder: {{sentence:N}} takes 1 argument, got %q", expr)
		}
		n, err := strconv.Atoi(parts[1])
		if err != nil || n < 1 {
			return nil, fmt.Errorf("placeholder: invalid sentence word count %q", parts[1])
		}
		return e.f.Sentence(n), nil
	default:
		return nil, fmt.Errorf("placeholder: unknown generator %q", parts[0])
	}
}

// ResolveDoc resolves placeholders in every string value of doc
// (a decoded JSON tree) in place.
func (e *Engine) ResolveDoc(doc any) error {
	switch d := doc.(type) {
	case map[string]any:
		for k, v := range d {
			if s, ok := v.(string); ok {
				nv, err := e.Resolve(s)
				if err != nil {
					return fmt.Errorf("key %q: %w", k, err)
				}
				d[k] = nv
				continue
			}
			if err := e.ResolveDoc(v); err != nil {
				return err
			}
		}
	case []any:
		for i, v := range d {
			if s, ok := v.(string); ok {
				nv, err := e.Resolve(s)
				if err != nil {
					return fmt.Errorf("index %d: %w", i, err)
				}
				d[i] = nv
				continue
			}
			if err := e.ResolveDoc(v); err != nil {
				return err
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/placeholder/ -v && gofmt -l . && go vet ./...`
Expected: all PASS; gofmt prints nothing; vet clean.

- [ ] **Step 5: Commit**

```bash
git add internal/placeholder/
git commit -m "feat: add placeholder engine backed by gofakeit"
```

---

### Task 4: `generate` package — conf parsing, cartesian product, orchestration

**Files:**
- Create: `internal/generate/conf.go` (conf file parsing)
- Create: `internal/generate/generate.go` (cartesian product, dedup, Run)
- Test: `internal/generate/conf_test.go`
- Test: `internal/generate/generate_test.go`

**Interfaces:**
- Consumes:
  - `jsonpath.Parse(path string) ([]jsonpath.Segment, error)`, `jsonpath.Set(doc any, path []jsonpath.Segment, value any) error` (Task 2)
  - `placeholder.New(seed int64) *Engine`, `(*Engine).ResolveDoc(doc any) error` (Task 3)
- Produces (used by `cli` in Task 5):
  - `type Options struct { TemplatePath string; ConfPath string; Count int; Seed int64; Pretty bool; OutDir string }`
  - `func Run(opts Options, stdout, stderr io.Writer) error`
  - `type Override struct { Raw string; Path []jsonpath.Segment; Values []string }`
  - `func ParseConf(r io.Reader) ([]Override, error)`

- [ ] **Step 1: Write the failing conf-parsing tests**

Create `internal/generate/conf_test.go`:

```go
package generate

import (
	"strings"
	"testing"
)

func TestParseConfBasic(t *testing.T) {
	in := "# comment line\n\nRecords.[0].eventName=INSERT,REMOVE,MODIFY\n"
	ovs, err := ParseConf(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ovs) != 1 {
		t.Fatalf("got %d overrides, want 1", len(ovs))
	}
	if ovs[0].Raw != "Records.[0].eventName" {
		t.Fatalf("raw path: %q", ovs[0].Raw)
	}
	if len(ovs[0].Path) != 3 {
		t.Fatalf("parsed path segments: %+v", ovs[0].Path)
	}
	want := []string{"INSERT", "REMOVE", "MODIFY"}
	if strings.Join(ovs[0].Values, ",") != strings.Join(want, ",") {
		t.Fatalf("values: %v", ovs[0].Values)
	}
}

func TestParseConfValueWithEquals(t *testing.T) {
	ovs, err := ParseConf(strings.NewReader("auth=Basic dXNlcj1wYXNz"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ovs[0].Values[0] != "Basic dXNlcj1wYXNz" {
		t.Fatalf("value mangled: %q", ovs[0].Values[0])
	}
}

func TestParseConfEscapedComma(t *testing.T) {
	ovs, err := ParseConf(strings.NewReader(`note=a\,b,c`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ovs[0].Values) != 2 || ovs[0].Values[0] != "a,b" || ovs[0].Values[1] != "c" {
		t.Fatalf("values: %q", ovs[0].Values)
	}
}

func TestParseConfErrors(t *testing.T) {
	for _, in := range []string{"noequals", "a..b=x", "=x"} {
		if _, err := ParseConf(strings.NewReader(in)); err == nil {
			t.Errorf("ParseConf(%q): expected error, got nil", in)
		}
	}
}
```

- [ ] **Step 2: Run conf tests to verify they fail**

Run: `go test ./internal/generate/ -run TestParseConf`
Expected: FAIL — no Go files / undefined `ParseConf`.

- [ ] **Step 3: Implement conf parsing**

Create `internal/generate/conf.go`:

```go
package generate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/alessiosavi/GoFakeJson/internal/jsonpath"
)

// Override is one conf line: a JSON path and the list of values to permute.
type Override struct {
	Raw    string
	Path   []jsonpath.Segment
	Values []string
}

// ParseConf reads conf lines of the form "path.to.[0].key=v1,v2,...".
// Blank lines and lines starting with '#' are ignored. The key is split
// from values at the first '='. Values are split on unescaped commas
// (`\,` yields a literal comma).
func ParseConf(r io.Reader) ([]Override, error) {
	var out []Override
	sc := bufio.NewScanner(r)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("conf line %d: expected path=value(s), got %q", lineNo, line)
		}
		raw := strings.TrimSpace(line[:eq])
		path, err := jsonpath.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("conf line %d: %w", lineNo, err)
		}
		out = append(out, Override{Raw: raw, Path: path, Values: splitValues(line[eq+1:])})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read conf: %w", err)
	}
	return out, nil
}

// splitValues splits s on commas not preceded by a backslash.
func splitValues(s string) []string {
	var out []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == ',' {
			cur.WriteByte(',')
			i++
			continue
		}
		if s[i] == ',' {
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(s[i])
	}
	return append(out, cur.String())
}

// parseValue interprets s as a JSON literal when possible (numbers,
// booleans, null, quoted strings, arrays, objects); otherwise s stays
// a string. Strings containing {{placeholders}} never parse as JSON and
// are left for the placeholder engine.
func parseValue(s string) any {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return s
}
```

- [ ] **Step 4: Run conf tests to verify they pass**

Run: `go test ./internal/generate/ -run TestParseConf -v`
Expected: PASS.

- [ ] **Step 5: Write the failing orchestration tests**

Create `internal/generate/generate_test.go`:

```go
package generate

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testTemplate = `{
	"eventName": "INSERT",
	"count": 1,
	"user": {"name": "placeholder", "id": "{{uuid}}"}
}`

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunPermutationCartesianProduct(t *testing.T) {
	dir := t.TempDir()
	tpl := writeTemp(t, dir, "t.json", testTemplate)
	conf := writeTemp(t, dir, "c.txt", "eventName=INSERT,REMOVE,MODIFY\ncount=1,2\n")
	out := filepath.Join(dir, "out")

	var stdout, stderr bytes.Buffer
	err := Run(Options{TemplatePath: tpl, ConfPath: conf, OutDir: out, Seed: 1}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 6 { // 3 x 2, true cartesian product
		t.Fatalf("got %d files, want 6", len(entries))
	}
	if entries[0].Name() != "0001.json" || entries[5].Name() != "0006.json" {
		t.Fatalf("unexpected file names: %q, %q", entries[0].Name(), entries[5].Name())
	}

	// Every combination must be present exactly once.
	type combo struct {
		Event string
		Count float64
	}
	seen := map[combo]bool{}
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(out, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s: invalid JSON: %v", e.Name(), err)
		}
		c := combo{Event: doc["eventName"].(string), Count: doc["count"].(float64)}
		if seen[c] {
			t.Fatalf("combination %+v emitted twice", c)
		}
		seen[c] = true
		// Placeholders in the template are resolved per document.
		if strings.Contains(doc["user"].(map[string]any)["id"].(string), "{{") {
			t.Fatalf("%s: unresolved placeholder", e.Name())
		}
	}
	if len(seen) != 6 {
		t.Fatalf("got %d distinct combinations, want 6", len(seen))
	}
	if !strings.Contains(stderr.String(), "generated 6 document(s)") {
		t.Fatalf("stderr summary: %q", stderr.String())
	}
}

func TestRunTypedValues(t *testing.T) {
	dir := t.TempDir()
	tpl := writeTemp(t, dir, "t.json", `{"flag": false, "n": 0, "s": "x"}`)
	conf := writeTemp(t, dir, "c.txt", "flag=true\nn=42\ns=\"quoted\"\n")

	var stdout, stderr bytes.Buffer
	if err := Run(Options{TemplatePath: tpl, ConfPath: conf, Seed: 1}, &stdout, &stderr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &doc); err != nil {
		t.Fatalf("invalid JSON on stdout: %v", err)
	}
	if doc["flag"] != true {
		t.Fatalf("flag: %v (%T)", doc["flag"], doc["flag"])
	}
	if doc["n"] != float64(42) {
		t.Fatalf("n: %v (%T)", doc["n"], doc["n"])
	}
	if doc["s"] != "quoted" {
		t.Fatalf("s: %v", doc["s"])
	}
}

func TestRunRandomModeStdoutJSONL(t *testing.T) {
	dir := t.TempDir()
	tpl := writeTemp(t, dir, "t.json", testTemplate)

	var stdout, stderr bytes.Buffer
	err := Run(Options{TemplatePath: tpl, Count: 3, Seed: 9}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d JSONL lines, want 3", len(lines))
	}
	for _, l := range lines {
		var doc map[string]any
		if err := json.Unmarshal([]byte(l), &doc); err != nil {
			t.Fatalf("invalid JSONL line: %v", err)
		}
		if strings.Contains(l, "{{") {
			t.Fatalf("unresolved placeholder in %q", l)
		}
	}
}

func TestRunDedupSkipsIdenticalDocs(t *testing.T) {
	dir := t.TempDir()
	tpl := writeTemp(t, dir, "t.json", `{"eventName": "INSERT"}`)
	conf := writeTemp(t, dir, "c.txt", "eventName=INSERT,INSERT,REMOVE\n")

	var stdout, stderr bytes.Buffer
	if err := Run(Options{TemplatePath: tpl, ConfPath: conf, Seed: 1}, &stdout, &stderr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d docs, want 2 after dedup", len(lines))
	}
	if !strings.Contains(stderr.String(), "1 duplicate(s) skipped") {
		t.Fatalf("stderr summary: %q", stderr.String())
	}
}

func TestRunErrors(t *testing.T) {
	dir := t.TempDir()
	tpl := writeTemp(t, dir, "t.json", testTemplate)
	badJSON := writeTemp(t, dir, "bad.json", "{not json")
	badPathConf := writeTemp(t, dir, "badpath.txt", "nosuchkey=1\n")

	var buf bytes.Buffer
	if err := Run(Options{TemplatePath: "does/not/exist.json"}, &buf, &buf); err == nil {
		t.Error("expected error for missing template")
	}
	if err := Run(Options{TemplatePath: badJSON}, &buf, &buf); err == nil {
		t.Error("expected error for invalid template JSON")
	}
	if err := Run(Options{TemplatePath: tpl, ConfPath: badPathConf}, &buf, &buf); err == nil {
		t.Error("expected error for override on missing key")
	}
}
```

- [ ] **Step 6: Run orchestration tests to verify they fail**

Run: `go test ./internal/generate/ -run TestRun`
Expected: FAIL — undefined `Run` / `Options`.

- [ ] **Step 7: Implement the orchestration**

Create `internal/generate/generate.go`:

```go
// Package generate builds JSON test documents from a template: either the
// exhaustive cartesian product of conf-file overrides, or N randomized
// documents via {{generator}} placeholders.
package generate

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/alessiosavi/GoFakeJson/internal/jsonpath"
	"github.com/alessiosavi/GoFakeJson/internal/placeholder"
)

// Options controls a Run.
type Options struct {
	TemplatePath string // required
	ConfPath     string // optional; enables permutation mode
	Count        int    // random mode only; <= 0 means 1
	Seed         int64  // 0 means random seed
	Pretty       bool   // indent output
	OutDir       string // "" means stdout (JSON Lines)
}

// Run generates documents and writes them to OutDir (as %04d.json files)
// or to stdout (JSON Lines). A summary goes to stderr.
func Run(opts Options, stdout, stderr io.Writer) error {
	if opts.TemplatePath == "" {
		return errors.New("generate: template path is required")
	}
	raw, err := os.ReadFile(opts.TemplatePath)
	if err != nil {
		return fmt.Errorf("generate: read template: %w", err)
	}
	var template any
	if err := json.Unmarshal(raw, &template); err != nil {
		return fmt.Errorf("generate: parse template %s: %w", opts.TemplatePath, err)
	}

	eng := placeholder.New(opts.Seed)

	var overrides []Override
	var tuples [][]string
	if opts.ConfPath != "" {
		f, err := os.Open(opts.ConfPath)
		if err != nil {
			return fmt.Errorf("generate: open conf: %w", err)
		}
		overrides, err = ParseConf(f)
		f.Close()
		if err != nil {
			return err
		}
		if len(overrides) == 0 {
			return errors.New("generate: conf file has no overrides")
		}
		lists := make([][]string, len(overrides))
		for i, ov := range overrides {
			lists[i] = ov.Values
		}
		tuples = cartesian(lists)
	} else {
		n := opts.Count
		if n <= 0 {
			n = 1
		}
		tuples = make([][]string, n) // empty tuples: template + placeholders only
	}

	if opts.OutDir != "" {
		if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
			return fmt.Errorf("generate: create output dir: %w", err)
		}
	}

	seen := make(map[[sha256.Size]byte]struct{})
	written, dup := 0, 0
	for _, tuple := range tuples {
		doc, err := clone(template)
		if err != nil {
			return err
		}
		for i, ov := range overrides {
			if err := jsonpath.Set(doc, ov.Path, parseValue(tuple[i])); err != nil {
				return fmt.Errorf("generate: override %q: %w", ov.Raw, err)
			}
		}
		if err := eng.ResolveDoc(doc); err != nil {
			return fmt.Errorf("generate: %w", err)
		}

		compact, err := json.Marshal(doc)
		if err != nil {
			return fmt.Errorf("generate: marshal document: %w", err)
		}
		sum := sha256.Sum256(compact)
		if _, ok := seen[sum]; ok {
			dup++
			continue
		}
		seen[sum] = struct{}{}

		out := compact
		if opts.Pretty {
			if out, err = json.MarshalIndent(doc, "", "  "); err != nil {
				return fmt.Errorf("generate: marshal document: %w", err)
			}
		}
		out = append(out, '\n')

		if opts.OutDir == "" {
			if _, err := stdout.Write(out); err != nil {
				return fmt.Errorf("generate: write stdout: %w", err)
			}
		} else {
			name := filepath.Join(opts.OutDir, fmt.Sprintf("%04d.json", written+1))
			if err := os.WriteFile(name, out, 0o644); err != nil {
				return fmt.Errorf("generate: write %s: %w", name, err)
			}
		}
		written++
	}
	fmt.Fprintf(stderr, "generated %d document(s), %d duplicate(s) skipped\n", written, dup)
	return nil
}

// clone deep-copies a decoded JSON tree via marshal/unmarshal.
func clone(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("generate: clone template: %w", err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("generate: clone template: %w", err)
	}
	return out, nil
}

// cartesian returns every tuple picking one element from each input list,
// in input order. With no lists it returns one empty tuple.
func cartesian(lists [][]string) [][]string {
	out := [][]string{{}}
	for _, list := range lists {
		var next [][]string
		for _, prefix := range out {
			for _, v := range list {
				tuple := append(append([]string{}, prefix...), v)
				next = append(next, tuple)
			}
		}
		out = next
	}
	return out
}
```

- [ ] **Step 8: Run all tests to verify they pass**

Run: `go test ./... && gofmt -l . && go vet ./...`
Expected: all PASS; gofmt prints nothing; vet clean.

- [ ] **Step 9: Commit**

```bash
git add internal/generate/
git commit -m "feat: add generate package (conf parsing, cartesian product, dedup)"
```

---

### Task 5: CLI — subcommand, flags, entry point

**Files:**
- Create: `internal/cli/cli.go`
- Create: `cmd/gofakejson/main.go`
- Test: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `generate.Options`, `generate.Run(opts, stdout, stderr) error` (Task 4)
- Produces: `func Main(args []string, stdout, stderr io.Writer) int` — called by `cmd/gofakejson/main.go` as `os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))`.

- [ ] **Step 1: Write the failing tests**

Create `internal/cli/cli_test.go`:

```go
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGeneratePermutationGolden(t *testing.T) {
	dir := t.TempDir()
	tpl := writeTemp(t, dir, "t.json", `{"eventName": "INSERT", "count": 1}`)
	conf := writeTemp(t, dir, "c.txt", "eventName=INSERT,REMOVE\ncount=1,2\n")
	out := filepath.Join(dir, "out")

	var stdout, stderr bytes.Buffer
	code := Main([]string{"generate", "-t", tpl, "-c", conf, "-o", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr.String())
	}

	want := map[string]string{
		"0001.json": `{"count":1,"eventName":"INSERT"}`,
		"0002.json": `{"count":1,"eventName":"REMOVE"}`,
		"0003.json": `{"count":2,"eventName":"INSERT"}`,
		"0004.json": `{"count":2,"eventName":"REMOVE"}`,
	}
	for name, w := range want {
		raw, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(raw)) != w {
			t.Errorf("%s: got %s, want %s", name, strings.TrimSpace(string(raw)), w)
		}
	}
}

func TestGenerateRandomModeSeeded(t *testing.T) {
	dir := t.TempDir()
	tpl := writeTemp(t, dir, "t.json", `{"id": "{{uuid}}", "age": "{{int:18:65}}", "email": "{{email}}"}`)

	var stdout, stderr bytes.Buffer
	code := Main([]string{"generate", "-t", tpl, "-n", "3", "-seed", "42"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d JSONL lines, want 3", len(lines))
	}
	for _, l := range lines {
		if strings.Contains(l, "{{") {
			t.Fatalf("unresolved placeholder: %q", l)
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(l), &doc); err != nil {
			t.Fatalf("invalid JSONL line: %v", err)
		}
		if _, ok := doc["age"].(float64); !ok {
			t.Fatalf("age should be a JSON number, got %T", doc["age"])
		}
		if !strings.Contains(doc["email"].(string), "@") {
			t.Fatalf("email looks wrong: %v", doc["email"])
		}
	}
}

func TestFlagAndUsageErrors(t *testing.T) {
	var buf bytes.Buffer
	if code := Main(nil, &buf, &buf); code != 1 {
		t.Errorf("no args: got code %d, want 1", code)
	}
	if code := Main([]string{"bogus"}, &buf, &buf); code != 1 {
		t.Errorf("unknown command: got code %d, want 1", code)
	}
	if code := Main([]string{"generate"}, &buf, &buf); code != 1 {
		t.Errorf("missing -t: got code %d, want 1", code)
	}
	dir := t.TempDir()
	tpl := writeTemp(t, dir, "t.json", `{}`)
	if code := Main([]string{"generate", "-t", tpl, "-o", "x", "-stdout"}, &buf, &buf); code != 1 {
		t.Errorf("-o with -stdout: got code %d, want 1", code)
	}
}

func TestHelpExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	for _, args := range [][]string{{"-h"}, {"--help"}, {"help"}, {"generate", "-h"}} {
		stdout.Reset()
		stderr.Reset()
		if code := Main(args, &stdout, &stderr); code != 0 {
			t.Errorf("%v: got code %d, want 0", args, code)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/`
Expected: FAIL — no Go files / undefined `Main`.

- [ ] **Step 3: Implement the CLI package**

Create `internal/cli/cli.go`:

```go
// Package cli implements the gofakejson command-line interface.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/alessiosavi/GoFakeJson/internal/generate"
)

const usage = `gofakejson — JSON test data generator

Usage:
  gofakejson generate -t template.json [-c conf.txt] [-n N] [-seed S] [-pretty] [-o outdir | -stdout]

Modes:
  With -c    permutation mode: emits the exhaustive cartesian product of the
             conf file's path=value(s) overrides.
  Without -c random mode: emits -n documents (default 1) with {{generator}}
             placeholders resolved per document.

Flags:
  -t       template JSON file (required)
  -c       conf file: lines of path.to.[0].key=value1,value2 ('#' comments ok)
  -n       number of documents in random mode (default 1)
  -seed    random seed for reproducible output (default 0 = random)
  -pretty  pretty-print JSON output
  -o       output directory; files are written as 0001.json, 0002.json, ...
  -stdout  write JSON Lines to stdout (default when -o is not given)

Placeholders (usable in template values and conf values):
  {{name}} {{firstname}} {{lastname}} {{email}} {{username}} {{uuid}}
  {{int:min:max}} {{float:min:max}} {{bool}} {{word}} {{sentence:N}}
  {{ipv4}} {{date}} {{timestamp}}
  A string that is exactly one placeholder keeps its native JSON type.

Examples:
  gofakejson generate -t conf/data.json -c conf/conf.txt -o generated
  gofakejson generate -t user.json -n 100 -seed 1 | jq .
`

// Main runs the CLI with args (excluding the program name) and returns the
// process exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 1
	}
	switch args[0] {
	case "generate":
		return generateCmd(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 1
	}
}

func generateCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	template := fs.String("t", "", "path to template JSON (required)")
	conf := fs.String("c", "", "path to conf file (enables permutation mode)")
	count := fs.Int("n", 1, "number of documents in random mode")
	seed := fs.Int64("seed", 0, "random seed (0 = random)")
	pretty := fs.Bool("pretty", false, "pretty-print output")
	outDir := fs.String("o", "", "output directory (default: stdout)")
	toStdout := fs.Bool("stdout", false, "write JSON Lines to stdout")
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		return 1
	}
	if *template == "" {
		fmt.Fprintln(stderr, "error: -t template is required")
		return 1
	}
	if *outDir != "" && *toStdout {
		fmt.Fprintln(stderr, "error: -o and -stdout are mutually exclusive")
		return 1
	}
	opts := generate.Options{
		TemplatePath: *template,
		ConfPath:     *conf,
		Count:        *count,
		Seed:         *seed,
		Pretty:       *pretty,
		OutDir:       *outDir,
	}
	if err := generate.Run(opts, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return 0
}
```

- [ ] **Step 4: Implement the entry point**

Create `cmd/gofakejson/main.go`:

```go
// Command gofakejson generates JSON test data from a template.
package main

import (
	"os"

	"github.com/alessiosavi/GoFakeJson/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./... && gofmt -l . && go vet ./... && go build ./cmd/gofakejson`
Expected: all PASS; gofmt prints nothing; vet clean; binary `gofakejson` builds at repo root.

Remove the test binary afterward: `rm -f gofakejson` (it is gitignored in Task 6).

- [ ] **Step 6: Commit**

```bash
git add internal/cli/ cmd/
git commit -m "feat: add generate subcommand CLI and entry point"
```

---

### Task 6: README, example conf, gitignore, end-to-end verification

**Files:**
- Modify: `Readme.md` (full rewrite)
- Create: `conf/conf.txt` (example matching `conf/data.json`)
- Modify: `.gitignore` (add built binary and generated output)

**Interfaces:**
- Consumes: everything above; verifies the built binary end to end.
- Produces: user-facing documentation.

- [ ] **Step 1: Create the example conf file**

Create `conf/conf.txt`:

```
# Permute the DynamoDB event name and one nested payload value
Records.[0].eventName=INSERT,REMOVE,MODIFY
Records.[0].dynamodb.NewImage.val.S=data,test
```

- [ ] **Step 2: Update .gitignore**

Run:

```bash
grep -qxF '/gofakejson' .gitignore || echo '/gofakejson' >> .gitignore
grep -qxF '/generated/' .gitignore || echo '/generated/' >> .gitignore
```

Expected: `.gitignore` ends with `/gofakejson` and `/generated/` (each once).

- [ ] **Step 3: Rewrite Readme.md**

Overwrite `Readme.md` with:

````markdown
# GoFakeJson

Generate JSON test data from a template — built for use by coding agents and humans.

Two modes, composable:

- **Permutation mode**: emit the exhaustive cartesian product of configured
  `path=value(s)` substitutions over a template JSON.
- **Random mode**: emit N documents with `{{generator}}` placeholders
  (names, emails, UUIDs, numbers, dates...) resolved per document.

## Install

```bash
go build -o gofakejson ./cmd/gofakejson
```

## Usage

```
gofakejson generate -t template.json [-c conf.txt] [-n N] [-seed S] [-pretty] [-o outdir | -stdout]
```

- `-t` template JSON file (required)
- `-c` conf file: enables permutation mode
- `-n` number of documents in random mode (default 1)
- `-seed` random seed for reproducible output (default: random)
- `-pretty` pretty-print JSON
- `-o` output directory (`0001.json`, `0002.json`, ...); without it, JSON Lines go to stdout

## Conf format

One override per line: `path.to.[0].key=value1,value2`

- Nested keys are dot-separated; array elements are `[index]`.
- Values are split on commas (`\,` escapes a literal comma).
- Values that parse as JSON literals keep their type: `42` → number,
  `true` → boolean, `null` → null, `"quoted"` → string.
- Blank lines and `#` comments are ignored.

Example (with the template in `conf/data.json`):

```
Records.[0].eventName=INSERT,REMOVE,MODIFY
Records.[0].dynamodb.NewImage.val.S=data,test
```

```bash
gofakejson generate -t conf/data.json -c conf/conf.txt -o generated
# writes generated/0001.json ... generated/0006.json (3 x 2 combinations)
```

## Placeholders

Usable in template string values and conf values:

```
{{name}} {{firstname}} {{lastname}} {{email}} {{username}} {{uuid}}
{{int:min:max}} {{float:min:max}} {{bool}} {{word}} {{sentence:N}}
{{ipv4}} {{date}} {{timestamp}}
```

If a string is exactly one placeholder, the value keeps its native JSON type
(`{{int:1:100}}` → number, `{{bool}}` → boolean). Embedded placeholders render
as text: `"user-{{uuid}}"`.

```bash
# 100 random users, reproducible with -seed, piped as JSON Lines:
gofakejson generate -t user-template.json -n 100 -seed 1 | jq .
```

## Agent usage notes

- Output on stdout is newline-delimited JSON — safe to pipe to `jq`, files, or
  another tool.
- All diagnostics (including the `generated N document(s)` summary) go to stderr.
- Exit code is 0 on success, 1 on any error.
- Use `-seed` when you need reproducible data; omit it for fresh data each run.
````

- [ ] **Step 4: End-to-end smoke test (permutation mode)**

Run:

```bash
go build -o gofakejson ./cmd/gofakejson
./gofakejson generate -t conf/data.json -c conf/conf.txt -o generated
ls generated | wc -l
head -c 200 generated/0001.json
```

Expected: exit 0; stderr prints `generated 6 document(s), 0 duplicate(s) skipped`; 6 files; each is valid JSON with the substituted values (no `{{`).

- [ ] **Step 5: End-to-end smoke test (random mode, seeded, stdout)**

Run:

```bash
./gofakejson generate -t conf/data.json -n 2 -seed 1 | head -2
./gofakejson generate -t conf/data.json -n 2 -seed 1 | grep -c '{{' || true
```

Expected: exit 0; two JSON lines on stdout; no `{{` sequences (grep prints 0). If `jq` is available, also run `./gofakejson generate -t conf/data.json -n 2 -seed 1 | jq -c . >/dev/null` — it must exit 0 (valid JSON on every line).

- [ ] **Step 6: Final quality gates**

Run: `gofmt -l . && go vet ./... && go test ./...`
Expected: gofmt prints nothing, vet clean, all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add Readme.md conf/conf.txt .gitignore
git commit -m "docs: rewrite README for v2 CLI, add example conf"
```

---
