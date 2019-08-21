package main

import (
	"fmt"
	"runtime"

	"github.com/sirupsen/logrus"
	"github.com/stationa/tilenol/server"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	runCmd = kingpin.
		Command("run", "Runs the Tilenol server")
	debug = runCmd.
		Flag("debug", "Enable debug mode").
		Short('d').
		Bool()
	configFile = runCmd.
			Flag("config-file", "Server configuration file").
			Envar("TILENOL_CONFIG_FILE").
			Short('f').
			Default("tilenol.yml").
			File()
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
	simplify = runCmd.
			Flag("simplify-shapes", "Simplifies geometries based on zoom level").
			Envar("TILENOL_SIMPLIFY_SHAPES").
			Short('s').
			Bool()
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

// Version is the tilenol version string
var Version string

// Commitish is the Git commit-ish for this binary
var Commitish string

func printVersionInfo() {
	fmt.Printf("tilenol version=%s (%s)\n", Version, Commitish)
}

func main() {
	if *numProcs < 1 {
		*numProcs = runtime.NumCPU()
	}
	runtime.GOMAXPROCS(*numProcs)

	cmd := kingpin.Parse()

	switch cmd {
	case runCmd.FullCommand():
		if *debug {
			server.Logger.SetLevel(logrus.DebugLevel)
		}

		var opts []server.ConfigOption
		opts = append(opts, server.Port(*port))
		opts = append(opts, server.InternalPort(*internalPort))
		opts = append(opts, server.ConfigFile(*configFile))
		if *cors {
			opts = append(opts, server.EnableCORS)
		}
		if *simplify {
			opts = append(opts, server.SimplifyShapes)
		}

		s, err := server.NewServer(opts...)
		if err != nil {
			panic(err)
		}
		s.Start()
	case versionCmd.FullCommand():
		printVersionInfo()
	}
}
