// Package placeholder resolves {{generator}} placeholders in strings and
// JSON document trees using gofakeit. A string that is exactly one
// placeholder yields the generator's native Go type (int, bool, ...);
// placeholders embedded in a larger string render as text.
package placeholder

import (
	"fmt"
	"regexp"
	"sort"
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
	return &Engine{f: gofakeit.New(uint64(seed))}
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
	// Zero-argument generators must not receive stray arguments.
	switch parts[0] {
	case "name", "firstname", "lastname", "email", "username", "uuid",
		"bool", "word", "ipv4", "date", "timestamp":
		if len(parts) != 1 {
			return nil, fmt.Errorf("placeholder: %s takes no arguments, got %q", parts[0], expr)
		}
	}
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
// (a decoded JSON tree) in place. Object keys are visited in sorted order:
// every resolution draws from the same seeded PRNG, so a stable traversal
// order is what makes -seed reproducible for documents with more than one
// placeholder.
func (e *Engine) ResolveDoc(doc any) error {
	switch d := doc.(type) {
	case map[string]any:
		keys := make([]string, 0, len(d))
		for k := range d {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := d[k]
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
