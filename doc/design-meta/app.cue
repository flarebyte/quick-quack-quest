package flyb

source: "design-meta"
name:   "quick-quack-quest-cli"

modules: ["design"]

notes: [
	{
		name:     "cli.overview"
		title:    "CLI Overview"
		markdown: "Go + Cobra CLI to validate datasets and execute parameterized DuckDB SQL queries against declared dataset sources."
		labels:   ["overview", "cli", "duckdb"]
	},
	{
		name:      "cli.features"
		title:     "Feature List"
		filepath:  "examples/catalog/features.csv"
		arguments: ["format-csv=table"]
		labels:    ["feature", "csv"]
	},
	{
		name:      "cli.commands"
		title:     "CLI Commands"
		filepath:  "examples/catalog/cli-commands.csv"
		arguments: ["format-csv=table"]
		labels:    ["command", "csv", "cobra"]
	},
	{
		name:      "datasets.catalog"
		title:     "Dataset Catalog"
		filepath:  "examples/catalog/datasets.csv"
		arguments: ["format-csv=table"]
		labels:    ["dataset", "csv"]
	},
	{
		name:      "datasets.fields"
		title:     "Dataset Fields"
		filepath:  "examples/catalog/dataset-fields.csv"
		arguments: ["format-csv=table"]
		labels:    ["dataset", "schema", "csv"]
	},
	{
		name:      "queries.catalog"
		title:     "Parameterized Query Catalog"
		filepath:  "examples/catalog/queries.csv"
		arguments: ["format-csv=table"]
		labels:    ["query", "duckdb", "csv"]
	},
	{
		name:      "queries.params"
		title:     "Query Parameters"
		filepath:  "examples/catalog/query-parameters.csv"
		arguments: ["format-csv=table"]
		labels:    ["query", "parameter", "csv"]
	},
	{
		name:     "cli.config.schema"
		title:    "CUE Schema For CLI Config"
		filepath: "examples/config/cli-config.schema.cue"
		labels:   ["cue", "schema"]
	},
	{
		name:     "cli.config.example"
		title:    "CUE Example CLI Config"
		filepath: "examples/config/cli-config.cue"
		labels:   ["cue", "config"]
	},
	{
		name:      "datasets.input.sales"
		title:     "Input Example: Sales CSV"
		filepath:  "examples/input/sales.csv"
		arguments: ["format-csv=table"]
		labels:    ["input", "csv", "example"]
	},
	{
		name:     "datasets.input.customers"
		title:    "Input Example: Customers JSON"
		filepath: "examples/input/customers.json"
		labels:   ["input", "json", "example"]
	},
	{
		name:     "datasets.input.events"
		title:    "Input Example: Events JSON"
		filepath: "examples/input/events.json"
		labels:   ["input", "json", "example"]
	},
]

relationships: [
	{from: "cli.overview", to: "cli.features", label: "details"},
	{from: "cli.overview", to: "cli.commands", label: "defines"},
	{from: "cli.commands", to: "datasets.catalog", label: "operates-on"},
	{from: "datasets.catalog", to: "datasets.fields", label: "contains"},
	{from: "queries.catalog", to: "queries.params", label: "parameterized-by"},
	{from: "queries.catalog", to: "datasets.catalog", label: "depends-on"},
	{from: "datasets.catalog", to: "datasets.input.sales", label: "example"},
	{from: "datasets.catalog", to: "datasets.input.customers", label: "example"},
	{from: "datasets.catalog", to: "datasets.input.events", label: "example"},
	{from: "cli.config.example", to: "cli.config.schema", label: "validated-by"},
]

reports: [{
	title:       "DuckDB CLI Specification"
	filepath:    "../design/duckdb-cli-spec.md"
	description: "Spec for a Go+Cobra CLI to validate datasets and execute DuckDB queries from CUE declarations."
	sections: [{
		title: "01 Overview"
		sections: [{
			title: "01 Purpose"
			notes: ["cli.overview", "cli.features"]
		}, {
			title: "02 Cobra Commands"
			notes: ["cli.commands"]
		}]
	}, {
		title: "02 Dataset Model"
		sections: [{
			title: "01 Dataset Registry"
			notes: ["datasets.catalog", "datasets.fields"]
		}, {
			title: "02 Input Samples"
			notes: ["datasets.input.sales", "datasets.input.customers", "datasets.input.events"]
		}]
	}, {
		title: "03 Query Model"
		sections: [{
			title: "01 Query Catalog"
			notes: ["queries.catalog", "queries.params"]
		}, {
			title: "02 CUE Schema And Example"
			notes: ["cli.config.schema", "cli.config.example"]
		}]
	}]
}]

argumentRegistry: {
	version: "1"
	arguments: [{
		name:          "format-csv"
		valueType:     "enum"
		scopes:        ["note"]
		allowedValues: ["table", "raw"]
		defaultValue:  "table"
	}]
}
