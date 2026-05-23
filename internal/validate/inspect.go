// purpose: Orchestrate dataset inspection and shape normalized inspection results.
// responsibilities: Resolve engine/compression, dispatch to inspectors, and deterministically order observed columns.
// architecture notes: Inspection reuses validation error-ID conventions for consistent CLI integration.
package validate

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/flarebyte/quick-quack-quest/internal/config"
)

type ObservedColumn struct {
	Name       string `json:"name"`
	DuckDBType string `json:"duckdb_type"`
	Nullable   bool   `json:"nullable"`
}

type InspectResult struct {
	DatasetID        string           `json:"dataset_id"`
	ValidationEngine string           `json:"validation_engine"`
	Compression      string           `json:"compression"`
	Status           string           `json:"status"`
	SampleRows       int              `json:"sample_rows"`
	ObservedColumns  []ObservedColumn `json:"observed_columns"`
	DurationMs       int64            `json:"duration_ms"`
	ErrorID          string           `json:"error_id,omitempty"`
	Message          string           `json:"message,omitempty"`
}

func failInspectResult(result InspectResult, start time.Time, errID, message string, cause error) (InspectResult, error) {
	result.Status = "error"
	result.ErrorID = errID
	result.Message = message
	result.DurationMs = time.Since(start).Milliseconds()
	return result, &Error{ID: errID, Message: message, Cause: cause}
}

func InspectDataset(spec *config.Spec, datasetID string, opts Options, sampleSize int) (InspectResult, error) {
	d, ok := findDataset(spec, datasetID)
	if !ok {
		return InspectResult{}, &Error{ID: ErrIDDatasetNotFound, Message: fmt.Sprintf("dataset %s is not declared", datasetID)}
	}
	return InspectDatasetDefinition(spec, d, opts, sampleSize)
}

func InspectDatasetDefinition(spec *config.Spec, d config.Dataset, opts Options, sampleSize int) (InspectResult, error) {
	start := time.Now()
	engine := resolveEngine(spec, d, opts.ValidationEngine)
	compression := resolveCompression(d, opts.Compression)
	result := InspectResult{
		DatasetID:        d.ID,
		ValidationEngine: engine,
		Compression:      compression,
		SampleRows:       sampleSize,
	}
	if engine != "duckdb" && engine != "native" {
		return failInspectResult(result, start, ErrIDValidationEngine, fmt.Sprintf("validation engine %s is not supported", engine), nil)
	}
	if !supportsValidationCombo(engine, d.Format, compression) {
		return failInspectResult(result, start, ErrIDCompatibilityUnsupported, fmt.Sprintf("validation is not supported for format=%s compression=%s engine=%s", d.Format, compression, engine), nil)
	}

	files, err := discoverFiles(d, opts)
	if err != nil {
		return failInspectResult(result, start, ErrIDPartitionEmpty, err.Error(), err)
	}

	var observed map[string]ObservedColumn
	switch engine {
	case "duckdb":
		observed, err = inspectWithDuckDB(d, files, sampleSize)
	case "native":
		observed, err = inspectWithNative(d, files, compression, sampleSize)
	}
	if err != nil {
		return failInspectResult(result, start, ErrIDDatasetReadFailed, err.Error(), err)
	}

	result.ObservedColumns = orderObservedColumns(d.Fields, observed)
	result.Status = "ok"
	result.DurationMs = time.Since(start).Milliseconds()
	return result, nil
}

func orderObservedColumns(fields []config.Field, observed map[string]ObservedColumn) []ObservedColumn {
	out := make([]ObservedColumn, 0, len(observed))
	used := map[string]bool{}
	for _, f := range fields {
		key := strings.ToLower(f.Name)
		if c, ok := observed[key]; ok {
			out = append(out, c)
			used[key] = true
		}
	}
	extras := make([]string, 0)
	for k := range observed {
		if !used[k] {
			extras = append(extras, k)
		}
	}
	slices.Sort(extras)
	for _, k := range extras {
		out = append(out, observed[k])
	}
	return out
}
