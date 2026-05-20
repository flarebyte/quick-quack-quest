package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

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

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"dataset", "list", "--format", "json", "--config", "../../doc/design-meta/examples/config/cli-config.cue"})

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

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"dataset", "list", "--format", "text", "--config", "../../doc/design-meta/examples/config/cli-config.cue"})

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

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"dataset", "validate", "customers_master",
		"--format", "json",
		"--config", "../../doc/design-meta/examples/config/cli-config.cue",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\noutput=%s", err, out.String())
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, out.String())
	}
	if got["status"] != "ok" {
		t.Fatalf("expected ok status, got %v", got["status"])
	}
	if got["dataset_id"] != "customers_master" {
		t.Fatalf("dataset mismatch: %v", got["dataset_id"])
	}
}

func TestDatasetValidateDatasetNotFound(t *testing.T) {
	t.Parallel()

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"dataset", "validate", "not_a_dataset",
		"--format", "json",
		"--config", "../../doc/design-meta/examples/config/cli-config.cue",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error, got nil\noutput=%s", out.String())
	}
	if !strings.Contains(err.Error(), "QQQ_DATASET_NOT_FOUND") {
		t.Fatalf("expected QQQ_DATASET_NOT_FOUND, got %v", err)
	}
}

func TestDatasetValidateUnsupportedEngine(t *testing.T) {
	t.Parallel()

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"dataset", "validate", "customers_master",
		"--validation-engine", "bogus",
		"--format", "json",
		"--config", "../../doc/design-meta/examples/config/cli-config.cue",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error, got nil\noutput=%s", out.String())
	}
	if !strings.Contains(err.Error(), "QQQ_VALIDATION_ENGINE_UNSUPPORTED") {
		t.Fatalf("expected QQQ_VALIDATION_ENGINE_UNSUPPORTED, got %v", err)
	}
}

func TestDatasetValidateNativeJSONOutputSuccess(t *testing.T) {
	t.Parallel()

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"dataset", "validate", "customers_master",
		"--validation-engine", "native",
		"--format", "json",
		"--config", "../../doc/design-meta/examples/config/cli-config.cue",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\noutput=%s", err, out.String())
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, out.String())
	}
	if got["status"] != "ok" {
		t.Fatalf("expected ok status, got %v", got["status"])
	}
}

func TestDatasetInspectDuckDBJSONOutputSuccess(t *testing.T) {
	t.Parallel()

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"dataset", "inspect", "customers_master",
		"--format", "json",
		"--sample-size", "10",
		"--config", "../../doc/design-meta/examples/config/cli-config.cue",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\noutput=%s", err, out.String())
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, out.String())
	}
	if got["status"] != "ok" {
		t.Fatalf("expected ok status, got %v", got["status"])
	}
	if got["output_schema_version"] != "v1" {
		t.Fatalf("expected output_schema_version v1, got %v", got["output_schema_version"])
	}
}

func TestDatasetInspectNativeGzipJSONOutputSuccess(t *testing.T) {
	t.Parallel()

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"dataset", "inspect", "events_stream",
		"--validation-engine", "native",
		"--format", "json",
		"--sample-size", "5",
		"--config", "../../doc/design-meta/examples/config/cli-config.cue",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\noutput=%s", err, out.String())
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, out.String())
	}
	if got["status"] != "ok" {
		t.Fatalf("expected ok status, got %v", got["status"])
	}
	if got["compression"] != "gzip" {
		t.Fatalf("expected gzip compression, got %v", got["compression"])
	}
}
