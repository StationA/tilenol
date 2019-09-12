package tilenol

// InMemoryCache implements the Cache interface backed by an in-memory map
type InMemoryCache struct {
	cache map[string][]byte
}

// NewInMemoryCache allocates a new InMemoryCache
func NewInMemoryCache() Cache {
	return &InMemoryCache{make(map[string][]byte)}
}

// Exists checks the internal map for the existence of the key
func (i *InMemoryCache) Exists(key string) bool {
	_, exists := i.cache[key]
	return exists
}

// Get retrieves the value stored in the internal map
func (i *InMemoryCache) Get(key string) ([]byte, error) {
	v, exists := i.cache[key]
	if !exists {
		return nil, ErrNoValue
	}
	return v, nil
}

// Put stores a new value in the internal map at a given key
func (i *InMemoryCache) Put(key string, val []byte) error {
	i.cache[key] = val
	return nil
}
