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
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/flarebyte/quick-quack-quest/internal/config"
	"github.com/flarebyte/quick-quack-quest/internal/validate"
	"github.com/spf13/cobra"
)

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
		Use:   "quick-quack-quest",
		Short: "Validate datasets and run parameterized DuckDB queries",
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
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := versionPayload{
				Name:    "quick-quack-quest",
				Version: Version,
				Commit:  Commit,
				BuiltAt: BuiltAt,
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
	}
	cmd.AddCommand(newConfigValidateCommand())
	return cmd
}

func newDatasetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dataset",
		Short: "Dataset operations",
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
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := config.LoadAndValidate(configPath)
			if err != nil {
				return renderConfigError(cmd, err)
			}
			q, ok := findQuery(spec, args[0])
			if !ok {
				return fmt.Errorf("QQQ_QUERY_NOT_FOUND: query %s is not declared", args[0])
			}
			paramMap, err := parseParams(params)
			if err != nil {
				return err
			}
			if len(paramMap) > 0 {
				for _, p := range q.Parameters {
					if p.Required && strings.TrimSpace(paramMap[p.Name]) == "" {
						return fmt.Errorf("QQQ_QUERY_PARAM_REQUIRED_MISSING: missing required parameter %s", p.Name)
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
	cmd := &cobra.Command{
		Use:   "run <query-id>",
		Short: "Run one parameterized query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := config.LoadAndValidate(configPath)
			if err != nil {
				return renderConfigError(cmd, err)
			}
			q, ok := findQuery(spec, args[0])
			if !ok {
				return fmt.Errorf("QQQ_QUERY_NOT_FOUND: query %s is not declared", args[0])
			}
			paramMap, err := parseParams(params)
			if err != nil {
				return err
			}
			for _, p := range q.Parameters {
				if p.Required && strings.TrimSpace(paramMap[p.Name]) == "" {
					return fmt.Errorf("QQQ_QUERY_PARAM_REQUIRED_MISSING: missing required parameter %s", p.Name)
				}
			}
			effLimit := limit
			if effLimit <= 0 {
				effLimit = spec.QueryExecution.Limits.DefaultResultLimitRows
			}
			effMaxRows := maxRows
			if effMaxRows <= 0 {
				effMaxRows = spec.QueryExecution.Limits.MaxRows
			}
			if effLimit > 0 && effMaxRows > 0 && effLimit > effMaxRows {
				return fmt.Errorf("QQQ_QUERY_LIMIT_EXCEEDS_MAX_ROWS: limit=%d max_rows=%d", effLimit, effMaxRows)
			}
			effTimeout := timeout
			if effTimeout <= 0 {
				effTimeout = spec.QueryExecution.Limits.TimeoutSeconds
			}
			effStream := stream
			if !stream {
				effStream = spec.QueryExecution.Streaming.DefaultEnabled
			}
			if chunkSize <= 0 {
				chunkSize = spec.QueryExecution.Streaming.ChunkSizeRows
			}
			if progress && isTTY(os.Stderr.Fd()) {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "running query_id=%s\n", q.ID)
			}
			start := time.Now()
			execRes, err := runQuery(spec, q, paramMap, runOptions{
				format:    format,
				stream:    effStream,
				limit:     effLimit,
				maxRows:   effMaxRows,
				timeout:   effTimeout,
				chunkSize: chunkSize,
				out:       cmd.OutOrStdout(),
			})
			if err != nil {
				return err
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

func runQuery(spec *config.Spec, q config.Query, params map[string]string, opts runOptions) (runResult, error) {
	_ = spec
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return runResult{}, err
	}
	defer db.Close()

	for _, dsID := range q.RequiredDatasets {
		ds, ok := findDatasetByID(spec, dsID)
		if !ok {
			return runResult{}, fmt.Errorf("QQQ_DATASET_NOT_FOUND: dataset %s is not declared", dsID)
		}
		glob := ds.Path
		if ds.Layout == "partitioned" {
			glob = ds.Prefix + "*" + ds.Suffix
		}
		src := sourceExprForRun(ds.Format, filepath.ToSlash(glob))
		stmt := fmt.Sprintf("CREATE OR REPLACE VIEW %s AS SELECT * FROM %s", ds.ID, src)
		if _, err := db.Exec(stmt); err != nil {
			return runResult{}, err
		}
	}

	sqlText, err := bindQuery(q, params, opts.limit)
	if err != nil {
		return runResult{}, err
	}
	ctx := context.Background()
	if opts.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.timeout)*time.Second)
		defer cancel()
	}
	rows, err := db.QueryContext(ctx, sqlText)
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

func sourceExprForRun(format, path string) string {
	escaped := strings.ReplaceAll(path, "'", "''")
	switch format {
	case "csv":
		return fmt.Sprintf("read_csv_auto('%s')", escaped)
	case "json":
		return fmt.Sprintf("read_json_auto('%s')", escaped)
	case "ndjson":
		return fmt.Sprintf("read_json_auto('%s', format='newline_delimited')", escaped)
	case "parquet":
		return fmt.Sprintf("read_parquet('%s')", escaped)
	default:
		return fmt.Sprintf("read_csv_auto('%s')", escaped)
	}
}

func bindQuery(q config.Query, params map[string]string, limit int) (string, error) {
	sqlText := q.SQL
	for _, p := range q.Parameters {
		val, ok := params[p.Name]
		if p.Required && (!ok || strings.TrimSpace(val) == "") {
			return "", fmt.Errorf("QQQ_QUERY_PARAM_REQUIRED_MISSING: missing required parameter %s", p.Name)
		}
		if !ok {
			continue
		}
		repl := quoteParam(p.Type, val)
		sqlText = strings.ReplaceAll(sqlText, "$"+p.Name, repl)
	}
	if limit > 0 {
		sqlText = fmt.Sprintf("SELECT * FROM (%s) AS qqq_q LIMIT %d", sqlText, limit)
	}
	return sqlText, nil
}

func quoteParam(typ, value string) string {
	t := strings.ToUpper(strings.TrimSpace(typ))
	if t == "INTEGER" || t == "DOUBLE" || t == "BIGINT" || t == "SMALLINT" {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
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
			return runResult{}, fmt.Errorf("QQQ_QUERY_MAX_ROWS_EXCEEDED: emitted=%d max_rows=%d", emitted, maxRows)
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
			return runResult{}, fmt.Errorf("QQQ_QUERY_MAX_ROWS_EXCEEDED: emitted=%d max_rows=%d", emitted, maxRows)
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
			return runResult{}, fmt.Errorf("QQQ_QUERY_MAX_ROWS_EXCEEDED: emitted=%d max_rows=%d", emitted, maxRows)
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
			return runResult{}, fmt.Errorf("QQQ_QUERY_MAX_ROWS_EXCEEDED: emitted=%d max_rows=%d", emitted, maxRows)
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
		Args:  cobra.ExactArgs(1),
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
		Args:  cobra.ExactArgs(1),
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
		if wErr := writeJSON(cmd.OutOrStdout(), result); wErr != nil {
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
			return nil, fmt.Errorf("QQQ_QUERY_PARAM_INVALID: expected key=value, got %q", raw)
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
