package validate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/flarebyte/quick-quack-quest/internal/config"
)

func TestInferStringType(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":                     "VARCHAR",
		"42":                   "INTEGER",
		"3.14":                 "DOUBLE",
		"2026-01-01":           "DATE",
		"2026-01-01T10:00:00Z": "TIMESTAMP",
		"abc":                  "VARCHAR",
	}
	for in, want := range cases {
		if got := inferStringType(in); got != want {
			t.Fatalf("inferStringType(%q) want=%s got=%s", in, want, got)
		}
	}
}

func TestInferValueType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   any
		want string
	}{
		{nil, "VARCHAR"},
		{true, "BOOLEAN"},
		{float64(4), "INTEGER"},
		{float64(4.2), "DOUBLE"},
		{"2026-01-01", "DATE"},
		{"2026-01-01T10:00:00Z", "TIMESTAMP"},
		{"abc", "VARCHAR"},
		{map[string]any{"k": "v"}, "JSON"},
		{[]any{1, 2}, "JSON"},
	}
	for _, tc := range cases {
		if got := inferValueType(tc.in); got != tc.want {
			t.Fatalf("inferValueType(%v) want=%s got=%s", tc.in, tc.want, got)
		}
	}
}

func TestPromoteType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		cur, nxt string
		want     string
	}{
		{"", "INTEGER", "INTEGER"},
		{"INTEGER", "INTEGER", "INTEGER"},
		{"INTEGER", "DOUBLE", "DOUBLE"},
		{"DOUBLE", "INTEGER", "DOUBLE"},
		{"DATE", "TIMESTAMP", "TIMESTAMP"},
		{"TIMESTAMP", "DATE", "TIMESTAMP"},
		{"BOOLEAN", "INTEGER", "VARCHAR"},
	}
	for _, tc := range cases {
		if got := promoteType(tc.cur, tc.nxt); got != tc.want {
			t.Fatalf("promoteType(%s,%s) want=%s got=%s", tc.cur, tc.nxt, tc.want, got)
		}
	}
}

func TestSupportsValidationCombo(t *testing.T) {
	t.Parallel()
	if !supportsValidationCombo("duckdb", "csv", "gzip") {
		t.Fatalf("expected duckdb/csv/gzip supported")
	}
	if supportsValidationCombo("native", "parquet", "none") {
		t.Fatalf("expected native/parquet unsupported")
	}
	if supportsValidationCombo("bogus", "csv", "none") {
		t.Fatalf("expected unknown engine unsupported")
	}
}

func TestResolveCompressionAndSampleRows(t *testing.T) {
	t.Parallel()
	spec := &config.Spec{}
	spec.Validation.RandomSampleRows = 11
	d := config.Dataset{Compression: "gzip"}
	d.Validation.RandomSampleRows = 7

	if got := resolveCompression(d, "none"); got != "none" {
		t.Fatalf("override should win, got %s", got)
	}
	if got := resolveCompression(d, ""); got != "gzip" {
		t.Fatalf("dataset compression should win, got %s", got)
	}
	if got := resolveSampleRows(spec, d, 5); got != 5 {
		t.Fatalf("override should win, got %d", got)
	}
	if got := resolveSampleRows(spec, d, 0); got != 7 {
		t.Fatalf("dataset random_sample_rows should win, got %d", got)
	}
	d.Validation.RandomSampleRows = 0
	if got := resolveSampleRows(spec, d, 0); got != 11 {
		t.Fatalf("spec random_sample_rows should win, got %d", got)
	}
}

func TestDiscoverFilesPartitionedAndSampling(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := []string{
		"sales/date=2026-01-01.csv",
		"sales/date=2026-01-02.csv",
		"sales/date=2026-01-03.csv",
	}
	for _, rel := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	d := config.Dataset{
		Layout: "partitioned",
		Prefix: filepath.ToSlash(filepath.Join(dir, "sales/date=")),
		Suffix: ".csv",
	}
	out, err := discoverFiles(d, Options{})
	if err != nil {
		t.Fatalf("discoverFiles: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 files, got %d", len(out))
	}

	filtered, err := discoverFiles(d, Options{PartitionFilter: "2026-01-02"})
	if err != nil {
		t.Fatalf("discoverFiles filtered: %v", err)
	}
	if len(filtered) != 1 || !reflect.DeepEqual(filepath.Base(filtered[0]), "date=2026-01-02.csv") {
		t.Fatalf("unexpected filtered files: %#v", filtered)
	}

	sampled, err := discoverFiles(d, Options{RandomSampleFiles: 2, SampleSeed: 123})
	if err != nil {
		t.Fatalf("discoverFiles sampled: %v", err)
	}
	if len(sampled) != 2 {
		t.Fatalf("expected sampled len 2, got %d", len(sampled))
	}
	resampled, err := discoverFiles(d, Options{RandomSampleFiles: 2, SampleSeed: 123})
	if err != nil {
		t.Fatalf("discoverFiles resampled: %v", err)
	}
	if !reflect.DeepEqual(sampled, resampled) {
		t.Fatalf("expected deterministic sampling, got %v vs %v", sampled, resampled)
	}
}

