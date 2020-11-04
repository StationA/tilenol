#!/bin/bash

set -xe

psql -U postgres -h localhost -c "CREATE SCHEMA IF NOT EXISTS tilenol"
psql -U postgres -h localhost -c "CREATE TABLE IF NOT EXISTS tilenol.buildings (id TEXT, name TEXT, height FLOAT, geometry GEOMETRY(POLYGON, 4326))"
psql -U postgres -h localhost -c "CREATE TABLE IF NOT EXISTS tilenol.buildings_imported (id TEXT, name TEXT, height FLOAT, geometry TEXT)"
psql -U postgres -h localhost -c "\COPY tilenol.buildings_imported FROM 'data.csv' DELIMITER ',' CSV HEADER"
psql -U postgres -h localhost -c "INSERT INTO tilenol.buildings SELECT id, name, height, ST_GeomFromGeoJSON(geometry) AS geometry FROM tilenol.buildings_imported"
psql -U postgres -h localhost -c "DROP TABLE tilenol.buildings_imported"
