package tilenol

import (
	"context"

	"github.com/paulmach/orb/geojson"
)

// NilSource implements the Source interface with no data
type NilSource struct{}

// GetFeatures returns no data for all requests
func (n *NilSource) GetFeatures(ctx context.Context, req *TileRequest) (*geojson.FeatureCollection, error) {
	return geojson.NewFeatureCollection(), nil
}
