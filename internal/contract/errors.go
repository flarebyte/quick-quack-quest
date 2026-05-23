// purpose: Provide shared query-domain error identifiers used across CLI and engine flows.
// responsibilities: Declare stable constants for not-found, parameter, limit, and timeout query failures.
// architecture notes: Centralized IDs prevent drift between runtime behavior, tests, and machine-readable outputs.
package contract

const (
	ErrIDQueryNotFound            = "QQQ_QUERY_NOT_FOUND"
	ErrIDQueryParamInvalid        = "QQQ_QUERY_PARAM_INVALID"
	ErrIDQueryParamRequired       = "QQQ_QUERY_PARAM_REQUIRED_MISSING"
	ErrIDQueryLimitExceedsMaxRows = "QQQ_QUERY_LIMIT_EXCEEDS_MAX_ROWS"
	ErrIDQueryMaxRowsExceeded     = "QQQ_MAX_ROWS_EXCEEDED"
	ErrIDQueryTimeout             = "QQQ_QUERY_TIMEOUT"
)
