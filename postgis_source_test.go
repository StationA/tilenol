package tilenol

import (
	"strings"
	"testing"

	"github.com/paulmach/orb"
)

func TestConfigTableAndSchemaDataset(t *testing.T) {
	tableAndSchema := &PostGISConfig{
		Schema: "my_schema",
		Table:  "my_locations",
	}
	ds, err := tableAndSchema.Dataset()
	if err != nil {
		t.Errorf("Couldn't create dataset from config: %v", err)
	}
	sql, _, err := ds.ToSQL()
	if err != nil {
		t.Errorf("Couldn't create SQL from config: %v", err)
	}
	if !strings.Contains(sql, "SELECT * FROM \"my_schema\".\"my_locations\"") {
		t.Errorf("Invalid SQL from config: %v", sql)
	}
}

func TestConfigTableExpressionDataset(t *testing.T) {
	tableExp := &PostGISConfig{
		TableExpression: "SELECT * FROM \"my_other_schema\".\"my_locations\"",
	}
	ds, err := tableExp.Dataset()
	if err != nil {
		t.Errorf("Couldn't create dataset from config: %v", err)
	}
	sql, _, err := ds.ToSQL()
	if err != nil {
		t.Errorf("Couldn't create SQL from config: %v", err)
	}
	if !strings.Contains(sql, "SELECT * FROM \"my_other_schema\".\"my_locations\"") {
		t.Errorf("Invalid SQL from config: %v", sql)
	}
}

func TestConfigCheckTable(t *testing.T) {
	tableAndExp := &PostGISConfig{
		Table:           "oops_my_table",
		TableExpression: "SELECT * FROM \"my_other_schema\".\"my_locations\"",
	}
	_, err := tableAndExp.Dataset()
	if err == nil {
		t.Error("PostGISConfig should not allow both table and tableExpression")
	}
}

func TestExtraFields(t *testing.T) {
	pgis := &PostGISSource{SourceFields: map[string]string{
		"id":   "id",
		"name": "name",
	}}
	newPGIS := pgis.withExtraFields(map[string]string{
		"new": "new",
	})
	if _, exists := newPGIS.SourceFields["new"]; !exists {
		t.Error("PostGISSource.withExtraFields() should add additional source fields")
	}
}

func TestSQLConstruction(t *testing.T) {
	tableAndSchema := &PostGISConfig{
		Schema: "my_schema",
		Table:  "my_locations",
	}
	ds, err := tableAndSchema.Dataset()
	if err != nil {
		t.Errorf("Couldn't create dataset from config: %v", err)
	}
	pgis := &PostGISSource{
		Dataset:       ds,
		GeometryField: "centroid",
		SourceFields: map[string]string{
			"id":   "id",
			"name": "name",
		}}
	tile := orb.Bound{Min: orb.Point{0.0, 0.0}, Max: orb.Point{1.0, 1.0}}
	sql, err := pgis.buildSQL(tile)
	if err != nil {
		t.Errorf("Failed to construct SQL: %v", err)
	}
	for col, _ := range pgis.SourceFields {
		if !strings.Contains(sql, col) {
			t.Errorf("Constructed SQL is missing column: %v", col)
		}
	}
	if !strings.Contains(sql, "ST_Intersects(") {
		t.Errorf("Constructed SQL lacks intersection query: %v", sql)
	}
}
