# recon

Reconcile two JSONL datasets and group every difference into a category, so a long list of mismatches becomes a short list of fixable problems.

I wrote it after doing large data-migration validation work, where the hard part is rarely finding differences. It is working out which ones share a cause. `recon` does that grouping and nothing else. It uses only the Go standard library.

## Usage

```
recon [flags] source.jsonl target.jsonl

  -key string      field that identifies a record (default "id")
  -ignore string   comma-separated fields to ignore
  -samples int     sample IDs to show per category (default 3)
  -json            print the report as JSON
```

Each line of each file is one JSON object. The exit code is `0` when the files match, `1` when there are differences and `2` on bad input, so it works in a CI step.

## Example

The `examples/` folder holds 2,000 synthetic records and a copy with deliberate faults:

```
$ recon examples/source.jsonl examples/target.jsonl
source: 2000  target: 1991  matched: 1821
  field:email                  87   e.g. 100, 1011, 1015
  field:tier                   47   e.g. 1048, 1058, 1116
  field:points                 24   e.g. 1002, 1127, 1329
  missing_in_target            21   e.g. 1072, 1076, 1219
  extra_in_target              12   e.g. 2001, 2002, 2003
```

179 differing records collapse into five categories, each with sample IDs to start from.

## Design notes

- **Categories, not a diff dump.** A record that differs in two fields counts once in each field's category. A missing field counts as a difference, so a dropped value is never silently ignored.
- **Deterministic output.** IDs are visited in sorted order, so the same input always gives the same samples, which keeps reports easy to compare between runs.
- **Bounded memory per category.** Only counts and a fixed number of sample IDs are kept. Both files are loaded into memory, so this fits datasets that fit in RAM. It is not a streaming tool.
- **Concurrent loading.** The two files are read in parallel goroutines.
- **Strict input.** Bad JSON, a missing key or a duplicate key fails with the line number, because a quiet skip would make the report wrong.

## Limitations

- Values are compared with `reflect.DeepEqual`, so `1` and `1.0` can differ depending on how they decode.
- There is no fuzzy matching, case folding or per-field normalisation. Everything is an exact comparison.
- No Kafka or database input yet. Anything that can export JSONL works.

## Tests

```
go test ./...
```
