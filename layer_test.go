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

func TestLayerHashesSame(t *testing.T) {
	layer := &Layer{Name: "a", source: &NilSource{}}
	layer2 := &Layer{Name: "a", source: &NilSource{}}

	assert.Equal(t, layer.Hash(), layer2.Hash(), "Excepted identical layers to have the same hash")
}

func TestLayerHashesDifferent(t *testing.T) {
	layer := &Layer{Name: "a", source: &NilSource{}}
	layer2 := &Layer{Name: "b", source: &NilSource{}}

	assert.NotEqual(t, layer.Hash(), layer2.Hash(), "Excepted different layers to have a different hash")
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
