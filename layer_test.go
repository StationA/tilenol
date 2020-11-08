package tilenol

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateLayerMultipleSources(t *testing.T) {
	config := LayerConfig{
		Source: SourceConfig{
			Elasticsearch: new(ElasticsearchConfig),
			PostGIS:       new(PostGISConfig),
		},
	}
	_, err := CreateLayer(config)
	assert.Equal(t, MultipleSourcesErr, err, "Expected to fail due to multiple sources for layer")
}

func TestCreateLayerNoSources(t *testing.T) {
	config := LayerConfig{
		Source: SourceConfig{},
	}
	_, err := CreateLayer(config)
	assert.Equal(t, NoSourcesErr, err, "Expected to fail due to no sources for layer")
}

func TestCreateLayerMinZoomOutOfBounds(t *testing.T) {
	config := LayerConfig{
		Minzoom: MinZoom - 1,
		Maxzoom: MaxZoom - 1,
		Source: SourceConfig{
			Elasticsearch: new(ElasticsearchConfig),
		},
	}
	_, err := CreateLayer(config)
	assert.Equal(t, LayerMinZoomOutOfBoundsErr, err, "Expected to fail because layer min zoom is less than absolute allowed min")
}
func TestCreateLayerMaxZoomOutOfBounds(t *testing.T) {
	config := LayerConfig{
		Minzoom: MinZoom + 1,
		Maxzoom: MaxZoom + 1,
		Source: SourceConfig{
			Elasticsearch: new(ElasticsearchConfig),
		},
	}
	_, err := CreateLayer(config)
	assert.Equal(t, LayerMaxZoomOutOfBoundsErr, err, "Expected to fail because layer max zoom is greater than absolute allowed max")
}
