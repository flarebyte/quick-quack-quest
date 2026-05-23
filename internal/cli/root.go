// purpose: Implement the CLI surface for config, dataset, and query workflows.
// responsibilities: Define commands and flags, call config/validate/query logic, and render output formats.
// architecture notes: Command wiring is centralized here to keep UX behavior and output-schema contracts consistent.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/flarebyte/quick-quack-quest/internal/config"
	"github.com/flarebyte/quick-quack-quest/internal/contract"
	"github.com/flarebyte/quick-quack-quest/internal/validate"
	"github.com/spf13/cobra"
)

var sqlIdentRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var (
	Version = "dev"
	Commit  = "none"
	BuiltAt = "unknown"
)

type outputFormat string

const (
	formatText outputFormat = "text"
	formatJSON outputFormat = "json"
)

type versionPayload struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	BuiltAt string `json:"built_at"`
}

type validatePayload struct {
	Status string `json:"status"`
	Path   string `json:"path"`
}

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "quack",
		Aliases: []string{"quick-quack-quest"},
		Short:   "Validate datasets and run parameterized DuckDB queries",
		Long: "quack validates declared datasets and executes parameterized DuckDB queries " +
			"from a CUE config file. Use --config on commands to point to your own cliSpec.",
		Example: strings.Join([]string{
			"quack config validate --config ./cli-config.cue",
			"quack dataset list --config ./cli-config.cue",
			"quack dataset validate sales_daily --config ./cli-config.cue --format json",
			"quack query run sales_by_country --config ./cli-config.cue --param start_date=2026-01-01 --param end_date=2026-01-31 --format table",
		}, "\n"),
	}

	rootCmd.AddCommand(newVersionCommand())
	rootCmd.AddCommand(newConfigCommand())
	rootCmd.AddCommand(newDatasetCommand())
	rootCmd.AddCommand(newQueryCommand())

	return rootCmd
}

func newVersionCommand() *cobra.Command {
	format := string(formatText)
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print CLI version and build metadata",
		Long:  "Show CLI version/build information for support and release traceability.",
		Example: strings.Join([]string{
			"quack version",
			"quack version --format json",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := versionPayload{
				Name:    "quick-quack-quest",
				Version: Version,
				Commit:  Commit,
				BuiltAt: BuiltAt,
			}
			if outputFormat(format) == formatJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"output_schema_version": "v1",
					"name":                  payload.Name,
					"version":               payload.Version,
					"commit":                payload.Commit,
					"built_at":              payload.BuiltAt,
					"go_version":            runtime.Version(),
				})
			}
			return writeOutput(cmd.OutOrStdout(), format, payload, fmt.Sprintf("%s %s (%s)", payload.Name, payload.Version, payload.Commit))
		},
	}
	cmd.Flags().StringVar(&format, "format", string(formatText), "Output format: text|json")
	return cmd
}

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Config operations",
		Long:  "Validate and inspect CUE configuration used by dataset and query commands.",
	}
	cmd.AddCommand(newConfigValidateCommand())
	return cmd
}

func newDatasetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dataset",
		Short: "Dataset operations",
		Long:  "List, validate, and inspect datasets declared in cliSpec.datasets.",
	}
	cmd.AddCommand(newDatasetListCommand())
	cmd.AddCommand(newDatasetValidateCommand())
	cmd.AddCommand(newDatasetValidateAllCommand())
	cmd.AddCommand(newDatasetInspectCommand())
	return cmd
}

func newQueryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query catalog operations",
		Long:  "List, explain, and run parameterized queries declared in cliSpec.queries.",
	}
	cmd.AddCommand(newQueryListCommand())
	cmd.AddCommand(newQueryExplainCommand())
	cmd.AddCommand(newQueryRunCommand())
	return cmd
}

func newConfigValidateCommand() *cobra.Command {
	format := string(formatText)
	configPath := "doc/design-meta/examples/config/cli-config.cue"
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate CUE config structure and references",
		Long:  "Load the CUE config, validate schema shape, and resolve referenced paths.",
		Example: strings.Join([]string{
			"quack config validate --config ./cli-config.cue",
			"quack config validate --config ./cli-config.cue --format json",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := config.LoadAndValidate(configPath)
			if err != nil {
				if cErr, ok := err.(*config.ConfigError); ok {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s\n", cErr.Error())
					return fmt.Errorf("%s", cErr.ID)
				}
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%v\n", err)
				return err
			}
			payload := validatePayload{Status: "ok", Path: configPath}
			return writeOutput(cmd.OutOrStdout(), format, payload, fmt.Sprintf("config is valid: %s", configPath))
		},
	}
	cmd.Flags().StringVar(&configPath, "config", configPath, "Path to CUE config file")
	cmd.Flags().StringVar(&format, "format", string(formatText), "Output format: text|json")
	return cmd
}

