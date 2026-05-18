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

#### CLI Overview

Go + Cobra CLI to validate datasets and execute parameterized DuckDB SQL queries against declared dataset sources.

### 02 Cobra Commands

#### CLI Commands

| arguments | command | command_id | flags | group | output | summary |
| --- | --- | --- | --- | --- | --- | --- |
|  | dataset list | C001 | --format | dataset | --format table\|json | List declared datasets and metadata |
| dataset-id | dataset validate <dataset-id> | C002 | --config --strict | dataset | --format table\|json | Validate one dataset file contract |
|  | dataset validate-all | C003 | --config --fail-fast | dataset | --format table\|json | Validate all declared datasets |
| dataset-id | dataset inspect <dataset-id> | C004 | --sample-size | dataset | --format table\|json | Print discovered columns and inferred types |
|  | query list | C005 | --format | query | --format table\|json | List registered parameterized queries |
| query-id | query run <query-id> | C006 | --param key=value --limit --output | query | --format table\|json\|csv | Execute one query with parameters |
| query-id | query explain <query-id> | C007 | --param key=value | query | --format text\|json | Show SQL and resolved datasets without execution |
|  | config validate | C008 | --config | config | --format table\|json | Validate CUE config structure and references |

## 02 Dataset Model

### 01 Dataset Registry

#### Dataset Catalog

| dataset_id | description | format | name | owner | path | primary_key | tags |
| --- | --- | --- | --- | --- | --- | --- | --- |
| sales_daily | Daily store product sales transactions | csv | Daily sales facts | analytics | doc/design-meta/examples/input/sales.csv | sale_id | sales;facts |
| customers_master | Customer profile master records | json | Customer master | crm | doc/design-meta/examples/input/customers.json | customer_id | customers;dimension |
| events_stream | Application behavior events for funnel analysis | ndjson | Application events | product | doc/design-meta/examples/input/events.ndjson | event_id | events;telemetry |

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
	datasets: [
		{
			id:          "sales_daily"
			format:      "csv"
			path:        "doc/design-meta/examples/input/sales.csv"
			description: "Daily store product sales transactions"
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
			path:        "doc/design-meta/examples/input/customers.json"
			description: "Customer profile master records"
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
			path:        "doc/design-meta/examples/input/events.ndjson"
			description: "Application behavior events for funnel analysis"
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

#DuckDBType:
	| "BOOLEAN"
	| "TINYINT"
	| "SMALLINT"
	| "INTEGER"
	| "BIGINT"
	| "UTINYINT"
	| "USMALLINT"
	| "UINTEGER"
	| "UBIGINT"
	| "FLOAT"
	| "DOUBLE"
	| "DECIMAL"
	| "VARCHAR"
	| "BLOB"
	| "DATE"
	| "TIME"
	| "TIMESTAMP"
	| "TIMESTAMPTZ"
	| "INTERVAL"
	| "UUID"
	| "JSON"

#DatasetFormat: "csv" | "json" | "ndjson" | "parquet"

#Field: {
	name:        string & !=""
	type:        #DuckDBType
	nullable:    bool
	description: string & !=""
}

#Dataset: {
	id:          string & !=""
	format:      #DatasetFormat
	path:        string & !=""
	description: string & !=""
	metadata: {
		owner:       string & !=""
		primary_key: string & !=""
	}
	fields: [...#Field] & [_, ...]
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

#CliSpec: {
	datasets: [...#Dataset] & [_, ...]
	queries:  [...#Query] & [_, ...]
}
```

