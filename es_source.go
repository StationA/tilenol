package tilenol

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/olivere/elastic"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
)

const (
	// TODO: Externalize these?

	// ScrollSize is the max number of documents per scroll page
	ScrollSize = 250
	// ScrollTimeout is the time.Duration to keep the scroll context alive
	ScrollTimeout = 10 * time.Second
)

// ElasticsearchConfig is the YAML configuration structure for configuring a new
// ElasticsearchSource
type ElasticsearchConfig struct {
	// Host is the hostname part of the backend Elasticsearch cluster
	Host string `yaml:"host"`
	// Host is the port number of the backend Elasticsearch cluster
	Port int `yaml:"port"`
	// Index is the name of the Elasticsearch index used for retrieving feature data
	Index string `yaml:"index"`
	// GeometryField is the name of the document field that holds the feature geometry
	GeometryField string `yaml:"geometryField"`
	// SourceFields is a mapping from the feature property name to the source document
	// field name
	SourceFields map[string]string `yaml:"sourceFields"`
}

// ElasticsearchSource is a Source implementation that retrieves feature data from an
// Elasticsearch cluster
type ElasticsearchSource struct {
	// ES is the internal Elasticsearch cluster client
	ES *elastic.Client
	// Index is the name of the Elasticsearch index used for retrieving feature data
	Index string
	// GeometryField is the name of the document field that holds the feature geometry
	GeometryField string
	// SourceFields is a mapping from the feature property name to the source document
	// field name
	SourceFields map[string]string
}

// Dict is a type alias for map[string]interface{} that cleans up literals and also adds
// a helper method to implement the elastic.Query interface
type Dict map[string]interface{}

// Source implements the elastic.Query interface, by simply returning the raw Dict
// contents
func (d *Dict) Source() (interface{}, error) {
	return d, nil
}

// Map is a helper method to cleanly alias back to map[string]interface{}
func (d *Dict) Map() map[string]interface{} {
	return *d
}

// NewElasticsearchSource creates a new Source that retrieves feature data from an
// Elasticsearch cluster
func NewElasticsearchSource(config *ElasticsearchConfig) (Source, error) {
	es, err := elastic.NewClient(
		elastic.SetURL(fmt.Sprintf("http://%s:%d", config.Host, config.Port)),
		elastic.SetGzip(true),
		// TODO: Should this be configurable?
		elastic.SetHealthcheckTimeoutStartup(10*time.Second),
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

// GetFeatures implements the Source interface, to get feature data from an
// Elasticsearch cluster
func (e *ElasticsearchSource) GetFeatures(ctx context.Context, req *TileRequest) (*geojson.FeatureCollection, error) {
	// TODO: Add optional support for other query constructs? (e.g. aggregations)
	return e.doGetFeatures(ctx, req)
}

// getSourceFields returns the list of source fields to include in the fetched features
func (e *ElasticsearchSource) getSourceFields() []string {
	fields := []string{e.GeometryField}
	for _, v := range e.SourceFields {
		fields = append(fields, v)
	}
	return fields
}

// newSearchSource constructs a full Elasticsearch request body from a given query and
// adds document source inclusions/exclusions
func (e *ElasticsearchSource) newSearchSource(query elastic.Query) *elastic.SearchSource {
	includes := e.getSourceFields()
	// TODO: Do we need to do anything fancier here?
	excludes := []string{}
	return elastic.NewSearchSource().
		FetchSourceIncludeExclude(includes, excludes).
		Query(query)
}

// boundsFilter converts an XYZ map tile into an Elasticsearch-friendly geo_shape query
func boundsFilter(geometryField string, tile maptile.Tile) *Dict {
	tileBounds := tile.Bound()
	return &Dict{
		"geo_shape": map[string]interface{}{
			geometryField: map[string]interface{}{
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
	}
}

// doGetFeatures scrolls the configured Elasticsearch index for all documents that fall
// within the tile boundaries
func (e *ElasticsearchSource) doGetFeatures(ctx context.Context, req *TileRequest) (*geojson.FeatureCollection, error) {
	query := elastic.NewBoolQuery().Filter(boundsFilter(e.GeometryField, req.MapTile()))
	ss := e.newSearchSource(query)
	s, _ := ss.Source()
	Logger.Debugf("Search source: %#v", s)

	fc := geojson.NewFeatureCollection()
	scroll := e.ES.Scroll(e.Index).SearchSource(ss).Size(ScrollSize)
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
		Logger.Tracef("Scrolling %d hits", len(results.Hits.Hits))
		for _, hit := range results.Hits.Hits {
			feat, err := e.HitToFeature(hit)
			if err != nil {
				return nil, err
			}
			fc.Append(feat)
		}
		scrollCancel()
	}
	return fc, nil
}

// HitToFeature converts an Elasticsearch hit object into a GeoJSON feature, by
// using the hit's geometry as the feature geometry, and mapping all other requested
// source fields to feature properties
func (e *ElasticsearchSource) HitToFeature(hit *elastic.SearchHit) (*geojson.Feature, error) {
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
		val, found := GetNested(source, strings.Split(fieldName, "."))
		if found {
			if val != nil {
				feat.Properties[prop] = val
			} else {
				Logger.Warningf("Couldn't find value at field '%s' for feature '%s' on layer '%s'", fieldName, id, hit.Index)
			}
		}
	}
	feat.Properties["id"] = id
	return feat, nil
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
