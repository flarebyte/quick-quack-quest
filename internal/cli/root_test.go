// purpose: Validate CLI command behavior and output contracts from an end-user perspective.
// responsibilities: Execute representative commands and assert success payloads, errors, and precedence rules.
// architecture notes: Tests are command-centric on purpose to catch wiring regressions across subcommands and flags.
package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/flarebyte/quick-quack-quest/internal/config"
	"github.com/flarebyte/quick-quack-quest/internal/validate"
	"github.com/spf13/cobra"
)

const testConfigPath = "../../doc/design-meta/examples/config/cli-config.cue"

func TestVersionJSONOutput(t *testing.T) {
	t.Parallel()

	Version = "0.1.0"
	Commit = "abc1234"
	BuiltAt = "2026-05-19T09:00:00Z"

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"version", "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["name"] != "quick-quack-quest" {
		t.Fatalf("name mismatch: %q", got["name"])
	}
	if got["version"] != "0.1.0" {
		t.Fatalf("version mismatch: %q", got["version"])
	}
}

func TestDatasetListJSONOutput(t *testing.T) {
	t.Parallel()

	cmd, out, _ := newTestCommand(false, "dataset", "list", "--format", "json", "--config", testConfigPath)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, out.String())
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 datasets, got %d", len(rows))
	}
	if rows[0]["dataset_id"] != "customers_master" {
		t.Fatalf("expected sorted first dataset customers_master, got %v", rows[0]["dataset_id"])
	}
	if rows[1]["dataset_id"] != "events_stream" {
		t.Fatalf("expected sorted second dataset events_stream, got %v", rows[1]["dataset_id"])
	}
	if rows[2]["dataset_id"] != "sales_daily" {
		t.Fatalf("expected sorted third dataset sales_daily, got %v", rows[2]["dataset_id"])
	}
}

func TestDatasetListTextOutput(t *testing.T) {
	t.Parallel()

	cmd, out, _ := newTestCommand(false, "dataset", "list", "--format", "text", "--config", testConfigPath)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "DATASET_ID") {
		t.Fatalf("expected header in output, got: %s", s)
	}
	if !strings.Contains(s, "customers_master") {
		t.Fatalf("expected customers_master in output, got: %s", s)
	}
	if !strings.Contains(s, "events_stream") {
		t.Fatalf("expected events_stream in output, got: %s", s)
	}
	if !strings.Contains(s, "sales_daily") {
		t.Fatalf("expected sales_daily in output, got: %s", s)
	}
}

func TestDatasetValidateJSONOutputSuccess(t *testing.T) {
	t.Parallel()
	runDatasetJSONStatusTest(t, []string{
		"dataset", "validate", "customers_master",
		"--format", "json",
		"--config", testConfigPath,
	}, map[string]any{
		"status":                "ok",
		"output_schema_version": "v1",
		"dataset_id":            "customers_master",
	})
}

func TestDatasetValidateDatasetNotFound(t *testing.T) {
	t.Parallel()

	cmd, out, _ := newTestCommand(false,
		"dataset", "validate", "not_a_dataset",
		"--format", "json",
		"--config", testConfigPath,
	)

	expectCommandErrorContains(t, cmd.Execute(), "QQQ_DATASET_NOT_FOUND", out.String())
}

func TestDatasetValidateUnsupportedEngine(t *testing.T) {
	t.Parallel()

	cmd, out, _ := newTestCommand(false,
		"dataset", "validate", "customers_master",
		"--validation-engine", "bogus",
		"--format", "json",
		"--config", testConfigPath,
	)

	expectCommandErrorContains(t, cmd.Execute(), "QQQ_VALIDATION_ENGINE_UNSUPPORTED", out.String())
}

func TestDatasetValidateNativeJSONOutputSuccess(t *testing.T) {
	t.Parallel()
	runDatasetJSONStatusTest(t, []string{
		"dataset", "validate", "customers_master",
		"--validation-engine", "native",
		"--format", "json",
		"--config", testConfigPath,
	}, map[string]any{
		"status": "ok",
	})
}

func TestDatasetInspectDuckDBJSONOutputSuccess(t *testing.T) {
	t.Parallel()
	runDatasetJSONStatusTest(t, []string{
		"dataset", "inspect", "customers_master",
		"--format", "json",
		"--sample-size", "10",
		"--config", testConfigPath,
	}, map[string]any{
		"status":                "ok",
		"output_schema_version": "v1",
	})
}

func TestDatasetInspectNativeGzipJSONOutputSuccess(t *testing.T) {
	t.Parallel()
	runDatasetJSONStatusTest(t, []string{
		"dataset", "inspect", "events_stream",
		"--validation-engine", "native",
		"--format", "json",
		"--sample-size", "5",
		"--config", testConfigPath,
	}, map[string]any{
		"status":      "ok",
		"compression": "gzip",
	})
}

func TestQueryListJSONOutput(t *testing.T) {
	t.Parallel()

	cmd, out, _ := newTestCommand(false, "query", "list", "--format", "json", "--config", testConfigPath)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, out.String())
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 queries, got %d", len(rows))
	}
	if rows[0]["query_id"] != "customer_360" {
		t.Fatalf("expected sorted first query customer_360, got %v", rows[0]["query_id"])
	}
}

