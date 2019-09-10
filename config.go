package tilenol

import (
	"os"

	"gopkg.in/go-yaml/yaml.v2"
)

// Config is a YAML server configuration object
type Config struct {
	// Cache configures the tile server cache
	Cache *CacheConfig `yaml:"cache"`
	// Layers configures the tile server layers
	Layers []LayerConfig `yaml:"layers"`
}

// LoadConfig loads the configuration from disk, and decodes it into a Config object
func LoadConfig(configFile *os.File) (*Config, error) {
	dec := yaml.NewDecoder(configFile)
	dec.SetStrict(true)
	var config Config
	err := dec.Decode(&config)
	if err != nil {
		return nil, err
	}
	Logger.Debugf("Loaded config: %+v", config)
	return &config, nil
}

// ConfigOption is a function that changes a configuration setting of the server.Server
type ConfigOption func(s *Server) error

// ConfigFile loads a YAML configuration file from disk to set up the server
func ConfigFile(configFile *os.File) ConfigOption {
	return func(s *Server) error {
		config, err := LoadConfig(configFile)
		if err != nil {
			return err
		}
		if config.Cache != nil {
			cache, err := CreateCache(config.Cache)
			if err != nil {
				return err
			}
			s.Cache = cache
		}
		var layers []Layer
		for _, layerConfig := range config.Layers {
			layer, err := CreateLayer(layerConfig)
			if err != nil {
				return err
			}
			layers = append(layers, *layer)
		}
		s.Layers = layers
		return nil
	}
}

// Port changes the port number used for serving tile data
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

// Simplify shapes enable geometry simplification based on the requested zoom level
func SimplifyShapes(s *Server) error {
	s.Simplify = true
	return nil
}
