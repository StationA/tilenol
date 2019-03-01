package server

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/olivere/elastic"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/orb/simplify"
)

const (
	MinZoom     = 0
	MaxZoom     = 22
	MinSimplify = 1.0
	MaxSimplify = 10.0
)

type Server struct {
	Port         uint16
	InternalPort uint16
	EnableCORS   bool
	CacheControl string
	ES           *elastic.Client
	ESMappings   map[string]string
	ZoomRanges   map[string][]int
}

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
	r.Get("/{featureType}/{z}/{x}/{y}.mvt", s.GetVectorTile)

	// TODO: Add GeoJSON endpoint?

	i := chi.NewRouter()
	i.Get("/healthcheck", s.HealthCheck)
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

func (s *Server) HealthCheck(w http.ResponseWriter, r *http.Request) {
	// TODO: Maybe in the future check that ES is reachable?
	fmt.Fprintf(w, "OK")
}

func calculateSimplificationThreshold(minZoom, maxZoom, currentZoom int) float64 {
	s := MinSimplify - MaxSimplify
	z := float64(maxZoom - minZoom)
	p := s / z
	return p*float64(currentZoom-minZoom) + MaxSimplify
}

func (s *Server) GetVectorTile(w http.ResponseWriter, r *http.Request) {
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
	Logger.Debugf("ES query for layer [%s] @ (%d, %d, %d) took %s", featureType, x, y, z, esElapsed)
	if esErr != nil {
		Logger.Errorf("Failed to do ES query: %+v", esErr)
		s.HandleError(esErr, w, r)
		return
	}

	// TODO: Allow for multi-layer queries
	layer := mvt.NewLayer(featureType, fc)
	layer.Version = 2 // Set to tile spec v2
	layer.ProjectToTile(tile)
	layer.Clip(mvt.MapboxGLDefaultExtentBound)
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

	// Set standard response headers
	w.Header().Set("Cache-Control", s.CacheControl)
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Type", "application/x-protobuf")
	// Lastly, marshal the object into the response output
	data, marshalErr := mvt.MarshalGzipped(mvt.Layers{layer})
	if marshalErr != nil {
		// TODO: Handle error
	}
	_, _ = w.Write(data)
}

func (s *Server) HandleError(err error, w http.ResponseWriter, r *http.Request) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
