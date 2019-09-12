package tilenol

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/orb/simplify"
	"golang.org/x/sync/errgroup"
)

const (
	// MinZoom is the default minimum zoom for a layer
	MinZoom = 0
	// MaxZoom is the default maximum zoom for a layer
	MaxZoom = 22
	// MinSimplify is the minimum simplification radius
	MinSimplify = 1.0
	// MaxSimplify is the maximum simplification radius
	MaxSimplify = 10.0
	// AllLayers is the special request parameter for returning all source layers
	AllLayers = "_all"
)

// TileRequest is an object containing the tile request context
type TileRequest struct {
	X int
	Y int
	Z int
}

// MapTile creates a maptile.Tile object from the TileRequest
func (t *TileRequest) MapTile() maptile.Tile {
	return maptile.New(uint32(t.X), uint32(t.Y), maptile.Zoom(t.Z))
}

// Server is a tilenol server instance
type Server struct {
	// Port is the port number to bind the tile server
	Port uint16
	// InternalPort is the port number to bind the internal metrics endpoints
	InternalPort uint16
	// EnableCORS configures whether or not the tile server responds with CORS headers
	EnableCORS bool
	// Simplify configures whether or not the tile server simplifies outgoing feature
	// geometries based on zoom level
	Simplify bool
	// Layers is the list of configured layers supported by the tile server
	Layers []Layer
	// Cache is an optional cache object that the server uses to cache responses
	Cache Cache
}

// Handler is a type alias for a more functional HTTP request handler
type Handler func(context.Context, io.Writer, *http.Request) error

// NewServer creates a new server instance pre-configured with the given ConfigOption's
func NewServer(configOpts ...ConfigOption) (*Server, error) {
	s := &Server{}
	for _, opt := range configOpts {
		err := opt(s)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Server) setupRoutes() (*chi.Mux, *chi.Mux) {
	r := chi.NewRouter()

	//-- MIDDLEWARE
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	logFormatter := &middleware.DefaultLogFormatter{Logger: Logger, NoColor: true}
	r.Use(middleware.RequestLogger(logFormatter))
	r.Use(middleware.Recoverer)

	if s.EnableCORS {
		Logger.Infoln("Enabling CORS support")
		cors := cors.New(cors.Options{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Accept-Encoding", "Authorization"},
			AllowCredentials: true,
		})
		r.Use(cors.Handler)
	}

	//-- ROUTES
	r.Get("/{layers}/{z}/{x}/{y}.mvt", s.cached(s.getVectorTile))

	// TODO: Add GeoJSON endpoint?

	i := chi.NewRouter()
	i.Get("/healthcheck", s.healthCheck)

	return r, i
}

// Start actually starts the server instance. Note that this blocks until an interrupting signal
func (s *Server) Start() {
	r, i := s.setupRoutes()

	go func() {
		log.Fatalln(http.ListenAndServe(fmt.Sprintf(":%d", s.Port), r))
	}()

	go func() {
		log.Fatalln(http.ListenAndServe(fmt.Sprintf(":%d", s.InternalPort), i))
	}()

	Logger.Infof("Tilenol server up and running @ 0.0.0.0:[%d,%d]", s.Port, s.InternalPort)

	select {}
}

// healthCheck implements a simple healthcheck endpoint for the internal metrics server
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	// TODO: Maybe in the future check that ES is reachable?
	fmt.Fprintf(w, "OK")
}

// calculateSimplificationThreshold determines the simplification threshold based on the
// current zoom level
func calculateSimplificationThreshold(minZoom, maxZoom, currentZoom int) float64 {
	s := MinSimplify - MaxSimplify
	z := float64(maxZoom - minZoom)
	p := s / z
	return p*float64(currentZoom-minZoom) + MaxSimplify
}

