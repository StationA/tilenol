package tilenol

import (
	"bytes"
	"database/sql"
	"time"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/wkb"
)

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

func (s *RowScanner) tryDecodeGeo(data []byte) (orb.Geometry, error) {
	dec := wkb.NewDecoder(bytes.NewBuffer(data))
	geom, err := dec.Decode()
	if err != nil {
		return nil, err
	}
	return geom, nil
}

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