type datasetListRow struct {
	DatasetID        string `json:"dataset_id"`
	Format           string `json:"format"`
	Layout           string `json:"layout"`
	Compression      string `json:"compression"`
	Path             string `json:"path"`
	Prefix           string `json:"prefix"`
	Suffix           string `json:"suffix"`
	PartitionKeys    string `json:"partition_keys"`
	HomepageURL      string `json:"homepage_url"`
	Owner            string `json:"owner"`
	PrimaryKey       string `json:"primary_key"`
	ValidationEngine string `json:"validation_engine"`
}

type queryListRow struct {
	QueryID          string `json:"query_id"`
	RequiredDatasets string `json:"required_datasets"`
	ParameterCount   int    `json:"parameter_count"`
}

func newDatasetListCommand() *cobra.Command {
	format := string(formatText)
	configPath := "doc/design-meta/examples/config/cli-config.cue"
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List declared datasets and metadata",
		Long:  "Print datasets from cliSpec.datasets with format/layout/compression and ownership metadata.",
		Example: strings.Join([]string{
			"quack dataset list --config ./cli-config.cue",
			"quack dataset list --config ./cli-config.cue --format json",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := config.LoadAndValidate(configPath)
			if err != nil {
				if cErr, ok := err.(*config.ConfigError); ok {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s\n", cErr.Error())
					return fmt.Errorf("%s", cErr.ID)
				}
				return err
			}
			rows := datasetRows(spec)
			switch outputFormat(format) {
			case formatJSON:
				return writeJSON(cmd.OutOrStdout(), rows)
			case formatText:
				return writeDatasetTable(cmd.OutOrStdout(), rows)
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&configPath, "config", configPath, "Path to CUE config file")
	cmd.Flags().StringVar(&format, "format", string(formatText), "Output format: text|json")
	return cmd
}

func newQueryListCommand() *cobra.Command {
	format := string(formatText)
	configPath := "doc/design-meta/examples/config/cli-config.cue"
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List declared queries",
		Long:  "Print query ids, required datasets, and parameter counts from cliSpec.queries.",
		Example: strings.Join([]string{
			"quack query list --config ./cli-config.cue",
			"quack query list --config ./cli-config.cue --format json",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := config.LoadAndValidate(configPath)
			if err != nil {
				return renderConfigError(cmd, err)
			}
			rows := queryRows(spec)
			switch outputFormat(format) {
			case formatJSON:
				return writeJSON(cmd.OutOrStdout(), rows)
			case formatText:
				return writeQueryTable(cmd.OutOrStdout(), rows)
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&configPath, "config", configPath, "Path to CUE config file")
	cmd.Flags().StringVar(&format, "format", string(formatText), "Output format: text|json")
	return cmd
}

func newQueryExplainCommand() *cobra.Command {
	format := string(formatText)
	configPath := "doc/design-meta/examples/config/cli-config.cue"
	params := []string{}
	cmd := &cobra.Command{
		Use:   "explain <query-id>",
		Short: "Explain one query definition",
		Long:  "Show resolved SQL template metadata without executing the query.",
		Example: strings.Join([]string{
			"quack query explain sales_by_country --config ./cli-config.cue",
			"quack query explain sales_by_country --config ./cli-config.cue --param start_date=2026-01-01 --param end_date=2026-01-31 --format json",
		}, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := config.LoadAndValidate(configPath)
			if err != nil {
				return renderConfigError(cmd, err)
			}
			q, ok := findQuery(spec, args[0])
			if !ok {
				return fmt.Errorf("%s: query %s is not declared", contract.ErrIDQueryNotFound, args[0])
			}
			paramMap, err := parseParams(params)
			if err != nil {
				return err
			}
			if len(paramMap) > 0 {
				for _, p := range q.Parameters {
					if p.Required && strings.TrimSpace(paramMap[p.Name]) == "" {
						return fmt.Errorf("%s: missing required parameter %s", contract.ErrIDQueryParamRequired, p.Name)
					}
				}
			}
			out := map[string]any{
				"output_schema_version": "v1",
				"query_id":              q.ID,
				"required_datasets":     q.RequiredDatasets,
				"parameters":            q.Parameters,
				"sql":                   q.SQL,
			}
			switch outputFormat(format) {
			case formatJSON:
				return writeJSON(cmd.OutOrStdout(), out)
			case formatText:
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "query=%s datasets=%s params=%d\n", q.ID, strings.Join(q.RequiredDatasets, ","), len(q.Parameters))
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), q.SQL)
				return nil
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&configPath, "config", configPath, "Path to CUE config file")
	cmd.Flags().StringVar(&format, "format", string(formatText), "Output format: text|json")
	cmd.Flags().StringArrayVar(&params, "param", nil, "Optional parameter values in key=value format")
	return cmd
}

func newQueryRunCommand() *cobra.Command {
	configPath := "doc/design-meta/examples/config/cli-config.cue"
	format := "json"
	params := []string{}
	stream := false
	progress := false
	limit := 0
	maxRows := 0
	timeout := 0
	chunkSize := 0
	outputPath := ""
	cmd := &cobra.Command{
		Use:   "run <query-id>",
		Short: "Run one parameterized query",
		Long:  "Execute a query against declared datasets with runtime parameter binding and safety limits.",
		Example: strings.Join([]string{
			"quack query run sales_by_country --config ./cli-config.cue --param start_date=2026-01-01 --param end_date=2026-01-31 --format table",
			"quack query run sales_by_country --config ./cli-config.cue --param start_date=2026-01-01 --param end_date=2026-01-31 --format jsonl --stream",
			"quack query run sales_by_country --config ./cli-config.cue --format csv --output ./out.csv",
		}, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := config.LoadAndValidate(configPath)
			if err != nil {
				return renderConfigError(cmd, err)
			}
			q, ok := findQuery(spec, args[0])
			if !ok {
				return fmt.Errorf("%s: query %s is not declared", contract.ErrIDQueryNotFound, args[0])
			}
			paramMap, err := parseParams(params)
			if err != nil {
				return err
			}
			for _, p := range q.Parameters {
				if p.Required && strings.TrimSpace(paramMap[p.Name]) == "" {
					return fmt.Errorf("%s: missing required parameter %s", contract.ErrIDQueryParamRequired, p.Name)
				}
			}
			effLimit := chooseInt(limit, envInt("QQQ_QUERY_LIMIT"), spec.QueryExecution.Limits.DefaultResultLimitRows)
			effMaxRows := chooseInt(maxRows, envInt("QQQ_MAX_ROWS"), spec.QueryExecution.Limits.MaxRows)
			if effLimit > 0 && effMaxRows > 0 && effLimit > effMaxRows {
				return fmt.Errorf("%s: limit=%d max_rows=%d", contract.ErrIDQueryLimitExceedsMaxRows, effLimit, effMaxRows)
			}
			effTimeout := chooseInt(timeout, envInt("QQQ_QUERY_TIMEOUT"), spec.QueryExecution.Limits.TimeoutSeconds)
			effStream := chooseBool(stream, envBool("QQQ_STREAM"), spec.QueryExecution.Streaming.DefaultEnabled)
			if chunkSize <= 0 {
				chunkSize = spec.QueryExecution.Streaming.ChunkSizeRows
			}
			if progress && isTTY(os.Stderr.Fd()) {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "running query_id=%s\n", q.ID)
			}
			start := time.Now()
			outWriter := cmd.OutOrStdout()
			if outputPath != "" {
				f, createErr := os.Create(outputPath)
				if createErr != nil {
					return wrapQueryRunError(q.ID, "create_output", format, effLimit, effMaxRows, effTimeout, outputPath, createErr)
				}
				defer f.Close()
				outWriter = f
			}
			execRes, err := runQuery(spec, q, paramMap, runOptions{
				format:    format,
				stream:    effStream,
				limit:     effLimit,
				maxRows:   effMaxRows,
				timeout:   effTimeout,
				chunkSize: chunkSize,
				out:       outWriter,
			})
			if err != nil {
				return wrapQueryRunError(q.ID, "execute", format, effLimit, effMaxRows, effTimeout, outputPath, err)
			}
			summary := map[string]any{
				"output_schema_version": "v1",
				"query_id":              q.ID,
				"status":                "ok",
				"rows_emitted":          execRes.rowsEmitted,
				"streaming":             effStream,
				"duration_ms":           time.Since(start).Milliseconds(),
				"limit":                 effLimit,
				"max_rows":              effMaxRows,
			}
			if format == "json" || format == "table" || format == "text" {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr())
				return writeJSON(cmd.ErrOrStderr(), summary)
			}
			return writeJSON(cmd.ErrOrStderr(), summary)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", configPath, "Path to CUE config file")
	cmd.Flags().StringVar(&format, "format", "json", "Output format: table|json|jsonl|csv")
	cmd.Flags().StringArrayVar(&params, "param", nil, "Parameter in key=value format")
	cmd.Flags().BoolVar(&stream, "stream", false, "Stream output rows")
	cmd.Flags().BoolVar(&progress, "progress", false, "Show query progress on stderr")
	cmd.Flags().IntVar(&limit, "limit", 0, "Result row limit")
	cmd.Flags().IntVar(&maxRows, "max-rows", 0, "Maximum rows allowed to emit")
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Timeout in seconds")
	cmd.Flags().IntVar(&chunkSize, "chunk-size", 0, "Chunk size rows")
	cmd.Flags().StringVar(&outputPath, "output", "", "Write query output rows to this file path")
	return cmd
}

type runOptions struct {
	format    string
	stream    bool
	limit     int
	maxRows   int
	timeout   int
	chunkSize int
	out       io.Writer
}

type runResult struct {
	rowsEmitted int
}

func wrapQueryRunError(queryID, stage, format string, limit, maxRows, timeout int, outputPath string, cause error) error {
	if cause == nil {
		return nil
	}
	if outputPath == "" {
		outputPath = "stdout"
	}
	return fmt.Errorf("query run failed: query_id=%s stage=%s format=%s limit=%d max_rows=%d timeout_s=%d output=%s: %w",
		queryID, stage, format, limit, maxRows, timeout, outputPath, cause)
}

func runQuery(spec *config.Spec, q config.Query, params map[string]string, opts runOptions) (runResult, error) {
	_ = spec
	db, err := sql.Open("duckdb", "")
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return runResult{}, fmt.Errorf("%s: timeout=%d", contract.ErrIDQueryTimeout, opts.timeout)
		}
		return runResult{}, err
	}
	defer db.Close()

	datasets := make([]config.Dataset, 0, len(q.RequiredDatasets))
	for _, dsID := range q.RequiredDatasets {
		ds, ok := findDatasetByID(spec, dsID)
		if !ok {
			return runResult{}, fmt.Errorf("QQQ_DATASET_NOT_FOUND: dataset %s is not declared", dsID)
		}
		if !sqlIdentRe.MatchString(ds.ID) {
			return runResult{}, fmt.Errorf("QQQ_DATASET_ID_INVALID: invalid dataset identifier %q", ds.ID)
		}
		datasets = append(datasets, ds)
	}

	sqlWithSources, sourceArgs, err := injectDatasetSources(q.SQL, datasets)
	if err != nil {
		return runResult{}, err
	}
	sqlText, paramArgs, err := bindQuery(sqlWithSources, q, params, opts.limit)
	if err != nil {
		return runResult{}, err
	}
	args := append(sourceArgs, paramArgs...)
	ctx := context.Background()
	if opts.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.timeout)*time.Second)
		defer cancel()
	}
	rows, err := db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return runResult{}, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return runResult{}, err
	}
	switch opts.format {
	case "jsonl":
		return emitJSONL(rows, cols, opts.maxRows, opts.out)
	case "csv":
		return emitCSV(rows, cols, opts.maxRows, opts.out)
	case "json":
		return emitJSON(rows, cols, opts.maxRows, opts.out)
	case "table", "text":
		return emitTable(rows, cols, opts.maxRows, opts.out)
	default:
		return runResult{}, fmt.Errorf("unsupported format %q", opts.format)
	}
}

