#!/bin/sh

docker run \
  --name=tilenol-es \
  -p 9200:9200 \
  -p 9300:9300 \
  -e "discovery.type=single-node" \
  -e "xpack.security.transport.ssl.enabled=false" \
  -e "xpack.security.http.ssl.enabled=false" \
  -e "xpack.license.self_generated.type=trial" \
  -e "ELASTIC_PASSWORD=elastic" \
  -d \
  docker.elastic.co/elasticsearch/elasticsearch:8.14.3


until curl --silent "http://elastic:elastic@localhost:9200" ; do
    echo "Waiting for ES cluster to come up..."
    sleep 2
done
