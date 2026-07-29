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