func sourceExprForRun(format string) string {
	switch format {
	case "csv":
		return "read_csv_auto(?)"
	case "json":
		return "read_json_auto(?)"
	case "ndjson":
		return "read_json_auto(?, format='newline_delimited')"
	case "parquet":
		return "read_parquet(?)"
	default:
		return "read_csv_auto(?)"
	}
}

func bindQuery(baseSQL string, q config.Query, params map[string]string, limit int) (string, []any, error) {
	sqlText := baseSQL
	args := make([]any, 0, len(q.Parameters))
	for _, p := range q.Parameters {
		val, ok := params[p.Name]
		if p.Required && (!ok || strings.TrimSpace(val) == "") {
			return "", nil, fmt.Errorf("%s: missing required parameter %s", contract.ErrIDQueryParamRequired, p.Name)
		}
		if !ok {
			continue
		}
		bound, err := coerceParam(p.Type, val)
		if err != nil {
			return "", nil, fmt.Errorf("%s: invalid value for parameter %s", contract.ErrIDQueryParamInvalid, p.Name)
		}
		token := "$" + p.Name
		occurrences := strings.Count(sqlText, token)
		if occurrences == 0 {
			continue
		}
		sqlText = strings.ReplaceAll(sqlText, token, "?")
		for i := 0; i < occurrences; i++ {
			args = append(args, bound)
		}
	}
	if limit > 0 {
		sqlText = fmt.Sprintf("SELECT * FROM (%s) AS qqq_q LIMIT %d", sqlText, limit)
	}
	return sqlText, args, nil
}

