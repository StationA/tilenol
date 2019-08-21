package tilenol

import (
	"context"
	"fmt"

	"github.com/paulmach/orb/geojson"
)

type SourceConfig struct {
	Elasticsearch *ElasticsearchConfig `yaml:"elasticsearch"`
}

type LayerConfig struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Minzoom     int          `yaml:"minzoom"`
	Maxzoom     int          `yaml:"maxzoom"`
	Source      SourceConfig `yaml:"source"`
}

type Source interface {
	GetFeatures(context.Context) (*geojson.FeatureCollection, error)
}

type Layer struct {
	Name        string
	Description string
	Minzoom     int
	Maxzoom     int
	Source      Source
}

func CreateLayer(layerConfig LayerConfig) (*Layer, error) {
	layer := &Layer{
		Name:        layerConfig.Name,
		Description: layerConfig.Description,
		Minzoom:     layerConfig.Minzoom,
		Maxzoom:     layerConfig.Maxzoom,
	}
	if layerConfig.Source.Elasticsearch != nil {
		source, err := NewElasticsearchSource(layerConfig.Source.Elasticsearch)
		if err != nil {
			return nil, err
		}
		layer.Source = source
		return layer, nil
	}
	return nil, fmt.Errorf("Invalid layer source config for layer: %s", layerConfig.Name)
}
