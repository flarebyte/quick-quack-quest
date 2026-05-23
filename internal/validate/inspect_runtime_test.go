package validate

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flarebyte/quick-quack-quest/internal/config"
)

func TestInspectWithDuckDBAndNative(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	csv1 := filepath.Join(dir, "a.csv")
	csv2 := filepath.Join(dir, "b.csv")
	if err := os.WriteFile(csv1, []byte("id,val\n1,10\n"), 0o644); err != nil {
		t.Fatalf("write csv1: %v", err)
	}
	if err := os.WriteFile(csv2, []byte("id,val\n2,11.5\n"), 0o644); err != nil {
		t.Fatalf("write csv2: %v", err)
	}
	d := config.Dataset{Format: "csv"}

	obsDuck, err := inspectWithDuckDB(d, []string{csv1, csv2}, 0)
	if err != nil {
		t.Fatalf("inspectWithDuckDB: %v", err)
	}
	if obsDuck["id"].Name == "" || obsDuck["val"].Name == "" {
		t.Fatalf("unexpected duckdb observed columns: %v", obsDuck)
	}

	obsNative, err := inspectWithNative(d, []string{csv1, csv2}, "none", 0)
	if err != nil {
		t.Fatalf("inspectWithNative: %v", err)
	}
	if obsNative["id"].DuckDBType == "" || !obsNative["id"].Nullable {
		t.Fatalf("unexpected native observed id column: %+v", obsNative["id"])
	}
}

func TestInspectWithDuckDBErrors(t *testing.T) {
	t.Parallel()
	d := config.Dataset{Format: "csv"}
	if _, err := inspectWithDuckDB(d, []string{"/definitely/missing.csv"}, 0); err == nil {
		t.Fatalf("expected inspectWithDuckDB error")
	}
}

func TestInspectDatasetDefinitionSuccessAndErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	csv := filepath.Join(dir, "s.csv")
	if err := os.WriteFile(csv, []byte("id,country\n1,uk\n2,fr\n"), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	spec := &config.Spec{}
	d := config.Dataset{ID: "s", Format: "csv", Layout: "single_file", Path: csv, Compression: "none"}

	res, err := InspectDatasetDefinition(spec, d, Options{ValidationEngine: "native"}, 1)
	if err != nil || res.Status != "ok" || len(res.ObservedColumns) == 0 {
		t.Fatalf("expected inspect success, res=%+v err=%v", res, err)
	}

	_, err = InspectDatasetDefinition(spec, d, Options{ValidationEngine: "native", Compression: "brotli"}, 1)
	if err == nil {
		t.Fatalf("expected compatibility error")
	}
	expectErrorID(t, err, ErrIDCompatibilityUnsupported)

	dBad := d
	dBad.Path = filepath.Join(dir, "missing.csv")
	_, err = InspectDatasetDefinition(spec, dBad, Options{ValidationEngine: "native"}, 1)
	if err == nil {
		t.Fatalf("expected dataset read failure")
	}
	if !strings.Contains(err.Error(), ErrIDPartitionEmpty) && !strings.Contains(err.Error(), ErrIDDatasetReadFailed) {
		t.Fatalf("expected partition/read error, got %v", err)
	}
}

func TestInspectDatasetWrapperSuccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	csv := filepath.Join(dir, "w.csv")
	if err := os.WriteFile(csv, []byte("id,name\n1,a\n"), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	spec := &config.Spec{Datasets: []config.Dataset{{ID: "w", Format: "csv", Layout: "single_file", Path: csv, Compression: "none"}}}
	res, err := InspectDataset(spec, "w", Options{ValidationEngine: "native"}, 10)
	if err != nil || res.Status != "ok" {
		t.Fatalf("expected inspect wrapper success, res=%+v err=%v", res, err)
	}
}

func TestInspectWithDuckDBNoColumnsBranch(t *testing.T) {
	t.Parallel()
	d := config.Dataset{Format: "csv"}
	if _, err := inspectWithDuckDB(d, []string{}, 0); err == nil {
		t.Fatalf("expected no columns observed error")
	}
}

func TestInspectWithDuckDBSampleSizeBreak(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	csv1 := filepath.Join(dir, "a.csv")
	csv2 := filepath.Join(dir, "b.csv")
	if err := os.WriteFile(csv1, []byte("id,val\n1,10\n"), 0o644); err != nil {
		t.Fatalf("write csv1: %v", err)
	}
	if err := os.WriteFile(csv2, []byte("id,val\n2,11\n"), 0o644); err != nil {
		t.Fatalf("write csv2: %v", err)
	}
	obs, err := inspectWithDuckDB(config.Dataset{Format: "csv"}, []string{csv1, csv2}, 1)
	if err != nil || len(obs) == 0 {
		t.Fatalf("expected success with sample-size break, obs=%v err=%v", obs, err)
	}
}

func TestInspectWithDuckDBMultiFileBranch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	j1 := filepath.Join(dir, "a.json")
	j2 := filepath.Join(dir, "b.json")
	if err := os.WriteFile(j1, []byte(`[{"val":1}]`), 0o644); err != nil {
		t.Fatalf("write json1: %v", err)
	}
	if err := os.WriteFile(j2, []byte(`[{"val":"abc"}]`), 0o644); err != nil {
		t.Fatalf("write json2: %v", err)
	}
	obs, err := inspectWithDuckDB(config.Dataset{Format: "json"}, []string{j1, j2}, 0)
	if err != nil || len(obs) == 0 {
		t.Fatalf("inspectWithDuckDB multi-file: obs=%v err=%v", obs, err)
	}
}

func TestInspectWithDuckDBOpenDBErrorBranch(t *testing.T) {
	orig := openInspectDuckDB
	defer func() { openInspectDuckDB = orig }()
	openInspectDuckDB = func() (*sql.DB, error) { return nil, io.ErrUnexpectedEOF }

	if _, err := inspectWithDuckDB(config.Dataset{Format: "csv"}, []string{"/tmp/x.csv"}, 0); err == nil {
		t.Fatalf("expected open db error")
	}
}

func TestInspectWithDuckDBRowsErrBranch(t *testing.T) {
	dir := t.TempDir()
	csv := filepath.Join(dir, "ok.csv")
	if err := os.WriteFile(csv, []byte("id,name\n1,alpha\n"), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	orig := inspectRowsErr
	defer func() { inspectRowsErr = orig }()
	inspectRowsErr = func(_ *sql.Rows) error { return io.ErrUnexpectedEOF }
	if _, err := inspectWithDuckDB(config.Dataset{Format: "csv"}, []string{csv}, 0); err == nil {
		t.Fatalf("expected rows.Err hook failure")
	}
}
