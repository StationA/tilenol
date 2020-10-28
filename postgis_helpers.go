package tilenol

import (
	"database/sql"
	"time"
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
			row[idx] = new(MetalScanner)
		}
		err := rows.Scan(row...)
		if err != nil {
			return maps, err
		}
		m := make(map[string]interface{})
		for idx, col := range cols {
			var scanner = row[idx].(*MetalScanner)
			m[col] = scanner.value
		}
		maps = append(maps, m)
	}

	return maps, nil
}

type MetalScanner struct {
	valid bool
	value interface{}
}

func (scanner *MetalScanner) getBytes(src interface{}) []byte {
	if a, ok := src.([]uint8); ok {
		return a
	}
	return nil
}

func (scanner *MetalScanner) Scan(src interface{}) error {
	switch src.(type) {
	case int64:
		if value, ok := src.(int64); ok {
			scanner.value = value
			scanner.valid = true
		}
	case float64:
		if value, ok := src.(float64); ok {
			scanner.value = value
			scanner.valid = true
		}
	case bool:
		if value, ok := src.(bool); ok {
			scanner.value = value
			scanner.valid = true
		}
	case string:
		value := scanner.getBytes(src)
		scanner.value = string(value)
		scanner.valid = true
	case []byte:
		value := scanner.getBytes(src)
		scanner.value = value
		scanner.valid = true
	case time.Time:
		if value, ok := src.(time.Time); ok {
			scanner.value = value
			scanner.valid = true
		}
	case nil:
		scanner.value = nil
		scanner.valid = true
	}
	return nil
}
