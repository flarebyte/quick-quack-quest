# quick-quack-quest

`quick-quack-quest` is a Go CLI (Cobra) to validate data files and run parameterized DuckDB queries from a CUE configuration.

## Illustration

![quick-quack-quest hero](doc/quick-quack-quest-hero.png)

## CLI perspective

From a user perspective, the CLI lets you:

- Declare datasets (CSV, JSON, NDJSON, Parquet), schema fields, compression, and optional homepage/source metadata.
- Validate datasets against declared structure with either:
  - `duckdb` engine
  - `native` engine
- Run reusable DuckDB queries with parameters.
- Control large workloads with sampling, streaming, progress, and safety limits.

## Configuration

The source of truth is CUE config under `doc/design-meta/examples/config/`:

- `cli-config.cue`: example runtime and dataset/query config
- `cli-config.schema.cue`: config schema

Design and command catalogs are documented under:

- `doc/design/duckdb-cli-spec.md`

## Command groups

Planned command surface:

- `dataset list`
- `dataset validate <dataset-id>`
- `dataset validate-all`
- `dataset inspect <dataset-id>`
- `query list`
- `query run <query-id>`
- `query explain <query-id>`
- `config validate`
- `version`

## Typical workflow

1. Update dataset/query definitions in CUE config.
2. Run config validation.
3. Run dataset validation (`duckdb` or `native` engine).
4. Execute queries with parameters and output format (`table`, `json`, `jsonl`, `csv`).
