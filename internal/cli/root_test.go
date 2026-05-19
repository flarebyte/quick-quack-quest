package cli

import (
	"bytes"
	"encoding/json"
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
