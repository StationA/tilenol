package tilenol

import (
	"context"
	"database/sql"

	// SQL deps
	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	_ "github.com/lib/pq"
	// Geo deps
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	// "github.com/paulmach/orb/maptile"
)

// PostGISConfig is the YAML configuration structure for configuring a new
// PostGISSource
type PostGISConfig struct {
	// DSN is the "data source name" that specifies how to connect to the database server
	DSN string `yaml:"dsn"`
	// Schema is the table space to use for queries
	Schema string `yaml:"schema"`
	// Table is the name of the table to use for queries
	Table string `yaml:"table"`
	// GeometryField is the name of the column that holds the feature geometry
	GeometryField string `yaml:"geometryField"`
	// SourceFields is a mapping from the feature property name to the source row
	// column names
	SourceFields map[string]string `yaml:"sourceFields"`
}

// PostGISSource is a Source implementation that retrieves feature data from a
// PostGIS server
type PostGISSource struct {
	DB            *goqu.Database
	Schema        string
	Table         string
	GeometryField string
	SourceFields  map[string]string
}

// NewPostGISSource creates a new Source that retrieves feature data from a
// PostGIS server
func NewPostGISSource(config *PostGISConfig) (Source, error) {
	dialect := goqu.Dialect("postgres")
	pgDB, pgErr := sql.Open("postgres", config.DSN)
	if pgErr != nil {
		return nil, pgErr
	}
	if connErr := pgDB.Ping(); connErr != nil {
		return nil, connErr
	}
	db := dialect.DB(pgDB)
	return &PostGISSource{
		DB:            db,
		Schema:        config.Schema,
		Table:         config.Table,
		GeometryField: config.GeometryField,
		SourceFields:  config.SourceFields,
	}, nil
}

// Create a new PostGISSource from the input object, but add extra SourceFields
// to include to the new PostGISSource instance.
func (p *PostGISSource) withExtraFields(extraFields map[string]string) *PostGISSource {
	sourceFields := make(map[string]string)
	for k, v := range p.SourceFields {
		sourceFields[k] = v
	}
	for k, v := range extraFields {
		sourceFields[k] = v
	}
	return &PostGISSource{
		DB:            p.DB,
		Schema:        p.Schema,
		Table:         p.Table,
		GeometryField: p.GeometryField,
		SourceFields:  sourceFields,
	}
}

func (p *PostGISSource) buildSQL(bounds orb.Bound) (string, error) {
	envelope := goqu.Func("ST_MakeEnvelope",
		bounds.Min.X(),
		bounds.Min.Y(),
		bounds.Max.X(),
		bounds.Max.Y(),
		4326)
	whereClause := goqu.Func("ST_Intersects", goqu.I(p.GeometryField), envelope)
	var selectColumns = []interface{}{
		goqu.Func("ST_AsBinary", goqu.I(p.GeometryField)).As(p.GeometryField),
	}
	for dst, src := range p.SourceFields {
		sourceColExpression := goqu.L(src).As(dst)
		selectColumns = append(selectColumns, sourceColExpression)
	}
	var relation = goqu.T(p.Table)
	if p.Schema != "" {
		relation = relation.Schema(p.Schema)
	}
	baseQuery := goqu.From(relation).Select(selectColumns...)
	geoBounds := baseQuery.Where(whereClause)
	sql, _, err := geoBounds.ToSQL()
	if err != nil {
		return "", err
	}
	return sql, nil
}

// GetFeatures implements the Source interface, to get feature data from an
// PostGIS server
func (p *PostGISSource) GetFeatures(ctx context.Context, req *TileRequest) (*geojson.FeatureCollection, error) {
	// Check for extra fields specifications. They must have the form of <property_name>:<SQL column expression>,
	// eg: height_times_two:height*2.
	if inc_args, exists := req.Args["s"]; exists {
		extraFields, err := makeFieldMap(inc_args)
		if err != nil {
			return nil, err
		}
		// Instead of the original PostGISSource use one that is augmented with the extra
		// source field requests for the remainder of this request.
		p = p.withExtraFields(extraFields)
	}

	sql, err := p.buildSQL(req.MapTile().Bound())
	if err != nil {
		return nil, err
	}
	Logger.Debugf("Executing SQL: %s\n", sql)
	rows, err := p.DB.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records, err := RowsToMaps(rows)
	if err != nil {
		return nil, err
	}
	fc := geojson.NewFeatureCollection()
	for _, r := range records {
		geom := r[p.GeometryField].(orb.Geometry)
		feature := geojson.NewFeature(geom)
		for k, v := range r {
			// Special-case the feature ID
			if k == "id" {
				feature.ID = v
			}
			// Omit the geometry field and null values
			if k != p.GeometryField && v != nil {
				feature.Properties[k] = v
			}
		}
		fc.Append(feature)
	}
	return fc, nil
}