func TestQueryExplainJSONOutput(t *testing.T) {
	t.Parallel()

	cmd, out, _ := newTestCommand(false, "query", "explain", "sales_by_country", "--format", "json", "--config", testConfigPath)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\noutput=%s", err, out.String())
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, out.String())
	}
	if got["query_id"] != "sales_by_country" {
		t.Fatalf("expected sales_by_country, got %v", got["query_id"])
	}
	if got["output_schema_version"] != "v1" {
		t.Fatalf("expected output schema version v1, got %v", got["output_schema_version"])
	}
}

func TestQueryExplainUnknownQuery(t *testing.T) {
	t.Parallel()

	cmd, _, _ := newTestCommand(false, "query", "explain", "missing_query", "--format", "json", "--config", testConfigPath)

	expectCommandErrorContains(t, cmd.Execute(), "QQQ_QUERY_NOT_FOUND", "")
}

func TestQueryExplainMissingRequiredParam(t *testing.T) {
	t.Parallel()

	cmd, _, _ := newTestCommand(false,
		"query", "explain", "sales_by_country",
		"--format", "json",
		"--config", testConfigPath,
		"--param", "start_date=2026-01-01",
	)

	expectCommandErrorContains(t, cmd.Execute(), "QQQ_QUERY_PARAM_REQUIRED_MISSING", "")
}

func TestQueryExplainInvalidParamFormat(t *testing.T) {
	t.Parallel()

	cmd, _, _ := newTestCommand(false,
		"query", "explain", "sales_by_country",
		"--format", "json",
		"--config", testConfigPath,
		"--param", "badparam",
	)

	expectCommandErrorContains(t, cmd.Execute(), "QQQ_QUERY_PARAM_INVALID", "")
}

func TestQueryRunJSONLSuccess(t *testing.T) {
	t.Parallel()

	cmd, out, errOut := newTestCommand(true,
		"query", "run", "sales_by_country",
		"--format", "jsonl",
		"--stream",
		"--config", testConfigPath,
		"--param", "start_date=2026-01-01",
		"--param", "end_date=2026-01-31",
	)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstdout=%s\nstderr=%s", err, out.String(), errOut.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) == 0 {
		t.Fatalf("expected non-empty jsonl output")
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
		t.Fatalf("invalid jsonl line: %v line=%s", err, lines[0])
	}
	if _, ok := row["country"]; !ok {
		t.Fatalf("expected country key in row: %v", row)
	}
	if !strings.Contains(errOut.String(), "\"query_id\": \"sales_by_country\"") {
		t.Fatalf("expected summary on stderr, got: %s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "\"output_schema_version\": \"v1\"") {
		t.Fatalf("expected schema version in summary, got: %s", errOut.String())
	}
}

func TestQueryRunLimitExceedsMaxRows(t *testing.T) {
	t.Parallel()

	cmd, _, _ := newTestCommand(false,
		"query", "run", "sales_by_country",
		"--format", "json",
		"--config", testConfigPath,
		"--param", "start_date=2026-01-01",
		"--param", "end_date=2026-01-31",
		"--limit", "5",
		"--max-rows", "2",
	)

	expectCommandErrorContains(t, cmd.Execute(), "QQQ_QUERY_LIMIT_EXCEEDS_MAX_ROWS", "")
}

func TestQueryRunEnvPrecedenceForLimit(t *testing.T) {
	cmd, _, errOut := newTestCommand(true,
		"query", "run", "sales_by_country",
		"--format", "jsonl",
		"--config", testConfigPath,
		"--param", "start_date=2026-01-01",
		"--param", "end_date=2026-01-31",
	)
	t.Setenv("QQQ_QUERY_LIMIT", "1")
	t.Setenv("QQQ_MAX_ROWS", "10")

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(errOut.String(), "\"limit\": 1") {
		t.Fatalf("expected env limit in summary, got: %s", errOut.String())
	}
}

func newTestCommand(separateErr bool, args ...string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	if separateErr {
		cmd.SetErr(&errOut)
	} else {
		cmd.SetErr(&out)
	}
	cmd.SetArgs(args)
	return cmd, &out, &errOut
}

func expectCommandErrorContains(t *testing.T, err error, want, output string) {
	t.Helper()
	if err == nil {
		if output != "" {
			t.Fatalf("expected error, got nil\noutput=%s", output)
		}
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected %s, got %v", want, err)
	}
}

func runDatasetJSONStatusTest(t *testing.T, args []string, expected map[string]any) {
	t.Helper()
	cmd, out, _ := newTestCommand(false, args...)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\noutput=%s", err, out.String())
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, out.String())
	}
	assertJSONFields(t, got, expected)
}

func assertJSONFields(t *testing.T, got map[string]any, expected map[string]any) {
	t.Helper()
	for k, v := range expected {
		if got[k] != v {
			t.Fatalf("expected %s=%v, got %v", k, v, got[k])
		}
	}
}

