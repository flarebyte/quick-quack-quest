# DuckDB CLI Specification

Spec for a Go+Cobra CLI to validate datasets and execute DuckDB queries from CUE declarations.

## 01 Overview

### 01 Purpose

#### Feature List

| description | feature_id | feature_name | priority |
| --- | --- | --- | --- |
| Declare dataset type paths metadata and schema fields in a single source of truth | F001 | Dataset catalog in CUE | high |
| Validate declared schema against real files and detect missing or mismatched fields | F002 | Dataset structure validation | high |
| Register reusable DuckDB SQL with named runtime parameters and dataset dependencies | F003 | Parameterized query catalog | high |
| Expose validation and query operations through clear command hierarchy | F004 | Cobra CLI command groups | high |
| Resolve datasets and parameters without executing SQL for safe inspection | F005 | Dry run query planning | medium |
| Support json or table output for CI and automation | F006 | Machine readable command output | medium |
| Optionally validate a random subset of rows for very large files while keeping full validation available | F007 | Random sample dataset validation | medium |
| Represent one dataset across many related files using prefix suffix and partition keys | F008 | Partitioned dataset support | high |
| Expose core DuckDB engine settings and extensions in CUE for deterministic CLI behavior across environments | F009 | DuckDB runtime configuration | high |
| Expose CLI version and build metadata for support and release traceability | F010 | Version command | medium |
| Support chunked streaming output for large result sets and pipe-friendly formats like jsonl and csv | F011 | Streaming query output | high |
| Expose optional progress indicators with sensible TTY defaults for long running queries | F012 | Query progress reporting | medium |
| Provide max rows and timeout controls to prevent runaway queries on very large datasets | F013 | Query guardrails | high |
| Support compressed input datasets such as gzip with explicit or auto compression configuration | F014 | Compressed dataset support | high |
| Allow dataset validation via DuckDB or via native parser pipeline selected globally or per dataset | F015 | Dual validation engines | high |

#### CLI Overview

Go + Cobra CLI to validate datasets and execute parameterized DuckDB SQL queries against declared dataset sources.

### 02 Cobra Commands

#### CLI Commands

| arguments | command | command_id | flags | group | output | summary |
| --- | --- | --- | --- | --- | --- | --- |
|  | dataset list | C001 | --format | dataset | --format table\|json | List declared datasets and metadata |
| dataset-id | dataset validate <dataset-id> | C002 | --config --strict --validation-engine --compression --random-sample-rows --sample-seed --partition-filter --max-files --random-sample-files | dataset | --format table\|json | Validate one dataset file contract |
|  | dataset validate-all | C003 | --config --fail-fast --validation-engine --compression --random-sample-rows --sample-seed --partition-filter --max-files --random-sample-files | dataset | --format table\|json | Validate all declared datasets |
| dataset-id | dataset inspect <dataset-id> | C004 | --sample-size --validation-engine --compression | dataset | --format table\|json | Print discovered columns and inferred types |
|  | query list | C005 | --format | query | --format table\|json | List registered parameterized queries |
| query-id | query run <query-id> | C006 | --param key=value --limit --output --stream --progress --max-rows --timeout --chunk-size | query | --format table\|json\|jsonl\|csv | Execute one query with parameters |
| query-id | query explain <query-id> | C007 | --param key=value | query | --format text\|json | Show SQL and resolved datasets without execution |
|  | config validate | C008 | --config | config | --format table\|json | Validate CUE config structure and references |
|  | version | C009 | --format | core | --format text\|json | Print CLI version and build metadata |

### 03 Libraries

#### Required Libraries

| library | library_id | module | notes | scope | why_needed |
| --- | --- | --- | --- | --- | --- |
| Cobra | L001 | github.com/spf13/cobra | Industry standard Go CLI framework | cli | Build a structured CLI with commands flags and shell completion |
| DuckDB Go Driver | L002 | github.com/duckdb/duckdb-go/v2 | Official DuckDB Go client maintained by DuckDB | query-runtime | Execute DuckDB SQL from Go and query local data files |
| CUE | L003 | cuelang.org/go | Single source of truth for datasets queries and engine settings | config | Load and validate declarative configuration and schemas |
| CSV helper | L004 | encoding/csv | Part of Go standard library | io | Parse command and dataset catalogs where needed |
| Structured logging | L005 | log/slog | Part of Go standard library in modern Go | observability | Consistent machine readable logs for validation and query runs |
| Gzip codec | L006 | compress/gzip | Required when validation-engine=native for gzip datasets | io | Handle gzip streams when pre-processing files outside DuckDB |
| Zstd codec | L007 | github.com/klauspost/compress/zstd | Required when validation-engine=native and compression=zstd | io-optional | Handle zstd streams when pre-processing files outside DuckDB |

