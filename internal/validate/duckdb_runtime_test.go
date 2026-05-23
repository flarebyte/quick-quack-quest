package validate

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/flarebyte/quick-quack-quest/internal/config"
)

func TestDescribeQueryAndSourceExpr(t *testing.T) {
	t.Parallel()
	d := config.Dataset{Format: "csv"}
	q, args := describeQuery(d, "/tmp/a.csv")
	if q != "DESCRIBE SELECT * FROM read_csv_auto(?)" {
		t.Fatalf("unexpected query: %s", q)
	}
	if len(args) != 1 || args[0] != "/tmp/a.csv" {
		t.Fatalf("unexpected args: %#v", args)
	}

	cases := []struct {
		format string
		wantQ  string
	}{
		{"csv", "read_csv_auto(?)"},
		{"json", "read_json_auto(?)"},
		{"ndjson", "read_json_auto(?, format='newline_delimited')"},
		{"parquet", "read_parquet(?)"},
		{"x", "read_csv_auto(?)"},
	}
	for _, tc := range cases {
		s, _ := sourceExpr(config.Dataset{Format: tc.format}, "p")
		if s != tc.wantQ {
			t.Fatalf("format=%s want=%s got=%s", tc.format, tc.wantQ, s)
		}
	}
}

func TestNormalizeType(t *testing.T) {
	t.Parallel()
	if got := normalizeType(" decimal(10,2)[] "); got != "DECIMAL" {
		t.Fatalf("unexpected decimal normalization: %s", got)
	}
	if got := normalizeType(" varchar "); got != "VARCHAR" {
		t.Fatalf("unexpected varchar normalization: %s", got)
	}
}

func TestDescribeFileAndCountRowsChecked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sales.csv")
	if err := os.WriteFile(path, []byte("id,country\n1,uk\n2,fr\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	d := config.Dataset{Format: "csv"}
	obs, err := describeFile(db, d, path)
	if err != nil {
		t.Fatalf("describeFile: %v", err)
	}
	if obs["id"] == "" || obs["country"] == "" {
		t.Fatalf("expected observed id/country columns, got %v", obs)
	}

	checked, err := countRowsChecked(db, d, path, 0)
	if err != nil {
		t.Fatalf("countRowsChecked full: %v", err)
	}
	if checked != 2 {
		t.Fatalf("expected 2 rows, got %d", checked)
	}

	sampled, err := countRowsChecked(db, d, path, 1)
	if err != nil {
		t.Fatalf("countRowsChecked sample: %v", err)
	}
	if sampled < 0 || sampled > 2 {
		t.Fatalf("expected sample rows in [0,2], got %d", sampled)
	}
}

func TestValidateWithDuckDBSuccessAndReadFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sales.csv")
	if err := os.WriteFile(path, []byte("id,country\n1,uk\n2,fr\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	spec := &config.Spec{}
	d := config.Dataset{
		ID:          "sales",
		Format:      "csv",
		Layout:      "single_file",
		Path:        path,
		Compression: "none",
		Fields: []config.Field{
			{Name: "id", Type: "BIGINT"},
			{Name: "country", Type: "VARCHAR"},
		},
	}
	start := time.Now()
	res, err := validateWithDuckDB(spec, d, Options{}, DatasetResult{DatasetID: d.ID, Compression: "none"}, start)
	if err != nil || res.Status != "ok" || res.FilesScanned != 1 || res.RowsChecked != 2 {
		t.Fatalf("expected success, res=%+v err=%v", res, err)
	}

	dBad := d
	dBad.Path = filepath.Join(dir, "missing.csv")
	_, err = validateWithDuckDB(spec, dBad, Options{}, DatasetResult{DatasetID: d.ID, Compression: "none"}, start)
	if err == nil {
		t.Fatalf("expected read failure")
	}
	expectErrorID(t, err, ErrIDDatasetReadFailed)
}

func TestValidateWithDuckDBSchemaMismatchAndCountError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sales.csv")
	if err := os.WriteFile(path, []byte("id,country\n1,uk\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	spec := &config.Spec{}
	start := time.Now()

	dMismatch := config.Dataset{
		ID: "sales", Format: "csv", Layout: "single_file", Path: path, Compression: "none",
		Fields: []config.Field{{Name: "id", Type: "BIGINT"}, {Name: "country", Type: "INTEGER"}},
	}
	_, err := validateWithDuckDB(spec, dMismatch, Options{}, DatasetResult{DatasetID: "sales", Compression: "none"}, start)
	if err == nil {
		t.Fatalf("expected schema mismatch")
	}
	expectErrorID(t, err, ErrIDSchemaTypeMismatch)

	dBad := config.Dataset{ID: "sales", Format: "csv", Layout: "single_file", Path: filepath.Join(dir, "missing.csv"), Compression: "none", Fields: dMismatch.Fields}
	_, err = validateWithDuckDB(spec, dBad, Options{}, DatasetResult{DatasetID: "sales", Compression: "none"}, start)
	if err == nil {
		t.Fatalf("expected read/count error")
	}
	expectErrorID(t, err, ErrIDDatasetReadFailed)
}

func TestCountRowsCheckedErrorBranch(t *testing.T) {
	t.Parallel()
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	d := config.Dataset{Format: "csv"}
	if _, err := countRowsChecked(db, d, "/definitely/missing.csv", 0); err == nil {
		t.Fatalf("expected countRowsChecked error")
	}
}

func TestValidateWithDuckDBDiscoverFilesErrorBranch(t *testing.T) {
	t.Parallel()
	spec := &config.Spec{}
	d := config.Dataset{ID: "d", Format: "csv", Layout: "unknown", Compression: "none"}
	_, err := validateWithDuckDB(spec, d, Options{}, DatasetResult{DatasetID: "d", Compression: "none"}, time.Now())
	if err == nil {
		t.Fatalf("expected discover files error")
	}
	expectErrorID(t, err, ErrIDPartitionEmpty)
}

func TestValidateWithDuckDBOpenDBErrorBranch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sales.csv")
	if err := os.WriteFile(path, []byte("id\n1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	orig := openValidateDuckDB
	defer func() { openValidateDuckDB = orig }()
	openValidateDuckDB = func() (*sql.DB, error) { return nil, io.ErrUnexpectedEOF }

	d := config.Dataset{ID: "d", Format: "csv", Layout: "single_file", Path: path, Compression: "none", Fields: []config.Field{{Name: "id", Type: "BIGINT"}}}
	_, err := validateWithDuckDB(&config.Spec{}, d, Options{}, DatasetResult{DatasetID: "d", Compression: "none"}, time.Now())
	if err == nil {
		t.Fatalf("expected open db error")
	}
	expectErrorID(t, err, ErrIDDatasetValidateFailed)
}
