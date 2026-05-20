package validate

import (
	"bufio"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/flarebyte/quick-quack-quest/internal/config"
	"github.com/klauspost/compress/zstd"
)

func validateWithNative(spec *config.Spec, d config.Dataset, opts Options, result DatasetResult, start time.Time) (DatasetResult, error) {
	if d.Format == "parquet" {
		result.Status = "error"
		result.ErrorID = ErrIDCompatibilityUnsupported
		result.Message = "native validation does not support parquet"
		result.DurationMs = time.Since(start).Milliseconds()
		return result, &Error{ID: ErrIDCompatibilityUnsupported, Message: result.Message}
	}

	files, err := discoverFiles(d, opts)
	if err != nil {
		result.Status = "error"
		result.ErrorID = ErrIDPartitionEmpty
		result.Message = err.Error()
		result.DurationMs = time.Since(start).Milliseconds()
		return result, &Error{ID: ErrIDPartitionEmpty, Message: err.Error(), Cause: err}
	}
	result.FilesScanned = len(files)

	declared := map[string]string{}
	for _, f := range d.Fields {
		declared[strings.ToLower(f.Name)] = strings.ToUpper(f.Type)
	}
	sampleRows := resolveSampleRows(spec, d, opts.RandomSampleRows)

	for _, p := range files {
		observed, checked, err := inspectNativeFile(d, p, result.Compression, sampleRows)
		if err != nil {
			id := ErrIDDatasetReadFailed
			if strings.Contains(err.Error(), "codec") {
				id = ErrIDNativeCodecUnavailable
			}
			result.Status = "error"
			result.ErrorID = id
			result.Message = fmt.Sprintf("failed reading %s", p)
			result.DurationMs = time.Since(start).Milliseconds()
			return result, &Error{ID: id, Message: result.Message, Cause: err}
		}
		result.RowsChecked += checked
		for name, expType := range declared {
			gotType, ok := observed[name]
			if !ok {
				result.SchemaMismatches++
				result.Status = "error"
				result.ErrorID = ErrIDSchemaFieldMissing
				result.Message = fmt.Sprintf("missing required field %s in dataset %s", name, d.ID)
				result.DurationMs = time.Since(start).Milliseconds()
				return result, &Error{ID: ErrIDSchemaFieldMissing, Message: result.Message}
			}
			if normalizeType(gotType) != normalizeType(expType) {
				result.SchemaMismatches++
				result.Status = "error"
				result.ErrorID = ErrIDSchemaTypeMismatch
				result.Message = fmt.Sprintf("field %s expected %s but got %s", name, expType, gotType)
				result.DurationMs = time.Since(start).Milliseconds()
				return result, &Error{ID: ErrIDSchemaTypeMismatch, Message: result.Message}
			}
		}
	}

	result.Status = "ok"
	result.DurationMs = time.Since(start).Milliseconds()
	return result, nil
}

func inspectNativeFile(d config.Dataset, path, compression string, sampleRows int) (map[string]string, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	r, err := decompressReader(f, path, compression)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = r.Close() }()

	switch d.Format {
	case "csv":
		return inspectCSV(r, sampleRows)
	case "json":
		return inspectJSON(r, sampleRows)
	case "ndjson":
		return inspectNDJSON(r, sampleRows)
	default:
		return nil, 0, fmt.Errorf("unsupported native format %s", d.Format)
	}
}

func inspectCSV(r io.Reader, sampleRows int) (map[string]string, int, error) {
	cr := csv.NewReader(r)
	headers, err := cr.Read()
	if err != nil {
		return nil, 0, err
	}
	out := map[string]string{}
	for _, h := range headers {
		out[strings.ToLower(strings.TrimSpace(h))] = "VARCHAR"
	}
	rows := 0
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, rows, err
		}
		rows++
		for i, v := range rec {
			if i >= len(headers) {
				continue
			}
			k := strings.ToLower(strings.TrimSpace(headers[i]))
			out[k] = promoteType(out[k], inferStringType(v))
		}
		if sampleRows > 0 && rows >= sampleRows {
			break
		}
	}
	return out, rows, nil
}

