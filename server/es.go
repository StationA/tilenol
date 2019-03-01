package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
)

const (
	// TODO: Externalize these?
	ScrollSize    = 250
	ScrollTimeout = time.Minute
)

func buildQuery(geometryField string, extraSources []string, tileBounds orb.Bound) interface{} {
	sourceParam := append(extraSources, geometryField)
	return map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": map[string]interface{}{
					"match_all": map[string]interface{}{},
				},
				"filter": map[string]interface{}{
					"geo_shape": map[string]interface{}{
						geometryField: map[string]interface{}{
							"shape": map[string]interface{}{
								"type": "envelope",
								"coordinates": [][]float64{
									[]float64{tileBounds.Left(), tileBounds.Top()},
									[]float64{tileBounds.Right(), tileBounds.Bottom()},
								},
							},
							"relation": "intersects",
						},
					},
				},
			},
		},
		"_source": sourceParam,
	}
}

func (s *Server) doQuery(ctx context.Context, index, geometryField string, extraSources []string, tileBounds orb.Bound) (*geojson.FeatureCollection, error) {
	fc := geojson.NewFeatureCollection()

	query := buildQuery(geometryField, extraSources, tileBounds)

	scroll := s.ES.Scroll(index).Body(query).Size(ScrollSize)
	for {
		scrollCtx, scrollCancel := context.WithTimeout(ctx, ScrollTimeout)
		results, err := scroll.Do(scrollCtx)
		if err == io.EOF {
			scrollCancel()
			break
		}
		if err != nil {
			scrollCancel()
			return nil, err
		}
		Logger.Debugf("Scrolling %d hits", len(results.Hits.Hits))
		for _, hit := range results.Hits.Hits {
			id := hit.Id
			var source map[string]interface{}
			err := json.Unmarshal(*hit.Source, &source)
			if err != nil {
				return nil, err
			}
			// Extract geometry value (potentially nested in the source)
			geometryFieldParts := strings.Split(geometryField, ".")
			numParts := len(geometryFieldParts)
			lastPart := geometryFieldParts[numParts-1]
			parent, found := GetNested(source, geometryFieldParts[0:numParts-1])
			if !found {
				return nil, fmt.Errorf("Couldn't find geometry at field: %s", geometryField)
			}
			parentMap := parent.(map[string]interface{})
			geometry := parentMap[lastPart]
			// Remove geometry from source to avoid sending extra data
			delete(parentMap, lastPart)
			gj, _ := json.Marshal(geometry)
			geom, _ := geojson.UnmarshalGeometry(gj)
			feat := geojson.NewFeature(geom.Geometry())
			feat.ID = id
			feat.Properties = make(map[string]interface{})
			flatten(source["properties"], feat.Properties)
			delete(source, "properties")
			flatten(source, feat.Properties)
			feat.Properties["id"] = id
			Logger.Debugf("Adding feature to layer: %+v", feat)
			fc.Append(feat)
		}
		scrollCancel()
	}
	return fc, nil
}

func flatten(something interface{}, accum map[string]interface{}, prefixParts ...string) {
	if something == nil {
		return
	}
	switch something.(type) {
	case []interface{}:
		for i, thing := range something.([]interface{}) {
			flatten(thing, accum, append(prefixParts, fmt.Sprintf("%d", i))...)
		}
	case map[string]interface{}:
		for key, value := range something.(map[string]interface{}) {
			flatten(value, accum, append(prefixParts, key)...)
		}
	default:
		newKey := strings.Join(prefixParts, ".")
		accum[newKey] = something
	}
}

func GetNested(something interface{}, keyParts []string) (interface{}, bool) {
	if len(keyParts) == 0 {
		return something, true
	}
	if something != nil {
		switch m := something.(type) {
		case map[string]interface{}:
			v, found := m[keyParts[0]]
			if found {
				return GetNested(v, keyParts[1:])
			}
		}
	}
	return nil, false
}
