package server

import (
	"context"
	"fmt"

	"github.com/paulmach/orb/geojson"
)

type Layer struct {
	Name        string
	Description string
	Minzoom     int
	Maxzoom     int
	Source      Source
}

type Source interface {
	GetFeatures(context.Context) (*geojson.FeatureCollection, error)
}

func CreateLayer(layerConfig LayerConfig) (*Layer, error) {
	layer := &Layer{
		Name:        layerConfig.Name,
		Description: layerConfig.Description,
		Minzoom:     layerConfig.Minzoom,
		Maxzoom:     layerConfig.Maxzoom,
	}
	if layerConfig.Elasticsearch != nil {
		source, err := NewElasticsearchSource(layerConfig.Elasticsearch)
		if err != nil {
			return nil, err
		}
		layer.Source = source
		return layer, nil
	}
	return nil, fmt.Errorf("Invalid layer config for layer: %s", layerConfig.Name)
}

func (l *Layer) GetFeatures(ctx context.Context) (*geojson.FeatureCollection, error) {
	return l.Source.GetFeatures(ctx)
}
