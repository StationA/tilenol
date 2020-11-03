package tilenol

import (
	"bytes"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/wkb"
)

const (
	FakeSQL = "SELECT blah FROM doesnt.matter"
)

func TestRowsToMap(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.Nil(t, err, "Failed to create mock DB: %s", err)

	cols := []string{"id", "height", "some_flag", "msg", "bmsg", "geometry", "timestamp", "empty"}
	fakeRows := mock.NewRows(cols)
	now := time.Now()
	point := orb.Point{0, 1}
	var pointWKB bytes.Buffer
	enc := wkb.NewEncoder(&pointWKB)
	encErr := enc.Encode(point)
	assert.Nil(t, encErr, "Failed to encode point to WKB")
	expectedRow := map[string]interface{}{
		"id":        int64(123),
		"height":    1.23,
		"some_flag": true,
		"msg":       "HELLO",
		"bmsg":      []byte("HOLA"),
		"geometry":  point,
		"timestamp": now,
		"empty":     nil,
	}
	fakeRows.AddRow(
		123,
		1.23,
		true,
		"HELLO",
		[]byte("HOLA"),
		pointWKB.Bytes(),
		now,
		nil,
	)
	mock.ExpectQuery(FakeSQL).WillReturnRows(fakeRows)

	rows, err := db.Query(FakeSQL)
	assert.Nil(t, err, "Failed to run mock query: %s", err)

	records, err := RowsToMaps(rows, "geometry")
	assert.Nil(t, err, "Failed to map rows to a record map: %s", err)
	assert.Len(t, records, 1, "Should only map one row to one output records")
	actualRow := records[0]
	assert.Len(t, actualRow, len(expectedRow), "Mapped record has fewer columns than expected")
	for k, actual := range actualRow {
		expected := expectedRow[k]
		assert.Equal(t, expected, actual, "Invalid row to record mapping: %s => %v (expected = %v)", k, actual, expected)
	}
}

func TestRowsToMapErr(t *testing.T) {
	db, mock, mockErr := sqlmock.New()
	assert.Nil(t, mockErr, "Failed to create mock DB: %s", mockErr)

	cols := []string{"id", "geometry"}
	fakeRows := mock.NewRows(cols)
	fakeRows.AddRow(
		123,
		[]byte("INVALID GEOMETRY"),
	)
	mock.ExpectQuery(FakeSQL).WillReturnRows(fakeRows)

	rows, sqlErr := db.Query(FakeSQL)
	assert.Nil(t, sqlErr, "Failed to run mock query: %s", sqlErr)

	_, err := RowsToMaps(rows, "geometry")
	assert.NotNil(t, err, "Expected to fail decoding the geometry")
}
