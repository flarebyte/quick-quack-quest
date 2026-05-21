package validate

import (
	"testing"

	"github.com/flarebyte/quick-quack-quest/internal/config"
)

func loadSpec(t *testing.T) *config.Spec {
	t.Helper()
	spec, err := config.LoadAndValidate("../../doc/design-meta/examples/config/cli-config.cue")
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	return spec
}

func TestResolveEnginePrecedence(t *testing.T) {
	spec := loadSpec(t)
	t.Setenv("QQQ_VALIDATION_ENGINE", "native")
	d := spec.Datasets[0]
	d.Validation.Engine = "duckdb"

	if got := resolveEngine(spec, d, "duckdb"); got != "duckdb" {
		t.Fatalf("flag should win, got %s", got)
	}
	if got := resolveEngine(spec, d, ""); got != "native" {
		t.Fatalf("env should win over config, got %s", got)
	}
	t.Setenv("QQQ_VALIDATION_ENGINE", "")
	if got := resolveEngine(spec, d, ""); got != "duckdb" {
		t.Fatalf("dataset config should win when env absent, got %s", got)
	}
}

func TestValidateDatasetNativeUnsupportedCompression(t *testing.T) {
	t.Parallel()
	spec := loadSpec(t)

	_, err := ValidateDataset(spec, "customers_master", Options{
		ValidationEngine: "native",
		Compression:      "brotli",
	})
	if err == nil {
		t.Fatalf("expected compatibility error")
	}
	expectErrorID(t, err, ErrIDCompatibilityUnsupported)
}

func TestValidateDatasetNativeParquetUnsupported(t *testing.T) {
	t.Parallel()
	spec := loadSpec(t)
	spec.Datasets = append(spec.Datasets, config.Dataset{
		ID:          "pq",
		Format:      "parquet",
		Layout:      "single_file",
		Path:        "/tmp/not-used.parquet",
		Compression: "none",
	})
	_, err := ValidateDataset(spec, "pq", Options{ValidationEngine: "native"})
	if err == nil {
		t.Fatalf("expected compatibility error")
	}
	expectErrorID(t, err, ErrIDCompatibilityUnsupported)
}

func expectErrorID(t *testing.T, err error, want string) {
	t.Helper()
	vErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected validate error type, got %T", err)
	}
	if vErr.ID != want {
		t.Fatalf("expected %s, got %s", want, vErr.ID)
	}
}
