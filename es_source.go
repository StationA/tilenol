package tilenol

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/geojson"
)

const (
	// TODO: Externalize these?
	// SearchTimeout is the time.Duration to keep the search context alive
	SearchTimeout = 30 * time.Second
)

// ElasticsearchConfig is the YAML configuration structure for configuring a new
// ElasticsearchSource
type ElasticsearchConfig struct {
	// Hosts are the Elasticsearch cluster URLs
	Hosts []string `yaml:"hosts"`
	// Username is the HTTP basic auth username
	Username string `yaml:"username"`
	// Password is the HTTP basic auth username
	Password string `yaml:"password"`
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
	ES *elasticsearch.TypedClient
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
	cfg := elasticsearch.Config{
		Addresses: config.Hosts,
		Username:  config.Username,
		Password:  config.Password,
	}
	es, err := elasticsearch.NewTypedClient(cfg)
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

// Given the list of extra source arguments that were specified with request, transform
// these into a map of property name to ES document source path, or return an error
// if there is a malformed extra source argument.
func makeFieldMap(incArgs []string) (map[string]string, error) {
	var result = make(map[string]string)

	for _, source := range incArgs {
		splits := strings.SplitN(source, ":", 2)
		if len(splits) < 2 {
			return nil, InvalidRequestError{fmt.Sprintf("Invalid source field specification: '%s'", source)}
		}
		result[splits[0]] = splits[1]
	}

	return result, nil
}

// doGetFeatures scrolls the configured Elasticsearch index for all documents that fall
// within the tile boundaries
func (e *ElasticsearchSource) doGetFeatures(ctx context.Context, req *TileRequest) (*geojson.FeatureCollection, error) {
	x := fmt.Sprint(req.X)
	y := fmt.Sprint(req.Y)
	z := fmt.Sprint(req.Z)

	var search = e.ES.SearchMvt(e.Index, e.GeometryField, z, x, y).
		Extent(mvt.DefaultExtent).
		// Avoids grid aggregations
		GridPrecision(0).
		TrackTotalHits(false)

	// Check for optional ES query argument.
	if qs, exists := req.Args["q"]; exists && len(qs) > 0 { // TODO: We ignore all but the first "q" arg.
		search = search.Query(&types.Query{
			Bool: &types.BoolQuery{
				Filter: []types.Query{
					{QueryString: &types.QueryStringQuery{Query: qs[0]}},
				},
			},
		})
	}

	var searchFields = []string{}
	allFieldMappings := make(map[string]string)
	for prop, fieldName := range e.SourceFields {
		allFieldMappings[prop] = fieldName
		searchFields = append(searchFields, fieldName)
	}

	// Check for extra fields specifications. They must have the form of <property_name>:<ES_document_path>,
	// eg: levels:building.stories.
	if inc_args, exists := req.Args["s"]; exists {
		extraFields, err := makeFieldMap(inc_args)
		if err != nil {
			return nil, err
		}

		for prop, fieldName := range extraFields {
			allFieldMappings[prop] = fieldName
			searchFields = append(searchFields, fieldName)
		}
	}

	search = search.Fields(searchFields...)

	searchCtx, searchCancel := context.WithTimeout(ctx, SearchTimeout)
	defer searchCancel()
	results, err := search.Do(searchCtx)

	layers, err := mvt.Unmarshal(results)
	if err != nil {
		return nil, err
	}

	fc := geojson.NewFeatureCollection()
	for _, layer := range layers {
		if layer.Name == "hits" {
			layer.ProjectToWGS84(req.MapTile())
			for _, feat := range layer.Features {
				newFeat := geojson.NewFeature(feat.Geometry)
				id := feat.Properties["_id"]
				newFeat.ID = id
				newFeat.Properties["id"] = id
				for prop, fieldName := range allFieldMappings {
					val, found := feat.Properties[fieldName]
					if found {
						if val != nil {
							newFeat.Properties[prop] = val
						} else {
							Logger.Warningf("Couldn't find value at field '%s' for feature '%s' on layer '%s'", fieldName, id, e.Index)
						}
					}
				}
				fc.Append(newFeat)
			}
		}
	}
	return fc, nil
}
