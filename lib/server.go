package lib

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ddliu/go-httpclient"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/geojson"
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
	Port          uint16
	InternalPort  uint16
	EnableCORS    bool
	GzipResponses bool
	CacheControl  string
	ESHost        string
	ESMappings    map[string]string
	ZoomRanges    map[string]string
}

func (s *Server) Start() {
	httpclient.Defaults(httpclient.Map{
		"Accept-Encoding": "gzip,deflate",
	})

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

func flatten(something interface{}, accum map[string]interface{}, prefixParts ...string) {
	if something == nil {
		return
	}
	switch something.(type) {
	case []interface{}:
		for i, thing := range something.([]interface{}) {
			flatten(thing, accum, append(prefixParts, fmt.Sprintf("%d", i))...)
		}
	case map[string]interface{}:
		for key, value := range something.(map[string]interface{}) {
			flatten(value, accum, append(prefixParts, key)...)
		}
	default:
		newKey := strings.Join(prefixParts, ".")
		accum[newKey] = something
	}
}

func esResultsToFeatureCollection(esRes map[string]interface{}, geometryField string) *geojson.FeatureCollection {
	fc := geojson.NewFeatureCollection()
	if esRes["hits"] != nil {
		outerHits := esRes["hits"].(map[string]interface{})
		hits := outerHits["hits"].([]interface{})
		for _, el := range hits {
			hit := el.(map[string]interface{})
			id := hit["_id"]
			source := hit["_source"].(map[string]interface{})
			geometry := source[geometryField]
			gj, _ := json.Marshal(geometry)
			geom, _ := geojson.UnmarshalGeometry(gj)
			feat := geojson.NewFeature(geom.Geometry())
			feat.ID = id
			props, hasProps := source["properties"]
			if hasProps && props != nil {
				feat.Properties = props.(map[string]interface{})
			} else {
				feat.Properties = make(map[string]interface{})
				flatten(source, feat.Properties)
			}
			feat.Properties["id"] = id
			fc.Append(feat)
		}
	}
	return fc
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
	esRes, esErr := s.doQuery(featureType, geometryField, extraSources, tileBounds)
	esElapsed := time.Since(esStart)
	Logger.Debugf("ES query for layer [%s] @ (%d, %d, %d) took %s", featureType, x, y, z, esElapsed)
	if esErr != nil {
		Logger.Errorf("Failed to do ES query: %+v", esErr)
		s.HandleError(esErr, w, r)
		return
	}

	// Create GeoJSON FeatureCollection from ES results
	fc := esResultsToFeatureCollection(esRes, geometryField)
	// TODO: Allow for multi-layer queries
	layer := mvt.NewLayer(featureType, fc)
	layer.Version = 2 // Set to tile spec v2
	layer.ProjectToTile(tile)
	layer.Clip(mvt.MapboxGLDefaultExtentBound)
	minZoom := MinZoom
	maxZoom := MaxZoom
	zoomRange, hasZoomRange := s.ZoomRanges[featureType]
	if hasZoomRange {
		zoomRangeParts := strings.Split(zoomRange, "-")
		if len(zoomRangeParts) >= 1 {
			minZoom, _ = strconv.Atoi(zoomRangeParts[0])
		}
		if len(zoomRangeParts) == 2 {
			maxZoom, _ = strconv.Atoi(zoomRangeParts[1])
		}
	}
	simplifyThreshold := calculateSimplificationThreshold(minZoom, maxZoom, z)
	Logger.Debugf("Simplifying @ zoom [%d], epsilon [%f]", z, simplifyThreshold)
	layer.Simplify(simplify.DouglasPeucker(simplifyThreshold))
	layer.RemoveEmpty(1.0, 1.0)

	// Set standard response headers
	w.Header().Set("Cache-Control", s.CacheControl)
	if s.GzipResponses {
		w.Header().Set("Content-Encoding", "gzip")
	}

	// Lastly, marshal the object into the response output
	data, marshalErr := mvt.MarshalGzipped(mvt.Layers{layer})
	if marshalErr != nil {
		// TODO: Handle error
	}
	_, _ = w.Write(data)
}

func (s *Server) doQuery(index, geometryField string, extraSources []string, tileBounds orb.Bound) (map[string]interface{}, error) {
	sourceParam := append(extraSources, geometryField)
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": map[string]interface{}{
					"match_all": map[string]interface{}{},
				},
				"filter": map[string]interface{}{
					"geo_shape": map[string]interface{}{
						geometryField: map[string]interface{}{
							"shape": map[string]interface{}{
								"type": "envelope",
								"coordinates": [][]float64{
									[]float64{tileBounds.Left(), tileBounds.Top()},
									[]float64{tileBounds.Right(), tileBounds.Bottom()},
								},
							},
							"relation": "intersects",
						},
					},
				},
			},
		},
		"_source": sourceParam,
		"size":    10000,
	}
	jsonQuery, _ := json.Marshal(query)
	url := fmt.Sprintf("http://%s/%s/_search", s.ESHost, index)
	res, esErr := httpclient.
		Begin().
		WithHeader("Accept", "application/json").
		PostJson(url, jsonQuery)
	if esErr != nil {
		return nil, esErr
	}
	// TODO: Also handle ES JSON-based errors
	bodyBytes, _ := res.ReadAll()
	defer func() {
		_ = res.Body.Close()
	}()
	var esRes map[string]interface{}
	_ = json.Unmarshal(bodyBytes, &esRes)
	return esRes, nil
}

func (s *Server) HandleError(err error, w http.ResponseWriter, r *http.Request) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
