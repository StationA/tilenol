# postgis

This example configures tilenol to use a PostGIS backend to service tile requests for our map.

## Setup

### Start a local PostGIS database

To start a locally-running PostGIS database, you can run the official Docker image:

```shell
docker run -it -p 5432:5432 -e POSTGRES_HOST_AUTH_METHOD=trust postgis/postgis
```

### Load sample data into your database

Once your database is ready to receive connections, you can load some sample data for testing:

```shell
./create_table.sh
```

### Start tilenol

Lastly, start tilenol:

```shell
tilenol run -x
```
