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
- `-stdout` write JSON Lines to stdout (the default when `-o` is not given); mutually exclusive with `-o`

## Conf format

One override per line: `path.to.[0].key=value1,value2`

- Nested keys are dot-separated; array elements are `[index]`.
- Path and values are split at the **first** `=`, so values may contain `=`.
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
- Duplicates are skipped (SHA-256 dedup of the rendered JSON), so random mode
  can emit fewer than `-n` documents if the rendered documents are identical.
- Exit code is 0 on success, 1 on any error.
- Use `-seed` when you need reproducible data; omit it for fresh data each run.
