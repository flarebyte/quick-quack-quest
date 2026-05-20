package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

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
