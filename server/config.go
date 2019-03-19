package server

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/olivere/elastic"
)

// ConfigOption is a function that changes a configuration setting of the server.Server
type ConfigOption func(s *Server) error

// Port hanges the port number used for serving tile data
func Port(port uint16) ConfigOption {
	return func(s *Server) error {
		s.Port = port
		return nil
	}
}

// InternalPort changes the port number used for administrative endpoints (e.g. healthcheck)
func InternalPort(internalPort uint16) ConfigOption {
	return func(s *Server) error {
		s.InternalPort = internalPort
		return nil
	}
}

// EnableCORS configures the server for CORS (cross-origin resource sharing)
func EnableCORS(s *Server) error {
	s.EnableCORS = true
	return nil
}

// CacheControl sets a fixed string to be used for the Cache-Control HTTP header
func CacheControl(cacheControl string) ConfigOption {
	return func(s *Server) error {
		s.CacheControl = cacheControl
		return nil
	}
}

// ESHost sets the Elasticsearch backend host:port
func ESHost(esHost string) ConfigOption {
	return func(s *Server) error {
		client, err := elastic.NewClient(
			elastic.SetURL(fmt.Sprintf("http://%s", esHost)),
			elastic.SetGzip(true),
			// TODO: Should this be configurable?
			elastic.SetHealthcheckTimeoutStartup(30*time.Second),
		)
		s.ES = client
		return err
	}
}

// ESMappings sets a custom mapping from index name to geometry field name
func ESMappings(esMappings map[string]string) ConfigOption {
	return func(s *Server) error {
		s.ESMappings = esMappings
		return nil
	}
}

// ZoomRanges sets min and max zoom limits for a specific index
func ZoomRanges(strZoomRanges map[string]string) ConfigOption {
	return func(s *Server) error {
		zoomRanges := make(map[string][]int)
		for featureType, rangeStr := range strZoomRanges {
			minZoom := MinZoom
			maxZoom := MaxZoom
			zoomRangeParts := strings.Split(rangeStr, "-")
			if len(zoomRangeParts) >= 1 {
				minZoom, _ = strconv.Atoi(zoomRangeParts[0])
			}
			if len(zoomRangeParts) == 2 {
				maxZoom, _ = strconv.Atoi(zoomRangeParts[1])
			}
			zoomRanges[featureType] = []int{minZoom, maxZoom}
		}
		s.ZoomRanges = zoomRanges
		return nil
	}
}