## 02 Dataset Model

### 01 Dataset Registry

#### Dataset Catalog

| compression | dataset_id | description | format | homepage_url | layout | name | owner | partition_keys | path | prefix | primary_key | suffix | tags |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| gzip | sales_daily | Daily store product sales transactions | csv | https://data.example.com/catalog/sales_daily | partitioned | Daily sales facts | analytics | date |  | doc/design-meta/examples/input/sales/date= | sale_id | .csv.gz | sales;facts |
| none | customers_master | Customer profile master records | json | https://data.example.com/catalog/customers_master | single_file | Customer master | crm |  | doc/design-meta/examples/input/customers.json |  | customer_id |  | customers;dimension |
| gzip | events_stream | Application behavior events for funnel analysis | ndjson | https://data.example.com/catalog/events_stream | single_file | Application events | product |  | doc/design-meta/examples/input/events.ndjson.gz |  | event_id |  | events;telemetry |

#### Dataset Fields

| dataset_id | description | duckdb_type | example | field_name | nullable |
| --- | --- | --- | --- | --- | --- |
| sales_daily | Unique sale identifier | VARCHAR | S-1001 | sale_id | false |
| sales_daily | Date of sale event | DATE | 2026-01-01 | sale_date | false |
| sales_daily | Customer foreign key | VARCHAR | C-001 | customer_id | false |
| sales_daily | Product identifier | VARCHAR | P-CHAIR | product_id | false |
| sales_daily | Quantity sold | INTEGER | 2 | quantity | false |
| sales_daily | Sale amount in local currency | DOUBLE | 129.99 | amount | false |
| customers_master | Unique customer identifier | VARCHAR | C-001 | customer_id | false |
| customers_master | Primary email address | VARCHAR | ada@example.com | email | false |
| customers_master | Country code | VARCHAR | GB | country | true |
| customers_master | First signup date | DATE | 2025-11-03 | signup_date | true |
| events_stream | Unique event identifier | VARCHAR | E-9001 | event_id | false |
| events_stream | Event timestamp in UTC | TIMESTAMP | 2026-01-01T11:30:00Z | event_ts | false |
| events_stream | Event name | VARCHAR | page_view | event_name | false |
| events_stream | Associated customer id | VARCHAR | C-001 | customer_id | true |
| events_stream | Arbitrary JSON payload | JSON | {"page":"/home"} | metadata_json | true |

### 02 Input Samples

#### Input Example: Customers JSON

```json
[
  {
    "customer_id": "C-001",
    "email": "ada@example.com",
    "country": "GB",
    "signup_date": "2025-11-03"
  },
  {
    "customer_id": "C-002",
    "email": "linus@example.com",
    "country": "US",
    "signup_date": "2025-12-19"
  }
]
```

#### Input Example: Events JSON

```json
[
  {
    "event_id": "E-9001",
    "event_ts": "2026-01-01T11:30:00Z",
    "event_name": "page_view",
    "customer_id": "C-001",
    "metadata_json": {"page": "/home"}
  },
  {
    "event_id": "E-9002",
    "event_ts": "2026-01-01T11:31:00Z",
    "event_name": "add_to_cart",
    "customer_id": "C-001",
    "metadata_json": {"sku": "P-CHAIR"}
  },
  {
    "event_id": "E-9003",
    "event_ts": "2026-01-02T09:10:00Z",
    "event_name": "page_view",
    "customer_id": "C-002",
    "metadata_json": {"page": "/pricing"}
  }
]
```

#### Input Example: Sales CSV

| amount | customer_id | product_id | quantity | sale_date | sale_id |
| --- | --- | --- | --- | --- | --- |
| 129.99 | C-001 | P-CHAIR | 2 | 2026-01-01 | S-1001 |
| 249.00 | C-002 | P-DESK | 1 | 2026-01-02 | S-1002 |
| 59.70 | C-001 | P-LAMP | 3 | 2026-01-02 | S-1003 |

## 03 Query Model

### 01 Query Catalog

#### Parameterized Query Catalog

