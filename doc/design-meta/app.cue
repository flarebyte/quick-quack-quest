package flyb

source: "design-meta"
name:   "quick-quack-quest-cli"

modules: ["design"]

notes: [
	{
		name:      "cli.overview"
		title:     "CLI Overview"
		markdown:  "Go + Cobra CLI to validate datasets and execute parameterized DuckDB SQL queries against declared dataset sources."
		labels:    ["overview", "cli", "duckdb"]
	},
	{
		name:      "cli.features"
		title:     "Feature List"
		filepath:  "examples/features.csv"
		arguments: ["format-csv=table"]
		labels:    ["feature", "csv"]
	},
	{
		name:      "cli.commands"
		title:     "CLI Commands"
		filepath:  "examples/cli-commands.csv"
		arguments: ["format-csv=table"]
		labels:    ["command", "csv", "cobra"]
	},
	{
		name:      "datasets.catalog"
		title:     "Dataset Catalog"
		filepath:  "examples/datasets.csv"
		arguments: ["format-csv=table"]
		labels:    ["dataset", "csv"]
	},
	{
		name:      "datasets.fields"
		title:     "Dataset Fields"
		filepath:  "examples/dataset-fields.csv"
		arguments: ["format-csv=table"]
		labels:    ["dataset", "schema", "csv"]
	},
	{
		name:      "queries.catalog"
		title:     "Parameterized Query Catalog"
		filepath:  "examples/queries.csv"
		arguments: ["format-csv=table"]
		labels:    ["query", "duckdb", "csv"]
	},
	{
		name:      "queries.params"
		title:     "Query Parameters"
		filepath:  "examples/query-parameters.csv"
		arguments: ["format-csv=table"]
		labels:    ["query", "parameter", "csv"]
	},
	{
		name:     "cli.config.cue"
		title:    "CUE Config Sections: Datasets And Queries"
		filepath: "examples/cli-config.cue"
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
		name:      "datasets.input.customers"
		title:     "Input Example: Customers JSON"
		filepath:  "examples/input/customers.json"
		labels:    ["input", "json", "example"]
	},
	{
		name:      "datasets.input.events"
		title:     "Input Example: Events NDJSON"
		filepath:  "examples/input/events.ndjson"
		labels:    ["input", "json", "ndjson", "example"]
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
]

reports: [{
	title: "DuckDB CLI Specification"
	filepath: "../design/duckdb-cli-spec.md"
	sections: [
		{
			title: "Overview"
			notes: ["cli.overview", "cli.features"]
		},
		{
			title: "CLI Commands"
			notes: ["cli.commands"]
		},
		{
			title: "Datasets"
			notes: ["datasets.catalog", "datasets.fields", "datasets.input.sales", "datasets.input.customers", "datasets.input.events"]
		},
		{
			title: "Queries"
			notes: ["queries.catalog", "queries.params", "cli.config.cue"]
		},
	]
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
