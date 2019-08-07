package server

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/go-redis/redis"
	"github.com/olivere/elastic"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/orb/simplify"
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
)

// Server is a tilenol server instance
type Server struct {
	Port         uint16
	InternalPort uint16
	EnableCORS   bool
	CacheControl string
	CacheClient  *redis.Client
	CacheTTL     time.Duration
	Simplify     bool
	ES           *elastic.Client
	ESMappings   map[string]string
	ZoomRanges   map[string][]int
}

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

// Start actually starts the server instance. Note that this blocks until an interrupting signal
func (s *Server) Start() {
	r := chi.NewRouter()

	//-- MIDDLEWARE
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
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
	r.Get("/{featureType}/{z}/{x}/{y}.mvt", s.cached(s.getVectorTile))

	// TODO: Add GeoJSON endpoint?

	i := chi.NewRouter()
	i.Get("/healthcheck", s.healthCheck)
	// TODO: Add healthcheck/status endpoint

	go func() {
		log.Fatalln(http.ListenAndServe(fmt.Sprintf(":%d", s.Port), r))
	}()

	go func() {
		log.Fatalln(http.ListenAndServe(fmt.Sprintf(":%d", s.InternalPort), i))
	}()

	Logger.Infof("Tilenol server up and running @ 0.0.0.0:[%d,%d]", s.Port, s.InternalPort)

	select {}
}

func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	// TODO: Maybe in the future check that ES is reachable?
	fmt.Fprintf(w, "OK")
}

func calculateSimplificationThreshold(minZoom, maxZoom, currentZoom int) float64 {
	s := MinSimplify - MaxSimplify
	z := float64(maxZoom - minZoom)
	p := s / z
	return p*float64(currentZoom-minZoom) + MaxSimplify
}

func (s *Server) cached(handler func(io.Writer, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				s.handleError(err.(error), w, r)
			}
		}()

		// Set standard response headers
		// TODO: Use the cache TTL to determine the Cache-Control
		w.Header().Set("Cache-Control", s.CacheControl)
		w.Header().Set("Content-Encoding", "gzip")
		// TODO: Store the content type somehow in the cache?
		w.Header().Set("Content-Type", "application/x-protobuf")

		key := r.URL.RequestURI()
		val, err := s.CacheClient.Get(key).Bytes()
		if err == redis.Nil {
			Logger.Debugf("Key [%s] is not cached", key)
			var buffer bytes.Buffer
			handler(&buffer, r)
			err := s.CacheClient.Set(key, buffer.Bytes(), s.CacheTTL).Err()
			if err != nil {
				// Log an error in case the key can't be stored in Redis, but continue
				Logger.Errorf("Could not store key [%s] in cache: %v", key, err)
			}
			_, err = io.Copy(w, &buffer)
			if err != nil {
				panic(err)
			}
		} else if err != nil {
			// Log an error in case the connection to Redis fails, but recompute the response
			Logger.Errorf("Could not talk to Redis: %v", err)
			handler(w, r)
		} else {
			Logger.Debugf("Key [%s] found in cache", key)
			buffer := bytes.NewBuffer(val)
			_, err := io.Copy(w, buffer)
			if err != nil {
				panic(err)
			}
		}
	}
}

func (s *Server) getVectorTile(w io.Writer, r *http.Request) {
	featureType := chi.URLParam(r, "featureType")
	z, _ := strconv.Atoi(chi.URLParam(r, "z"))
	x, _ := strconv.Atoi(chi.URLParam(r, "x"))
	y, _ := strconv.Atoi(chi.URLParam(r, "y"))

	Logger.Debugf("Retrieving vector tile for layer [%s] @ (%d, %d, %d)", featureType, x, y, z)

	geometryField, hasGeometryField := s.ESMappings[featureType]
	if !hasGeometryField {
		geometryField = "geometry"
	}
	Logger.Debugf("Using geometry field [%s]", geometryField)

	extraSources, hasExtraSources := r.URL.Query()["source"]
	if hasExtraSources {
		Logger.Debugf("Requesting additional source fields [%s]", extraSources)
	}

	// Convert x,y,z into lat-lon bounds for ES query construction
	tile := maptile.New(uint32(x), uint32(y), maptile.Zoom(z))
	tileBounds := tile.Bound()
	esStart := time.Now()
	fc, esErr := s.doQuery(r.Context(), featureType, geometryField, extraSources, tileBounds)
	esElapsed := time.Since(esStart)
	Logger.Debugf("ES query for layer [%s] @ (%d, %d, %d) with %d features took %s", featureType, x, y, z, len(fc.Features), esElapsed)
	if esErr != nil {
		Logger.Errorf("Failed to do ES query: %+v", esErr)
		panic(esErr)
	}

	// TODO: Allow for multi-layer queries
	layer := mvt.NewLayer(featureType, fc)
	layer.Version = 2 // Set to tile spec v2
	layer.ProjectToTile(tile)
	layer.Clip(mvt.MapboxGLDefaultExtentBound)

	if s.Simplify {
		minZoom := MinZoom
		maxZoom := MaxZoom
		zoomRange, hasZoomRange := s.ZoomRanges[featureType]
		if hasZoomRange {
			minZoom, maxZoom = zoomRange[0], zoomRange[1]
		}
		simplifyThreshold := calculateSimplificationThreshold(minZoom, maxZoom, z)
		Logger.Debugf("Simplifying @ zoom [%d], epsilon [%f]", z, simplifyThreshold)
		layer.Simplify(simplify.DouglasPeucker(simplifyThreshold))
		layer.RemoveEmpty(1.0, 1.0)
	}

	// Lastly, marshal the object into the response output
	data, marshalErr := mvt.MarshalGzipped(mvt.Layers{layer})
	if marshalErr != nil {
		// TODO: Handle error
	}
	_, _ = w.Write(data)
}

func (s *Server) handleError(err error, w http.ResponseWriter, r *http.Request) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