func TestSourceExprForRun(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"csv":     "read_csv_auto(?)",
		"json":    "read_json_auto(?)",
		"ndjson":  "read_json_auto(?, format='newline_delimited')",
		"parquet": "read_parquet(?)",
		"unknown": "read_csv_auto(?)",
	}
	for format, want := range cases {
		if got := sourceExprForRun(format); got != want {
			t.Fatalf("format=%s want=%s got=%s", format, want, got)
		}
	}
}

func TestCoerceParam(t *testing.T) {
	t.Parallel()

	if v, err := coerceParam("INTEGER", "42"); err != nil || v.(int64) != 42 {
		t.Fatalf("expected int64 42, got %v err=%v", v, err)
	}
	if v, err := coerceParam("DOUBLE", "3.5"); err != nil || v.(float64) != 3.5 {
		t.Fatalf("expected float64 3.5, got %v err=%v", v, err)
	}
	if v, err := coerceParam("VARCHAR", "abc"); err != nil || v.(string) != "abc" {
		t.Fatalf("expected string abc, got %v err=%v", v, err)
	}
	if _, err := coerceParam("INTEGER", "not-an-int"); err == nil {
		t.Fatalf("expected integer coercion error")
	}
}

func TestParseParams(t *testing.T) {
	t.Parallel()

	got, err := parseParams([]string{"a=1", " b = two "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["a"] != "1" || got["b"] != "two" {
		t.Fatalf("unexpected parsed map: %#v", got)
	}
	if _, err := parseParams([]string{"badparam"}); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestWriteOutput(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	if err := writeOutput(&out, "text", map[string]any{"x": 1}, "hello"); err != nil {
		t.Fatalf("text write failed: %v", err)
	}
	if !strings.Contains(out.String(), "hello") {
		t.Fatalf("expected text output, got %q", out.String())
	}

	out.Reset()
	if err := writeOutput(&out, "json", map[string]any{"x": 1}, "ignored"); err != nil {
		t.Fatalf("json write failed: %v", err)
	}
	if !strings.Contains(out.String(), "\"x\": 1") {
		t.Fatalf("expected json output, got %q", out.String())
	}

	if err := writeOutput(&out, "xml", nil, ""); err == nil {
		t.Fatalf("expected unsupported format error")
	}
}

func TestWriteQueryTable(t *testing.T) {
	t.Parallel()

	rows := []queryListRow{{
		QueryID:          "sales_by_country",
		RequiredDatasets: "sales_daily,customers_master",
		ParameterCount:   2,
	}}
	var out bytes.Buffer
	if err := writeQueryTable(&out, rows); err != nil {
		t.Fatalf("writeQueryTable failed: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "QUERY_ID") || !strings.Contains(s, "sales_by_country") {
		t.Fatalf("unexpected table output: %s", s)
	}
}

func TestQueryRunErrorContainsContext(t *testing.T) {
	t.Parallel()

	cmd, _, _ := newTestCommand(true,
		"query", "run", "sales_by_country",
		"--format", "json",
		"--config", testConfigPath,
		"--param", "start_date=2026-01-01",
		"--param", "end_date=2026-01-31",
		"--output", "/definitely/not/writable/out.json",
	)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error")
	}
	s := err.Error()
	if !strings.Contains(s, "query run failed:") || !strings.Contains(s, "stage=create_output") || !strings.Contains(s, "query_id=sales_by_country") {
		t.Fatalf("expected contextual query run error, got: %s", s)
	}
}

func TestEmittersAndScanRowMap(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	for _, tc := range []struct {
		name  string
		emit  func(*sql.Rows, []string, int, io.Writer) (runResult, error)
		check func(string) bool
	}{
		{name: "jsonl", emit: emitJSONL, check: func(s string) bool { return strings.Contains(s, "\"country\":\"uk\"") }},
		{name: "csv", emit: emitCSV, check: func(s string) bool { return strings.Contains(s, "id,country") && strings.Contains(s, "1,uk") }},
		{name: "json", emit: emitJSON, check: func(s string) bool { return strings.Contains(s, "\"country\": \"uk\"") }},
		{name: "table", emit: emitTable, check: func(s string) bool { return strings.Contains(s, "country") && strings.Contains(s, "uk") }},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rows, qErr := db.Query("SELECT 1 AS id, 'uk' AS country")
			if qErr != nil {
				t.Fatalf("query: %v", qErr)
			}
			defer rows.Close()
			cols, cErr := rows.Columns()
			if cErr != nil {
				t.Fatalf("columns: %v", cErr)
			}
			var out bytes.Buffer
			res, eErr := tc.emit(rows, cols, 10, &out)
			if eErr != nil {
				t.Fatalf("emit error: %v", eErr)
			}
			if res.rowsEmitted != 1 {
				t.Fatalf("expected 1 emitted row, got %d", res.rowsEmitted)
			}
			if !tc.check(out.String()) {
				t.Fatalf("unexpected output: %s", out.String())
			}
		})
	}

	rows, err := db.Query("SELECT 1 AS id, 'uk' AS country")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	if !rows.Next() {
		t.Fatalf("expected one row")
	}
	m, err := scanRowMap(rows, cols)
	if err != nil {
		t.Fatalf("scanRowMap: %v", err)
	}
	if m["country"] != "uk" {
		t.Fatalf("expected country=uk, got %v", m["country"])
	}
}

func TestRenderHelpers(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	cfgErr := &config.ConfigError{ID: config.ErrIDConfigInvalid, Message: "bad config"}
	err := renderConfigError(cmd, cfgErr)
	if err == nil || !strings.Contains(err.Error(), config.ErrIDConfigInvalid) {
		t.Fatalf("expected config id error, got %v", err)
	}
	if !strings.Contains(errOut.String(), "bad config") {
		t.Fatalf("expected stderr details, got %q", errOut.String())
	}

	out.Reset()
	vr := validate.DatasetResult{DatasetID: "sales_daily", Status: "ok", FilesScanned: 1, RowsChecked: 3}
	if err := renderValidateResult(cmd, "json", vr, nil); err != nil {
		t.Fatalf("renderValidateResult json: %v", err)
	}
	if !strings.Contains(out.String(), "\"dataset_id\": \"sales_daily\"") {
		t.Fatalf("unexpected validate json: %s", out.String())
	}

	out.Reset()
	results := []validate.DatasetResult{{DatasetID: "sales_daily", Status: "ok"}, {DatasetID: "events_stream", Status: "error", ErrorID: "E1"}}
	if err := renderValidateAllResult(cmd, "text", results, nil); err != nil {
		t.Fatalf("renderValidateAllResult text: %v", err)
	}
	if !strings.Contains(out.String(), "dataset=sales_daily") || !strings.Contains(out.String(), "dataset=events_stream") {
		t.Fatalf("unexpected validate-all text: %s", out.String())
	}
}

type failWriter struct{}

func (failWriter) Write(_ []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestRenderHelpersBranches(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	// generic non-ConfigError path
	err := renderConfigError(cmd, io.ErrUnexpectedEOF)
	if err == nil || !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("expected generic config error, got %v", err)
	}

	// renderValidateResult text and unsupported format branches
	vr := validate.DatasetResult{DatasetID: "d1", Status: "ok", FilesScanned: 1, RowsChecked: 2}
	out.Reset()
	if err := renderValidateResult(cmd, "text", vr, nil); err != nil {
		t.Fatalf("renderValidateResult text: %v", err)
	}
	if !strings.Contains(out.String(), "dataset=d1") {
		t.Fatalf("unexpected text output: %s", out.String())
	}
	if err := renderValidateResult(cmd, "yaml", vr, nil); err == nil {
		t.Fatalf("expected unsupported format error")
	}

	// renderValidateAllResult json and unsupported branches
	results := []validate.DatasetResult{{DatasetID: "d1", Status: "ok"}}
	out.Reset()
	if err := renderValidateAllResult(cmd, "json", results, nil); err != nil {
		t.Fatalf("renderValidateAllResult json: %v", err)
	}
	if !strings.Contains(out.String(), "\"dataset_id\": \"d1\"") {
		t.Fatalf("unexpected json output: %s", out.String())
	}
	if err := renderValidateAllResult(cmd, "yaml", results, nil); err == nil {
		t.Fatalf("expected unsupported format error")
	}
}

func TestWriteJSONAndOutputErrorPaths(t *testing.T) {
	t.Parallel()

	if err := writeJSON(failWriter{}, map[string]any{"k": "v"}); err == nil {
		t.Fatalf("expected writeJSON writer error")
	}
	if err := writeOutput(failWriter{}, "text", nil, "x"); err == nil {
		t.Fatalf("expected writeOutput text writer error")
	}
}

func TestEnvAndChooserHelpers(t *testing.T) {

	t.Setenv("QQQ_INT_OK", "12")
	t.Setenv("QQQ_INT_BAD", "x")
	t.Setenv("QQQ_BOOL_TRUE", " true ")
	t.Setenv("QQQ_BOOL_FALSE", "0")

	if got := envInt("QQQ_INT_OK"); got != 12 {
		t.Fatalf("expected 12, got %d", got)
	}
	if got := envInt("QQQ_INT_BAD"); got != 0 {
		t.Fatalf("expected 0 for bad int, got %d", got)
	}
	if !envBool("QQQ_BOOL_TRUE") {
		t.Fatalf("expected true bool")
	}
	if envBool("QQQ_BOOL_FALSE") {
		t.Fatalf("expected false bool")
	}

	if !chooseBool(true, false, false) {
		t.Fatalf("flag true should win")
	}
	if !chooseBool(false, false, true) {
		t.Fatalf("cfg true should apply when flag/env are false")
	}
	if !chooseBool(false, false, false) {
		// this line intentionally checks function returns cfg(false); should be false
	}
	if chooseBool(false, false, false) {
		t.Fatalf("expected all false")
	}
}

func TestUnsupportedFormatBranches(t *testing.T) {
	t.Parallel()

	cmd, _, _ := newTestCommand(false,
		"dataset", "list",
		"--format", "xml",
		"--config", testConfigPath,
	)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("expected dataset list unsupported format error, got %v", err)
	}

	cmd2, _, _ := newTestCommand(false,
		"query", "run", "sales_by_country",
		"--format", "xml",
		"--config", testConfigPath,
		"--param", "start_date=2026-01-01",
		"--param", "end_date=2026-01-31",
	)
	if err := cmd2.Execute(); err == nil || !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("expected query run unsupported format error, got %v", err)
	}
}

