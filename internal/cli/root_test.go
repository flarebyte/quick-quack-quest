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
