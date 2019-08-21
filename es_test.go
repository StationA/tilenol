package tilenol

import (
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
