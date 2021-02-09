package tilenol

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	// SQL deps
	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	_ "github.com/lib/pq"
	// Geo deps
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
)

const (
	CTEName = "__tilenol__table"
	// TODO: Externalize this?
	QueryTimeout = 30 * time.Second
)

var (
	InvalidTableConfig = errors.New("Either \"tableExpression\" or \"table\" + \"schema\" can be set, not both.")
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
	// TableExpression is a valid SQL query that is used as an alternative to Schema and Table
	TableExpression string `yaml:"tableExpression"`
	// GeometryField is the name of the column that holds the feature geometry
	GeometryField string `yaml:"geometryField"`
	// SourceFields is a mapping from the feature property name to the source row
	// column names
	SourceFields map[string]string `yaml:"sourceFields"`
}

// Dataset constructs a CTE-based SelectDataset to be used as the source table for all request-time
// queries
func (c *PostGISConfig) Dataset() (*goqu.SelectDataset, error) {
	// Ensure that table configuration makes sense
	if c.TableExpression != "" && (c.Schema != "" || c.Table != "") {
		return nil, InvalidTableConfig
	}

	var table goqu.Expression
	if c.Table != "" {
		var relation = goqu.T(c.Table)
		if c.Schema != "" {
			relation = relation.Schema(c.Schema)
		}
		table = goqu.From(relation)
	} else if c.TableExpression != "" {
		var tableExp = strings.TrimSpace(c.TableExpression)
		if !strings.HasPrefix(tableExp, "(") {
			tableExp = fmt.Sprintf("(%s)", tableExp)
		}
		table = goqu.Literal(tableExp)
	}
	return goqu.From(CTEName).With(CTEName, table), nil
}

// PostGISSource is a Source implementation that retrieves feature data from a
// PostGIS server
type PostGISSource struct {
	DB            *goqu.Database
	Dataset       *goqu.SelectDataset
	GeometryField string
	SourceFields  map[string]string
}

// CheckPing asserts that we can ping the connected database
func CheckPing(db *sql.DB) error {
	if connErr := db.Ping(); connErr != nil {
		return connErr
	}
	return nil
}

// CheckReadOnly warns if the current database connection is capable of read-write transactions
func CheckReadOnly(db *sql.DB) error {
	var readOnlyCheck string
	row := db.QueryRow("SHOW transaction_read_only")
	if err := row.Scan(&readOnlyCheck); err != nil {
		return err
	}
	if readOnlyCheck == "off" {
		Logger.Warnf("Using a read-write database connection may expose your server to security " +
			"vulnerabilities. Please consider using a read-only user for this connection.")
	}
	return nil
}

// NewPostGISSource creates a new Source that retrieves feature data from a
// PostGIS server
func NewPostGISSource(config *PostGISConfig) (Source, error) {
	// Open the database connection
	pgDB, pgErr := sql.Open("postgres", config.DSN)
	if pgErr != nil {
		return nil, pgErr
	}

	// Check to make sure we can ping the database
	if err := CheckPing(pgDB); err != nil {
		return nil, err
	}

	// Also check to see if we are using a read-only connection
	if err := CheckReadOnly(pgDB); err != nil {
		return nil, err
	}

	// Create the base select dataset for request-time queries
	dataset, err := config.Dataset()
	if err != nil {
		return nil, err
	}

	return &PostGISSource{
		DB:            goqu.Dialect("postgres").DB(pgDB),
		Dataset:       dataset,
		GeometryField: config.GeometryField,
		SourceFields:  config.SourceFields,
	}, nil
}

// Creates a new PostGISSource from the input object, but adds extra SourceFields
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
		Dataset:       p.Dataset,
		GeometryField: p.GeometryField,
		SourceFields:  sourceFields,
	}
}

// Constructs a raw SQL statement from the tile request parameters
func (p *PostGISSource) buildSQL(bounds orb.Bound, extraFilters ...goqu.Expression) (string, error) {
	// Create the base query from the provided table or table expression
	var q = p.Dataset.Clone().(*goqu.SelectDataset)

	// Add the columns we want to select out of the table
	var selectColumns = []interface{}{
		goqu.Func("ST_AsBinary", goqu.I(p.GeometryField)).As(p.GeometryField),
	}
	for dst, src := range p.SourceFields {
		sourceColExpression := goqu.L(src).As(dst)
		selectColumns = append(selectColumns, sourceColExpression)
	}
	q = q.Select(selectColumns...)

	// Create an envelope expression from the tile bounds
	envelope := goqu.Func("ST_MakeEnvelope",
		bounds.Min.X(),
		bounds.Min.Y(),
		bounds.Max.X(),
		bounds.Max.Y(),
		4326)
	// Add a geo-bounds WHERE clause to the query
	geoBoundsExpression := goqu.Func("ST_Intersects", goqu.I(p.GeometryField), envelope)
	q = q.Where(geoBoundsExpression)

	// Add any extra request-time filter expressions to the WHERE clause of the query
	q = q.Where(extraFilters...)

	// Lastly, compile and return the results
	sql, _, err := q.ToSQL()
	if err != nil {
		return "", err
	}
	return sql, nil
}

// Actually runs the compiled SQL query, and returns a list of mapped records upon success
func (p *PostGISSource) runQuery(ctx context.Context, q string) ([]map[string]interface{}, error) {
	// Create a cancellable context using a timeout
	qCtx, qCancel := context.WithTimeout(ctx, QueryTimeout)
	defer qCancel()

	// Use a read-only transaction to ensure that we can't execute write operations to the database
	txOps := &sql.TxOptions{ReadOnly: true}
	tx, err := p.DB.BeginTx(qCtx, txOps)
	// Note that the database backend will rollback the transaction upon context cancellation.
	if err != nil {
		return nil, err
	}

	// Actually execute the query
	Logger.Debugf("Executing SQL: %s\n", q)
	rows, err := tx.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Re-map the row objects to a list of map-like records
	records, err := RowsToMaps(rows, p.GeometryField)
	if err != nil {
		return nil, err
	}
	return records, err
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

	// Also check extra source filtering ("q" parameter)
	var extraFilters []goqu.Expression
	if qs, exists := req.Args["q"]; exists && len(qs) > 0 {
		for _, q := range qs {
			extraFilters = append(extraFilters, goqu.Literal(q))
		}
	}

	// Create the final SQL query
	q, err := p.buildSQL(req.MapTile().Bound(), extraFilters...)
	if err != nil {
		return nil, err
	}

	// Execute the SQL query and retrieve the mapped records
	records, err := p.runQuery(ctx, q)
	if err != nil {
		return nil, err
	}

	// Then turn each record into a feature
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