| datasets | description | name | query_id | sql_template |
| --- | --- | --- | --- | --- |
| sales_daily;customers_master | Aggregate sales by customer country within a date range | Sales by country | sales_by_country | SELECT c.country, SUM(s.amount) AS total_amount, COUNT(*) AS tx_count FROM sales_daily s JOIN customers_master c ON s.customer_id = c.customer_id WHERE s.sale_date BETWEEN DATE($start_date) AND DATE($end_date) GROUP BY c.country ORDER BY total_amount DESC |
| events_stream | Count events per day filtered by event name | Events per day | events_per_day | SELECT DATE_TRUNC('day', event_ts) AS day, COUNT(*) AS total_events FROM events_stream WHERE event_name = $event_name GROUP BY 1 ORDER BY 1 |
| customers_master;sales_daily;events_stream | Join customer profile with purchase and event counts | Customer 360 summary | customer_360 | SELECT c.customer_id, c.email, COALESCE(SUM(s.amount), 0) AS lifetime_value, COUNT(DISTINCT e.event_id) AS event_count FROM customers_master c LEFT JOIN sales_daily s ON s.customer_id = c.customer_id LEFT JOIN events_stream e ON e.customer_id = c.customer_id WHERE c.customer_id = $customer_id GROUP BY c.customer_id, c.email |

#### Query Parameters

| description | duckdb_type | example | param_name | query_id | required |
| --- | --- | --- | --- | --- | --- |
| Inclusive start date | DATE | 2026-01-01 | start_date | sales_by_country | true |
| Inclusive end date | DATE | 2026-01-31 | end_date | sales_by_country | true |
| Event type to include | VARCHAR | page_view | event_name | events_per_day | true |
| Customer id to inspect | VARCHAR | C-001 | customer_id | customer_360 | true |

### 02 CUE Schema And Example

#### CUE Example CLI Config

