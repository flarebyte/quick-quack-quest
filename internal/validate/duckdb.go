// purpose: Orchestrate dataset validation and DuckDB-backed schema/row checks.
// responsibilities: Resolve options, discover files, validate declared schema, and return structured result/errors.
// architecture notes: Shared validation orchestration lives here while engine-specific readers are delegated.
package validate

import (
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/flarebyte/quick-quack-quest/internal/config"
)

const (
	ErrIDDatasetNotFound          = "QQQ_DATASET_NOT_FOUND"
	ErrIDValidationEngine         = "QQQ_VALIDATION_ENGINE_UNSUPPORTED"
	ErrIDPartitionEmpty           = "QQQ_PARTITION_DISCOVERY_EMPTY"
	ErrIDSchemaFieldMissing       = "QQQ_SCHEMA_FIELD_MISSING"
	ErrIDSchemaTypeMismatch       = "QQQ_SCHEMA_TYPE_MISMATCH"
	ErrIDDatasetReadFailed        = "QQQ_DATASET_READ_FAILED"
	ErrIDDatasetValidateFailed    = "QQQ_DATASET_VALIDATE_FAILED"
	ErrIDNativeCodecUnavailable   = "QQQ_NATIVE_CODEC_UNAVAILABLE"
	ErrIDCompatibilityUnsupported = "QQQ_COMPATIBILITY_UNSUPPORTED"
)

type Error struct {
	ID      string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.ID, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.ID, e.Message, e.Cause)
}

func (e *Error) Unwrap() error { return e.Cause }

type Options struct {
	ValidationEngine  string
	Compression       string
	RandomSampleRows  int
	SampleSeed        int64
	PartitionFilter   string
	MaxFiles          int
	RandomSampleFiles int
}

type DatasetResult struct {
	DatasetID        string `json:"dataset_id"`
	ValidationEngine string `json:"validation_engine"`
	Compression      string `json:"compression"`
	Status           string `json:"status"`
	FilesScanned     int    `json:"files_scanned"`
	RowsChecked      int    `json:"rows_checked"`
	SchemaMismatches int    `json:"schema_mismatches"`
	DurationMs       int64  `json:"duration_ms"`
	ErrorID          string `json:"error_id,omitempty"`
	Message          string `json:"message,omitempty"`
}

func failDatasetResult(result DatasetResult, start time.Time, errID, message string, cause error) (DatasetResult, error) {
	result.Status = "error"
	result.ErrorID = errID
	result.Message = message
	result.DurationMs = time.Since(start).Milliseconds()
	return result, &Error{ID: errID, Message: message, Cause: cause}
}

func validateDeclaredSchema(d config.Dataset, observed map[string]string, result *DatasetResult, start time.Time) (DatasetResult, error) {
	declared := map[string]string{}
	for _, f := range d.Fields {
		declared[strings.ToLower(f.Name)] = strings.ToUpper(f.Type)
	}
	for name, expType := range declared {
		gotType, ok := observed[name]
		if !ok {
			result.SchemaMismatches++
			message := fmt.Sprintf("missing required field %s in dataset %s", name, d.ID)
			return failDatasetResult(*result, start, ErrIDSchemaFieldMissing, message, nil)
		}
		if normalizeType(gotType) != normalizeType(expType) {
			result.SchemaMismatches++
			message := fmt.Sprintf("field %s expected %s but got %s", name, expType, gotType)
			return failDatasetResult(*result, start, ErrIDSchemaTypeMismatch, message, nil)
		}
	}
	return *result, nil
}

func ValidateDataset(spec *config.Spec, datasetID string, opts Options) (DatasetResult, error) {
	d, ok := findDataset(spec, datasetID)
	if !ok {
		return DatasetResult{}, &Error{ID: ErrIDDatasetNotFound, Message: fmt.Sprintf("dataset %s is not declared", datasetID)}
	}
	return ValidateDatasetDefinition(spec, d, opts)
}