func injectDatasetSources(sqlText string, datasets []config.Dataset) (string, []any, error) {
	args := make([]any, 0, len(datasets))
	for _, ds := range datasets {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(ds.ID) + `\b`)
		count := len(re.FindAllStringIndex(sqlText, -1))
		if count == 0 {
			continue
		}
		sqlText = re.ReplaceAllString(sqlText, "(SELECT * FROM "+sourceExprForRun(ds.Format)+")")
		path := ds.Path
		if ds.Layout == "partitioned" {
			path = ds.Prefix + "*" + ds.Suffix
		}
		for i := 0; i < count; i++ {
			args = append(args, filepath.ToSlash(path))
		}
	}
	return sqlText, args, nil
}

func coerceParam(typ, value string) (any, error) {
	t := strings.ToUpper(strings.TrimSpace(typ))
	switch t {
	case "INTEGER", "BIGINT", "SMALLINT", "TINYINT", "UINTEGER", "UBIGINT", "USMALLINT", "UTINYINT":
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, err
		}
		return v, nil
	case "DOUBLE", "FLOAT", "DECIMAL":
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, err
		}
		return v, nil
	default:
		return value, nil
	}
}

func findDatasetByID(spec *config.Spec, datasetID string) (config.Dataset, bool) {
	for _, d := range spec.Datasets {
		if d.ID == datasetID {
			return d, true
		}
	}
	return config.Dataset{}, false
}

