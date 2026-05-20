package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/flarebyte/quick-quack-quest/internal/config"
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

func writeDatasetTable(out io.Writer, rows []datasetListRow) error {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "DATASET_ID\tFORMAT\tLAYOUT\tCOMPRESSION\tPATH\tPREFIX\tSUFFIX\tPARTITION_KEYS\tHOMEPAGE_URL\tOWNER\tPRIMARY_KEY\tVALIDATION_ENGINE")
	for _, r := range rows {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.DatasetID, r.Format, r.Layout, r.Compression, r.Path, r.Prefix, r.Suffix, r.PartitionKeys, r.HomepageURL, r.Owner, r.PrimaryKey, r.ValidationEngine)
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