// cached is a wrapper function that optionally tries to cache outgoing responses from
// a Handler
func (s *Server) cached(handler Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		defer func() {
			if ctx.Err() == context.Canceled {
				Logger.Debugf("Request canceled by client")
				w.WriteHeader(499)
				return
			}
		}()

		var buffer bytes.Buffer
		key := r.URL.RequestURI()
		if s.Cache.Exists(key) {
			Logger.Debugf("Key [%s] found in cache", key)
			val, err := s.Cache.Get(key)
			if err != nil {
				s.handleError(err.(error), w, r)
				return
			}
			buffer.Write(val)
		} else {
			Logger.Debugf("Key [%s] is not cached", key)
			herr := handler(ctx, &buffer, r)
			if herr != nil {
				return
			}
			err := s.Cache.Put(key, buffer.Bytes())
			if err != nil {
				// Log an error in case the key can't be stored in cache, but continue
				Logger.Warnf("Could not store key [%s] in cache: %v", key, err)
			}
		}
		// Set standard response headers
		// TODO: Use the cache TTL to determine the Cache-Control
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Header().Set("Content-Encoding", "gzip")
		// TODO: Store the content type somehow in the cache?
		w.Header().Set("Content-Type", "application/x-protobuf")
		io.Copy(w, &buffer)
	}
}

// filterLayersByNames filters the tile server layers by the names of layers being
// requested
func filterLayersByNames(inLayers []Layer, names []string) []Layer {
	var outLayers []Layer
	for _, name := range names {
		for _, layer := range inLayers {
			if layer.Name == name {
				outLayers = append(outLayers, layer)
			}
		}
	}
	return outLayers
}

// filterLayersByZoom filters the tile server layers by zoom level bounds
func filterLayersByZoom(inLayers []Layer, z int) []Layer {
	var outLayers []Layer
	for _, layer := range inLayers {
		if layer.Minzoom <= z && (layer.Maxzoom >= z || layer.Maxzoom == 0) {
			outLayers = append(outLayers, layer)
		}
	}
	return outLayers
}

// getVectorTile computes a vector tile response for the incoming request
func (s *Server) getVectorTile(rctx context.Context, w io.Writer, r *http.Request) error {
	z, _ := strconv.Atoi(chi.URLParam(r, "z"))
	x, _ := strconv.Atoi(chi.URLParam(r, "x"))
	y, _ := strconv.Atoi(chi.URLParam(r, "y"))
	requestedLayers := chi.URLParam(r, "layers")
	req := &TileRequest{x, y, z}

	var layersToCompute = filterLayersByZoom(s.Layers, z)
	if requestedLayers != AllLayers {
		layersToCompute = filterLayersByNames(layersToCompute, strings.Split(requestedLayers, ","))
	}

	// Create an errgroup with the request context so that we can get cancellable,
	// fork-join parallelism behavior
	eg, ctx := errgroup.WithContext(rctx)

	fcLayers := make(mvt.Layers, len(layersToCompute))
	for i, layer := range layersToCompute {
		i, layer := i, layer // Fun stuff: https://blog.cloudflare.com/a-go-gotcha-when-closures-and-goroutines-collide/
		eg.Go(func() error {
			Logger.Debugf("Retrieving vector tile for layer [%s] @ (%d, %d, %d)", layer.Name, x, y, z)
			fc, err := layer.Source.GetFeatures(ctx, req)
			if err != nil {
				return err
			}
			fcLayer := mvt.NewLayer(layer.Name, fc)
			fcLayer.Version = 2 // Set to tile spec v2
			fcLayer.ProjectToTile(req.MapTile())
			fcLayer.Clip(mvt.MapboxGLDefaultExtentBound)

			if s.Simplify {
				minZoom := layer.Minzoom
				maxZoom := layer.Maxzoom
				simplifyThreshold := calculateSimplificationThreshold(minZoom, maxZoom, z)
				Logger.Debugf("Simplifying @ zoom [%d], epsilon [%f]", z, simplifyThreshold)
				fcLayer.Simplify(simplify.DouglasPeucker(simplifyThreshold))
				fcLayer.RemoveEmpty(1.0, 1.0)
			}
			fcLayers[i] = fcLayer
			return nil
		})
	}
	// Wait for all of the goroutines spawned in this errgroup to complete or fail
	if err := eg.Wait(); err != nil {
		// If any of them fail, return the error
		return err
	}

	// Lastly, marshal the object into the response output
	data, marshalErr := mvt.MarshalGzipped(fcLayers)
	if marshalErr != nil {
		return marshalErr
	}
	_, err := w.Write(data)
	return err
}

// handleError is a helper function to generate a generic tile server error response
func (s *Server) handleError(err error, w http.ResponseWriter, r *http.Request) {
	Logger.Errorf("Tile request failed: %s", err.Error())
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
