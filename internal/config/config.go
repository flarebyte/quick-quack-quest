// purpose: Load and validate CUE configuration into the typed runtime specification.
// responsibilities: Compile CUE input, decode cliSpec, normalize dataset paths, and map failures to config error IDs.
// architecture notes: Path normalization happens at load time so downstream components can assume resolved paths.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

const (
	ErrIDConfigLoad    = "QQQ_CONFIG_LOAD_FAILED"
	ErrIDConfigInvalid = "QQQ_CONFIG_INVALID"
	ErrIDConfigDecode  = "QQQ_CONFIG_DECODE_FAILED"
)

type ConfigError struct {
	ID      string
	Message string
	Cause   error
}

func (e *ConfigError) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.ID, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.ID, e.Message, e.Cause)
}

func (e *ConfigError) Unwrap() error {
	return e.Cause
}

type Spec struct {
	Validation struct {
		Engine           string `json:"engine"`
		RandomSampleRows int    `json:"random_sample_rows"`
	} `json:"validation"`
	DuckDB         DuckDBConfig         `json:"duckdb"`
	QueryExecution QueryExecutionConfig `json:"query_execution"`
	Datasets       []Dataset            `json:"datasets"`
	Queries        []Query              `json:"queries"`
}

type Dataset struct {
	ID            string            `json:"id"`
	Format        string            `json:"format"`
	Layout        string            `json:"layout"`
	Compression   string            `json:"compression"`
	Path          string            `json:"path"`
	Prefix        string            `json:"prefix"`
	Suffix        string            `json:"suffix"`
	PartitionKeys []string          `json:"partition_keys"`
	Description   string            `json:"description"`
	HomepageURL   string            `json:"homepage_url"`
	Validation    DatasetValidation `json:"validation"`
	Metadata      DatasetMetadata   `json:"metadata"`
	Fields        []Field           `json:"fields"`
}

type Query struct {
	ID               string           `json:"id"`
	RequiredDatasets []string         `json:"required_datasets"`
	Parameters       []QueryParameter `json:"parameters"`
	SQL              string           `json:"sql"`
}

type QueryParameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type DuckDBConfig struct {
	DatabasePath      string            `json:"database_path"`
	TempDirectory     string            `json:"temp_directory"`
	Threads           int               `json:"threads"`
	MemoryLimit       string            `json:"memory_limit"`
	AccessMode        string            `json:"access_mode"`
	EnableProgressBar bool              `json:"enable_progress_bar"`
	Extensions        []string          `json:"extensions"`
	Settings          map[string]string `json:"settings"`
}

type QueryExecutionConfig struct {
	Streaming struct {
		DefaultEnabled      bool     `json:"default_enabled"`
		ChunkSizeRows       int      `json:"chunk_size_rows"`
		AllowedOutputFormat []string `json:"allowed_output_formats"`
	} `json:"streaming"`
	Progress struct {
		EnabledByDefault bool `json:"enabled_by_default"`
		TTYOnly          bool `json:"tty_only"`
		MinQueryMs       int  `json:"min_query_ms"`
	} `json:"progress"`
	Limits struct {
		DefaultResultLimitRows int `json:"default_result_limit_rows"`
		MaxRows                int `json:"max_rows"`
		TimeoutSeconds         int `json:"timeout_seconds"`
	} `json:"limits"`
}

type DatasetValidation struct {
	Engine           string `json:"engine"`
	RandomSampleRows int    `json:"random_sample_rows"`
}

type DatasetMetadata struct {
	Owner      string `json:"owner"`
	PrimaryKey string `json:"primary_key"`
}

type Field struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Nullable    bool   `json:"nullable"`
	Description string `json:"description"`
}

func LoadAndValidate(path string) (*Spec, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, &ConfigError{ID: ErrIDConfigLoad, Message: "resolve config path", Cause: err}
	}
	dir := filepath.Dir(abs)
	_ = filepath.Base(abs)

	instances := load.Instances([]string{"."}, &load.Config{Dir: dir})
	if len(instances) == 0 {
		return nil, &ConfigError{ID: ErrIDConfigLoad, Message: "no CUE instances found"}
	}

	ctx := cuecontext.New()
	val := ctx.BuildInstance(instances[0])
	if err := val.Err(); err != nil {
		return nil, &ConfigError{ID: ErrIDConfigInvalid, Message: "build CUE instance", Cause: err}
	}

	cliSpec := val.LookupPath(cue.ParsePath("cliSpec"))
	if !cliSpec.Exists() {
		return nil, &ConfigError{ID: ErrIDConfigInvalid, Message: "missing required value cliSpec"}
	}
	if err := cliSpec.Validate(cue.Final(), cue.Concrete(true)); err != nil {
		return nil, &ConfigError{ID: ErrIDConfigInvalid, Message: "validation failed", Cause: err}
	}

	var spec Spec
	if err := cliSpec.Decode(&spec); err != nil {
		return nil, &ConfigError{ID: ErrIDConfigDecode, Message: "decode cliSpec", Cause: err}
	}
	normalizeDatasetPaths(&spec, abs)

	return &spec, nil
}

func normalizeDatasetPaths(spec *Spec, configPath string) {
	base := findRepoRoot(filepath.Dir(configPath))
	for i := range spec.Datasets {
		if spec.Datasets[i].Path != "" {
			spec.Datasets[i].Path = toAbsPath(base, spec.Datasets[i].Path)
		}
		if spec.Datasets[i].Prefix != "" {
			spec.Datasets[i].Prefix = toAbsPath(base, spec.Datasets[i].Prefix)
		}
	}
}

func findRepoRoot(start string) string {
	cur := start
	for {
		if _, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil {
			return cur
		}
		next := filepath.Dir(cur)
		if next == cur {
			return start
		}
		cur = next
	}
}

func toAbsPath(base, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}
