package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

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
