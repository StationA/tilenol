package tilenol

import (
	"context"

	"github.com/paulmach/orb/geojson"
)

type NilSource struct{}

func (n *NilSource) GetFeatures(ctx context.Context, req *TileRequest) (*geojson.FeatureCollection, error) {
	return geojson.NewFeatureCollection(), nil
}
