# tilenol

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
      --help                Show context-sensitive help (also try --help-long and --help-man).
  -e, --es-host="localhost:9200"
                            ElasticSearch host-port
  -m, --es-mappings=_all=geometry ...
                            ElasticSearch index name to geo-field mappings
  -Z, --zoom-ranges=_all=0-18 ...
                            ElasticSearch index name to zoom range mappings
  -p, --port=3000           Port to serve tiles on
  -i, --internal-port=3001  Port for internal metrics and healthchecks
  -x, --enable-cors         Enables cross-origin resource sharing (CORS)
  -c, --cache-control="no-cache"
                            Sets the "Cache-Control" header
  -n, --num-processes=0     Sets the number of processes to be used
```

## Contributing

When contributing to this repository, please follow the steps below:

1. Fork the repository
1. Submit your patch in one commit, or a series of well-defined commits
1. Submit your pull request and make sure you reference the issue you are addressing
