#!/bin/sh

set -ex

curl -X PUT -H 'Content-Type: application/json' -d @schema.json localhost:9200/buildings

curl -X POST -H 'Content-Type: application/x-ndjson' --data-binary @data.ndjson localhost:9200/_bulk
