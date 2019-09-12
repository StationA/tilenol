package tilenol

// NilCache implements the Cache interface as a no-op
type NilCache struct{}

// Exists always returns false, since the NilCache stores no values
func (n *NilCache) Exists(key string) bool {
	return false
}

// Get should not be called for the NilCache, as it stores no values
func (n *NilCache) Get(key string) ([]byte, error) {
	return nil, ErrNoValue
}

// Put is a no-op for the NilCache
func (n *NilCache) Put(key string, val []byte) error {
	return nil
}
