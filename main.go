package main

import (
	tilenol "github.com/jerluc/tilenol/lib"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	runCmd = kingpin.
		Command("run", "Runs the Tilenol server")
	esHost = runCmd.
		Flag("es-host", "ElasticSearch host-port").
		Envar("ES_HOST").
		Short('e').
		Default("localhost:9200").
		String()
	esMap = runCmd.
		Flag("es-mappings", "ElasticSearch index name to geo-field mappings").
		Envar("ES_MAPPINGS").
		Short('m').
		Default("_all=geometry").
		StringMap()
	port = runCmd.
		Flag("port", "Port to serve tiles on").
		Envar("PORT").
		Short('p').
		Default("3000").
		Uint16()
	internalPort = runCmd.
			Flag("internal-port", "Port for internal metrics and healthchecks").
			Envar("INTERNAL_PORT").
			Short('i').
			Default("3001").
			Uint16()
	cors = runCmd.
		Flag("enable-cors", "Enables cross-origin resource sharing (CORS)").
		Envar("ENABLE_CORS").
		Short('x').
		Bool()
	gzip = runCmd.
		Flag("enable-gzip", "Enables GZIPed responses").
		Envar("ENABLE_GZIP").
		Short('z').
		Bool()
	cache = runCmd.
		Flag("cache-control", "Sets the \"Cache-Control\" header").
		Envar("CACHE_CONTROL").
		Short('c').
		Default("max-age=315360000").
		String()
	versionCmd = kingpin.
			Command("version", "Prints out the version")
)

func main() {
	cmd := kingpin.Parse()

	switch cmd {
	case runCmd.FullCommand():
		(&tilenol.Server{
			Port:          *port,
			InternalPort:  *internalPort,
			EnableCORS:    *cors,
			GzipResponses: *gzip,
			CacheControl:  *cache,
			ESHost:        *esHost,
			ESMappings:    *esMap,
		}).Start()
	case versionCmd.FullCommand():
		PrintVersionInfo()
	}
}
