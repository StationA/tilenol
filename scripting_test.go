package tilenol

import (
	"testing"

	"github.com/paulmach/orb/geojson"
)

func TestScriptingBasic(t *testing.T) {
	scriptSource := `
	function map(input) {
		var newProps = input.properties;
		newProps.c = "C";
		return {
			type: input.type,
			geometry: input.geometry,
			properties: newProps
		}
	}
	`

	script, err := CompileScript(scriptSource, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to compile script: %v", err)
	}

	feature := `{
		"type": "Feature",
        "geometry": { "type": "Point", "coordinates": [0, 1] },
		"properties": { "a": "A", "b": "B"}
	}`

	input, err := geojson.UnmarshalFeature([]byte(feature))
	if err != nil {
		t.Fatalf("Failed to unmarshal feature: %v", err)
	}
	output, err := script.Apply(input)

	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	if output == nil {
		t.Fatalf("Expected non-nil output")
	}

	if c, exists := output.Properties["c"]; !exists || c != "C" {
		t.Fatalf("Expected to add 'c' key to output properties")
	}
}

func TestScriptingFilter(t *testing.T) {
	scriptSource := `
	function map(input) {
		return null;
	}
	`

	script, err := CompileScript(scriptSource, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to compile script: %v", err)
	}

	feature := `{
		"type": "Feature",
        "geometry": { "type": "Point", "coordinates": [0, 1] },
		"properties": { "a": "A", "b": "B"}
	}`

	input, err := geojson.UnmarshalFeature([]byte(feature))
	if err != nil {
		t.Fatalf("Failed to unmarshal feature: %v", err)
	}
	output, err := script.Apply(input)

	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	if output != nil {
		t.Fatalf("Expected nil output, got %+v", output)
	}
}