func emitJSONL(rows *sql.Rows, cols []string, maxRows int, out io.Writer) (runResult, error) {
	emitted := 0
	for rows.Next() {
		m, err := scanRowMap(rows, cols)
		if err != nil {
			return runResult{}, err
		}
		b, err := json.Marshal(m)
		if err != nil {
			return runResult{}, err
		}
		_, _ = fmt.Fprintln(out, string(b))
		emitted++
		if maxRows > 0 && emitted > maxRows {
			return runResult{}, fmt.Errorf("%s: emitted=%d max_rows=%d", contract.ErrIDQueryMaxRowsExceeded, emitted, maxRows)
		}
	}
	return runResult{rowsEmitted: emitted}, rows.Err()
}

func emitCSV(rows *sql.Rows, cols []string, maxRows int, out io.Writer) (runResult, error) {
	_, _ = fmt.Fprintln(out, strings.Join(cols, ","))
	emitted := 0
	for rows.Next() {
		m, err := scanRowMap(rows, cols)
		if err != nil {
			return runResult{}, err
		}
		values := make([]string, 0, len(cols))
		for _, c := range cols {
			values = append(values, fmt.Sprintf("%v", m[c]))
		}
		_, _ = fmt.Fprintln(out, strings.Join(values, ","))
		emitted++
		if maxRows > 0 && emitted > maxRows {
			return runResult{}, fmt.Errorf("%s: emitted=%d max_rows=%d", contract.ErrIDQueryMaxRowsExceeded, emitted, maxRows)
		}
	}
	return runResult{rowsEmitted: emitted}, rows.Err()
}

func emitJSON(rows *sql.Rows, cols []string, maxRows int, out io.Writer) (runResult, error) {
	arr := make([]map[string]any, 0)
	emitted := 0
	for rows.Next() {
		m, err := scanRowMap(rows, cols)
		if err != nil {
			return runResult{}, err
		}
		arr = append(arr, m)
		emitted++
		if maxRows > 0 && emitted > maxRows {
			return runResult{}, fmt.Errorf("%s: emitted=%d max_rows=%d", contract.ErrIDQueryMaxRowsExceeded, emitted, maxRows)
		}
	}
	if err := rows.Err(); err != nil {
		return runResult{}, err
	}
	if err := writeJSON(out, arr); err != nil {
		return runResult{}, err
	}
	return runResult{rowsEmitted: emitted}, nil
}

