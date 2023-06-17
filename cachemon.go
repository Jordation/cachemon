package cachemon

import (
	"fmt"
	"hash/maphash"
	"sync"
	"time"
)

// struct for one cache
type Cache struct {
	sync.RWMutex
	cMap map[uint64]*cachedItem
	hash maphash.Hash

	opts *cacheOpts
}

// struct of an entry in the cache
type cachedItem struct {
	data       []byte
	from       time.Time
	lastUpdate time.Time
}

// struct for cache options
type cacheOpts struct {
	// How often the cache checks for new items that match the following 2 fields
	checkFrequency time.Duration
	// How long a value for a request is stored before being deleted (regardless of use)
	itemTimeout time.Duration
	// How long a stored item will go before the data is re-fetched
	itemRefresh time.Duration
	// Maximum store of cache
	maxSize uint64
}

/*
*********************************************************************************
Default values for options struct, setting any of these to 0 is safe,
it effectively disables the checking criteria
i.e. 0 check frequency means it will never check to refresh or timeout
0 for refresh or timeout will yield the same result
0 max size means the cache will infinitely accept entries (probably a bad idea..)
*********************************************************************************
*/
const (
	DEFAULT_CHECK_FREQUENCY = time.Minute * 30
	DEFAULT_ITEM_REFRESH    = time.Hour * 4
	DEFAULT_ITEM_TIMEOUT    = time.Hour * 24
	DEFAULT_MAX_SIZE        = uint64(1_000_000)
)

func newCacheItem(data []byte) *cachedItem {
	return &cachedItem{
		from:       time.Now(),
		lastUpdate: time.Now(),
		data:       data,
	}
}

func NewCache(opts ...cfgFunc) *Cache {
	o := &cacheOpts{
		maxSize:        DEFAULT_MAX_SIZE,
		checkFrequency: DEFAULT_CHECK_FREQUENCY,
		itemTimeout:    DEFAULT_ITEM_TIMEOUT,
		itemRefresh:    DEFAULT_ITEM_REFRESH,
	}

	for _, fn := range opts {
		fn(o)
	}

	cache := &Cache{
		cMap: make(map[uint64]*cachedItem),
		opts: o,
	}

	if o.checkFrequency != 0 {
		go cache.watch()
	}

	return cache
}

func (c *Cache) Hit(request []byte) (data []byte, ok bool) {
	c.Lock()
	defer c.Unlock()

	c.hash.Reset()
	c.hash.Write(request)
	key := c.hash.Sum64()

	// hit
	if _, ok = c.cMap[key]; ok {
		return c.cMap[key].data, ok
	}

	// new entry
	c.cMap[key] = newCacheItem(request)
	return c.cMap[key].data, ok
}

func (c *Cache) Del(key uint64) (ok bool) {
	defer c.Unlock()
	c.Lock()

	_, ok = c.cMap[key]
	if ok {
		delete(c.cMap, key)
		return
	}

	return
}

/*
Funcs for checking and validating the contents of a cache
*/
func (c *Cache) watch() {
	ticker := time.NewTicker(c.opts.checkFrequency)
	for ; ; <-ticker.C {
		for k, v := range c.cMap {
			if v.needsRefresh(c.opts.itemRefresh) {
				fmt.Println("Wow Im refetching this data -> Wow!")
			}
			if v.needsTimeout(c.opts.itemTimeout) {
				c.Del(k)
				fmt.Println("Deleting item with \nkey ", k, "\ndata", string(v.data), " from cache")
			}
		}
	}
}

func (ci *cachedItem) needsRefresh(refresh time.Duration) bool {
	if refresh == 0 {
		return false
	}
	return time.Since(ci.lastUpdate) > refresh
}

func (ci *cachedItem) needsTimeout(timeout time.Duration) bool {
	if timeout == 0 {
		return false
	}
	return time.Since(ci.from) > timeout
}

/*
config funcs
*/

type cfgFunc func(*cacheOpts)

func WithCheckingFrequency(cf time.Duration) cfgFunc {
	return func(co *cacheOpts) {
		co.checkFrequency = cf
	}
}

func WithRefreshTime(rt time.Duration) cfgFunc {
	return func(co *cacheOpts) {
		co.itemRefresh = rt
	}
}

func WithTimeout(to time.Duration) cfgFunc {
	return func(co *cacheOpts) {
		co.itemTimeout = to
	}
}

func WithMaxSize(ms uint64) cfgFunc {
	return func(co *cacheOpts) {
		co.maxSize = ms
	}
}
