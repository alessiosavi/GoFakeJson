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
