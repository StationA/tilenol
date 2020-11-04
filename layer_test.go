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
