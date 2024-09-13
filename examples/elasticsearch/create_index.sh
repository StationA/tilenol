#!/bin/sh

set -ex

curl -X PUT -H 'Content-Type: application/json' -d @schema.json http://elastic:elastic@localhost:9200/buildings

curl -X POST -H 'Content-Type: application/x-ndjson' --data-binary @data.ndjson http://elastic:elastic@localhost:9200/_bulk
