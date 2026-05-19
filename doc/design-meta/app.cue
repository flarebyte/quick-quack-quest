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
		name:      "cli.libraries"
		title:     "Required Libraries"
		filepath:  "examples/catalog/libraries.csv"
		arguments: ["format-csv=table"]
		labels:    ["library", "csv", "dependency"]
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
		name:     "queries.run.semantics"
		title:    "Query Run: Limit And Max Rows Semantics"
		filepath: "examples/config/query-run-semantics.md"
		labels:   ["query", "semantics", "limits"]
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
	{
		name:     "outputs.query.run.jsonl"
		title:    "Output Example: Query Run Stream (JSONL)"
		filepath: "examples/output/query-run-sales-by-country.md"
		labels:   ["output", "query", "jsonl", "example"]
	},
	{
		name:     "outputs.query.run.summary"
		title:    "Output Example: Query Run Summary"
		filepath: "examples/output/query-run-summary.json"
		labels:   ["output", "query", "json", "example"]
	},
	{
		name:     "outputs.dataset.validate"
		title:    "Output Example: Dataset Validate"
		filepath: "examples/output/dataset-validate-sales_daily.json"
		labels:   ["output", "dataset", "json", "example"]
	},
	{
		name:     "outputs.version"
		title:    "Output Example: Version Command"
		filepath: "examples/output/version.json"
		labels:   ["output", "version", "json", "example"]
	},
]

relationships: [
	{from: "cli.overview", to: "cli.features", label: "details"},
	{from: "cli.overview", to: "cli.commands", label: "defines"},
	{from: "cli.overview", to: "cli.libraries", label: "depends-on"},
	{from: "cli.commands", to: "datasets.catalog", label: "operates-on"},
	{from: "datasets.catalog", to: "datasets.fields", label: "contains"},
	{from: "queries.catalog", to: "queries.params", label: "parameterized-by"},
	{from: "queries.catalog", to: "queries.run.semantics", label: "governed-by"},
	{from: "queries.catalog", to: "datasets.catalog", label: "depends-on"},
	{from: "datasets.catalog", to: "datasets.input.sales", label: "example"},
	{from: "datasets.catalog", to: "datasets.input.customers", label: "example"},
	{from: "datasets.catalog", to: "datasets.input.events", label: "example"},
	{from: "queries.run.semantics", to: "outputs.query.run.jsonl", label: "example"},
	{from: "queries.run.semantics", to: "outputs.query.run.summary", label: "example"},
	{from: "cli.commands", to: "outputs.dataset.validate", label: "example"},
	{from: "cli.commands", to: "outputs.version", label: "example"},
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
		}, {
			title: "03 Libraries"
			notes: ["cli.libraries"]
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
		}, {
			title: "03 Query Run Semantics"
			notes: ["queries.run.semantics"]
		}]
	}, {
		title: "04 Expected Outputs"
		sections: [{
			title: "01 Query Outputs"
			notes: ["outputs.query.run.jsonl", "outputs.query.run.summary"]
		}, {
			title: "02 Validation And Version Outputs"
			notes: ["outputs.dataset.validate", "outputs.version"]
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
