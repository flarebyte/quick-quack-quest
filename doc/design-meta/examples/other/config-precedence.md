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
