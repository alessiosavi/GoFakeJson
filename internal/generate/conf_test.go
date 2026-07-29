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