func TestWrapQueryRunErrorNilAndFields(t *testing.T) {
	t.Parallel()
	if err := wrapQueryRunError("q", "s", "json", 1, 2, 3, "", nil); err != nil {
		t.Fatalf("expected nil cause to return nil")
	}
	err := wrapQueryRunError("q1", "execute", "json", 10, 20, 30, "", io.ErrClosedPipe)
	if err == nil {
		t.Fatalf("expected wrapped error")
	}
	s := err.Error()
	if !strings.Contains(s, "query_id=q1") || !strings.Contains(s, "output=stdout") || !strings.Contains(s, "stage=execute") {
		t.Fatalf("unexpected wrapped error: %s", s)
	}
}

func TestRunQueryUnsupportedFormatBranch(t *testing.T) {
	t.Parallel()
	spec := &config.Spec{}
	q := config.Query{ID: "q1", SQL: "SELECT 1 AS n", RequiredDatasets: nil}
	_, err := runQuery(spec, q, map[string]string{}, runOptions{format: "xml"})
	if err == nil || !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
}

func TestDatasetValidateAllFailFast(t *testing.T) {
	t.Parallel()
	cmd, _, _ := newTestCommand(false,
		"dataset", "validate-all",
		"--config", testConfigPath,
		"--validation-engine", "native",
		"--compression", "brotli",
		"--fail-fast",
		"--format", "json",
	)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected fail-fast validation error")
	}
}

