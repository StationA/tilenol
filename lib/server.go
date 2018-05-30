package lib

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

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

type Server struct {
	Port          uint16
	InternalPort  uint16
	EnableCORS    bool
	GzipResponses bool
	CacheControl  string
	ESHost        string
	ESMappings    map[string]string
	ESSource      map[string]string
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

	Logger.Infof("tilenol server up and running @ 0.0.0.0:[%d,%d]", s.Port, s.InternalPort)

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
			fc.Append(feat)
		}
	}
	return fc
}

func (s *Server) GetVectorTile(w http.ResponseWriter, r *http.Request) {
	featureType := chi.URLParam(r, "featureType")
	z, _ := strconv.Atoi(chi.URLParam(r, "z"))
	x, _ := strconv.Atoi(chi.URLParam(r, "x"))
	y, _ := strconv.Atoi(chi.URLParam(r, "y"))

	Logger.Infof("Retrieving vector tile for layer [%s] @ (%d, %d, %d)", featureType, x, y, z)

	geometryField, hasGeometryField := s.ESMappings[featureType]
	if !hasGeometryField {
		geometryField = "geometry"
	}
	sourceFields, hasSourceFields := s.ESSource[featureType]
	if !hasSourceFields {
		sourceFields = geometryField
	}
	tile := maptile.New(uint32(x), uint32(y), maptile.Zoom(z))
	tileBounds := tile.Bound()
	esRes, esErr := s.doQuery(featureType, geometryField, sourceFields, tileBounds)
	if esErr != nil {
		Logger.Errorf("Failed to do ES query: %+v", esErr)
		return
	}
	fc := esResultsToFeatureCollection(esRes, geometryField)
	layers := mvt.NewLayers(map[string]*geojson.FeatureCollection{
		// TODO: Allow for multi-layer queries
		featureType: fc,
	})
	layers.ProjectToTile(tile)
	layers.Clip(mvt.MapboxGLDefaultExtentBound)
	layers.Simplify(simplify.DouglasPeucker(1.0))
	layers.RemoveEmpty(1.0, 1.0)
	data, marshalErr := mvt.MarshalGzipped(layers)
	if marshalErr != nil {
		// TODO: Handle error
	}
	w.Header().Set("Cache-Control", s.CacheControl)
	if s.GzipResponses {
		w.Header().Set("Content-Encoding", "gzip")
	}
	_, _ = w.Write(data)
}

func (s *Server) doQuery(index, geometryField, sourceFields string, tileBounds orb.Bound) (map[string]interface{}, error) {
	sourceParam := append(strings.Split(sourceFields, ","), geometryField)
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