func emitTable(rows *sql.Rows, cols []string, maxRows int, out io.Writer) (runResult, error) {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, strings.Join(cols, "\t"))
	emitted := 0
	for rows.Next() {
		m, err := scanRowMap(rows, cols)
		if err != nil {
			return runResult{}, err
		}
		line := make([]string, 0, len(cols))
		for _, c := range cols {
			line = append(line, fmt.Sprintf("%v", m[c]))
		}
		_, _ = fmt.Fprintln(tw, strings.Join(line, "\t"))
		emitted++
		if maxRows > 0 && emitted > maxRows {
			return runResult{}, fmt.Errorf("%s: emitted=%d max_rows=%d", contract.ErrIDQueryMaxRowsExceeded, emitted, maxRows)
		}
	}
	if err := rows.Err(); err != nil {
		return runResult{}, err
	}
	if err := tw.Flush(); err != nil {
		return runResult{}, err
	}
	return runResult{rowsEmitted: emitted}, nil
}

func scanRowMap(rows *sql.Rows, cols []string) (map[string]any, error) {
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}
	m := make(map[string]any, len(cols))
	for i, c := range cols {
		switch v := vals[i].(type) {
		case []byte:
			m[c] = string(v)
		default:
			m[c] = v
		}
	}
	return m, nil
}

func isTTY(_ uintptr) bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func newDatasetValidateCommand() *cobra.Command {
	format := string(formatText)
	configPath := "doc/design-meta/examples/config/cli-config.cue"
	opts := validate.Options{}
	cmd := &cobra.Command{
		Use:   "validate <dataset-id>",
		Short: "Validate one dataset file contract",
		Long:  "Validate one declared dataset against schema, format, compression, and engine settings.",
		Example: strings.Join([]string{
			"quack dataset validate sales_daily --config ./cli-config.cue",
			"quack dataset validate sales_daily --config ./cli-config.cue --format json --validation-engine native",
		}, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := config.LoadAndValidate(configPath)
			if err != nil {
				return renderConfigError(cmd, err)
			}
			result, err := validate.ValidateDataset(spec, args[0], opts)
			if err != nil {
				return renderValidateResult(cmd, format, result, err)
			}
			return renderValidateResult(cmd, format, result, nil)
		},
	}
	wireValidateFlags(cmd, &configPath, &format, &opts)
	// TODO(spec): implement strict=false semantics; currently strict mode is always enforced.
	cmd.Flags().Bool("strict", true, "Fail when schema mismatch is found")
	return cmd
}