func ValidateDatasetDefinition(spec *config.Spec, d config.Dataset, opts Options) (DatasetResult, error) {
	start := time.Now()
	engine := resolveEngine(spec, d, opts.ValidationEngine)
	compression := resolveCompression(d, opts.Compression)
	result := DatasetResult{
		DatasetID:        d.ID,
		ValidationEngine: engine,
		Compression:      compression,
	}
	if engine != "duckdb" && engine != "native" {
		return failDatasetResult(result, start, ErrIDValidationEngine, fmt.Sprintf("validation engine %s is not supported", engine), nil)
	}
	if !supportsValidationCombo(engine, d.Format, compression) {
		return failDatasetResult(result, start, ErrIDCompatibilityUnsupported, fmt.Sprintf("validation is not supported for format=%s compression=%s engine=%s", d.Format, compression, engine), nil)
	}

	if engine == "duckdb" {
		return validateWithDuckDB(spec, d, opts, result, start)
	}
	return validateWithNative(spec, d, opts, result, start)
}

var openValidateDuckDB = func() (*sql.DB, error) {
	return sql.Open("duckdb", "")
}

func validateWithDuckDB(spec *config.Spec, d config.Dataset, opts Options, result DatasetResult, start time.Time) (DatasetResult, error) {
	files, err := discoverFiles(d, opts)
	if err != nil {
		return failDatasetResult(result, start, ErrIDPartitionEmpty, err.Error(), err)
	}
	result.FilesScanned = len(files)

	db, err := openValidateDuckDB()
	if err != nil {
		return failDatasetResult(result, start, ErrIDDatasetValidateFailed, "open duckdb", err)
	}
	defer db.Close()

	for _, p := range files {
		observed, err := describeFile(db, d, p)
		if err != nil {
			return failDatasetResult(result, start, ErrIDDatasetReadFailed, fmt.Sprintf("failed reading %s", p), err)
		}
		if out, schemaErr := validateDeclaredSchema(d, observed, &result, start); schemaErr != nil {
			return out, schemaErr
		}
		checked, err := countRowsChecked(db, d, p, resolveSampleRows(spec, d, opts.RandomSampleRows))
		if err != nil {
			return failDatasetResult(result, start, ErrIDDatasetReadFailed, fmt.Sprintf("failed counting rows for %s", p), err)
		}
		result.RowsChecked += checked
	}

	result.Status = "ok"
	result.DurationMs = time.Since(start).Milliseconds()
	return result, nil
}

func findDataset(spec *config.Spec, datasetID string) (config.Dataset, bool) {
	for _, d := range spec.Datasets {
		if d.ID == datasetID {
			return d, true
		}
	}
	return config.Dataset{}, false
}

func resolveEngine(spec *config.Spec, d config.Dataset, override string) string {
	if override != "" {
		return override
	}
	if env := strings.TrimSpace(os.Getenv("QQQ_VALIDATION_ENGINE")); env != "" {
		return env
	}
	if d.Validation.Engine != "" {
		return d.Validation.Engine
	}
	if spec.Validation.Engine != "" {
		return spec.Validation.Engine
	}
	return "duckdb"
}

func resolveCompression(d config.Dataset, override string) string {
	if override != "" {
		return override
	}
	if d.Compression != "" {
		return d.Compression
	}
	return "auto"
}

func resolveSampleRows(spec *config.Spec, d config.Dataset, override int) int {
	if override > 0 {
		return override
	}
	if d.Validation.RandomSampleRows > 0 {
		return d.Validation.RandomSampleRows
	}
	if spec.Validation.RandomSampleRows > 0 {
		return spec.Validation.RandomSampleRows
	}
	return 0
}

func discoverFiles(d config.Dataset, opts Options) ([]string, error) {
	var files []string
	switch d.Layout {
	case "single_file":
		files = []string{d.Path}
	case "partitioned":
		glob := fmt.Sprintf("%s*%s", d.Prefix, d.Suffix)
		matches, err := filepath.Glob(glob)
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			if opts.PartitionFilter != "" && !strings.Contains(m, opts.PartitionFilter) {
				continue
			}
			files = append(files, m)
		}
	default:
		return nil, fmt.Errorf("unknown layout %s", d.Layout)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files matched dataset layout")
	}
	slices.Sort(files)
	if opts.MaxFiles > 0 && len(files) > opts.MaxFiles {
		files = files[:opts.MaxFiles]
	}
	if opts.RandomSampleFiles > 0 && len(files) > opts.RandomSampleFiles {
		seed := opts.SampleSeed
		if seed == 0 {
			seed = randomSeed()
		}
		picked := pickDeterministicSample(files, opts.RandomSampleFiles, seed)
		slices.Sort(picked)
		files = picked
	}
	return files, nil
}