```cue
package designmeta

cliSpec: #CliSpec & {
	validation: {
		engine: "duckdb"
		// Optional default sample size used by dataset validation on very large files.
		random_sample_rows: 100000
	}
	duckdb: {
		database_path:       "var/duckdb/quick-quack-quest.duckdb"
		temp_directory:      "var/duckdb/tmp"
		threads:             4
		memory_limit:        "2GB"
		access_mode:         "automatic"
		enable_progress_bar: false
		extensions: ["json", "parquet"]
		settings: {
			preserve_insertion_order: "false"
		}
	}
	query_execution: {
		streaming: {
			default_enabled: true
			chunk_size_rows: 10000
			allowed_output_formats: ["jsonl", "csv", "json", "table"]
		}
		progress: {
			enabled_by_default: false
			tty_only:           true
			min_query_ms:       1500
		}
		limits: {
			default_result_limit_rows: 10000
			max_rows:        1000000
			timeout_seconds: 600
		}
	}

	datasets: [
		{
			id:          "sales_daily"
			format:      "csv"
			layout:      "partitioned"
			compression: "gzip"
			prefix:      "doc/design-meta/examples/input/sales/date="
			suffix:      ".csv.gz"
			partition_keys: ["date"]
			description: "Daily store product sales transactions"
			homepage_url: "https://data.example.com/catalog/sales_daily"
			validation: {
				engine: "duckdb"
				// Per-dataset override for faster validation loops.
				random_sample_rows: 50000
			}
			metadata: {
				owner:       "analytics"
				primary_key: "sale_id"
			}
			fields: [
				{name: "sale_id", type: "VARCHAR", nullable: false, description: "Unique sale identifier"},
				{name: "sale_date", type: "DATE", nullable: false, description: "Date of sale event"},
				{name: "customer_id", type: "VARCHAR", nullable: false, description: "Customer foreign key"},
				{name: "product_id", type: "VARCHAR", nullable: false, description: "Product identifier"},
				{name: "quantity", type: "INTEGER", nullable: false, description: "Quantity sold"},
				{name: "amount", type: "DOUBLE", nullable: false, description: "Sale amount in local currency"},
			]
		},
		{
			id:          "customers_master"
			format:      "json"
			layout:      "single_file"
			compression: "none"
			path:        "doc/design-meta/examples/input/customers.json"
			description: "Customer profile master records"
			homepage_url: "https://data.example.com/catalog/customers_master"
			metadata: {
				owner:       "crm"
				primary_key: "customer_id"
			}
			fields: [
				{name: "customer_id", type: "VARCHAR", nullable: false, description: "Unique customer identifier"},
				{name: "email", type: "VARCHAR", nullable: false, description: "Primary email address"},
				{name: "country", type: "VARCHAR", nullable: true, description: "Country code"},
				{name: "signup_date", type: "DATE", nullable: true, description: "First signup date"},
			]
		},
		{
			id:          "events_stream"
			format:      "ndjson"
			layout:      "single_file"
			compression: "gzip"
			path:        "doc/design-meta/examples/input/events.ndjson.gz"
			description: "Application behavior events for funnel analysis"
			homepage_url: "https://data.example.com/catalog/events_stream"
			validation: {
				// Example override: force native validation pipeline for this dataset.
				engine: "native"
			}
			metadata: {
				owner:       "product"
				primary_key: "event_id"
			}
			fields: [
				{name: "event_id", type: "VARCHAR", nullable: false, description: "Unique event identifier"},
				{name: "event_ts", type: "TIMESTAMP", nullable: false, description: "Event timestamp in UTC"},
				{name: "event_name", type: "VARCHAR", nullable: false, description: "Event name"},
				{name: "customer_id", type: "VARCHAR", nullable: true, description: "Associated customer id"},
				{name: "metadata_json", type: "JSON", nullable: true, description: "Arbitrary JSON payload"},
			]
		},
	]

	queries: [
		{
			id:                "sales_by_country"
			required_datasets: ["sales_daily", "customers_master"]
			parameters: [
				{name: "start_date", type: "DATE", required: true, description: "Inclusive start date"},
				{name: "end_date", type: "DATE", required: true, description: "Inclusive end date"},
			]
			sql: "SELECT c.country, SUM(s.amount) AS total_amount, COUNT(*) AS tx_count FROM sales_daily s JOIN customers_master c ON s.customer_id = c.customer_id WHERE s.sale_date BETWEEN DATE($start_date) AND DATE($end_date) GROUP BY c.country ORDER BY total_amount DESC"
		},
		{
			id:                "events_per_day"
			required_datasets: ["events_stream"]
			parameters: [
				{name: "event_name", type: "VARCHAR", required: true, description: "Event type to include"},
			]
			sql: "SELECT DATE_TRUNC('day', event_ts) AS day, COUNT(*) AS total_events FROM events_stream WHERE event_name = $event_name GROUP BY 1 ORDER BY 1"
		},
		{
			id:                "customer_360"
			required_datasets: ["customers_master", "sales_daily", "events_stream"]
			parameters: [
				{name: "customer_id", type: "VARCHAR", required: true, description: "Customer id to inspect"},
			]
			sql: "SELECT c.customer_id, c.email, COALESCE(SUM(s.amount), 0) AS lifetime_value, COUNT(DISTINCT e.event_id) AS event_count FROM customers_master c LEFT JOIN sales_daily s ON s.customer_id = c.customer_id LEFT JOIN events_stream e ON e.customer_id = c.customer_id WHERE c.customer_id = $customer_id GROUP BY c.customer_id, c.email"
		},
	]
}
```

#### CUE Schema For CLI Config