func TestDiscoverFilesErrors(t *testing.T) {
	t.Parallel()
	if _, err := discoverFiles(config.Dataset{Layout: "bad"}, Options{}); err == nil {
		t.Fatalf("expected unknown layout error")
	}
	if _, err := discoverFiles(config.Dataset{Layout: "partitioned", Prefix: "/definitely/not/exist/", Suffix: ".csv"}, Options{}); err == nil {
		t.Fatalf("expected empty partition error")
	}
}

func TestInspectDatasetErrorsAndOrdering(t *testing.T) {
	t.Parallel()
	spec := &config.Spec{}
	if _, err := InspectDataset(spec, "missing", Options{}, 10); err == nil {
		t.Fatalf("expected dataset not found")
	}

	d := config.Dataset{ID: "d1", Format: "csv", Layout: "single_file", Path: "/does/not/exist.csv", Compression: "none"}
	_, err := InspectDatasetDefinition(spec, d, Options{ValidationEngine: "bogus"}, 1)
	if err == nil {
		t.Fatalf("expected unsupported engine")
	}
	expectErrorID(t, err, ErrIDValidationEngine)

	ordered := orderObservedColumns(
		[]config.Field{{Name: "b"}, {Name: "a"}},
		map[string]ObservedColumn{
			"a": {Name: "a"},
			"b": {Name: "b"},
			"z": {Name: "z"},
		},
	)
	if len(ordered) != 3 || ordered[0].Name != "b" || ordered[1].Name != "a" || ordered[2].Name != "z" {
		t.Fatalf("unexpected observed order: %#v", ordered)
	}
}

func TestResolveEngineSpecFallbackAndRandomSeed(t *testing.T) {
	t.Parallel()
	spec := &config.Spec{}
	spec.Validation.Engine = "native"
	d := config.Dataset{}
	if got := resolveEngine(spec, d, ""); got != "native" {
		t.Fatalf("expected spec engine fallback native, got %s", got)
	}
	if s := randomSeed(); s == 0 {
		t.Fatalf("expected non-zero seed")
	}
}

func TestInspectDatasetDefinitionErrorBranches(t *testing.T) {
	t.Parallel()
	spec := &config.Spec{}
	d := config.Dataset{ID: "d", Format: "csv", Layout: "single_file", Path: "/missing.csv", Compression: "none"}

	_, err := InspectDatasetDefinition(spec, d, Options{ValidationEngine: "native"}, 1)
	if err == nil {
		t.Fatalf("expected read/partition error")
	}
	if !strings.Contains(err.Error(), ErrIDPartitionEmpty) && !strings.Contains(err.Error(), ErrIDDatasetReadFailed) {
		t.Fatalf("expected partition/read error, got %v", err)
	}

	_, err = InspectDatasetDefinition(spec, d, Options{ValidationEngine: "native", Compression: "brotli"}, 1)
	if err == nil {
		t.Fatalf("expected compatibility unsupported")
	}
	expectErrorID(t, err, ErrIDCompatibilityUnsupported)
}

func TestValidateErrorHelpersAndDatasetBranches(t *testing.T) {
	t.Parallel()

	e := &Error{ID: "E1", Message: "plain"}
	if e.Unwrap() != nil {
		t.Fatalf("expected nil unwrap")
	}
	if !strings.Contains(e.Error(), "E1: plain") {
		t.Fatalf("unexpected error string: %s", e.Error())
	}

	cause := os.ErrInvalid
	e2 := &Error{ID: "E2", Message: "with cause", Cause: cause}
	if e2.Unwrap() != cause {
		t.Fatalf("expected unwrap cause")
	}
	if !strings.Contains(e2.Error(), "with cause") {
		t.Fatalf("unexpected error with cause: %s", e2.Error())
	}

	spec := &config.Spec{}
	if _, err := ValidateDataset(spec, "missing", Options{}); err == nil {
		t.Fatalf("expected missing dataset error")
	}

	d := config.Dataset{ID: "d1", Format: "csv", Layout: "single_file", Path: "/missing.csv", Compression: "none"}
	_, err := ValidateDatasetDefinition(spec, d, Options{ValidationEngine: "bogus"})
	if err == nil {
		t.Fatalf("expected unsupported engine")
	}
	expectErrorID(t, err, ErrIDValidationEngine)

	_, err = ValidateDatasetDefinition(spec, d, Options{ValidationEngine: "native", Compression: "brotli"})
	if err == nil {
		t.Fatalf("expected unsupported combo")
	}
	expectErrorID(t, err, ErrIDCompatibilityUnsupported)
}

