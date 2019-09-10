tilenol [![GoDoc](https://godoc.org/github.com/StationA/tilenol?status.svg)](https://godoc.org/github.com/StationA/tilenol) [![Go Report Card](https://goreportcard.com/badge/github.com/stationa/tilenol)](https://goreportcard.com/report/github.com/stationa/tilenol) [![Build Status](https://api.travis-ci.com/StationA/tilenol.svg?branch=master)](https://travis-ci.com/StationA/tilenol)
=========

Tilenol is a scalable web server for serving geospatial data stored in an ElasticSearch cluster as
Mapbox Vector Tiles.

## Installation

Navigate to the root `tilenol/` directory (where the `Makefile` is located) and run:

```
make install
```

## Usage

### `tilenol`

```
usage: tilenol [<flags>] <command> [<args> ...]

Flags:
  --help  Show context-sensitive help (also try --help-long and --help-man).

Commands:
  help [<command>...]
    Show help.

  run [<flags>]
    Runs the Tilenol server

  version
    Prints out the version
```

### `tilenol run`

```
usage: tilenol run [<flags>]

Runs the Tilenol server

Flags:
      --help                 Show context-sensitive help (also try --help-long and --help-man).
  -d, --debug                    Enable debug mode
  -f, --config-file=tilenol.yml  Server configuration file
  -p, --port=3000                Port to serve tiles on
  -i, --internal-port=3001       Port for internal metrics and healthchecks
  -x, --enable-cors              Enables cross-origin resource sharing (CORS)
  -s, --simplify-shapes          Simplifies geometries based on zoom level
  -n, --num-processes=0          Sets the number of processes to be used
```

### Configuration

```yaml
# Cache configuration (optional)
cache:
  redis:
    host: localhost
    port: 6379
    ttl: 24h
# Layer configuration
layers:
  - name: buildings
    minzoom: 14
    source:
      elasticsearch:
        host: localhost
        port: 9200
        index: buildings
        geometryField: geometry
        sourceFields:
          area_sqft: building.area_sqft
          height_ft: building.height_ft
```

## Contributing

When contributing to this repository, please follow the steps below:

1. Fork the repository
1. Submit your patch in one commit, or a series of well-defined commits
1. Submit your pull request and make sure you reference the issue you are addressing
