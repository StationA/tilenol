package tilenol

import (
	"database/sql"
	"errors"

	"github.com/paulmach/orb/encoding/wkb"
)

var (
	InvalidGeometryErr = errors.New("Column value was not a valid geometry")
)

// RowsToMaps converts Go's sql.Rows data structure into a list of maps where the key is a string
// column name, and the value is the raw SQL value
func RowsToMaps(rows *sql.Rows, geomColumn string) ([]map[string]interface{}, error) {
	var maps []map[string]interface{}

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		row := make([]interface{}, len(cols))
		for idx, col := range cols {
			if col == geomColumn {
				row[idx] = new(wkb.GeometryScanner)
			} else {
				row[idx] = new(DumbScanner)
			}
		}
		err := rows.Scan(row...)
		if err != nil {
			return maps, err
		}
		m := make(map[string]interface{})
		for idx, col := range cols {
			if geom, isGeomScanner := row[idx].(*wkb.GeometryScanner); isGeomScanner {
				if geom.Valid {
					m[col] = geom.Geometry
				} else {
					return nil, InvalidGeometryErr
				}
			} else {
				ds := row[idx].(*DumbScanner)
				m[col] = ds.Value
			}
		}
		maps = append(maps, m)
	}

	return maps, nil
}

type DumbScanner struct {
	Value interface{}
}

func (d *DumbScanner) Scan(src interface{}) error {
	d.Value = src
	return nil
}
