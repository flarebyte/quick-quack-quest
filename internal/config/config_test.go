// purpose: Protect configuration loading and validation behavior against regressions.
// responsibilities: Build temporary config fixtures and assert decode, validation, and path-resolution outcomes.
// architecture notes: Tests assert error IDs directly so CLI and automation can rely on stable diagnostics.
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

func TestConfigErrorMethodsAndHelpers(t *testing.T) {
	t.Parallel()

	cause := os.ErrNotExist
	e := &ConfigError{ID: ErrIDConfigLoad, Message: "x", Cause: cause}
	if e.Unwrap() != cause {
		t.Fatalf("expected unwrap cause")
	}
	if e.Error() == "" {
		t.Fatalf("expected non-empty error string")
	}

	if got := toAbsPath("/tmp/base", "/already/abs"); got != "/already/abs" {
		t.Fatalf("expected absolute path untouched, got %s", got)
	}
	if got := toAbsPath("/tmp/base", "rel/file.csv"); got != filepath.Join("/tmp/base", "rel/file.csv") {
		t.Fatalf("unexpected toAbsPath join: %s", got)
	}

	root := t.TempDir()
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mustWrite(t, filepath.Join(root, "go.mod"), "module x\n")
	if got := findRepoRoot(nested); got != root {
		t.Fatalf("expected repo root %s, got %s", root, got)
	}
}

func TestLoadAndValidateDecodeAndMissingCliSpecErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// missing cliSpec branch
	mustWrite(t, filepath.Join(dir, "cli-config.schema.cue"), "package designmeta\n#CliSpec: {}\n")
	path := filepath.Join(dir, "cli-config.cue")
	mustWrite(t, path, "package designmeta\nfoo: 1\n")
	_, err := LoadAndValidate(path)
	if err == nil {
		t.Fatal("expected missing cliSpec error")
	}
	cErr, ok := err.(*ConfigError)
	if !ok || cErr.ID != ErrIDConfigInvalid {
		t.Fatalf("expected config invalid, got %T %v", err, err)
	}

	// decode branch: required datasets/queries concrete but wrong type for datasets
	dir2 := t.TempDir()
	mustWrite(t, filepath.Join(dir2, "cli-config.schema.cue"), `package designmeta
#CliSpec: {
  datasets: [..._]
  queries: [..._]
}`)
	path2 := filepath.Join(dir2, "cli-config.cue")
	mustWrite(t, path2, `package designmeta
cliSpec: #CliSpec & {
  datasets: ["bad"]
  queries: []
}`)
	_, err = LoadAndValidate(path2)
	if err == nil {
		t.Fatal("expected decode/validation error")
	}
	cErr, ok = err.(*ConfigError)
	if !ok || (cErr.ID != ErrIDConfigDecode && cErr.ID != ErrIDConfigInvalid) {
		t.Fatalf("expected config decode/invalid error, got %T %v", err, err)
	}
}

func TestNormalizeDatasetPaths(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mustWrite(t, filepath.Join(repo, "go.mod"), "module x\n")

	spec := &Spec{
		Datasets: []Dataset{{
			ID:     "d1",
			Path:   "data/file.csv",
			Prefix: "data/part/",
		}},
	}
	cfgPath := filepath.Join(repo, "sub", "cli-config.cue")
	normalizeDatasetPaths(spec, cfgPath)

	if !filepath.IsAbs(spec.Datasets[0].Path) {
		t.Fatalf("expected absolute dataset path, got %s", spec.Datasets[0].Path)
	}
	if !filepath.IsAbs(spec.Datasets[0].Prefix) {
		t.Fatalf("expected absolute dataset prefix, got %s", spec.Datasets[0].Prefix)
	}
}

func TestConfigErrorStringWithoutCause(t *testing.T) {
	t.Parallel()
	e := &ConfigError{ID: ErrIDConfigInvalid, Message: "bad"}
	if e.Unwrap() != nil {
		t.Fatalf("expected nil unwrap")
	}
	if got := e.Error(); got != "QQQ_CONFIG_INVALID: bad" {
		t.Fatalf("unexpected error string: %s", got)
	}
}

func TestLoadAndValidateDecodeErrorID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "cli-config.schema.cue"), `package designmeta
#CliSpec: {
  datasets: [..._]
  queries: [..._]
}`)
	path := filepath.Join(dir, "cli-config.cue")
	mustWrite(t, path, `package designmeta
cliSpec: #CliSpec & {
  datasets: [123]
  queries: []
}`)
	_, err := LoadAndValidate(path)
	if err == nil {
		t.Fatalf("expected decode/validation error")
	}
	cErr, ok := err.(*ConfigError)
	if !ok {
		t.Fatalf("expected ConfigError, got %T", err)
	}
	if cErr.ID != ErrIDConfigDecode && cErr.ID != ErrIDConfigInvalid {
		t.Fatalf("expected decode/invalid id, got %s", cErr.ID)
	}
}