```cue
package designmeta

#DuckDBType: "BOOLEAN" | "TINYINT" | "SMALLINT" | "INTEGER" | "BIGINT" | "UTINYINT" | "USMALLINT" | "UINTEGER" | "UBIGINT" | "FLOAT" | "DOUBLE" | "DECIMAL" | "VARCHAR" | "BLOB" | "DATE" | "TIME" | "TIMESTAMP" | "TIMESTAMPTZ" | "INTERVAL" | "UUID" | "JSON"

#DatasetFormat: "csv" | "json" | "ndjson" | "parquet"
#DatasetLayout: "single_file" | "partitioned"
#Compression: "auto" | "none" | "gzip" | "zstd"
#ValidationEngine: "duckdb" | "native"

#Field: {
	name:        string & !=""
	type:        #DuckDBType
	nullable:    bool
	description: string & !=""
}

#Dataset: {
	id:          string & !=""
	format:      #DatasetFormat
	layout:      #DatasetLayout
	compression?: #Compression
	path?:       string & !=""
	prefix?:     string & !=""
	suffix?:     string & !=""
	partition_keys?: [...string]
	description: string & !=""
	homepage_url?: string & != ""
	validation?: {
		engine?: #ValidationEngine
		// Optional: when set, validate this random sample size instead of full scan.
		random_sample_rows?: int & >0
	}
	metadata: {
		owner:       string & !=""
		primary_key: string & !=""
	}
	fields: [...#Field] & [_, ...]
	if layout == "single_file" {
		path: string & != ""
	}
	if layout == "partitioned" {
		prefix: string & != ""
		suffix: string & != ""
		partition_keys: [...string] & [_, ...]
	}
}

#QueryParameter: {
	name:        string & !=""
	type:        #DuckDBType
	required:    bool
	description: string & !=""
}

#Query: {
	id:                string & !=""
	required_datasets: [...string] & [_, ...]
	parameters:        [...#QueryParameter]
	sql:               string & !=""
}

#DuckDBConfig: {
	database_path?:      string & != ""
	temp_directory?:     string & != ""
	threads?:            int & >0
	memory_limit?:       string & != ""
	access_mode?:        "automatic" | "read_only" | "read_write"
	enable_progress_bar?: bool
	extensions?:         [...string]
	// Optional passthrough for additional DuckDB settings.
	settings?: [string]: string
}

#QueryExecutionConfig: {
	streaming?: {
		default_enabled?: bool
		chunk_size_rows?: int & >0
		allowed_output_formats?: [...("jsonl" | "csv" | "json" | "table")]
	}
	progress?: {
		enabled_by_default?: bool
		tty_only?:           bool
		min_query_ms?:       int & >=0
	}
	limits?: {
		default_result_limit_rows?: int & >0
		max_rows?:        int & >0
		timeout_seconds?: int & >0
	}
}

#CliSpec: {
	validation?: {
		engine?: #ValidationEngine
		// Optional global default for large datasets; can be overridden per dataset.
		random_sample_rows?: int & >0
	}
	duckdb?: #DuckDBConfig
	query_execution?: #QueryExecutionConfig
	datasets: [...#Dataset] & [_, ...]
	queries:  [...#Query] & [_, ...]
}
```

### 03 Query Run Semantics

#### Query Run: Limit And Max Rows Semantics

```markdown
# Query run semantics

For `query run`, `--limit` and `--max-rows` have different roles:

- `--limit`: logical SQL result limit. The CLI applies it to the query shape (for example by wrapping with a final `LIMIT n` when no explicit final limit is present).
- `--max-rows`: safety guardrail on emitted rows. If the command would emit more than this cap, execution stops with an error.

Precedence and defaults:

- Default `limit` comes from `cliSpec.query_execution.limits.default_result_limit_rows` when set.
- Default `max-rows` comes from `cliSpec.query_execution.limits.max_rows` when set.
- CLI flags override config defaults.
- If both values are set and `limit > max-rows`, fail fast with a clear configuration error.

Suggested error id:

- `QQQ_QUERY_LIMIT_EXCEEDS_MAX_ROWS`
```

## 04 Expected Outputs

### 01 Query Outputs

#### Output Example: Query Run Stream (JSONL)

```markdown
JSONL stream example (`query run --stream --format jsonl`):

```json
{"country":"GB","total_amount":189.69,"tx_count":2}
{"country":"US","total_amount":249.0,"tx_count":1}
```
```

#### Output Example: Query Run Summary

```json
{
  "query_id": "sales_by_country",
  "status": "ok",
  "rows_emitted": 2,
  "streaming": true,
  "duration_ms": 86,
  "limit": 1000,
  "max_rows": 1000000
}
```

### 02 Validation And Version Outputs

#### Output Example: Dataset Inspect

```json
{
  "dataset_id": "sales_daily",
  "validation_engine": "duckdb",
  "compression": "gzip",
  "status": "ok",
  "sample_rows": 1000,
  "observed_columns": [
    {"name": "sale_id", "duckdb_type": "VARCHAR", "nullable": false},
    {"name": "sale_date", "duckdb_type": "DATE", "nullable": false},
    {"name": "customer_id", "duckdb_type": "VARCHAR", "nullable": false},
    {"name": "product_id", "duckdb_type": "VARCHAR", "nullable": false},
    {"name": "quantity", "duckdb_type": "INTEGER", "nullable": false},
    {"name": "amount", "duckdb_type": "DOUBLE", "nullable": false}
  ],
  "duration_ms": 138
}
```

#### Output Example: Dataset Validate

```json
{
  "dataset_id": "sales_daily",
  "validation_engine": "duckdb",
  "compression": "gzip",
  "status": "ok",
  "files_scanned": 31,
  "rows_checked": 50000,
  "schema_mismatches": 0,
  "duration_ms": 412
}
```

