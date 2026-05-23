// purpose: Collect observed column metadata using native readers across dataset files.
// responsibilities: Inspect each file with native decoders and merge discovered column type information.
// architecture notes: Aggregation mirrors DuckDB inspection semantics for cross-engine output consistency.
package validate

import (
	"strings"

	"github.com/flarebyte/quick-quack-quest/internal/config"
)

func inspectWithNative(d config.Dataset, files []string, compression string, sampleSize int) (map[string]ObservedColumn, error) {
	out := map[string]ObservedColumn{}
	for _, p := range files {
		observed, _, err := inspectNativeFile(d, p, compression, sampleSize)
		if err != nil {
			return nil, err
		}
		for k, typ := range observed {
			cur := out[k]
			if cur.Name == "" {
				out[k] = ObservedColumn{
					Name:       k,
					DuckDBType: strings.ToUpper(typ),
					Nullable:   true,
				}
				continue
			}
			if normalizeType(cur.DuckDBType) != normalizeType(typ) {
				cur.DuckDBType = "VARCHAR"
			}
			cur.Nullable = true
			out[k] = cur
		}
	}
	return out, nil
}
