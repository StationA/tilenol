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

// Server is a tilenol server instance
type Server struct {
	Port         uint16
	InternalPort uint16
	EnableCORS   bool
	Simplify     bool
	Layers       []Layer
	Cache        Cache
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
	r.Get("/{layers}/{z}/{x}/{y}.mvt", s.cached(s.getVectorTile))

	// TODO: Add GeoJSON endpoint?

	i := chi.NewRouter()
	i.Get("/healthcheck", s.healthCheck)

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
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Header().Set("Content-Encoding", "gzip")
		// TODO: Store the content type somehow in the cache?
		w.Header().Set("Content-Type", "application/x-protobuf")

		if s.Cache != nil {
			key := r.URL.RequestURI()
			if s.Cache.Exists(key) {
				Logger.Debugf("Key [%s] found in cache", key)
				val, err := s.Cache.Get(key)
				if err != nil {
					panic(err)
				}
				buffer := bytes.NewBuffer(val)
				io.Copy(w, buffer)
			} else {
				Logger.Debugf("Key [%s] is not cached", key)
				var buffer bytes.Buffer
				handler(&buffer, r)
				err := s.Cache.Put(key, buffer.Bytes())
				if err != nil {
					// Log an error in case the key can't be stored in cache, but continue
					Logger.Errorf("Could not store key [%s] in cache: %v", key, err)
				}
				_, err = io.Copy(w, &buffer)
			}
		} else {
			handler(w, r)
		}
	}
}

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

func filterLayersByZoom(inLayers []Layer, z int) []Layer {
	var outLayers []Layer
	for _, layer := range inLayers {
		if layer.Minzoom <= z && (layer.Maxzoom >= z || layer.Maxzoom == 0) {
			outLayers = append(outLayers, layer)
		}
	}
	return outLayers
}

func (s *Server) getVectorTile(w io.Writer, r *http.Request) {
	z, _ := strconv.Atoi(chi.URLParam(r, "z"))
	x, _ := strconv.Atoi(chi.URLParam(r, "x"))
	y, _ := strconv.Atoi(chi.URLParam(r, "y"))
	requestedLayers := chi.URLParam(r, "layers")
	tile := maptile.New(uint32(x), uint32(y), maptile.Zoom(z))

	var layersToCompute = filterLayersByZoom(s.Layers, z)
	if requestedLayers != AllLayers {
		layersToCompute = filterLayersByNames(layersToCompute, strings.Split(requestedLayers, ","))
	}

	eg, ectx := errgroup.WithContext(r.Context())
	ctx := context.WithValue(ectx, "tile", tile)

	fcLayers := make(mvt.Layers, len(layersToCompute))
	for i, layer := range layersToCompute {
		i, layer := i, layer // Fun stuff: https://blog.cloudflare.com/a-go-gotcha-when-closures-and-goroutines-collide/
		eg.Go(func() error {
			Logger.Debugf("Retrieving vector tile for layer [%s] @ (%d, %d, %d)", layer.Name, x, y, z)
			fc, err := layer.Source.GetFeatures(ctx)
			if err != nil {
				return err
			}
			fcLayer := mvt.NewLayer(layer.Name, fc)
			fcLayer.Version = 2 // Set to tile spec v2
			fcLayer.ProjectToTile(tile)
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
	if err := eg.Wait(); err != nil {
		panic(err)
	}

	// Lastly, marshal the object into the response output
	data, marshalErr := mvt.MarshalGzipped(fcLayers)
	if marshalErr != nil {
		// TODO: Handle error
	}
	_, _ = w.Write(data)
}

func (s *Server) handleError(err error, w http.ResponseWriter, r *http.Request) {
	Logger.Errorf("Tile request failed: %v", err)

	http.Error(w, err.Error(), http.StatusInternalServerError)
}
