package main

import (
	"github.com/spezifisch/stmps/logger"
)

// Cache fetches assets and holds a copy, returning them on request.
// A Cache is composed of four mechanisms:
//
// 1. a zero object
// 2. a function for fetching assets
// 3. a function for invalidating assets
// 4. a call-back function for when an asset is fetched
//
// When an asset is requested, Cache returns the asset if it is cached.
// Otherwise, it returns the zero object, and queues up a fetch for the object
// in the background. When the fetch is complete, the callback function is
// called, allowing the caller to get the real asset. An invalidation function
// allows Cache to manage the cache size by removing cached invalid objects.
//
// Caches are indexed by strings, because. They don't have to be, but
// stmps doesn't need them to be anything different.
type Cache[T any] struct {
	zero       T
	cache      map[string]T
	pipeline   chan string
	quit       func()
	cacheCheck func(string) string
}

// NewCache sets up a new cache, given
//
//   - a zeroValue, returned immediately on cache misses
//   - a fetcher, which can be a long-running function that loads assets.
//     fetcher should take a key ID and return an asset, or an error.
//   - a fetchedItem call-back function, which will be called when a requested asset is available. It
//     will be called with the asset ID, and the loaded asset.
//   - a cacheCheck function which, when given a key, returns a key to remove from the
//     cache, or the empty string if nothing is to be removed.
//   - a logger, used for reporting errors returned by the fetching function
//
// The invalidation should be reasonably efficient.
func NewCache[T any](
	zeroValue T,
	fetcher func(string) (T, error),
	fetchedItem func(string, T),
	cacheCheck func(string) string,
	logger *logger.Logger,
) Cache[T] {

	cache := make(map[string]T)
	getPipe := make(chan string, 1000)

	go func() {
		for i := range getPipe {
			asset, err := fetcher(i)
			if err != nil {
				logger.Printf("error fetching asset %s: %s", i, err)
				continue
			}
			cache[i] = asset
			if remove := cacheCheck(i); remove != "" {
				delete(cache, remove)
			}
			fetchedItem(i, asset)
		}
	}()

	return Cache[T]{
		zero:     zeroValue,
		cache:    cache,
		pipeline: getPipe,
		quit: func() {
			close(getPipe)
		},
		cacheCheck: cacheCheck,
	}
}

// Get returns a cached asset, or the zero asset on a cache miss.
// On a cache miss, the requested asset is queued for fetching.
func (c *Cache[T]) Get(key string) T {
	if v, ok := c.cache[key]; ok {
		// We're just touching something in the cache, not putting anything in it,
		// so we just call cacheCheck to refresh this key
		c.cacheCheck(key)
		return v
	}
	c.pipeline <- key
	return c.zero
}

// Close releases resources used by the cache, clearing the cache
// and shutting down goroutines. It should be called when the
// Cache is no longer used, and before program exit.
//
// Note: since the current iteration of Cache is a memory cache, it isn't
// strictly necessary to call this on program exit; however, as the caching
// mechanism may change and use other system resources, it's good practice to
// call this on exit.
func (c Cache[T]) Close() {
	for k := range c.cache {
		delete(c.cache, k)
	}
	c.quit()
}