func TestDatasetInspectUnsupportedFormat(t *testing.T) {
	t.Parallel()
	cmd, _, _ := newTestCommand(false,
		"dataset", "inspect", "customers_master",
		"--config", testConfigPath,
		"--format", "xml",
	)
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
}

func TestConfigValidateCommandFormatsAndErrors(t *testing.T) {
	t.Parallel()

	cmd, out, _ := newTestCommand(false,
		"config", "validate",
		"--config", testConfigPath,
		"--format", "json",
	)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected config validate json success, got %v", err)
	}
	if !strings.Contains(out.String(), `"status": "ok"`) {
		t.Fatalf("expected json ok output, got %s", out.String())
	}

	cmd2, out2, _ := newTestCommand(false,
		"config", "validate",
		"--config", testConfigPath,
		"--format", "text",
	)
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("expected config validate text success, got %v", err)
	}
	if !strings.Contains(out2.String(), "config is valid") {
		t.Fatalf("expected text ok output, got %s", out2.String())
	}

	cmd3, _, _ := newTestCommand(false,
		"config", "validate",
		"--config", "/definitely/missing/cli-config.cue",
	)
	err := cmd3.Execute()
	if err == nil || (!strings.Contains(err.Error(), config.ErrIDConfigLoad) && !strings.Contains(err.Error(), config.ErrIDConfigInvalid)) {
		t.Fatalf("expected config load id error, got %v", err)
	}
}

func TestIsTTYWithFileBackedStderr(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "stderr-*")
	if err != nil {
		t.Fatalf("create temp stderr: %v", err)
	}
	defer f.Close()

	orig := os.Stderr
	os.Stderr = f
	t.Cleanup(func() { os.Stderr = orig })

	if isTTY(0) {
		t.Fatalf("expected file-backed stderr to not be a tty")
	}
}

func TestChooseBoolEnvBranch(t *testing.T) {
	t.Parallel()
	if !chooseBool(false, true, false) {
		t.Fatalf("expected env true to win when flag is false")
	}
}

func TestFindDatasetByIDBranches(t *testing.T) {
	t.Parallel()
	spec := &config.Spec{Datasets: []config.Dataset{{ID: "d1"}}}
	if _, ok := findDatasetByID(spec, "d1"); !ok {
		t.Fatalf("expected dataset present")
	}
	if _, ok := findDatasetByID(spec, "d2"); ok {
		t.Fatalf("expected dataset absent")
	}
}

func TestBindQueryBranches(t *testing.T) {
	t.Parallel()

	q := config.Query{
		ID:         "q1",
		Parameters: []config.QueryParameter{{Name: "a", Type: "INTEGER", Required: true}, {Name: "b", Type: "VARCHAR", Required: false}},
	}
	if _, _, err := bindQuery("SELECT $a", q, map[string]string{}, 0); err == nil {
		t.Fatalf("expected missing required param error")
	}

	sqlText, args, err := bindQuery("SELECT $a AS x, $a AS y, 1", q, map[string]string{"a": "7", "b": "x"}, 3)
	if err != nil {
		t.Fatalf("bindQuery error: %v", err)
	}
	if strings.Count(sqlText, "?") != 2 || !strings.Contains(sqlText, "LIMIT 3") {
		t.Fatalf("unexpected sql text: %s", sqlText)
	}
	if len(args) != 2 {
		t.Fatalf("expected two bound args for repeated token, got %d", len(args))
	}
}

