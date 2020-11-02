package tilenol

import (
	"bytes"
	"database/sql"
	"time"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/wkb"
)

// RowsToMaps converts Go's sql.Rows data structure into a list of maps where the key is a string
// column name, and the value is the raw SQL value
func RowsToMaps(rows *sql.Rows) ([]map[string]interface{}, error) {
	var maps []map[string]interface{}

	cols, err := rows.Columns()
	if err != nil {
		return maps, err
	}

	for rows.Next() {
		row := make([]interface{}, len(cols))
		for idx := range cols {
			row[idx] = new(RowScanner)
		}
		err := rows.Scan(row...)
		if err != nil {
			return maps, err
		}
		m := make(map[string]interface{})
		for idx, col := range cols {
			var s = row[idx].(*RowScanner)
			if s.valid {
				m[col] = s.value
			}
		}
		maps = append(maps, m)
	}

	return maps, nil
}

// Loosely adapted "Scanner" implementation that knows how to unmarshal data from the raw database
// response
type RowScanner struct {
	valid bool
	value interface{}
}

func (s *RowScanner) getBytes(src interface{}) []byte {
	if a, ok := src.([]uint8); ok {
		return a
	}
	return nil
}

// Attempts to decode a WKB-encoded blob from the raw database response
func (s *RowScanner) tryDecodeGeo(data []byte) (orb.Geometry, error) {
	dec := wkb.NewDecoder(bytes.NewBuffer(data))
	geom, err := dec.Decode()
	if err != nil {
		return nil, err
	}
	return geom, nil
}

// Scan implements the primary Scanner interface and is called for each column value in the raw
// database response; we may need to handle additional data types, but for now this should cover
// the majority of data types
func (s *RowScanner) Scan(src interface{}) error {
	switch src.(type) {
	case int64:
		if value, ok := src.(int64); ok {
			s.value = value
			s.valid = true
		}
	case float64:
		if value, ok := src.(float64); ok {
			s.value = value
			s.valid = true
		}
	case bool:
		if value, ok := src.(bool); ok {
			s.value = value
			s.valid = true
		}
	case string:
		s.value = src
		s.valid = true
	case []byte:
		geom, err := s.tryDecodeGeo(src.([]byte))
		if err != nil {
			value := s.getBytes(src)
			s.value = value
			s.valid = true
		} else {
			s.value = geom
			s.valid = true
		}
	case time.Time:
		if value, ok := src.(time.Time); ok {
			s.value = value
			s.valid = true
		}
	case nil:
		s.value = nil
		s.valid = true
	}
	return nil
}
