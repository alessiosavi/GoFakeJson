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
