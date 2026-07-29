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
// in input order, with the first list varying fastest. With no lists it
// returns one empty tuple.
func cartesian(lists [][]string) [][]string {
	out := [][]string{{}}
	for i := len(lists) - 1; i >= 0; i-- {
		var next [][]string
		for _, suffix := range out {
			for _, v := range lists[i] {
				tuple := append([]string{v}, suffix...)
				next = append(next, tuple)
			}
		}
		out = next
	}
	return out
}
