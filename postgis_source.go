package tilenol

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"text/template"

	// SQL deps
	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	_ "github.com/lib/pq"
	// Geo deps
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	// "github.com/paulmach/orb/maptile"
)

const (
	// TODO: Should the whole DSN be configurable?
	DSNTemplate = "host={{.Host}} port={{.Port}} dbname={{.Database}} user={{.User}} password='{{.Password}}' sslmode=disable"
)

// PostGISConfig is the YAML configuration structure for configuring a new
// PostGISSource
type PostGISConfig struct {
	// Host is the hostname part of the backend PostGIS server
	Host string `yaml:"host"`
	// Host is the port number of the backend PostGIS server
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Schema   string `yaml:"schema"`
	Table    string `yaml:"table"`
	// GeometryField is the name of the column that holds the feature geometry
	GeometryField string `yaml:"geometryField"`
	// SourceFields is a mapping from the feature property name to the source row
	// column names
	SourceFields map[string]string `yaml:"sourceFields"`
}

func (c *PostGISConfig) DSN() (string, error) {
	var buf bytes.Buffer
	dsnTemplate := template.Must(template.New("").Parse(DSNTemplate))
	if err := dsnTemplate.Execute(&buf, c); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// PostGISSource is a Source implementation that retrieves feature data from a
// PostGIS server
type PostGISSource struct {
	DB            *goqu.Database
	Schema        string
	Table         string
	GeometryField string
}

// NewPostGISSource creates a new Source that retrieves feature data from a
// PostGIS server
func NewPostGISSource(config *PostGISConfig) (Source, error) {
	dialect := goqu.Dialect("postgres")
	dsn, dsnErr := config.DSN()
	if dsnErr != nil {
		return nil, dsnErr
	}
	pgDB, pgErr := sql.Open("postgres", dsn)
	if pgErr != nil {
		return nil, pgErr
	}
	if connErr := pgDB.Ping(); connErr != nil {
		return nil, connErr
	}
	db := dialect.DB(pgDB)
	return &PostGISSource{DB: db, Schema: config.Schema, Table: config.Table, GeometryField: config.GeometryField}, nil
}

// Create a new PostGISSource from the input object, but add extra SourceFields
// to include to the new PostGISSource instance.
func (p *PostGISSource) withExtraFields(extraFields map[string]string) *PostGISSource {
	return p
}

// GetFeatures implements the Source interface, to get feature data from an
// PostGIS server
func (p *PostGISSource) GetFeatures(ctx context.Context, req *TileRequest) (*geojson.FeatureCollection, error) {
	bounds := req.MapTile().Bound()
	envelope := fmt.Sprintf("ST_MakeEnvelope(%f, %f, %f, %f, 4326)", bounds.Min.X(), bounds.Min.Y(), bounds.Max.X(), bounds.Max.Y())
	baseQuery := goqu.From(fmt.Sprintf("%s.%s", p.Schema, p.Table)).Select(goqu.L(fmt.Sprintf("ST_AsEWKT(%s) AS geometry", p.GeometryField)))
	geoBounds := baseQuery.Where(goqu.L(fmt.Sprintf("ST_Intersects(%s, %s)", p.GeometryField, envelope)))
	sql, args, _ := geoBounds.ToSQL()
	fmt.Println(sql)
	fmt.Println(args)
	rows, err := p.DB.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records, err := RowsToMaps(rows)
	if err != nil {
		return nil, err
	}
	for _, r := range records {
		for k, v := range r {
			fmt.Println(k)
			fmt.Printf("%#v\n", v)
		}
		var geom orb.Geometry
		rawGeojson := r[p.GeometryField].(string)
		if err := json.Unmarshal([]byte(rawGeojson), &geom); err != nil {
			return nil, err
		}
		fmt.Println(geom)
	}
	return nil, fmt.Errorf("Not implemented")
}