func TestEmitMaxRowsExceededBranches(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	for _, tc := range []struct {
		name string
		emit func(*sql.Rows, []string, int, io.Writer) (runResult, error)
	}{
		{name: "jsonl", emit: emitJSONL},
		{name: "csv", emit: emitCSV},
		{name: "json", emit: emitJSON},
		{name: "table", emit: emitTable},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rows, qErr := db.Query("SELECT 1 AS id UNION ALL SELECT 2 AS id")
			if qErr != nil {
				t.Fatalf("query: %v", qErr)
			}
			defer rows.Close()
			cols, cErr := rows.Columns()
			if cErr != nil {
				t.Fatalf("columns: %v", cErr)
			}
			_, err := tc.emit(rows, cols, 1, io.Discard)
			if err == nil || !strings.Contains(err.Error(), "QQQ_MAX_ROWS_EXCEEDED") {
				t.Fatalf("expected max rows error, got %v", err)
			}
		})
	}
}

func TestRunQueryEarlyErrorPaths(t *testing.T) {
	t.Parallel()

	specMissing := &config.Spec{}
	qMissing := config.Query{ID: "q", SQL: "SELECT 1", RequiredDatasets: []string{"d1"}}
	_, err := runQuery(specMissing, qMissing, map[string]string{}, runOptions{format: "json", out: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "QQQ_DATASET_NOT_FOUND") {
		t.Fatalf("expected dataset not found, got %v", err)
	}

	specInvalid := &config.Spec{Datasets: []config.Dataset{{ID: "bad-id", Format: "csv", Layout: "single_file", Path: "x.csv"}}}
	qInvalid := config.Query{ID: "q", SQL: "SELECT * FROM bad-id", RequiredDatasets: []string{"bad-id"}}
	_, err = runQuery(specInvalid, qInvalid, map[string]string{}, runOptions{format: "json", out: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "QQQ_DATASET_ID_INVALID") {
		t.Fatalf("expected invalid dataset id, got %v", err)
	}
}

func TestQueryListAndExplainTextOutput(t *testing.T) {
	t.Parallel()

	cmd, out, _ := newTestCommand(false, "query", "list", "--format", "text", "--config", testConfigPath)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("query list text: %v", err)
	}
	if !strings.Contains(out.String(), "QUERY_ID") || !strings.Contains(out.String(), "sales_by_country") {
		t.Fatalf("unexpected query list text output: %s", out.String())
	}

	cmd2, out2, _ := newTestCommand(false,
		"query", "explain", "sales_by_country",
		"--format", "text",
		"--config", testConfigPath,
		"--param", "start_date=2026-01-01",
		"--param", "end_date=2026-01-31",
	)
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("query explain text: %v", err)
	}
	if !strings.Contains(out2.String(), "query=sales_by_country") || !strings.Contains(out2.String(), "SELECT") {
		t.Fatalf("unexpected query explain text output: %s", out2.String())
	}
}

func TestQueryRunTableOutputBranch(t *testing.T) {
	t.Parallel()

	cmd, out, errOut := newTestCommand(true,
		"query", "run", "sales_by_country",
		"--format", "table",
		"--config", testConfigPath,
		"--param", "start_date=2026-01-01",
		"--param", "end_date=2026-01-31",
	)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("query run table: %v\nstdout=%s\nstderr=%s", err, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "country") {
		t.Fatalf("expected table output, got: %s", out.String())
	}
	if !strings.Contains(errOut.String(), "\"status\": \"ok\"") {
		t.Fatalf("expected summary on stderr, got: %s", errOut.String())
	}
}

func TestDatasetValidateAllJSONAndInspectTextBranches(t *testing.T) {
	t.Parallel()

	cmd, out, _ := newTestCommand(false,
		"dataset", "validate-all",
		"--format", "json",
		"--config", testConfigPath,
	)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected validate-all to return non-zero when at least one dataset fails")
	}
	if !strings.Contains(out.String(), "\"dataset_id\": \"customers_master\"") {
		t.Fatalf("unexpected validate-all json output: %s", out.String())
	}

	cmd2, out2, _ := newTestCommand(false,
		"dataset", "inspect", "customers_master",
		"--format", "text",
		"--config", testConfigPath,
	)
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("inspect text: %v", err)
	}
	if !strings.Contains(out2.String(), "dataset=customers_master") || !strings.Contains(out2.String(), "status=ok") {
		t.Fatalf("unexpected inspect text output: %s", out2.String())
	}
}

func TestRenderValidateWriterErrorBranches(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	cmd.SetOut(failWriter{})
	cmd.SetErr(io.Discard)

	r := validate.DatasetResult{DatasetID: "d", Status: "ok"}
	if err := renderValidateResult(cmd, "json", r, nil); err == nil {
		t.Fatalf("expected renderValidateResult writer error")
	}
	if err := renderValidateAllResult(cmd, "json", []validate.DatasetResult{r}, nil); err == nil {
		t.Fatalf("expected renderValidateAllResult writer error")
	}
}

