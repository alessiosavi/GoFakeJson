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
