# Tilenol example

This directory contains a basic setup for trying out Tilenol locally using a simple Mapbox GL JS
integration on a webpage.

## Without installing Tilenol

1. Run `docker-compose up` to bring up both a local Tilenol and ElasticSearch instance, waiting until
   Tilenol shows that it is up and serving before continuing
2. Run the `create_index.sh` script to create a new ElasticSearch index (`buildings`) with some data
   pre-populated
3. Replace the string `<YOUR_MAPBOX_ACCESS_TOKEN>` in the `index.html` file
4. Open `index.html` in your browser of choice to view the vector building shapes

## With Tilenol already installed

1. Run `docker-compose up es` to bring up only a local ElasticSearch instance
2. Run `tilenol run -x` to start a local Tilenol server (note the `-x` to enable broad CORS support)
3. Run the `create_index.sh` script to create a new ElasticSearch index (`buildings`) with some data
   pre-populated
4. Replace the string `<YOUR_MAPBOX_ACCESS_TOKEN>` in the `index.html` file
5. Open `index.html` in your browser of choice to view the vector building shapes
