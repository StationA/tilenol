package main

import (
	"fmt"
	"runtime"

	"github.com/stationa/tilenol/server"
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
	zoomRanges = runCmd.
			Flag("zoom-ranges", "ElasticSearch index name to zoom range mappings").
			Short('Z').
			Default("_all=0-18").
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
	cache = runCmd.
		Flag("cache-control", "Sets the \"Cache-Control\" header").
		Envar("TILENOL_CACHE_CONTROL").
		Short('c').
		Default("no-cache").
		String()
	numProcs = runCmd.
			Flag("num-processes", "Sets the number of processes to be used").
			Envar("TILENOL_NUM_PROCESSES").
			Short('n').
			Default("0").
			Int()
	versionCmd = kingpin.
			Command("version", "Prints out the version")
)

// Auto-filled by build
var Version string
var Commitish string

func PrintVersionInfo() {
	fmt.Printf("tilenol version=%s (%s)\n", Version, Commitish)
}

func main() {
	numCores := runtime.NumCPU()
	if *numProcs < 1 {
		*numProcs = numCores
	}
	runtime.GOMAXPROCS(numCores)

	cmd := kingpin.Parse()

	switch cmd {
	case runCmd.FullCommand():
		var opts []server.ConfigOption
		opts = append(opts, server.Port(*port))
		opts = append(opts, server.InternalPort(*internalPort))
		opts = append(opts, server.CacheControl(*cache))
		opts = append(opts, server.ESHost(*esHost))
		opts = append(opts, server.ESMappings(*esMap))
		opts = append(opts, server.ZoomRanges(*zoomRanges))
		if *cors {
			opts = append(opts, server.EnableCORS)
		}

		s, err := server.NewServer(opts...)
		if err != nil {
			panic(err)
		}
		s.Start()
	case versionCmd.FullCommand():
		PrintVersionInfo()
	}
}
