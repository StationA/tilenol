# Elasticsearch example

This example configures tilenol to use an Elasticsearch backend to service tile requests for our map.

## Setup

### Start a local Elasticsearch cluster

To start a locally-running Elasticsearch cluster, you can run the official Docker image:

```shell
docker run -it -p 9200:9200 -p 9300:9300 -e discovery.type=single-node docker.elastic.co/elasticsearch/elasticsearch:6.5.1
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

## Alternative Dev Environment Setup with `docker-compose`

To start elasticsearch and tilenol together via docker-compose:

```shell
#build tilenol image
docker-compose build
#start up elasticsearch and tilenol containers 
docker-compose up
#load sample data
./create_index.sh
```

This could be extended to include the redis cache.