func TestValidateDeclaredSchemaMissingField(t *testing.T) {
	t.Parallel()

	d := config.Dataset{ID: "d", Fields: []config.Field{{Name: "id", Type: "INTEGER"}, {Name: "country", Type: "VARCHAR"}}}
	res := DatasetResult{DatasetID: "d"}
	_, err := validateDeclaredSchema(d, map[string]string{"id": "INTEGER"}, &res, time.Now())
	if err == nil {
		t.Fatalf("expected missing field schema error")
	}
	expectErrorID(t, err, ErrIDSchemaFieldMissing)
}

func TestResolveEngineDefaultDuckDBAndCompressionAuto(t *testing.T) {
	t.Setenv("QQQ_VALIDATION_ENGINE", "")
	spec := &config.Spec{}
	d := config.Dataset{}
	if got := resolveEngine(spec, d, ""); got != "duckdb" {
		t.Fatalf("expected default duckdb, got %s", got)
	}
	if got := resolveCompression(d, ""); got != "auto" {
		t.Fatalf("expected default auto compression, got %s", got)
	}
}

func TestDiscoverFilesSingleFileAndMaxFiles(t *testing.T) {
	t.Parallel()
	d := config.Dataset{Layout: "single_file", Path: "/tmp/single.csv"}
	out, err := discoverFiles(d, Options{})
	if err != nil || len(out) != 1 || out[0] != "/tmp/single.csv" {
		t.Fatalf("unexpected single_file discovery out=%v err=%v", out, err)
	}

	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		p := filepath.Join(dir, "sales", fmt.Sprintf("date=2026-01-0%d.csv", i+1))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	d2 := config.Dataset{Layout: "partitioned", Prefix: filepath.ToSlash(filepath.Join(dir, "sales/date=")), Suffix: ".csv"}
	out, err = discoverFiles(d2, Options{MaxFiles: 2})
	if err != nil {
		t.Fatalf("discover max files: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected maxfiles trim to 2, got %d", len(out))
	}
}

func TestPickDeterministicSampleOverLimit(t *testing.T) {
	t.Parallel()
	files := []string{"a", "b"}
	picked := pickDeterministicSample(files, 5, 123)
	if len(picked) != 2 {
		t.Fatalf("expected capped sample size 2, got %d", len(picked))
	}
}

func TestValidateDatasetDefinitionDuckDBPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "d.csv")
	if err := os.WriteFile(path, []byte("id,name\n1,a\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	spec := &config.Spec{}
	d := config.Dataset{
		ID: "d", Format: "csv", Layout: "single_file", Path: path, Compression: "none",
		Fields: []config.Field{{Name: "id", Type: "BIGINT"}, {Name: "name", Type: "VARCHAR"}},
	}
	res, err := ValidateDatasetDefinition(spec, d, Options{ValidationEngine: "duckdb"})
	if err != nil || res.Status != "ok" {
		t.Fatalf("expected duckdb validate success, res=%+v err=%v", res, err)
	}
}

func TestRandomSeedFallbackOnReadError(t *testing.T) {
	t.Parallel()
	orig := randRead
	defer func() { randRead = orig }()
	randRead = func(_ []byte) (int, error) { return 0, io.ErrUnexpectedEOF }
	if got := randomSeed(); got != 1 {
		t.Fatalf("expected fallback seed 1, got %d", got)
	}
}

func TestValidateDatasetDefinitionNativeSwitchBranch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "d.json")
	if err := os.WriteFile(jsonPath, []byte(`[{"id":1}]`), 0o644); err != nil {
		t.Fatalf("write json: %v", err)
	}
	spec := &config.Spec{}
	d := config.Dataset{ID: "d", Format: "json", Layout: "single_file", Path: jsonPath, Compression: "none", Fields: []config.Field{{Name: "id", Type: "INTEGER"}}}
	res, err := ValidateDatasetDefinition(spec, d, Options{ValidationEngine: "native"})
	if err != nil || res.Status != "ok" {
		t.Fatalf("expected native switch branch success, res=%+v err=%v", res, err)
	}
}