#### Output Example: Version Command

```json
{
  "name": "quick-quack-quest",
  "version": "0.1.0",
  "commit": "abc1234",
  "built_at": "2026-05-19T09:00:00Z",
  "go_version": "go1.24.3"
}
```

## 05 Operational Details

### 01 Config Precedence

#### Configuration Precedence

```markdown
# Configuration precedence

The CLI resolves effective values using this precedence order:

1. Command-line flags (`--...`)
2. Environment variables (`QQQ_...`)
3. CUE config (`cliSpec`)
4. Built-in defaults

Examples:

- `query run --max-rows=5000` overrides `cliSpec.query_execution.limits.max_rows`.
- `QQQ_VALIDATION_ENGINE=native` overrides `cliSpec.validation.engine`.
- If neither flag nor env is set, values come from CUE config when present.
```

### 02 Error Catalog

#### Error Catalog

| category | error_id | message_template | recommended_action | severity | trigger |
| --- | --- | --- | --- | --- | --- |
| query-config | QQQ_QUERY_LIMIT_EXCEEDS_MAX_ROWS | limit {limit} cannot be greater than max_rows {max_rows} | Lower --limit or increase --max-rows | error | Query run starts with limit greater than max rows |
| dataset | QQQ_DATASET_NOT_FOUND | dataset {dataset_id} is not declared in cliSpec.datasets | Use dataset list and check dataset id spelling | error | Dataset id does not exist in config |
| dataset-schema | QQQ_SCHEMA_FIELD_MISSING | missing required field {field_name} in dataset {dataset_id} | Fix source data or update declared fields | error | Expected field is absent in source data |
| dataset-schema | QQQ_SCHEMA_TYPE_MISMATCH | field {field_name} expected {expected_type} but got {observed_type} | Align source type coercion or adjust declared type | error | Observed type differs from declared type |
| partition | QQQ_PARTITION_DISCOVERY_EMPTY | no files matched prefix {prefix} and suffix {suffix} | Verify path prefix suffix and partition filters | error | No files found for partitioned dataset |
| compression | QQQ_NATIVE_CODEC_UNAVAILABLE | native validation requires codec for compression {compression} | Install codec dependency or switch to duckdb engine | error | Native engine selected but required codec missing |
| query-runtime | QQQ_QUERY_TIMEOUT | query timed out after {timeout_seconds} seconds | Increase timeout or optimize query | error | Execution exceeded timeout |
| query-runtime | QQQ_MAX_ROWS_EXCEEDED | result exceeded max_rows {max_rows} | Reduce data scope with parameters or increase max rows | error | Result emission exceeded max rows |

### 03 Compatibility Matrix

#### Compatibility Matrix

| compression | format | notes | supported | validation_engine |
| --- | --- | --- | --- | --- |
| none | csv | Primary and fastest path for large files | yes | duckdb |
| gzip | csv | Uses DuckDB compressed reader path | yes | duckdb |
| zstd | csv | Requires DuckDB zstd support in runtime | yes | duckdb |
| none | json | For array style JSON files | yes | duckdb |
| gzip | json | Supports compressed JSON reads | yes | duckdb |
| none | ndjson | Line-delimited JSON supported | yes | duckdb |
| gzip | ndjson | Recommended for compressed event streams | yes | duckdb |
| none | parquet | Native columnar support | yes | duckdb |
| gzip | parquet | Parquet should rely on parquet-level compression not outer gzip | no | duckdb |
| none | csv | Uses native parser pipeline | yes | native |
| gzip | csv | Requires gzip codec library | yes | native |
| zstd | csv | Requires zstd codec library | yes | native |
| none | json | Uses native JSON parser | yes | native |
| gzip | json | Requires gzip codec library | yes | native |
| gzip | ndjson | Requires gzip codec library | yes | native |
| none | parquet | No native parquet parser planned in v1 | no | native |

### 04 Output Schema

#### Output Schema Versioning

```json
{
  "output_schema_version": "v1",
  "common_fields": {
    "status": "ok|error",
    "command": "string",
    "duration_ms": "integer"
  },
  "error_fields": {
    "error_id": "stable machine-readable id",
    "message": "human readable message"
  },
  "notes": "All JSON outputs should include output_schema_version for backward-compatible evolution."
}
```

