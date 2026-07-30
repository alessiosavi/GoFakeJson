# GoFakeJson v2 — CLI Test-Data Generator

Date: 2026-07-29
Status: Approved by user (design phase)

## Goal

Rewrite GoFakeJson as a best-practice CLI tool that coding agents (and humans) use to
generate JSON test data. Two generation modes, composable:

1. **Permutation mode** — exhaustive cartesian product of configured value substitutions
   over a template JSON (the original tool's purpose, with its combinatorial bug fixed).
2. **Random mode** — N documents with `{{generator}}` placeholders resolved per document.

Both modes share one placeholder engine and one JSON path engine.

## Bugs in v1 that this rewrite fixes

- Permutation loop iterates over *all previously generated variants* for each config
  entry, producing an exploding, incorrect combination set (3 values × 3 values yields
  16 files instead of 9) and always emitting the unmodified template.
- All injected values are forced to JSON strings — numbers, booleans, null impossible.
- A trailing empty line in the conf file triggers `panic("not a valid key=value(s)")`.
- `strings.Split(line, "=")` corrupts values containing `=`.
- All errors handled with `panic()`.
- `deepCompare` compares full 1024-byte buffers ignoring bytes actually read — wrong
  results on trailing partial chunks; dedup is O(n²) over files on disk.
- Deprecated `ioutil`, Go 1.13 directive, heavy deps (`GoGPUtils`, `jsonparser`).

## Architecture

Module `github.com/alessiosavi/GoFakeJson`, `go 1.22` directive. Single dependency:
`github.com/brianvoe/gofakeit/v7`. Dropped: `GoGPUtils`, `jsonparser`.

```
cmd/gofakejson/main.go      — entry point, wires os.Exit
internal/cli/               — flag-based subcommand parsing, usage text
internal/jsonpath/          — parse "a.b.[0].c" into segments; Set on decoded any
internal/placeholder/       — {{generator}} engine over gofakeit, seedable
internal/generate/          — cartesian product, orchestration, hash-based dedup
```

Each package has one clear purpose and is unit-testable in isolation; the CLI is a thin
shell over `internal/generate`.

## CLI

```
gofakejson generate -t template.json [-c conf.txt] [-n N] [-seed S] [-pretty]
                    [-o outdir | -stdout]
```

- `-t` template JSON file (required).
- `-c` conf file (optional).
- With `-c`: emits the true cartesian product of all `path=v1,v2,...` lines; `-n` ignored.
- Without `-c`: emits `-n` documents (default 1), placeholders resolved per document.
- Output: `-o dir` writes `0001.json`, `0002.json`, …; `-stdout` prints JSON Lines
  (one document per line). Default is `-stdout` when `-o` is not given.
- `-pretty` indents output; default is compact.
- `-seed` makes gofakeit output reproducible; default is randomly seeded.
- Errors: message on stderr, exit code 1. No panics. `-h` prints usage.

## Conf format

Backward compatible with v1: `path.to.[0].field=value1,value2`.

- Split on the **first** `=` only (values may contain `=`).
- Values split on unescaped commas; `\,` escapes a literal comma.
- A value that parses as a JSON literal (`123`, `3.14`, `true`, `null`, `"quoted"`,
  arrays/objects) is injected with that JSON type; anything else is a string.
- Blank lines and lines starting with `#` are ignored.
- Paths: dot-separated keys; array elements addressed as `[index]`
  (e.g. `Records.[0].eventName`).

## Placeholder engine

Same engine applies to template string values and conf values.
Supported generators (backed by gofakeit):

```
{{name}} {{firstname}} {{lastname}} {{email}} {{username}}
{{uuid}} {{int:min:max}} {{float:min:max}} {{bool}}
{{word}} {{sentence:N}} {{ipv4}} {{date}} {{timestamp}}
```

Typing rule: if a string value is **exactly one** placeholder, the generator's native
JSON type is used (`{{int:1:100}}` → number, `{{bool}}` → boolean, `{{uuid}}` → string).
If the placeholder is embedded in a larger string, it renders as text.

## Permutation semantics

For conf lines `p1 = v1..vk`, `p2 = w1..wm`, … the output is the cartesian product:
for every tuple of values, take the template and set each path to its tuple value.
Template itself is **not** emitted (unless the product happens to reproduce it).
Placeholders inside conf values are resolved per emitted document.

## Dedup

SHA-256 of each document's canonical (compact, key-sorted via encoding/json) bytes in an
in-memory set, checked **before** writing. Duplicates are skipped and counted in the
summary line on stderr.

## Error handling

All errors wrapped with context (`fmt.Errorf("...: %w", err)`), surfaced once in main.
Exit codes: 0 success, 1 any failure (bad flags, unreadable files, invalid JSON,
unknown placeholder, bad path).

## Testing

Table-driven unit tests per package:

- `jsonpath`: parse valid/invalid paths; Set on objects/arrays; error cases
  (missing key, index out of range, type mismatch).
- `placeholder`: each generator with a fixed seed; typing rule (exact vs embedded);
  unknown placeholder error.
- `generate`: cartesian-product correctness (count and content), dedup, conf parsing
  edge cases (comments, blank lines, `=` in values, escaped commas, JSON-typed values).
- CLI integration test: golden-file comparison for both modes with `-seed`.

Quality gates: `go vet ./...` and `gofmt -l .` clean, `go test ./...` green.
