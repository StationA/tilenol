package tilenol

type CacheConfig struct {
	Redis *RedisConfig `yaml:"redis"`
}

type Cache interface {
	Exists(key string) bool
	Get(key string) ([]byte, error)
	Put(key string, val []byte) error
}

func CreateCache(config *CacheConfig) (Cache, error) {
	if config.Redis != nil {
		cache, err := NewRedisCache(config.Redis)
		if err != nil {
			return nil, err
		}
		return cache, nil
	}
	// TODO: Is this problematic?
	return nil, nil
}