func inspectJSON(r io.Reader, sampleRows int) (map[string]string, int, error) {
	var arr []map[string]any
	if err := json.NewDecoder(r).Decode(&arr); err != nil {
		return nil, 0, err
	}
	if sampleRows > 0 && len(arr) > sampleRows {
		arr = arr[:sampleRows]
	}
	out := map[string]string{}
	for _, obj := range arr {
		for k, v := range obj {
			key := strings.ToLower(k)
			out[key] = promoteType(out[key], inferValueType(v))
		}
	}
	return out, len(arr), nil
}

func inspectNDJSON(r io.Reader, sampleRows int) (map[string]string, int, error) {
	s := bufio.NewScanner(r)
	out := map[string]string{}
	rows := 0
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			return nil, rows, err
		}
		rows++
		for k, v := range obj {
			key := strings.ToLower(k)
			out[key] = promoteType(out[key], inferValueType(v))
		}
		if sampleRows > 0 && rows >= sampleRows {
			break
		}
	}
	if err := s.Err(); err != nil {
		return nil, rows, err
	}
	return out, rows, nil
}

func decompressReader(f *os.File, path, compression string) (io.ReadCloser, error) {
	codec := compression
	if codec == "" || codec == "auto" {
		switch {
		case strings.HasSuffix(path, ".gz"):
			codec = "gzip"
		case strings.HasSuffix(path, ".zst") || strings.HasSuffix(path, ".zstd"):
			codec = "zstd"
		default:
			codec = "none"
		}
	}
	switch codec {
	case "none":
		return f, nil
	case "gzip":
		gr, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		return wrapCloser{Reader: gr, closeFn: func() error {
			_ = gr.Close()
			return nil
		}}, nil
	case "zstd":
		zr, err := zstd.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("codec zstd unavailable: %w", err)
		}
		return wrapCloser{Reader: zr, closeFn: func() error {
			zr.Close()
			return nil
		}}, nil
	default:
		return nil, fmt.Errorf("codec %s unsupported", codec)
	}
}

type wrapCloser struct {
	io.Reader
	closeFn func() error
}

func (w wrapCloser) Close() error {
	if w.closeFn == nil {
		return nil
	}
	return w.closeFn()
}

func inferStringType(v string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return "VARCHAR"
	}
	if _, err := strconv.ParseInt(s, 10, 64); err == nil {
		return "INTEGER"
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return "DOUBLE"
	}
	if _, err := time.Parse("2006-01-02", s); err == nil {
		return "DATE"
	}
	if _, err := time.Parse(time.RFC3339, s); err == nil {
		return "TIMESTAMP"
	}
	return "VARCHAR"
}

func inferValueType(v any) string {
	switch x := v.(type) {
	case nil:
		return "VARCHAR"
	case bool:
		return "BOOLEAN"
	case float64:
		if x == float64(int64(x)) {
			return "INTEGER"
		}
		return "DOUBLE"
	case string:
		if _, err := time.Parse("2006-01-02", x); err == nil {
			return "DATE"
		}
		if _, err := time.Parse(time.RFC3339, x); err == nil {
			return "TIMESTAMP"
		}
		return "VARCHAR"
	case map[string]any, []any:
		return "JSON"
	default:
		return "VARCHAR"
	}
}

func promoteType(cur, nxt string) string {
	if cur == "" {
		return nxt
	}
	if cur == nxt {
		return cur
	}
	if cur == "DOUBLE" || nxt == "DOUBLE" {
		if (cur == "INTEGER" || cur == "DOUBLE") && (nxt == "INTEGER" || nxt == "DOUBLE") {
			return "DOUBLE"
		}
	}
	if cur == "TIMESTAMP" && nxt == "DATE" {
		return "TIMESTAMP"
	}
	if cur == "DATE" && nxt == "TIMESTAMP" {
		return "TIMESTAMP"
	}
	return "VARCHAR"
}
