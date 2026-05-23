// purpose: Collect observed column metadata by inspecting datasets through DuckDB.
// responsibilities: Query file schemas via DuckDB and merge type/nullability observations across files.
// architecture notes: Merge behavior favors widened nullability and promoted types to avoid data loss.
package validate

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/flarebyte/quick-quack-quest/internal/config"
)

var openInspectDuckDB = func() (*sql.DB, error) {
	return sql.Open("duckdb", "")
}

var inspectRowsErr = func(rows *sql.Rows) error {
	return rows.Err()
}

func inspectWithDuckDB(d config.Dataset, files []string, sampleSize int) (map[string]ObservedColumn, error) {
	db, err := openInspectDuckDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	out := map[string]ObservedColumn{}
	for _, p := range files {
		q, args := describeQuery(d, p)
		rows, err := db.Query(q, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var colName, colType, nullable string
			var key, def, extra sql.NullString
			if err := rows.Scan(&colName, &colType, &nullable, &key, &def, &extra); err != nil {
				_ = rows.Close()
				return nil, err
			}
			k := strings.ToLower(colName)
			cur := out[k]
			if cur.Name == "" {
				out[k] = ObservedColumn{
					Name:       colName,
					DuckDBType: strings.ToUpper(colType),
					Nullable:   strings.EqualFold(nullable, "YES"),
				}
				continue
			}
			if strings.EqualFold(nullable, "YES") {
				cur.Nullable = true
				out[k] = cur
			}
		}
		if err := inspectRowsErr(rows); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
		if sampleSize > 0 {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no columns observed")
	}
	return out, nil
}
