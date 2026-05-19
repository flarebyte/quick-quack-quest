package cli

import (
	"encoding/json"
	"fmt"
	"io"

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

func writeOutput(out io.Writer, format string, payload any, text string) error {
	switch outputFormat(format) {
	case formatJSON:
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	case formatText:
		_, err := fmt.Fprintln(out, text)
		return err
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}
