package tilenol

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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
	X    int
	Y    int
	Z    int
	Args map[string][]string
}

func (r *TileRequest) String() string {
	var s = fmt.Sprintf("%d/%d/%d", r.Z, r.X, r.Y)
	if len(r.Args) > 0 {
		q := make(url.Values)
		for k, vs := range r.Args {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		s = strings.Join([]string{s, q.Encode()}, "?")
	}
	return s
}

// Error type for HTTP Status code 400
type InvalidRequestError struct {
	s string
}

func (f InvalidRequestError) Error() string {
	return f.s
}

// Sanitize TileRequest arguments and return an error if sanity checking fails.
func MakeTileRequest(req *http.Request, x int, y int, z int) (*TileRequest, error) {
	if z < MinZoom || z > MaxZoom {
		return nil, InvalidRequestError{fmt.Sprintf("Invalid zoom level: [%d].", z)}
	}
	maxTileIdx := 1<<uint32(z) - 1
	if x < 0 || x > maxTileIdx {
		return nil, InvalidRequestError{fmt.Sprintf("Invalid X value [%d] for zoom level [%d].", x, z)}
	}
	if y < 0 || y > maxTileIdx {
		return nil, InvalidRequestError{fmt.Sprintf("Invalid Y value [%d] for zoom level [%d].", y, z)}
	}

	args := make(map[string][]string)
	for k, values := range req.URL.Query() {
		args[k] = values
	}

	return &TileRequest{x, y, z, args}, nil
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
			AllowedHeaders:   []string{"Accept", "Accept-Encoding", "Authorization", "Cache-Control"},
			AllowCredentials: true,
		})
		r.Use(cors.Handler)
	}

	//-- ROUTES
	r.Get("/{layers}/{z}/{x}/{y}.mvt", s.getVectorTile)

	// TODO: Add GeoJSON endpoint?
	// TODO: Add TileJSON endpoint (see StationA/tilenol#36)

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
	// TODO: Maybe in the future check that each layer source is reachable?
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

// getLayerDataFromSource retrieves layer data from the original backend source
func (s *Server) getLayerDataFromSource(ctx context.Context, layer Layer, req *TileRequest) (*mvt.Layer, error) {
	fc, err := layer.GetFeatures(ctx, req)
	if err != nil {
		return nil, err
	}

	fcLayer := mvt.NewLayer(layer.Name, fc)
	fcLayer.Version = 2 // Set to tile spec v2
	fcLayer.ProjectToTile(req.MapTile())
	fcLayer.Clip(mvt.MapboxGLDefaultExtentBound)
	return fcLayer, nil
}

// getLayerDataFromCache retrieves layer data from the configured cache
func (s *Server) getLayerDataFromCache(ctx context.Context, cacheKey string) (*mvt.Layer, error) {
	raw, err := s.Cache.Get(cacheKey)
	if err != nil {
		return nil, err
	}
	layers, err := mvt.UnmarshalGzipped(raw)
	if err != nil {
		return nil, err
	}

	// Note: since we store an mvt.Layers array with only a single layer, we need to pull out
	// the first layer to match the return interface
	return layers[0], nil
}

// getLayerData retrieves layer data either from cache or the original source
func (s *Server) getLayerData(ctx context.Context, layer Layer, req *TileRequest) (*mvt.Layer, error) {
	cacheKey := fmt.Sprintf("%s/%s", layer.String(), req.String())
	if layer.Cacheable && s.Cache.Exists(cacheKey) {
		Logger.Debugf("Key [%s] found in cache", cacheKey)
		if fcLayer, err := s.getLayerDataFromCache(ctx, cacheKey); err == nil {
			return fcLayer, nil
		} else {
			Logger.Warningf("Failed to retrieve layer data from cache [%s]: %s", cacheKey, err)
		}
	}

	Logger.Debugf("Key [%s] is not cached", cacheKey)

	fcLayer, err := s.getLayerDataFromSource(ctx, layer, req)
	if err != nil {
		return nil, err
	}

	if layer.Cacheable {
		// Note: paulmach/orb only implements marshalling code for an array of layer objects,
		// so we need to wrap the computed fcLayer into a single-item array
		raw, err := mvt.MarshalGzipped(mvt.Layers{fcLayer})
		if err != nil {
			return nil, err
		}

		if err := s.Cache.Put(cacheKey, raw); err != nil {
			Logger.Warningf("Failed to store layer data in cache [%s]: %s", cacheKey, err)
		}
	}

	return fcLayer, nil
}

// getVectorTile computes a vector tile response for the incoming request
func (s *Server) getVectorTile(w http.ResponseWriter, r *http.Request) {
	rctx := r.Context()

	// Setup deferred error handler for request cancellations
	defer func() {
		// TODO: Fix this behavior for certain cancellation scenarios (see StationA/tilenol#42)
		if rctx.Err() == context.Canceled {
			Logger.Debugf("Request canceled by client")
			w.WriteHeader(499)
			return
		}
	}()

	z, _ := strconv.Atoi(chi.URLParam(r, "z"))
	x, _ := strconv.Atoi(chi.URLParam(r, "x"))
	y, _ := strconv.Atoi(chi.URLParam(r, "y"))
	requestedLayers := chi.URLParam(r, "layers")

	req, err := MakeTileRequest(r, x, y, z)
	if err != nil {
		s.handleError(err, w, r)
		return
	}

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

		// Start a goroutine for each layer
		eg.Go(func() error {
			Logger.Debugf("Retrieving layer data for [%s] @ (%d, %d, %d)", layer, z, x, y)

			fcLayer, err := s.getLayerData(ctx, layer, req)
			if err != nil {
				return err
			}

			// TODO: Consider the tradeoffs of cacheing pre-simplified vs. post-simplified layers
			if s.Simplify {
				minZoom := layer.Minzoom
				maxZoom := layer.Maxzoom
				simplifyThreshold := calculateSimplificationThreshold(minZoom, maxZoom, req.Z)
				Logger.Debugf("Simplifying @ zoom [%d], epsilon [%f]", req.Z, simplifyThreshold)
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
		s.handleError(err, w, r)
		return
	}

	// Lastly, marshal the object into the response output
	data, marshalErr := mvt.MarshalGzipped(fcLayers)
	if marshalErr != nil {
		s.handleError(marshalErr, w, r)
		return
	}

	// Set standard response headers
	// TODO: Figure out a smarter cacheing mechanism (see StationA/tilenol#30)
	w.Header().Set("Cache-Control", "max-age=86400")
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Type", "application/x-protobuf")

	if _, err := w.Write(data); err != nil {
		s.handleError(err, w, r)
		return
	}
}

// handleError is a helper function to generate a generic tile server error response
func (s *Server) handleError(err error, w http.ResponseWriter, r *http.Request) {
	var errCode int
	switch err.(type) {
	case InvalidRequestError:
		errCode = http.StatusBadRequest
	default:
		errCode = http.StatusInternalServerError
	}
	Logger.Errorf("Tile request failed: %s (HTTP error %d)", err.Error(), errCode)
	http.Error(w, err.Error(), errCode)
}
