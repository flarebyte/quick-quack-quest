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

#CliSpec: {
	validation?: {
		// Optional global default for large datasets; can be overridden per dataset.
		random_sample_rows?: int & >0
	}
	datasets: [...#Dataset] & [_, ...]
	queries:  [...#Query] & [_, ...]
}
