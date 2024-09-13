# Elasticsearch example

This example configures tilenol to use an Elasticsearch backend to service tile requests for our map.

## Setup

### Start a local Elasticsearch cluster

To start a locally-running Elasticsearch cluster, you can run the official Docker image via the
helper script:

```shell
./start_local_es.sh
```

### Load sample data into your cluster

Once your search cluster is up and running, you can load some sample data for testing:

```shell
./create_index.sh
```

### Start tilenol

Lastly, start tilenol:

```shell
tilenol run -x
```
