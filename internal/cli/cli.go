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
