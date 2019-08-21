package tilenol

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/olivere/elastic"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
)

const (
	// TODO: Externalize these?

	// ScrollSize is the max number of documents per scroll page
	ScrollSize = 250
	// ScrollTimeout is the time.Duration to keep the scroll context alive
	ScrollTimeout = time.Minute
)

type ElasticsearchConfig struct {
	Host          string            `yaml:"host"`
	Port          int               `yaml:"port"`
	Index         string            `yaml:"index"`
	GeometryField string            `yaml:"geometryField"`
	SourceFields  map[string]string `yaml:"sourceFields"`
}

type ElasticsearchSource struct {
	ES            *elastic.Client
	Index         string
	GeometryField string
	SourceFields  map[string]string
}

func NewElasticsearchSource(config *ElasticsearchConfig) (Source, error) {
	es, err := elastic.NewClient(
		elastic.SetURL(fmt.Sprintf("http://%s:%d", config.Host, config.Port)),
		elastic.SetGzip(true),
		// TODO: Should this be configurable?
		elastic.SetHealthcheckTimeoutStartup(30*time.Second),
	)
	if err != nil {
		return nil, err
	}
	return &ElasticsearchSource{
		ES:            es,
		Index:         config.Index,
		GeometryField: config.GeometryField,
		SourceFields:  config.SourceFields,
	}, nil
}

func (e *ElasticsearchSource) getSourceFields() []string {
	fields := []string{e.GeometryField}
	for _, v := range e.SourceFields {
		fields = append(fields, v)
	}
	return fields
}

func (e *ElasticsearchSource) buildQuery(tileBounds orb.Bound) interface{} {
	return map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": map[string]interface{}{
					"match_all": map[string]interface{}{},
				},
				"filter": map[string]interface{}{
					"geo_shape": map[string]interface{}{
						e.GeometryField: map[string]interface{}{
							"shape": map[string]interface{}{
								"type": "envelope",
								"coordinates": [][]float64{
									{tileBounds.Left(), tileBounds.Top()},
									{tileBounds.Right(), tileBounds.Bottom()},
								},
							},
							"relation": "intersects",
						},
					},
				},
			},
		},
		"_source": e.getSourceFields(),
	}
}

func (e *ElasticsearchSource) hitToFeature(hit *elastic.SearchHit) (*geojson.Feature, error) {
	id := hit.Id
	var source map[string]interface{}
	err := json.Unmarshal(*hit.Source, &source)
	if err != nil {
		return nil, err
	}
	// Extract geometry value (potentially nested in the source)
	geometryFieldParts := strings.Split(e.GeometryField, ".")
	numParts := len(geometryFieldParts)
	lastPart := geometryFieldParts[numParts-1]
	parent, found := GetNested(source, geometryFieldParts[0:numParts-1])
	if !found {
		return nil, fmt.Errorf("Couldn't find geometry at field: %s", e.GeometryField)
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
	// Populate the feature with the mapped source fields
	for prop, fieldName := range e.SourceFields {
		Logger.Debugf("Mapping %s -> %s", prop, fieldName)
		val, found := GetNested(source, strings.Split(fieldName, "."))
		if found {
			feat.Properties[prop] = val
		}
	}
	feat.Properties["id"] = id
	return feat, nil
}

func (e *ElasticsearchSource) GetFeatures(ctx context.Context) (*geojson.FeatureCollection, error) {
	tile := ctx.Value("tile").(maptile.Tile)
	tileBounds := tile.Bound()

	fc := geojson.NewFeatureCollection()

	query := e.buildQuery(tileBounds)
	Logger.Debugf("Built ES query: %v", query)

	scroll := e.ES.Scroll(e.Index).Body(query).Size(ScrollSize)
	for {
		scrollCtx, scrollCancel := context.WithTimeout(ctx, ScrollTimeout)
		defer scrollCancel()
		results, err := scroll.Do(scrollCtx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		Logger.Debugf("Scrolling %d hits", len(results.Hits.Hits))
		for _, hit := range results.Hits.Hits {
			feat, err := e.hitToFeature(hit)
			if err != nil {
				return nil, err
			}
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

// GetNested is a utility function to traverse a path of keys in a nested JSON object
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
