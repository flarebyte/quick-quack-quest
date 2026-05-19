package config

import (
	"fmt"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

const (
	ErrIDConfigLoad    = "QQQ_CONFIG_LOAD_FAILED"
	ErrIDConfigInvalid = "QQQ_CONFIG_INVALID"
	ErrIDConfigDecode  = "QQQ_CONFIG_DECODE_FAILED"
)

type ConfigError struct {
	ID      string
	Message string
	Cause   error
}

func (e *ConfigError) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.ID, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.ID, e.Message, e.Cause)
}

func (e *ConfigError) Unwrap() error {
	return e.Cause
}

type Spec struct {
	Validation struct {
		Engine           string `json:"engine"`
		RandomSampleRows int    `json:"random_sample_rows"`
	} `json:"validation"`
	Datasets []Dataset `json:"datasets"`
	Queries  []Query   `json:"queries"`
}

type Dataset struct {
	ID          string `json:"id"`
	Format      string `json:"format"`
	Layout      string `json:"layout"`
	Compression string `json:"compression"`
	Description string `json:"description"`
}

type Query struct {
	ID string `json:"id"`
}

func LoadAndValidate(path string) (*Spec, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, &ConfigError{ID: ErrIDConfigLoad, Message: "resolve config path", Cause: err}
	}
	dir := filepath.Dir(abs)
	_ = filepath.Base(abs)

	instances := load.Instances([]string{"."}, &load.Config{Dir: dir})
	if len(instances) == 0 {
		return nil, &ConfigError{ID: ErrIDConfigLoad, Message: "no CUE instances found"}
	}

	ctx := cuecontext.New()
	val := ctx.BuildInstance(instances[0])
	if err := val.Err(); err != nil {
		return nil, &ConfigError{ID: ErrIDConfigInvalid, Message: "build CUE instance", Cause: err}
	}

	cliSpec := val.LookupPath(cue.ParsePath("cliSpec"))
	if !cliSpec.Exists() {
		return nil, &ConfigError{ID: ErrIDConfigInvalid, Message: "missing required value cliSpec"}
	}
	if err := cliSpec.Validate(cue.Final(), cue.Concrete(true)); err != nil {
		return nil, &ConfigError{ID: ErrIDConfigInvalid, Message: "validation failed", Cause: err}
	}

	var spec Spec
	if err := cliSpec.Decode(&spec); err != nil {
		return nil, &ConfigError{ID: ErrIDConfigDecode, Message: "decode cliSpec", Cause: err}
	}

	return &spec, nil
}
