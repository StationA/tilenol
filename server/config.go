package server

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/olivere/elastic"
)

type ConfigOption func(s *Server) error

func Port(port uint16) ConfigOption {
	return func(s *Server) error {
		s.Port = port
		return nil
	}
}

func InternalPort(internalPort uint16) ConfigOption {
	return func(s *Server) error {
		s.InternalPort = internalPort
		return nil
	}
}

func EnableCORS(s *Server) error {
	s.EnableCORS = true
	return nil
}

func CacheControl(cacheControl string) ConfigOption {
	return func(s *Server) error {
		s.CacheControl = cacheControl
		return nil
	}
}

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

func ESMappings(esMappings map[string]string) ConfigOption {
	return func(s *Server) error {
		s.ESMappings = esMappings
		return nil
	}
}

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
