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
		Envar("TILENOL_ES_HOST").
		Short('e').
		Default("localhost:9200").
		String()
	esMap = runCmd.
		Flag("es-mappings", "ElasticSearch index name to geo-field mappings").
		Short('m').
		Default("_all=geometry").
		StringMap()
	esSource = runCmd.
			Flag("es-sources", "ElasticSearch index name to source field pattern mappings").
			Short('s').
			Default("_all=geometry").
			StringMap()
	port = runCmd.
		Flag("port", "Port to serve tiles on").
		Envar("TILENOL_PORT").
		Short('p').
		Default("3000").
		Uint16()
	internalPort = runCmd.
			Flag("internal-port", "Port for internal metrics and healthchecks").
			Envar("TILENOL_INTERNAL_PORT").
			Short('i').
			Default("3001").
			Uint16()
	cors = runCmd.
		Flag("enable-cors", "Enables cross-origin resource sharing (CORS)").
		Envar("TILENOL_ENABLE_CORS").
		Short('x').
		Bool()
	gzip = runCmd.
		Flag("enable-gzip", "Enables GZIPed responses").
		Envar("TILENOL_ENABLE_GZIP").
		Short('z').
		Bool()
	cache = runCmd.
		Flag("cache-control", "Sets the \"Cache-Control\" header").
		Envar("TILENOL_CACHE_CONTROL").
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
		server := &tilenol.Server{}
		server.Port = *port
		server.InternalPort = *internalPort
		server.EnableCORS = *cors
		server.GzipResponses = *gzip
		server.CacheControl = *cache
		server.ESHost = *esHost
		server.ESMappings = *esMap
		server.ESSource = *esSource
		server.Start()
	case versionCmd.FullCommand():
		PrintVersionInfo()
	}
}