var randRead = rand.Read

func randomSeed() int64 {
	var b [8]byte
	if _, err := randRead(b[:]); err != nil {
		return 1
	}
	return int64(binary.LittleEndian.Uint64(b[:]))
}

func pickDeterministicSample(files []string, sampleSize int, seed int64) []string {
	type scored struct {
		file  string
		score uint64
	}
	scoredFiles := make([]scored, 0, len(files))
	for _, f := range files {
		h := fnv.New64a()
		_, _ = h.Write([]byte(fmt.Sprintf("%d|%s", seed, f)))
		scoredFiles = append(scoredFiles, scored{
			file:  f,
			score: h.Sum64(),
		})
	}
	slices.SortFunc(scoredFiles, func(a, b scored) int {
		if a.score < b.score {
			return -1
		}
		if a.score > b.score {
			return 1
		}
		return strings.Compare(a.file, b.file)
	})
	n := sampleSize
	if n > len(scoredFiles) {
		n = len(scoredFiles)
	}
	picked := make([]string, 0, n)
	for i := 0; i < n; i++ {
		picked = append(picked, scoredFiles[i].file)
	}
	return picked
}

func describeFile(db *sql.DB, d config.Dataset, path string) (map[string]string, error) {
	q, args := describeQuery(d, path)
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var colName, colType, nullable string
		var key, def, extra sql.NullString
		if err := rows.Scan(&colName, &colType, &nullable, &key, &def, &extra); err != nil {
			return nil, err
		}
		out[strings.ToLower(colName)] = strings.ToUpper(colType)
	}
	return out, rows.Err()
}

func countRowsChecked(db *sql.DB, d config.Dataset, path string, sampleRows int) (int, error) {
	q, args := sourceExpr(d, path)
	if sampleRows > 0 {
		q = fmt.Sprintf("SELECT COUNT(*) FROM (SELECT * FROM %s USING SAMPLE %d ROWS)", q, sampleRows)
	} else {
		q = fmt.Sprintf("SELECT COUNT(*) FROM %s", q)
	}
	var c int
	if err := db.QueryRow(q, args...).Scan(&c); err != nil {
		return 0, err
	}
	return c, nil
}

func describeQuery(d config.Dataset, path string) (string, []any) {
	s, args := sourceExpr(d, path)
	return fmt.Sprintf("DESCRIBE SELECT * FROM %s", s), args
}

func sourceExpr(d config.Dataset, path string) (string, []any) {
	switch d.Format {
	case "csv":
		return "read_csv_auto(?)", []any{path}
	case "json":
		return "read_json_auto(?)", []any{path}
	case "ndjson":
		return "read_json_auto(?, format='newline_delimited')", []any{path}
	case "parquet":
		return "read_parquet(?)", []any{path}
	default:
		return "read_csv_auto(?)", []any{path}
	}
}

func normalizeType(t string) string {
	t = strings.ToUpper(strings.TrimSpace(t))
	t = strings.TrimSuffix(t, "[]")
	if strings.HasPrefix(t, "DECIMAL") {
		return "DECIMAL"
	}
	return t
}

func supportsValidationCombo(engine, format, compression string) bool {
	allowed := map[string]map[string]map[string]bool{
		"duckdb": {
			"csv":     {"none": true, "gzip": true, "zstd": true, "auto": true},
			"json":    {"none": true, "gzip": true, "auto": true},
			"ndjson":  {"none": true, "gzip": true, "auto": true},
			"parquet": {"none": true, "auto": true},
		},
		"native": {
			"csv":    {"none": true, "gzip": true, "zstd": true, "auto": true},
			"json":   {"none": true, "gzip": true, "auto": true},
			"ndjson": {"none": true, "gzip": true, "auto": true},
		},
	}
	engineMap, ok := allowed[engine]
	if !ok {
		return false
	}
	formatMap, ok := engineMap[format]
	if !ok {
		return false
	}
	return formatMap[compression]
}
