package tilenol

import (
	"testing"
)

func TestCreateNilCache(t *testing.T) {
	cacheConfig := &CacheConfig{}
	cache, err := CreateCache(cacheConfig)
	if err != nil {
		t.Error("Could not create Cache")
	}
	if _, canCast := cache.(*NilCache); !canCast {
		t.Error("Did not create a NilCache")
	}
}

func TestCreateRedisCache(t *testing.T) {
	cacheConfig := &CacheConfig{Redis: &RedisConfig{}}
	cache, err := CreateCache(cacheConfig)
	if err != nil {
		t.Error("Could not create Cache")
	}
	if _, canCast := cache.(*RedisCache); !canCast {
		t.Error("Did not create a RedisCache")
	}
}
