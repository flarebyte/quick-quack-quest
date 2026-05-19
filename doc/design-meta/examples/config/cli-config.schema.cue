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
#DatasetLayout: "single_file" | "partitioned"

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
	path?:       string & !=""
	prefix?:     string & !=""
	suffix?:     string & !=""
	partition_keys?: [...string]
	description: string & !=""
	validation?: {
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
		max_rows?:        int & >0
		timeout_seconds?: int & >0
	}
}

#CliSpec: {
	validation?: {
		// Optional global default for large datasets; can be overridden per dataset.
		random_sample_rows?: int & >0
	}
	duckdb?: #DuckDBConfig
	query_execution?: #QueryExecutionConfig
	datasets: [...#Dataset] & [_, ...]
	queries:  [...#Query] & [_, ...]
}
