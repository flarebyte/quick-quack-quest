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

## Quick CUE Config Snippets

Use these minimal snippets to get started quickly.

```cue
package designmeta

cliSpec: {
  validation: {
    engine: "duckdb"
  }

  datasets: [{
    id:          "sales_daily"
    name:        "Sales Daily"
    format:      "csv"
    layout:      "single_file"
    path:        "./data/sales.csv"
    compression: "none"
    fields: [
      {name: "sale_date", type: "DATE", nullable: false},
      {name: "country", type: "VARCHAR", nullable: false},
      {name: "revenue", type: "DOUBLE", nullable: false},
    ]
  }]

  queries: [{
    id:                "sales_by_country"
    name:              "Sales by Country"
    required_datasets: ["sales_daily"]
    parameters: [
      {name: "start_date", type: "DATE", required: true},
      {name: "end_date", type: "DATE", required: true},
    ]
    sql: """
      SELECT country, SUM(revenue) AS total_revenue
      FROM sales_daily
      WHERE sale_date BETWEEN $start_date AND $end_date
      GROUP BY country
      ORDER BY total_revenue DESC
    """
  }]
}
```

Run with a custom config path:

```bash
quick-quack-quest config validate --config ./path/to/cli-config.cue
quick-quack-quest dataset validate sales_daily --config ./path/to/cli-config.cue --format json
quick-quack-quest query run sales_by_country --config ./path/to/cli-config.cue --param start_date=2026-01-01 --param end_date=2026-01-31 --format table
```

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
