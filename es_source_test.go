package tilenol

import (
	"github.com/paulmach/orb/maptile"
	"testing"
)

func TestGetNested(t *testing.T) {
	m := map[string]interface{}{
		"building": map[string]interface{}{
			"shape": "SHAPE",
		},
		"outer": 123,
	}
	v, found := GetNested(m, []string{"building", "shape"})
	if !found {
		t.Error("Couldn't find key")
	}
	if v != "SHAPE" {
		t.Errorf("Invalid value: %+v", v)
	}
}

func TestGetNestedOuter(t *testing.T) {
	m := map[string]interface{}{
		"building": map[string]interface{}{
			"shape": "SHAPE",
		},
		"outer": 123,
	}
	v, found := GetNested(m, []string{"outer"})
	if !found {
		t.Error("Couldn't find key")
	}
	if v != 123 {
		t.Errorf("Invalid value: %+v", v)
	}
}

func TestGetNestedNotFound(t *testing.T) {
	m := map[string]interface{}{}
	v, found := GetNested(m, []string{"DOES NOT EXIST"})
	if found {
		t.Errorf("Shouldn't have found anything: %+v", v)
	}
	if v != nil {
		t.Errorf("Invalid value: %+v", v)
	}
}

func floatEquals(a, b float64) bool {
	e := 0.0001
	return (a-b) < e && (b-a) < e
}

func TestGetBoundsFilter(t *testing.T) {
	geometryField := "geometry"
	tile := maptile.New(0, 0, 0)
	filter := boundsFilter(geometryField, tile)
	v, exists := GetNested(filter.Map(), []string{"geo_shape", geometryField, "shape", "coordinates"})
	if !exists {
		t.Errorf("Invalid filter construction: %#v", filter)
	}
	coords := v.([][]float64)
	if coords[0][0] != -180.0 || !floatEquals(coords[0][1], 85.0511) ||
		coords[1][0] != 180.0 || !floatEquals(coords[1][1], -85.0511) {
		t.Errorf("Invalid envelope: %#v", coords)
	}
}