func TestDatasetInspectJSONErrorBranch(t *testing.T) {
	t.Parallel()

	cmd, out, _ := newTestCommand(false,
		"dataset", "inspect", "missing_dataset",
		"--format", "json",
		"--config", testConfigPath,
	)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected inspect error")
	}
	if !strings.Contains(out.String(), "QQQ_DATASET_NOT_FOUND") {
		t.Fatalf("expected inspect missing dataset error id, got: %s", out.String())
	}
}

func TestVersionTextOutput(t *testing.T) {
	t.Parallel()
	cmd, out, _ := newTestCommand(false, "version", "--format", "text")
	if err := cmd.Execute(); err != nil {
		t.Fatalf("version text execute: %v", err)
	}
	if !strings.Contains(out.String(), "quick-quack-quest") {
		t.Fatalf("unexpected version text output: %s", out.String())
	}
}

func TestListCommandsConfigErrorBranches(t *testing.T) {
	t.Parallel()

	cmd, _, _ := newTestCommand(false,
		"dataset", "list",
		"--config", "/definitely/missing/cli-config.cue",
	)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected dataset list config error")
	}

	cmd2, _, _ := newTestCommand(false,
		"query", "list",
		"--config", "/definitely/missing/cli-config.cue",
	)
	if err := cmd2.Execute(); err == nil {
		t.Fatalf("expected query list config error")
	}
}

func TestConfigValidateUnsupportedFormat(t *testing.T) {
	t.Parallel()
	cmd, _, _ := newTestCommand(false,
		"config", "validate",
		"--config", testConfigPath,
		"--format", "xml",
	)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
}

func TestQueryRunCommandBranches(t *testing.T) {
	t.Parallel()

	cmd, _, _ := newTestCommand(false,
		"query", "run", "missing_query",
		"--config", testConfigPath,
	)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "QQQ_QUERY_NOT_FOUND") {
		t.Fatalf("expected missing query error, got %v", err)
	}

	cmd2, _, _ := newTestCommand(false,
		"query", "run", "sales_by_country",
		"--config", testConfigPath,
		"--param", "start_date=2026-01-01",
	)
	if err := cmd2.Execute(); err == nil || !strings.Contains(err.Error(), "QQQ_QUERY_PARAM_REQUIRED_MISSING") {
		t.Fatalf("expected missing required param error, got %v", err)
	}
}

func TestRunQueryFormatsAndQueryError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(csvPath, []byte("id,country\n1,uk\n2,fr\n"), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	spec := &config.Spec{Datasets: []config.Dataset{{ID: "d", Format: "csv", Layout: "single_file", Path: csvPath, Compression: "none"}}}
	q := config.Query{ID: "q", SQL: "SELECT * FROM d", RequiredDatasets: []string{"d"}}

	for _, format := range []string{"jsonl", "csv", "json", "table"} {
		var out bytes.Buffer
		res, err := runQuery(spec, q, map[string]string{}, runOptions{format: format, timeout: 1, out: &out})
		if err != nil {
			t.Fatalf("runQuery format=%s err=%v", format, err)
		}
		if res.rowsEmitted != 2 {
			t.Fatalf("format=%s expected 2 rows, got %d", format, res.rowsEmitted)
		}
	}

	_, err := runQuery(spec, config.Query{ID: "bad", SQL: "SELECT * FROM missing_table", RequiredDatasets: nil}, map[string]string{}, runOptions{format: "json", out: io.Discard})
	if err == nil {
		t.Fatalf("expected query execution error")
	}
}

func TestEmittersScanErrorBranches(t *testing.T) {
	t.Parallel()
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	for _, tc := range []struct {
		name string
		emit func(*sql.Rows, []string, int, io.Writer) (runResult, error)
	}{
		{name: "jsonl", emit: emitJSONL},
		{name: "json", emit: emitJSON},
		{name: "table", emit: emitTable},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rows, qErr := db.Query("SELECT 1 AS id")
			if qErr != nil {
				t.Fatalf("query: %v", qErr)
			}
			defer rows.Close()
			_, err := tc.emit(rows, []string{"id", "extra"}, 0, io.Discard)
			if err == nil {
				t.Fatalf("expected scan error")
			}
		})
	}

	rows, err := db.Query("SELECT 1 AS id")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatalf("expected row")
	}
	if _, err := scanRowMap(rows, []string{"id", "extra"}); err == nil {
		t.Fatalf("expected scanRowMap error")
	}
}

type fakeFileInfo struct{ mode os.FileMode }

func (f fakeFileInfo) Name() string       { return "fake" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

func TestIsTTYBranchesWithStub(t *testing.T) {
	t.Parallel()
	orig := stderrStat
	defer func() { stderrStat = orig }()

	stderrStat = func() (os.FileInfo, error) { return nil, io.ErrUnexpectedEOF }
	if isTTY(0) {
		t.Fatalf("expected false on stat error")
	}

	stderrStat = func() (os.FileInfo, error) { return fakeFileInfo{mode: os.ModeCharDevice}, nil }
	if !isTTY(0) {
		t.Fatalf("expected true on char device")
	}
}

func TestConfigValidateWriteOutputErrorBranch(t *testing.T) {
	t.Parallel()
	cmd := newConfigValidateCommand()
	cmd.SetOut(failWriter{})
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--config", testConfigPath, "--format", "json"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected output writer error")
	}
}

