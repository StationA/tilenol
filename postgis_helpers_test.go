package tilenol

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/wkb"
)

type Pair [2]interface{}

func TestScanner(t *testing.T) {
	now := time.Now()
	point := orb.Point{0, 1}
	var pointWKB bytes.Buffer
	enc := wkb.NewEncoder(&pointWKB)
	err := enc.Encode(point)
	assert.Nil(t, err, "Failed to encode point to WKB")
	values := []Pair{
		// Input, expected output
		{int64(123), int64(123)},
		{float64(1.23), float64(1.23)},
		{true, true},
		{"HELLO", "HELLO"},
		{[]byte("HOLA"), []byte("HOLA")},
		{pointWKB.Bytes(), point},
		{now, now},
		{nil, nil},
	}
	s := new(RowScanner)
	for _, pair := range values {
		input, expected := pair[0], pair[1]
		err := s.Scan(input)
		assert.Nil(t, err, "Failed to scan: %v", input)
		assert.True(t, s.valid, "Invalid scan: %v", input)
		assert.Equal(t, s.value, expected, "Mismatch scan value: %v != %v", s.value, expected)
	}
}
