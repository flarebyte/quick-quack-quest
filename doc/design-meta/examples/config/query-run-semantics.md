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