func TestRunQueryBindErrorBranch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(csvPath, []byte("id\n1\n"), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	spec := &config.Spec{Datasets: []config.Dataset{{ID: "d", Format: "csv", Layout: "single_file", Path: csvPath, Compression: "none"}}}
	q := config.Query{ID: "q", SQL: "SELECT * FROM d WHERE id > $min_id", RequiredDatasets: []string{"d"}, Parameters: []config.QueryParameter{{Name: "min_id", Type: "INTEGER", Required: true}}}
	_, err := runQuery(spec, q, map[string]string{}, runOptions{format: "json", out: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "QQQ_QUERY_PARAM_REQUIRED_MISSING") {
		t.Fatalf("expected bind required param error, got %v", err)
	}
}

func TestEmitJSONAndTableWriterErrorBranches(t *testing.T) {
	t.Parallel()
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT 1 AS id")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	cols, _ := rows.Columns()
	if _, err := emitJSON(rows, cols, 0, failWriter{}); err == nil {
		t.Fatalf("expected emitJSON writer error")
	}
	rows.Close()

	rows2, err := db.Query("SELECT 1 AS id")
	if err != nil {
		t.Fatalf("query2: %v", err)
	}
	cols2, _ := rows2.Columns()
	if _, err := emitTable(rows2, cols2, 0, failWriter{}); err == nil {
		t.Fatalf("expected emitTable writer/flush error")
	}
	rows2.Close()
}

func TestDatasetInspectCommandJSONSuccessBranch(t *testing.T) {
	t.Parallel()
	cmd, out, _ := newTestCommand(false,
		"dataset", "inspect", "customers_master",
		"--format", "json",
		"--config", testConfigPath,
	)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("inspect json execute: %v", err)
	}
	if !strings.Contains(out.String(), "\"status\": \"ok\"") {
		t.Fatalf("expected inspect json success, got: %s", out.String())
	}
}

func TestRenderValidateResultErrPropagation(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	want := io.ErrClosedPipe
	if err := renderValidateResult(cmd, "text", validate.DatasetResult{DatasetID: "d", Status: "error"}, want); err != want {
		t.Fatalf("expected propagated error %v, got %v", want, err)
	}
}

func TestDatasetInspectConfigErrorBranch(t *testing.T) {
	t.Parallel()
	cmd, _, _ := newTestCommand(false,
		"dataset", "inspect", "customers_master",
		"--config", "/definitely/missing/cli-config.cue",
	)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected inspect config error")
	}
}

func TestDatasetInspectResultErrorFieldsBranch(t *testing.T) {
	t.Parallel()
	cmd, out, _ := newTestCommand(false,
		"dataset", "inspect", "customers_master",
		"--format", "json",
		"--config", testConfigPath,
		"--validation-engine", "native",
		"--compression", "brotli",
	)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected inspect failure with result payload")
	}
	s := out.String()
	if !strings.Contains(s, "\"error_id\":") || !strings.Contains(s, "\"message\":") {
		t.Fatalf("expected error_id/message fields, got: %s", s)
	}
}

func TestRenderValidateResultJSONErrPropagation(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	want := io.ErrClosedPipe
	if err := renderValidateResult(cmd, "json", validate.DatasetResult{DatasetID: "d", Status: "error"}, want); err != want {
		t.Fatalf("expected propagated error %v, got %v", want, err)
	}
}

func TestRunQueryOpenDBErrorBranches(t *testing.T) {
	t.Parallel()
	orig := openDuckDB
	defer func() { openDuckDB = orig }()

	spec := &config.Spec{}
	q := config.Query{ID: "q", SQL: "SELECT 1"}

	openDuckDB = func() (*sql.DB, error) { return nil, context.DeadlineExceeded }
	_, err := runQuery(spec, q, map[string]string{}, runOptions{format: "json", timeout: 2, out: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "QQQ_QUERY_TIMEOUT") {
		t.Fatalf("expected timeout error, got %v", err)
	}

	openDuckDB = func() (*sql.DB, error) { return nil, io.ErrUnexpectedEOF }
	_, err = runQuery(spec, q, map[string]string{}, runOptions{format: "json", out: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("expected open error passthrough, got %v", err)
	}
}

func TestRenderValidateResultErrorIDMessageBranches(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)

	r := validate.DatasetResult{DatasetID: "d", Status: "error", ErrorID: "E1", Message: "boom"}
	if err := renderValidateResult(cmd, "text", r, nil); err != nil {
		t.Fatalf("render text failed: %v", err)
	}
	if !strings.Contains(out.String(), "error_id=E1") || !strings.Contains(out.String(), "message=") {
		t.Fatalf("expected error id/message in text output, got: %s", out.String())
	}
}
