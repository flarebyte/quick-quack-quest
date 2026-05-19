package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndValidateSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	schema := `package designmeta
#CliSpec: {
  datasets: [..._]
  queries: [..._]
}
`
	cfg := `package designmeta
cliSpec: #CliSpec & {
  datasets: []
  queries: []
}
`
	mustWrite(t, filepath.Join(dir, "cli-config.schema.cue"), schema)
	path := filepath.Join(dir, "cli-config.cue")
	mustWrite(t, path, cfg)

	spec, err := LoadAndValidate(path)
	if err != nil {
		t.Fatalf("LoadAndValidate returned error: %v", err)
	}
	if spec == nil {
		t.Fatal("expected non-nil spec")
	}
}

func TestLoadAndValidateFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	schema := `package designmeta
#CliSpec: {
  datasets: [..._]
  queries: [..._]
}
`
	cfg := `package designmeta
cliSpec: #CliSpec & {
  datasets: []
  queries: 1
}
`
	mustWrite(t, filepath.Join(dir, "cli-config.schema.cue"), schema)
	path := filepath.Join(dir, "cli-config.cue")
	mustWrite(t, path, cfg)

	_, err := LoadAndValidate(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	cErr, ok := err.(*ConfigError)
	if !ok {
		t.Fatalf("expected ConfigError, got %T", err)
	}
	if cErr.ID != ErrIDConfigInvalid {
		t.Fatalf("expected %s, got %s", ErrIDConfigInvalid, cErr.ID)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
