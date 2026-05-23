package validate

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flarebyte/quick-quack-quest/internal/config"
	"github.com/klauspost/compress/zstd"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func writeGzipFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	if _, err := gw.Write([]byte(content)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
}

func TestInspectNativeFileCSVJSONNDJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	csvPath := filepath.Join(dir, "sales.csv")
	writeFile(t, csvPath, "id,date,val\n1,2026-01-01,3.14\n2,2026-01-02,2\n")
	obs, rows, err := inspectNativeFile(config.Dataset{Format: "csv"}, csvPath, "none", 0)
	if err != nil {
		t.Fatalf("inspect csv: %v", err)
	}
	if rows != 2 || obs["id"] != "VARCHAR" || obs["date"] != "VARCHAR" || obs["val"] != "VARCHAR" {
		t.Fatalf("unexpected csv inspect rows=%d obs=%v", rows, obs)
	}

	jsonPath := filepath.Join(dir, "customers.json")
	arr := []map[string]any{{"id": 1.0, "name": "a"}, {"id": 2.0, "name": "b"}}
	b, _ := json.Marshal(arr)
	writeFile(t, jsonPath, string(b))
	obs, rows, err = inspectNativeFile(config.Dataset{Format: "json"}, jsonPath, "none", 1)
	if err != nil {
		t.Fatalf("inspect json: %v", err)
	}
	if rows != 1 || obs["id"] != "INTEGER" {
		t.Fatalf("unexpected json inspect rows=%d obs=%v", rows, obs)
	}

	ndjsonPath := filepath.Join(dir, "events.ndjson")
	writeFile(t, ndjsonPath, "{\"id\":1,\"kind\":\"click\"}\n{\"id\":2,\"kind\":\"view\"}\n")
	obs, rows, err = inspectNativeFile(config.Dataset{Format: "ndjson"}, ndjsonPath, "none", 0)
	if err != nil {
		t.Fatalf("inspect ndjson: %v", err)
	}
	if rows != 2 || obs["kind"] != "VARCHAR" {
		t.Fatalf("unexpected ndjson inspect rows=%d obs=%v", rows, obs)
	}
}

func TestDecompressReaderAndCodecErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plain := filepath.Join(dir, "a.txt")
	writeFile(t, plain, "hello")
	gz := filepath.Join(dir, "a.txt.gz")
	writeGzipFile(t, gz, "hello")

	f, _ := os.Open(plain)
	r, err := decompressReader(f, plain, "none")
	if err != nil {
		t.Fatalf("none codec: %v", err)
	}
	_ = r.Close()

	fgz, _ := os.Open(gz)
	r, err = decompressReader(fgz, gz, "auto")
	if err != nil {
		t.Fatalf("auto gzip codec: %v", err)
	}
	_ = r.Close()

	f2, _ := os.Open(plain)
	if _, err := decompressReader(f2, plain, "brotli"); err == nil {
		t.Fatalf("expected unsupported codec error")
	}
}

func TestValidateWithNativeSuccessAndErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "sales.csv")
	writeFile(t, csvPath, "id,date,val\n1,2026-01-01,3.14\n")

	spec := &config.Spec{}
	jsonPath := filepath.Join(dir, "sales.json")
	writeFile(t, jsonPath, `[{"id":1,"date":"2026-01-01","val":3.14}]`)
	d := config.Dataset{
		ID:          "sales",
		Format:      "json",
		Layout:      "single_file",
		Path:        jsonPath,
		Compression: "none",
		Fields: []config.Field{
			{Name: "id", Type: "INTEGER"},
			{Name: "date", Type: "DATE"},
			{Name: "val", Type: "DOUBLE"},
		},
	}
	start := time.Now()
	res, err := validateWithNative(spec, d, Options{}, DatasetResult{DatasetID: d.ID, Compression: "none"}, start)
	if err != nil || res.Status != "ok" || res.RowsChecked != 1 {
		t.Fatalf("expected success, res=%+v err=%v", res, err)
	}

	// schema mismatch path
	d.Fields[2].Type = "INTEGER"
	_, err = validateWithNative(spec, d, Options{}, DatasetResult{DatasetID: d.ID, Compression: "none"}, start)
	if err == nil {
		t.Fatalf("expected schema mismatch error")
	}
	expectErrorID(t, err, ErrIDSchemaTypeMismatch)

	// parquet compatibility path
	dParquet := d
	dParquet.Format = "parquet"
	_, err = validateWithNative(spec, dParquet, Options{}, DatasetResult{DatasetID: d.ID, Compression: "none"}, start)
	if err == nil {
		t.Fatalf("expected parquet unsupported error")
	}
	expectErrorID(t, err, ErrIDCompatibilityUnsupported)

	// codec unavailable path (bad zstd content with zstd codec)
	bad := filepath.Join(dir, "bad.zst")
	writeFile(t, bad, "not-zstd")
	dBad := d
	dBad.Path = bad
	dBad.Compression = "zstd"
	_, err = validateWithNative(spec, dBad, Options{}, DatasetResult{DatasetID: d.ID, Compression: "zstd"}, start)
	if err == nil {
		t.Fatalf("expected codec unavailable error")
	}
	if !strings.Contains(err.Error(), ErrIDDatasetReadFailed) {
		t.Fatalf("expected dataset read failed, got %v", err)
	}
}

func TestInspectNativeFileUnsupportedFormat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "x.bin")
	writeFile(t, p, "x")
	if _, _, err := inspectNativeFile(config.Dataset{Format: "parquet"}, p, "none", 0); err == nil {
		t.Fatalf("expected unsupported native format error")
	}
}

func TestInspectCSVAndNDJSONErrorBranches(t *testing.T) {
	t.Parallel()
	if _, _, err := inspectCSV(strings.NewReader(""), 0); err == nil {
		t.Fatalf("expected csv header read error")
	}
	if _, _, err := inspectNDJSON(strings.NewReader("not-json\n"), 0); err == nil {
		t.Fatalf("expected ndjson parse error")
	}
}

func TestDecompressReaderGzipErrorAndWrapCloserNil(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain.txt")
	writeFile(t, plain, "hello")
	f, err := os.Open(plain)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	if _, err := decompressReader(f, plain, "gzip"); err == nil {
		t.Fatalf("expected gzip reader error on plain file")
	}

	wc := wrapCloser{Reader: strings.NewReader("x"), closeFn: nil}
	if err := wc.Close(); err != nil {
		t.Fatalf("expected nil close for nil closeFn, got %v", err)
	}
}

func TestInspectNDJSONBlankLinesAndSampleLimit(t *testing.T) {
	t.Parallel()
	content := "\n{\"id\":1}\n\n{\"id\":2}\n"
	obs, rows, err := inspectNDJSON(strings.NewReader(content), 1)
	if err != nil {
		t.Fatalf("inspectNDJSON error: %v", err)
	}
	if rows != 1 || obs["id"] == "" {
		t.Fatalf("expected one sampled row with id type, rows=%d obs=%v", rows, obs)
	}
}

func TestDecompressReaderZstdSuccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "a.zst")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	zw, err := zstd.NewWriter(f)
	if err != nil {
		t.Fatalf("zstd writer: %v", err)
	}
	if _, err := zw.Write([]byte("hello")); err != nil {
		t.Fatalf("zstd write: %v", err)
	}
	zw.Close()
	f.Close()

	rf, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer rf.Close()
	r, err := decompressReader(rf, path, "auto")
	if err != nil {
		t.Fatalf("decompressReader auto zstd: %v", err)
	}
	buf, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read zstd: %v", err)
	}
	_ = r.Close()
	if string(buf) != "hello" {
		t.Fatalf("unexpected zstd payload: %s", string(buf))
	}
}