func newDatasetValidateAllCommand() *cobra.Command {
	format := string(formatText)
	configPath := "doc/design-meta/examples/config/cli-config.cue"
	failFast := false
	opts := validate.Options{}
	cmd := &cobra.Command{
		Use:   "validate-all",
		Short: "Validate all declared datasets",
		Long:  "Run dataset validation for every dataset in cliSpec.datasets.",
		Example: strings.Join([]string{
			"quack dataset validate-all --config ./cli-config.cue",
			"quack dataset validate-all --config ./cli-config.cue --fail-fast --format json",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := config.LoadAndValidate(configPath)
			if err != nil {
				return renderConfigError(cmd, err)
			}
			results := make([]validate.DatasetResult, 0, len(spec.Datasets))
			for _, d := range spec.Datasets {
				r, vErr := validate.ValidateDatasetDefinition(spec, d, opts)
				results = append(results, r)
				if vErr != nil && failFast {
					return renderValidateAllResult(cmd, format, results, vErr)
				}
			}
			var finalErr error
			for _, r := range results {
				if r.Status != "ok" {
					finalErr = errors.New(r.ErrorID)
					break
				}
			}
			return renderValidateAllResult(cmd, format, results, finalErr)
		},
	}
	wireValidateFlags(cmd, &configPath, &format, &opts)
	cmd.Flags().BoolVar(&failFast, "fail-fast", false, "Stop on first validation failure")
	return cmd
}

func newDatasetInspectCommand() *cobra.Command {
	format := string(formatText)
	configPath := "doc/design-meta/examples/config/cli-config.cue"
	sampleSize := 1000
	opts := validate.Options{}
	cmd := &cobra.Command{
		Use:   "inspect <dataset-id>",
		Short: "Inspect observed dataset schema",
		Long:  "Inspect a dataset and report discovered columns/types from sampled rows.",
		Example: strings.Join([]string{
			"quack dataset inspect sales_daily --config ./cli-config.cue",
			"quack dataset inspect events_stream --config ./cli-config.cue --validation-engine native --sample-size 100 --format json",
		}, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := config.LoadAndValidate(configPath)
			if err != nil {
				return renderConfigError(cmd, err)
			}
			result, err := validate.InspectDataset(spec, args[0], opts, sampleSize)
			out := map[string]any{
				"output_schema_version": "v1",
				"dataset_id":            result.DatasetID,
				"validation_engine":     result.ValidationEngine,
				"compression":           result.Compression,
				"status":                result.Status,
				"sample_rows":           result.SampleRows,
				"observed_columns":      result.ObservedColumns,
				"duration_ms":           result.DurationMs,
			}
			if result.ErrorID != "" {
				out["error_id"] = result.ErrorID
			}
			if result.Message != "" {
				out["message"] = result.Message
			}
			switch outputFormat(format) {
			case formatJSON:
				if wErr := writeJSON(cmd.OutOrStdout(), out); wErr != nil {
					return wErr
				}
			case formatText:
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "dataset=%s status=%s columns=%d sample_rows=%d duration_ms=%d\n",
					result.DatasetID, result.Status, len(result.ObservedColumns), result.SampleRows, result.DurationMs)
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
			return err
		},
	}
	wireValidateFlags(cmd, &configPath, &format, &opts)
	cmd.Flags().IntVar(&sampleSize, "sample-size", sampleSize, "Number of rows to sample during inspection")
	return cmd
}

func wireValidateFlags(cmd *cobra.Command, configPath *string, format *string, opts *validate.Options) {
	cmd.Flags().StringVar(configPath, "config", *configPath, "Path to CUE config file")
	cmd.Flags().StringVar(format, "format", string(formatText), "Output format: text|json")
	cmd.Flags().StringVar(&opts.ValidationEngine, "validation-engine", "", "Validation engine override: duckdb|native")
	cmd.Flags().StringVar(&opts.Compression, "compression", "", "Compression override: auto|none|gzip|zstd")
	cmd.Flags().IntVar(&opts.RandomSampleRows, "random-sample-rows", 0, "Sample row count for validation")
	cmd.Flags().Int64Var(&opts.SampleSeed, "sample-seed", 0, "Deterministic seed for random sampling")
	cmd.Flags().StringVar(&opts.PartitionFilter, "partition-filter", "", "Filter matched partition files")
	cmd.Flags().IntVar(&opts.MaxFiles, "max-files", 0, "Cap number of files to validate")
	cmd.Flags().IntVar(&opts.RandomSampleFiles, "random-sample-files", 0, "Randomly select N files from discovered set")
}

func renderConfigError(cmd *cobra.Command, err error) error {
	if cErr, ok := err.(*config.ConfigError); ok {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s\n", cErr.Error())
		return fmt.Errorf("%s", cErr.ID)
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%v\n", err)
	return err
}

func renderValidateResult(cmd *cobra.Command, format string, result validate.DatasetResult, err error) error {
	switch outputFormat(format) {
	case formatJSON:
		payload := map[string]any{
			"output_schema_version": "v1",
			"dataset_id":            result.DatasetID,
			"validation_engine":     result.ValidationEngine,
			"compression":           result.Compression,
			"status":                result.Status,
			"files_scanned":         result.FilesScanned,
			"rows_checked":          result.RowsChecked,
			"schema_mismatches":     result.SchemaMismatches,
			"duration_ms":           result.DurationMs,
		}
		if result.ErrorID != "" {
			payload["error_id"] = result.ErrorID
		}
		if result.Message != "" {
			payload["message"] = result.Message
		}
		if wErr := writeJSON(cmd.OutOrStdout(), payload); wErr != nil {
			return wErr
		}
	case formatText:
		msg := fmt.Sprintf(
			"dataset=%s status=%s files_scanned=%d rows_checked=%d schema_mismatches=%d duration_ms=%d",
			result.DatasetID, result.Status, result.FilesScanned, result.RowsChecked, result.SchemaMismatches, result.DurationMs,
		)
		if result.ErrorID != "" {
			msg += " error_id=" + result.ErrorID
		}
		if result.Message != "" {
			msg += " message=" + strconv.Quote(result.Message)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), msg)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
	if err != nil {
		return err
	}
	return nil
}

func renderValidateAllResult(cmd *cobra.Command, format string, results []validate.DatasetResult, err error) error {
	switch outputFormat(format) {
	case formatJSON:
		if wErr := writeJSON(cmd.OutOrStdout(), results); wErr != nil {
			return wErr
		}
	case formatText:
		for _, r := range results {
			line := fmt.Sprintf(
				"dataset=%s status=%s files_scanned=%d rows_checked=%d schema_mismatches=%d duration_ms=%d",
				r.DatasetID, r.Status, r.FilesScanned, r.RowsChecked, r.SchemaMismatches, r.DurationMs,
			)
			if r.ErrorID != "" {
				line += " error_id=" + r.ErrorID
			}
			if r.Message != "" {
				line += " message=" + strconv.Quote(r.Message)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
		}
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
	return err
}

func datasetRows(spec *config.Spec) []datasetListRow {
	rows := make([]datasetListRow, 0, len(spec.Datasets))
	for _, d := range spec.Datasets {
		engine := d.Validation.Engine
		if engine == "" {
			engine = spec.Validation.Engine
		}
		rows = append(rows, datasetListRow{
			DatasetID:        d.ID,
			Format:           d.Format,
			Layout:           d.Layout,
			Compression:      d.Compression,
			Path:             d.Path,
			Prefix:           d.Prefix,
			Suffix:           d.Suffix,
			PartitionKeys:    strings.Join(d.PartitionKeys, ","),
			HomepageURL:      d.HomepageURL,
			Owner:            d.Metadata.Owner,
			PrimaryKey:       d.Metadata.PrimaryKey,
			ValidationEngine: engine,
		})
	}
	slices.SortFunc(rows, func(a, b datasetListRow) int {
		return strings.Compare(a.DatasetID, b.DatasetID)
	})
	return rows
}

func queryRows(spec *config.Spec) []queryListRow {
	rows := make([]queryListRow, 0, len(spec.Queries))
	for _, q := range spec.Queries {
		rows = append(rows, queryListRow{
			QueryID:          q.ID,
			RequiredDatasets: strings.Join(q.RequiredDatasets, ","),
			ParameterCount:   len(q.Parameters),
		})
	}
	slices.SortFunc(rows, func(a, b queryListRow) int {
		return strings.Compare(a.QueryID, b.QueryID)
	})
	return rows
}

func findQuery(spec *config.Spec, queryID string) (config.Query, bool) {
	for _, q := range spec.Queries {
		if q.ID == queryID {
			return q, true
		}
	}
	return config.Query{}, false
}

func parseParams(args []string) (map[string]string, error) {
	out := map[string]string{}
	for _, raw := range args {
		k, v, ok := strings.Cut(raw, "=")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, fmt.Errorf("%s: expected key=value, got %q", contract.ErrIDQueryParamInvalid, raw)
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out, nil
}

func writeDatasetTable(out io.Writer, rows []datasetListRow) error {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "DATASET_ID\tFORMAT\tLAYOUT\tCOMPRESSION\tPATH\tPREFIX\tSUFFIX\tPARTITION_KEYS\tHOMEPAGE_URL\tOWNER\tPRIMARY_KEY\tVALIDATION_ENGINE")
	for _, r := range rows {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.DatasetID, r.Format, r.Layout, r.Compression, r.Path, r.Prefix, r.Suffix, r.PartitionKeys, r.HomepageURL, r.Owner, r.PrimaryKey, r.ValidationEngine)
	}
	return tw.Flush()
}

func writeQueryTable(out io.Writer, rows []queryListRow) error {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "QUERY_ID\tREQUIRED_DATASETS\tPARAMETER_COUNT")
	for _, r := range rows {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d\n", r.QueryID, r.RequiredDatasets, r.ParameterCount)
	}
	return tw.Flush()
}

func writeJSON(out io.Writer, payload any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func writeOutput(out io.Writer, format string, payload any, text string) error {
	switch outputFormat(format) {
	case formatJSON:
		return writeJSON(out, payload)
	case formatText:
		_, err := fmt.Fprintln(out, text)
		return err
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func envInt(name string) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return v
}

func envBool(name string) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	return raw == "1" || raw == "true" || raw == "yes"
}

func chooseInt(flag, env, cfg int) int {
	if flag > 0 {
		return flag
	}
	if env > 0 {
		return env
	}
	return cfg
}

func chooseBool(flag, env, cfg bool) bool {
	if flag {
		return true
	}
	if env {
		return true
	}
	return cfg
}
