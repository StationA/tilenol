package tilenol

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/dop251/goja"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/simplify"
)

type JSObject = map[string]interface{}

type Script struct {
	Runtime *goja.Runtime
}

func CompileScript(source string, globals map[string]interface{}) (*Script, error) {
	runtime := goja.New()
	runtime.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
	addGlobals(runtime, globals)
	_, err := runtime.RunString(strings.TrimSpace(source))
	if err != nil {
		return nil, err
	}
	return &Script{runtime}, nil
}

func addGlobals(runtime *goja.Runtime, globals map[string]interface{}) {
	globals["geo"] = map[string]interface{}{
		"simplify": func(call goja.FunctionCall) goja.Value {
			var threshold float64 = 0.1
			feature, err := jsObjectToFeature(call.Arguments[0].Export().(JSObject))
			if err != nil {
				// TODO: Wuuut
				panic(err)
			}
			if len(call.Arguments) == 2 {
				threshold = call.Arguments[1].ToFloat()
			}
			simplifier := simplify.DouglasPeucker(threshold)
			simplified := simplifier.Simplify(feature.Geometry)
			output := geojson.NewFeature(simplified)
			output.Properties = feature.Properties.Clone()
			return runtime.ToValue(featureToJSObject(output))
		},
	}
	runtime.Set("tilenol", globals)
}

func jsObjectToFeature(js JSObject) (*geojson.Feature, error) {
	// TODO: Can we do this without turning it back into JSON bytes?
	j, err := json.Marshal(js)
	if err != nil {
		return nil, err
	}
	return geojson.UnmarshalFeature(j)
}

func featureToJSObject(feature *geojson.Feature) JSObject {
	var js JSObject
	// TODO: Can we do this without turning it back into JSON bytes?
	j, _ := feature.MarshalJSON()
	json.Unmarshal(j, &js)
	return js
}

func (s *Script) Apply(input *geojson.Feature) (*geojson.Feature, error) {
	mapFn, ok := goja.AssertFunction(s.Runtime.Get("map"))
	if !ok {
		return input, nil
	}
	v, err := mapFn(goja.Undefined(), s.Runtime.ToValue(featureToJSObject(input)))
	if err != nil {
		return nil, err
	}
	if goja.IsNull(v) || goja.IsUndefined(v) {
		return nil, nil
	}

	exported := v.Export()
	switch v.ExportType().Kind() {
	case reflect.Map:
		return jsObjectToFeature(exported.(JSObject))
	default:
		panic("What")
	}
}

func (s *Script) Process(inputs <-chan *geojson.Feature, outputs chan<- *geojson.Feature) error {
	for input := range inputs {
		output, err := s.Apply(input)
		if err != nil {
			return err
		}
		outputs <- output
	}
	return nil
}
